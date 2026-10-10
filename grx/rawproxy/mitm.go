package rawproxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// Note: This package uses uTLS (utls_transport.go) to mimic browser TLS fingerprints
// for upstream connections, bypassing Cloudflare and other CDN bot detection.

type MitmCA struct {
	caCert *x509.Certificate
	caKey  any
	mu     sync.Mutex
	cache  map[string]*tls.Certificate
	// Reused leaf private key to keep SPKI stable across generated leaf certs
	leafKey *rsa.PrivateKey
}

func LoadMITMCA(certPath, keyPath string) (*MitmCA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("invalid CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	priv, err := parsePrivateKeyPEM(keyPEM)
	if err != nil {
		return nil, err
	}

	// Generate a reusable leaf RSA key to keep SPKI stable for pinning
	lk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// Persist a base64 SHA-256 SPKI fingerprint for the reusable leaf key for launchers to consume
	if spkiDER, err := x509.MarshalPKIXPublicKey(&lk.PublicKey); err == nil {
		spkiHash := sha256.Sum256(spkiDER)
		b64 := base64.StdEncoding.EncodeToString(spkiHash[:])
		dir := filepath.Dir(certPath)
		_ = os.WriteFile(filepath.Join(dir, "leaf.spki"), []byte(b64), 0o644)
	}

	return &MitmCA{caCert: caCert, caKey: priv, cache: make(map[string]*tls.Certificate), leafKey: lk}, nil
}

func (m *MitmCA) CertForHost(host string) (*tls.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.cache[host]; ok {
		return c, nil
	}

	serial := big.NewInt(0).SetUint64(uint64(time.Now().UnixNano()))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		// Present a Subject matching the site, like typical MITM proxies do
		Subject: pkix.Name{
			CommonName: host,
			// Organization: m.caCert.Subject.Organization,
		},
		NotBefore:   time.Now().Add(-5 * time.Minute),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}

	// Set SKI on leaf from its public key, and AKI from CA SKI when available
	if pubKeyDER, err := x509.MarshalPKIXPublicKey(&m.leafKey.PublicKey); err == nil {
		ski := sha1.Sum(pubKeyDER)
		tmpl.SubjectKeyId = ski[:]
	}
	if len(m.caCert.SubjectKeyId) > 0 {
		tmpl.AuthorityKeyId = m.caCert.SubjectKeyId
	}

	// Reuse the single process-wide key so SPKI stays constant
	der, err := x509.CreateCertificate(rand.Reader, tmpl, m.caCert, &m.leafKey.PublicKey, m.caKey)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(m.leafKey)})
	leaf, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	m.cache[host] = &leaf
	return &leaf, nil
}

func parsePrivateKeyPEM(keyPEM []byte) (any, error) {
	for {
		blk, rest := pem.Decode(keyPEM)
		if blk == nil {
			break
		}
		switch blk.Type {
		case "RSA PRIVATE KEY":
			return x509.ParsePKCS1PrivateKey(blk.Bytes)
		case "EC PRIVATE KEY":
			return x509.ParseECPrivateKey(blk.Bytes)
		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
			if err != nil {
				return nil, err
			}
			return key, nil
		}
		keyPEM = rest
	}
	return nil, fmt.Errorf("unsupported private key PEM")
}

// MitmHTTPS terminates TLS with client, sends requests upstream over TLS, captures both sides
// Supports both HTTP/1.1 and HTTP/2 automatically via ALPN negotiation.
// clientBuf is the hijacked connection's buffered reader; it may hold bytes the
// proxy's HTTP server read ahead past the CONNECT request, and may be nil.
func MitmHTTPS(clientConn net.Conn, clientBuf *bufio.ReadWriter, connectReq *http.Request, requestID string, config *Config) {
	// NOTE: Don't defer clientConn.Close() here - the HTTP server will manage the connection
	host := connectReq.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}

	leaf, err := config.MITM.CertForHost(host)
	if err != nil {
		log.Printf("[ERROR] requestID=%s leaf cert error for %s: %v", requestID, host, err)
		return
	}

	// Create TLS config that supports both HTTP/2 and HTTP/1.1
	tlsConfig := &tls.Config{
		Certificates:             []tls.Certificate{*leaf},
		NextProtos:               []string{"h2", "http/1.1"}, // Advertise both protocols
		MinVersion:               tls.VersionTLS12,
		MaxVersion:               tls.VersionTLS13,
		PreferServerCipherSuites: true,
	}

	log.Printf("[MITM] requestID=%s Starting TLS server for %s (advertising h2, http/1.1)", requestID, host)

	// Create upstream round tripper with uTLS to mimic browser TLS fingerprint
	// This bypasses Cloudflare and other CDN bot detection that use JA3/JA4 fingerprinting
	roundTripper := GetUTLSRoundTripper(host, FingerprintChrome)

	// Create a custom handler for this MITM connection
	handler := &mitmHandler{
		host:         host,
		connectHost:  connectReq.Host,
		roundTripper: roundTripper,
		baseReqID:    requestID,
		config:       config,
	}

	// Create HTTP server that supports both HTTP/1.1 and HTTP/2
	server := &http.Server{
		Handler:      handler,
		TLSConfig:    tlsConfig,
		ReadTimeout:  10 * time.Minute,
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  10 * time.Minute,
	}

	// Configure HTTP/2 support
	if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
		log.Printf("[ERROR] requestID=%s Failed to configure HTTP/2: %v", requestID, err)
	}

	// Serve the connection with TLS - server will handle the TLS handshake.
	// Replay any bytes the proxy's HTTP server buffered past the CONNECT request
	// first; a client that pipelines its TLS ClientHello into the same segment
	// would otherwise have it swallowed and the handshake would stall.
	server.ServeTLS(&singleConnListener{conn: withBufferedPrefix(clientConn, clientBuf)}, "", "")
}

// withBufferedPrefix returns a conn that yields buf's already-read bytes before
// reading from the connection itself.
func withBufferedPrefix(conn net.Conn, buf *bufio.ReadWriter) net.Conn {
	if buf == nil {
		return conn
	}
	n := buf.Reader.Buffered()
	if n == 0 {
		return conn
	}
	return &prefixConn{
		Conn: conn,
		r:    io.MultiReader(io.LimitReader(buf.Reader, int64(n)), conn),
	}
}

// prefixConn is a net.Conn whose reads are served from r first.
type prefixConn struct {
	net.Conn
	r io.Reader
}

func (c *prefixConn) Read(b []byte) (int, error) { return c.r.Read(b) }

// mitmHandler handles requests in MITM mode
type mitmHandler struct {
	host         string
	connectHost  string
	roundTripper http.RoundTripper
	baseReqID    string
	reqCount     uint64
	config       *Config
}

func (h *mitmHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Generate sub-request ID
	h.reqCount++
	subRequestID := fmt.Sprintf("%s-sub-%d", h.baseReqID, h.reqCount)

	// Create RequestData to pass custom data between request and response handlers
	reqData := &RequestData{
		RequestID: subRequestID,
		Data:      nil, // Will be populated by OnRequestHandler
	}

	// Check if this is a WebSocket upgrade request
	upgradeHeader := strings.ToLower(req.Header.Get("Upgrade"))
	connectionHeader := strings.ToLower(req.Header.Get("Connection"))

	if upgradeHeader == "websocket" && strings.Contains(connectionHeader, "upgrade") {
		log.Printf("[WEBSOCKET-MITM] requestID=%s WebSocket upgrade request to %s", subRequestID, req.URL.String())
		h.handleWebSocketUpgrade(w, req, reqData)
		return
	}

	// Read request body
	body, _ := io.ReadAll(req.Body)
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))

	// Fix request protocol to match what the upstream server actually speaks.
	// The client-to-proxy MITM connection may use HTTP/2 (we advertise h2),
	// but the upstream server may only support HTTP/1.1.
	// Check the global protocol cache and override req.Proto accordingly.
	if cachedProto := GetCachedProto(h.host); cachedProto == "http/1.1" {
		req.Proto = "HTTP/1.1"
		req.ProtoMajor = 1
		req.ProtoMinor = 1
	}

	// Apply onRequest handler if configured
	var processedRequest = req
	if h.config.OnRequestHandler != nil {
		var err error
		processedRequest, err = h.config.OnRequestHandler(reqData, req)
		if err != nil {
			log.Printf("[ERROR] requestID=%s MITM onRequest handler failed for %s: %v", subRequestID, req.URL.String(), err)
			http.Error(w, fmt.Sprintf("Request processing error: %v", err), http.StatusBadRequest)
			return
		}
		if processedRequest == nil {
			log.Printf("[ERROR] requestID=%s MITM onRequest handler returned nil request for %s", subRequestID, req.URL.String())
			http.Error(w, "Request processing returned nil", http.StatusBadRequest)
			return
		}
		// Re-read body if request was modified
		if processedRequest.Body != nil {
			body, _ = io.ReadAll(processedRequest.Body)
			processedRequest.Body.Close()
			processedRequest.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	// Capture request
	reqDump, _ := httputil.DumpRequest(processedRequest, true)

	// Prepare upstream request
	upstreamReq := processedRequest.Clone(context.Background())
	upstreamReq.URL.Scheme = "https"
	upstreamReq.URL.Host = h.connectHost
	upstreamReq.Host = h.host
	upstreamReq.RequestURI = ""

	// The body we send is exactly the bytes we hold, so describe it that way.
	// A request rebuilt from raw text (intercept edits) can carry a stale
	// Content-Length; announcing a length that disagrees with the body desyncs
	// HTTP/1.1 keep-alive connections and earns a PROTOCOL_ERROR over HTTP/2.
	//
	// GetBody is what makes the request retryable, and it is not optional here.
	// Incoming server requests always have GetBody == nil, and Clone copies that
	// nil through. Without it, http2's shouldRetryRequest cannot rewind us and
	// turns a routine graceful-shutdown GOAWAY - which it otherwise classifies
	// as retryable - into a hard failure. Upstreams recycle h2 connections on a
	// schedule and http2.Transport has no idle-conn timeout of its own, so this
	// is a matter of when, not if.
	if len(body) == 0 {
		// A bodyless request must present as bodyless. http2 reuses a request
		// directly only when Body is nil or http.NoBody, so an empty-but-non-nil
		// body would take the slower path and cost an empty DATA frame.
		upstreamReq.Body = http.NoBody
		upstreamReq.GetBody = func() (io.ReadCloser, error) { return http.NoBody, nil }
	} else {
		upstreamReq.Body = io.NopCloser(bytes.NewReader(body))
		upstreamReq.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}
	upstreamReq.ContentLength = int64(len(body))
	upstreamReq.TransferEncoding = nil

	// Connection-scoped headers belong to the client's connection, not the
	// upstream one. http2.Transport refuses any request still carrying
	// Connection, Upgrade or Transfer-Encoding.
	stripHopByHopRequest(upstreamReq.Header)

	// Forward to upstream
	resp, err := h.roundTripper.RoundTrip(upstreamReq)
	if err != nil {
		log.Printf("[ERROR] requestID=%s MITM upstream request failed for %s: %v", subRequestID, req.URL.String(), err)
		errorMsg := fmt.Sprintf("Upstream error: %v", err)
		http.Error(w, errorMsg, http.StatusBadGateway)

		// Capture error
		errorResp := []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg))
		asyncCapture(reqDump, errorResp, req, subRequestID, h.config)
		return
	}
	defer resp.Body.Close()

	// Set the actual upstream protocol on reqData so handlers can access it.
	// Also update req.Proto to match what the upstream actually speaks,
	// since the client-to-proxy connection may use a different protocol (e.g. HTTP/2).
	reqData.HttpProto = resp.Proto
	if resp.Proto != "" && resp.Proto != req.Proto {
		req.Proto = resp.Proto
		req.ProtoMajor = resp.ProtoMajor
		req.ProtoMinor = resp.ProtoMinor
	}

	// Read response body
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Apply onResponse handler if configured
	var processedResponse = resp
	if h.config.OnResponseHandler != nil {
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		var err error
		processedResponse, err = h.config.OnResponseHandler(reqData, resp, req)
		if err != nil {
			log.Printf("[ERROR] requestID=%s MITM onResponse handler failed for %s: %v", subRequestID, req.URL.String(), err)
			http.Error(w, fmt.Sprintf("Response processing error: %v", err), http.StatusInternalServerError)
			return
		}
		if processedResponse == nil {
			log.Printf("[ERROR] requestID=%s MITM onResponse handler returned nil response for %s", subRequestID, req.URL.String())
			http.Error(w, "Response processing returned nil", http.StatusInternalServerError)
			return
		}
		// Re-read body if response was modified
		if processedResponse.Body != nil {
			respBody, _ = io.ReadAll(processedResponse.Body)
			processedResponse.Body.Close()
		}
	}

	// Capture response
	respForDump := cloneResponseMeta(processedResponse, io.NopCloser(bytes.NewReader(respBody)))
	respDump, _ := httputil.DumpResponse(respForDump, true)
	asyncCapture(reqDump, respDump, req, subRequestID, h.config)

	// Send response to client. Strip only now, after the capture above, so the
	// recorded traffic still shows exactly what the upstream sent.
	stripHopByHopResponse(processedResponse.Header)
	copyHeader(w.Header(), processedResponse.Header)

	// respBody is the body we are about to write, which an onResponse handler
	// may have resized. Re-state its length rather than forwarding a stale one.
	if bodyAllowedForStatus(processedResponse.StatusCode) && req.Method != http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
	} else {
		w.Header().Del("Content-Length")
	}

	w.WriteHeader(processedResponse.StatusCode)
	w.Write(respBody)
}

// bodyAllowedForStatus mirrors net/http's rule for which status codes may carry
// a response body.
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent, status == http.StatusNotModified:
		return false
	}
	return true
}

func (h *mitmHandler) handleWebSocketUpgrade(w http.ResponseWriter, req *http.Request, reqData *RequestData) {
	// Apply onRequest handler if configured
	var processedRequest = req
	if h.config.OnRequestHandler != nil {
		var err error
		processedRequest, err = h.config.OnRequestHandler(reqData, req)
		if err != nil {
			log.Printf("[ERROR] requestID=%s MITM WebSocket onRequest handler failed for %s: %v", reqData.RequestID, req.URL.String(), err)
			http.Error(w, fmt.Sprintf("WebSocket request processing error: %v", err), http.StatusBadRequest)
			return
		}
		if processedRequest == nil {
			log.Printf("[ERROR] requestID=%s MITM WebSocket onRequest handler returned nil request for %s", reqData.RequestID, req.URL.String())
			http.Error(w, "WebSocket request processing returned nil", http.StatusBadRequest)
			return
		}
	}

	// Capture the WebSocket upgrade request
	reqDump, err := httputil.DumpRequest(processedRequest, false)
	if err != nil {
		log.Printf("[ERROR] requestID=%s Failed to dump MITM WebSocket request: %v", reqData.RequestID, err)
		reqDump = []byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\n\r\n", processedRequest.URL.Path, processedRequest.Host))
	}

	// Hijack the connection to handle WebSocket upgrade
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket upgrade not supported", http.StatusInternalServerError)
		asyncWebSocketCapture(reqDump, []byte("HTTP/1.1 500 Internal Server Error\r\n\r\nWebSocket upgrade not supported\n"), req, reqData.RequestID, h.config)
		log.Printf("[ERROR] requestID=%s MITM WebSocket hijacking not supported for %s", reqData.RequestID, req.URL.String())
		return
	}

	clientConn, clientBuf, err := hj.Hijack()
	if err != nil {
		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 500 Internal Server Error\r\n\r\n%v\n", err)), req, reqData.RequestID, h.config)
		log.Printf("[ERROR] requestID=%s MITM WebSocket hijack failed for %s: %v", reqData.RequestID, req.URL.String(), err)
		return
	}
	defer clientConn.Close()

	// Prepare upstream URL
	targetURL := *processedRequest.URL
	targetURL.Scheme = "wss"
	targetURL.Host = h.connectHost

	// Determine target address
	target := targetURL.Host
	if !strings.Contains(target, ":") {
		target += ":443"
	}

	log.Printf("[WEBSOCKET-MITM] requestID=%s Connecting to %s (wss) for WebSocket", reqData.RequestID, target)

	// For WebSocket, use standard crypto/tls with strict HTTP/1.1 ALPN
	// uTLS may not enforce ALPN strictly enough, causing servers to respond with HTTP/2
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	tcpConn, err := dialer.Dial("tcp", target)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to connect to WebSocket server: %v", err)
		log.Printf("[ERROR] requestID=%s %s", reqData.RequestID, errorMsg)
		responseErr := fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(errorMsg), errorMsg)
		clientConn.Write([]byte(responseErr))
		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), req, reqData.RequestID, h.config)
		return
	}

	// TLS config that ONLY allows HTTP/1.1 - critical for WebSocket
	tlsConfig := &tls.Config{
		ServerName:         h.host,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		NextProtos:         []string{"http/1.1"}, // ONLY HTTP/1.1, no h2
	}

	upstreamConn := tls.Client(tcpConn, tlsConfig)
	if err := upstreamConn.Handshake(); err != nil {
		tcpConn.Close()
		errorMsg := fmt.Sprintf("TLS handshake failed for WebSocket: %v", err)
		log.Printf("[ERROR] requestID=%s %s", reqData.RequestID, errorMsg)
		responseErr := fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(errorMsg), errorMsg)
		clientConn.Write([]byte(responseErr))
		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), req, reqData.RequestID, h.config)
		return
	}
	defer upstreamConn.Close()

	// Verify we got HTTP/1.1 and not HTTP/2
	negotiatedProto := upstreamConn.ConnectionState().NegotiatedProtocol
	if negotiatedProto == "h2" {
		errorMsg := "Server requires HTTP/2 which is not supported for WebSocket proxying"
		log.Printf("[ERROR] requestID=%s %s (negotiated: %s)", reqData.RequestID, errorMsg, negotiatedProto)
		responseErr := fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(errorMsg), errorMsg)
		clientConn.Write([]byte(responseErr))
		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), req, reqData.RequestID, h.config)
		return
	}

	log.Printf("[WEBSOCKET-MITM] requestID=%s TLS handshake successful with %s (proto: %s)", reqData.RequestID, target, negotiatedProto)

	// Manually write request to upstream to preserve hop-by-hop headers (Upgrade, Connection)
	// which http.Request.Write strips.
	var reqBuf bytes.Buffer
	path := processedRequest.URL.Path
	if path == "" {
		path = "/"
	}
	if processedRequest.URL.RawQuery != "" {
		path += "?" + processedRequest.URL.RawQuery
	}

	fmt.Fprintf(&reqBuf, "%s %s HTTP/1.1\r\n", processedRequest.Method, path)

	// Ensure Host header matches upstream
	if processedRequest.Host != "" {
		fmt.Fprintf(&reqBuf, "Host: %s\r\n", processedRequest.Host)
	} else {
		fmt.Fprintf(&reqBuf, "Host: %s\r\n", h.connectHost)
	}

	for k, vs := range processedRequest.Header {
		if strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vs {
			fmt.Fprintf(&reqBuf, "%s: %s\r\n", k, v)
		}
	}
	fmt.Fprintf(&reqBuf, "\r\n")

	// Forward the WebSocket upgrade request to upstream server
	if _, err := upstreamConn.Write(reqBuf.Bytes()); err != nil {
		errorMsg := fmt.Sprintf("Failed to send WebSocket upgrade request: %v", err)
		log.Printf("[ERROR] requestID=%s %s", reqData.RequestID, errorMsg)

		// Send 502 Bad Gateway to client
		responseErr := fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(errorMsg), errorMsg)
		clientConn.Write([]byte(responseErr))

		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), req, reqData.RequestID, h.config)
		return
	}

	// Read the upgrade response from upstream server
	upstreamReader := bufio.NewReader(upstreamConn)
	resp, err := http.ReadResponse(upstreamReader, processedRequest)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to read WebSocket upgrade response: %v", err)
		log.Printf("[ERROR] requestID=%s %s", reqData.RequestID, errorMsg)

		// Send 502 Bad Gateway to client
		responseErr := fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(errorMsg), errorMsg)
		clientConn.Write([]byte(responseErr))

		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), req, reqData.RequestID, h.config)
		return
	}

	// Check if upgrade was successful
	if resp.StatusCode != http.StatusSwitchingProtocols {
		errorMsg := fmt.Sprintf("WebSocket upgrade failed: %s", resp.Status)
		log.Printf("[ERROR] requestID=%s %s", reqData.RequestID, errorMsg)

		// Forward the error response to client
		respDump, _ := httputil.DumpResponse(resp, true)
		asyncWebSocketCapture(reqDump, respDump, req, reqData.RequestID, h.config)

		// Write response to client
		resp.Write(clientConn)
		return
	}

	log.Printf("[WEBSOCKET-MITM] requestID=%s WebSocket upgrade successful: %s", reqData.RequestID, resp.Status)

	// Capture successful WebSocket upgrade
	respDump, _ := httputil.DumpResponse(resp, false)
	asyncWebSocketCapture(reqDump, respDump, req, reqData.RequestID, h.config)

	// Apply onResponse handler if configured
	if h.config.OnResponseHandler != nil {
		processedResponse, err := h.config.OnResponseHandler(reqData, resp, processedRequest)
		if err != nil {
			log.Printf("[ERROR] requestID=%s MITM WebSocket onResponse handler failed for %s: %v", reqData.RequestID, req.URL.String(), err)
			return
		}
		if processedResponse != nil {
			resp = processedResponse
		}
	}

	// Forward the successful upgrade response to client
	if err := resp.Write(clientConn); err != nil {
		log.Printf("[ERROR] requestID=%s Failed to send WebSocket upgrade response to client: %v", reqData.RequestID, err)
		return
	}

	log.Printf("[WEBSOCKET-MITM] requestID=%s Established WebSocket tunnel to %s", reqData.RequestID, targetURL.String())

	// Create WebSocket context for message tracking
	wsCtx := &WebSocketContext{
		RequestID: reqData.RequestID,
		Host:      targetURL.Host,
		Path:      targetURL.Path,
		URL:       targetURL.String(),
	}

	// Start bidirectional copying with WebSocket frame logging
	StartWebSocketTunnel(clientConn, upstreamConn, wsCtx, clientBuf, h.config)
}

// singleConnListener is a net.Listener that returns a single connection then closes
type singleConnListener struct {
	conn net.Conn
	once sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	var c net.Conn
	l.once.Do(func() {
		c = l.conn
	})
	if c != nil {
		return c, nil
	}
	return nil, io.EOF
}

func (l *singleConnListener) Close() error {
	return nil
}

func (l *singleConnListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}
