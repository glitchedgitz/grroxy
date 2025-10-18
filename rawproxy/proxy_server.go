package rawproxy

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// Proxy represents a proxy server instance
type Proxy struct {
	config *Config
	server *http.Server
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
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 60 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 60 * time.Second
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
	}, nil
}

// Start starts the proxy server (blocking)
func (p *Proxy) Start() error {
	if p.server != nil {
		return fmt.Errorf("proxy server is already running")
	}

	p.server = &http.Server{
		Addr:         p.config.ListenAddr,
		ReadTimeout:  p.config.ReadTimeout,
		WriteTimeout: p.config.WriteTimeout,
		IdleTimeout:  p.config.IdleTimeout,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ProxyHandler(w, r, p.config)
		}),
	}

	return p.server.ListenAndServe()
}

// Stop gracefully shuts down the proxy server
func (p *Proxy) Stop(ctx context.Context) error {
	if p.server == nil {
		return fmt.Errorf("proxy server is not running")
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	err := p.server.Shutdown(ctx)
	p.server = nil
	return err
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
