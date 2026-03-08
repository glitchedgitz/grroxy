package rawproxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Proxy represents a proxy server instance
type Proxy struct {
	config   *Config
	server   *http.Server
	listener net.Listener // Keep reference to the listener so we can close it

	// Track connections to force-close them on shutdown
	connsMu sync.Mutex
	conns   map[net.Conn]http.ConnState // Track all active connections
}

// New creates a new proxy instance with the given configuration
func New(config *Config) (*Proxy, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// If ConfigFolder is set, use it for certificate paths only
	if config.ConfigFolder != "" {
		if config.CertPath == "" {
			config.CertPath = filepath.Join(config.ConfigFolder, "ca.crt")
		}
		if config.KeyPath == "" {
			config.KeyPath = filepath.Join(config.ConfigFolder, "ca.key")
		}
	}

	// Set defaults
	if config.ListenAddr == "" {
		config.ListenAddr = ":8080"
	}
	if config.OutputDir == "" {
		config.OutputDir = "captures"
	}
	if config.WebSocketDir == "" {
		config.WebSocketDir = filepath.Join(config.OutputDir, "websockets")
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 10 * time.Minute
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Minute
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 10 * time.Minute
	}
	if config.ReqCounter == nil {
		config.ReqCounter = &atomic.Uint64{}
	}

	// Create output directories
	if err := os.MkdirAll(config.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.MkdirAll(config.WebSocketDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create WebSocket directory: %w", err)
	}

	// Handle MITM CA certificate (optional - if nil, HTTPS will be tunneled without inspection)
	if config.MITM == nil {
		// Set default certificate paths if not specified
		if config.CertPath == "" {
			config.CertPath = filepath.Join("cert", "ca.crt")
		}
		if config.KeyPath == "" {
			config.KeyPath = filepath.Join("cert", "ca.key")
		}

		// Try to load existing CA or generate new one
		if FileExists(config.CertPath) && FileExists(config.KeyPath) {
			mitm, err := LoadMITMCA(config.CertPath, config.KeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load MITM CA: %w", err)
			}
			config.MITM = mitm
		} else {
			// Generate new CA
			caDir := filepath.Dir(config.CertPath)
			if err := os.MkdirAll(caDir, 0o755); err != nil {
				return nil, fmt.Errorf("failed to create CA directory: %w", err)
			}
			mitm, certPath, keyPath, err := GenerateMITMCA(caDir)
			if err != nil {
				return nil, fmt.Errorf("failed to generate MITM CA: %w", err)
			}
			config.MITM = mitm
			config.CertPath = certPath
			config.KeyPath = keyPath
		}
	}

	return &Proxy{
		config: config,
		conns:  make(map[net.Conn]http.ConnState),
	}, nil
}

// Start starts the proxy server (blocking)
func (p *Proxy) Start() error {
	if p.server != nil {
		return fmt.Errorf("proxy server is already running")
	}

	log.Printf("[RawProxy.Start] Starting proxy server on %s", p.config.ListenAddr)

	p.server = &http.Server{
		Addr:         p.config.ListenAddr,
		ReadTimeout:  p.config.ReadTimeout,
		WriteTimeout: p.config.WriteTimeout,
		IdleTimeout:  p.config.IdleTimeout,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ProxyHandler(w, r, p.config)
		}),
		ConnState: p.trackConnection,
	}

	log.Printf("[RawProxy.Start] Server configured, creating listener...")
	listener, err := net.Listen("tcp", p.config.ListenAddr)
	if err != nil {
		log.Printf("[RawProxy.Start] Failed to create listener: %v", err)
		return err
	}

	p.listener = listener
	log.Printf("[RawProxy.Start] Listener created, calling Serve()...")
	err = p.server.Serve(p.listener)

	if err != nil && err != http.ErrServerClosed {
		log.Printf("[RawProxy.Start] Serve error: %v", err)
	} else if err == http.ErrServerClosed {
		log.Printf("[RawProxy.Start] Server closed gracefully")
	}

	return err
}

// trackConnection tracks all connections so we can force-close them on shutdown
func (p *Proxy) trackConnection(c net.Conn, state http.ConnState) {
	p.connsMu.Lock()
	defer p.connsMu.Unlock()

	switch state {
	case http.StateNew, http.StateActive, http.StateIdle, http.StateHijacked:
		p.conns[c] = state
		log.Printf("[RawProxy.ConnState] Connection %s entered state: %s (total active: %d)", c.RemoteAddr(), state, len(p.conns))
	case http.StateClosed:
		delete(p.conns, c)
		log.Printf("[RawProxy.ConnState] Connection %s closed (total active: %d)", c.RemoteAddr(), len(p.conns))
	}
}

// forceCloseConnections closes all remaining connections forcefully
func (p *Proxy) forceCloseConnections() {
	p.connsMu.Lock()
	defer p.connsMu.Unlock()

	count := len(p.conns)
	if count == 0 {
		return
	}

	log.Printf("[RawProxy.Stop] Force closing %d remaining connections...", count)
	for c, st := range p.conns {
		// Set a short deadline to nudge writers to exit quickly
		_ = c.SetDeadline(time.Now().Add(200 * time.Millisecond))
		_ = c.Close()
		delete(p.conns, c)
		log.Printf("[RawProxy.Stop] Forced close of %s (state=%s)", c.RemoteAddr(), st)
	}
	log.Printf("[RawProxy.Stop] All %d connections forcefully closed", count)
}

// Stop gracefully shuts down the proxy server
func (p *Proxy) Stop(ctx context.Context) error {
	log.Printf("[RawProxy.Stop] Attempting to stop proxy server on %s", p.config.ListenAddr)

	if p.server == nil {
		log.Printf("[RawProxy.Stop] ERROR: server is nil, proxy was never started")
		return fmt.Errorf("proxy server is not running")
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// First, disable keep-alives to prevent new requests on existing connections
	log.Printf("[RawProxy.Stop] Disabling keep-alives...")
	p.server.SetKeepAlivesEnabled(false)

	// Try graceful shutdown first
	log.Printf("[RawProxy.Stop] Attempting graceful shutdown (waiting %v)...", 5*time.Second)
	shutdownErr := p.server.Shutdown(ctx)

	if shutdownErr != nil {
		log.Printf("[RawProxy.Stop] Graceful shutdown error: %v", shutdownErr)
	} else {
		log.Printf("[RawProxy.Stop] Graceful shutdown completed")
	}

	// Close the listener to free the port
	if p.listener != nil {
		log.Printf("[RawProxy.Stop] Closing listener to free port %s...", p.config.ListenAddr)
		if err := p.listener.Close(); err != nil && err.Error() != "use of closed network connection" {
			log.Printf("[RawProxy.Stop] Error closing listener: %v", err)
		} else {
			log.Printf("[RawProxy.Stop] Listener closed, port freed")
		}
		p.listener = nil
	}

	// Force-close any remaining connections (hijacked/keep-alive that didn't drain)
	p.forceCloseConnections()

	p.server = nil

	log.Printf("[RawProxy.Stop] Proxy server stopped successfully")
	return nil
}

// GetConfig returns the proxy configuration
func (p *Proxy) GetConfig() *Config {
	return p.config
}

// SetRequestHandler sets the request handler function
func (p *Proxy) SetRequestHandler(handler OnRequestHandler) {
	p.config.OnRequestHandler = handler
}

// SetResponseHandler sets the response handler function
func (p *Proxy) SetResponseHandler(handler OnResponseHandler) {
	p.config.OnResponseHandler = handler
}

// SetWebSocketMessageHandler sets the websocket message handler function
func (p *Proxy) SetWebSocketMessageHandler(handler OnWebSocketMessageHandler) {
	p.config.OnWebSocketMessageHandler = handler
}
