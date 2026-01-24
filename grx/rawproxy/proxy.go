package rawproxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

// ProxyHandler is the main HTTP proxy handler
func ProxyHandler(w http.ResponseWriter, r *http.Request, config *Config) {
	// Generate unique request ID
	requestID := generateRequestID()

	// Debug: Log RAW request as received before any processing (uncomment to enable)
	// rawDump, _ := httputil.DumpRequest(r, false)
	// log.Printf("[RAW] requestID=%s Raw request received:\n%s", requestID, string(rawDump))

	// For CONNECT requests, handle separately
	if strings.EqualFold(r.Method, http.MethodConnect) {
		// Quick dump for CONNECT (no body)
		reqDump, _ := httputil.DumpRequest(r, false)
		handleConnect(w, r, reqDump, requestID, config)
		return
	}

	// Check if this is a WebSocket upgrade request
	upgradeHeader := strings.ToLower(r.Header.Get("Upgrade"))
	connectionHeader := strings.ToLower(r.Header.Get("Connection"))

	if upgradeHeader == "websocket" && strings.Contains(connectionHeader, "upgrade") {
		log.Printf("[WEBSOCKET] requestID=%s WebSocket upgrade request to %s", requestID, r.URL.String())
		HandleWebSocketUpgrade(w, r, requestID, config)
		return
	}

	// Create RequestData to pass custom data between request and response handlers
	reqData := &RequestData{
		RequestID: requestID,
		Data:      nil, // Will be populated by OnRequestHandler
	}

	// Apply onRequest handler if configured
	var processedRequest = r
	if config.OnRequestHandler != nil {
		var err error
		processedRequest, err = config.OnRequestHandler(reqData, r)
		if err != nil {
			log.Printf("[ERROR] requestID=%s onRequest handler failed for %s: %v", requestID, r.URL.String(), err)
			http.Error(w, fmt.Sprintf("request processing error: %v", err), http.StatusBadRequest)
			return
		}
		if processedRequest == nil {
			log.Printf("[ERROR] requestID=%s onRequest handler returned nil request for %s", requestID, r.URL.String())
			http.Error(w, "request processing returned nil", http.StatusBadRequest)
			return
		}
	}

	// Capture request dump with body efficiently
	var reqBody []byte
	var reqDump []byte

	if processedRequest.Body != nil {
		// Read body once
		b, err := io.ReadAll(processedRequest.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("error reading request body: %v", err), http.StatusBadRequest)
			return
		}
		reqBody = b
		processedRequest.Body = io.NopCloser(bytes.NewReader(reqBody))

		// Create dump with body
		reqDump, _ = httputil.DumpRequest(processedRequest, true)
	} else {
		// No body to read
		reqDump, _ = httputil.DumpRequest(processedRequest, false)
	}

	// Forward regular HTTP/HTTPS request using shared transport
	// Transport expects RequestURI to be empty
	rUpstream := processedRequest.Clone(context.Background())
	if reqBody != nil {
		rUpstream.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	rUpstream.RequestURI = ""

	// Set proper host and URL for HTTPS requests
	if rUpstream.URL.Scheme == "" {
		if processedRequest.TLS != nil || rUpstream.Header.Get("X-Forwarded-Proto") == "https" {
			rUpstream.URL.Scheme = "https"
		} else {
			rUpstream.URL.Scheme = "http"
		}
	}
	if rUpstream.URL.Host == "" {
		rUpstream.URL.Host = processedRequest.Host
	}

	// Get transport with browser TLS fingerprint for HTTPS (bypasses Cloudflare)
	// For HTTP, uses standard transport
	transport := GetTransportForHost(rUpstream.URL.Scheme, rUpstream.URL.Hostname())

	// Make the request with browser-like TLS fingerprint
	resp, err := transport.RoundTrip(rUpstream)

	if err != nil {
		// Log detailed error information for debugging
		log.Printf("[ERROR] requestID=%s RoundTrip failed for %s: %v", requestID, rUpstream.URL.String(), err)

		// Provide more specific error messages
		var errorMsg string
		if strings.Contains(err.Error(), "tls") || strings.Contains(err.Error(), "certificate") {
			errorMsg = fmt.Sprintf("TLS/Certificate error: %v", err)
		} else if strings.Contains(err.Error(), "timeout") {
			errorMsg = fmt.Sprintf("Connection timeout: %v", err)
		} else if strings.Contains(err.Error(), "connection refused") {
			errorMsg = fmt.Sprintf("Connection refused: %v", err)
		} else {
			errorMsg = fmt.Sprintf("Network error: %v", err)
		}

		http.Error(w, errorMsg, http.StatusBadGateway)
		// Async capture for errors
		errorResp := []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg))
		asyncCapture(reqDump, errorResp, r, requestID, config)
		return
	}
	defer resp.Body.Close()

	// Apply onResponse handler if configured (use same reqData from onRequest)
	var processedResponse = resp
	if config.OnResponseHandler != nil {
		var err error
		processedResponse, err = config.OnResponseHandler(reqData, resp, r)
		if err != nil {
			log.Printf("[ERROR] requestID=%s onResponse handler failed for %s: %v", requestID, r.URL.String(), err)
			http.Error(w, fmt.Sprintf("response processing error: %v", err), http.StatusInternalServerError)
			return
		}
		if processedResponse == nil {
			log.Printf("[ERROR] requestID=%s onResponse handler returned nil response for %s", requestID, r.URL.String())
			http.Error(w, "response processing returned nil", http.StatusInternalServerError)
			return
		}
	}

	// Copy headers first
	copyHeader(w.Header(), processedResponse.Header)
	w.WriteHeader(processedResponse.StatusCode)

	// Stream response while capturing
	var respBuf bytes.Buffer
	respBuf.WriteString(fmt.Sprintf("%s %s\r\n", processedResponse.Proto, processedResponse.Status))
	processedResponse.Header.Write(&respBuf)
	respBuf.WriteString("\r\n")

	// Use TeeReader to stream and capture simultaneously
	teeReader := io.TeeReader(processedResponse.Body, &respBuf)

	// Stream to client while capturing
	if _, err := io.Copy(w, teeReader); err != nil {
		log.Printf("[WARN] requestID=%s error streaming response for %s: %v", requestID, r.URL.String(), err)
	}

	// Async capture (non-blocking)
	asyncCapture(reqDump, respBuf.Bytes(), r, requestID, config)
}

func handleConnect(w http.ResponseWriter, r *http.Request, reqDump []byte, requestID string, config *Config) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "proxy does not support hijacking", http.StatusInternalServerError)
		asyncCapture(reqDump, []byte("HTTP/1.1 500 Internal Server Error\r\n\r\nHijacking not supported\n"), r, requestID, config)
		log.Printf("[ERROR] requestID=%s url=%s hijacking not supported", requestID, r.Host)
		return
	}

	clientConn, clientBuf, err := hj.Hijack()
	if err != nil {
		asyncCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 500 Internal Server Error\r\n\r\n%v\n", err)), r, requestID, config)
		log.Printf("[ERROR] requestID=%s url=%s hijack failed: %v", requestID, r.Host, err)
		return
	}
	// Send 200 Connection Established to client
	established := "HTTP/1.1 200 Connection Established\r\nProxy-Agent: go-capture-proxy\r\n\r\n"
	if clientBuf.Reader.Buffered() > 0 {
		_ = clientBuf.Writer.Flush()
	}
	_, _ = clientConn.Write([]byte(established))

	// Check if this is a WebSocket CONNECT (port 80) or HTTPS CONNECT (port 443)
	target := r.Host
	if !strings.Contains(target, ":") {
		target += ":443" // Default to HTTPS
	}

	isWebSocketConnect := strings.HasSuffix(target, ":80")

	if isWebSocketConnect {
		log.Printf("[WEBSOCKET-CONNECT] requestID=%s WebSocket CONNECT to %s", requestID, target)
		// Handle WebSocket CONNECT - don't do MITM, just establish tunnel
		HandleWebSocketConnect(clientConn, target, reqDump, requestID, config)
		return
	}

	// Decide whether to MITM or passthrough for HTTPS
	if config.MITM != nil {
		// Man-in-the-middle: terminate TLS and proxy HTTP messages
		MitmHTTPS(clientConn, r, requestID, config)
		return
	}

	log.Printf("[PASSTHROUGH] requestID=%s url=%s", requestID, r.Host)
	defer clientConn.Close()
	// Use dialer with better error handling
	dialer := &net.Dialer{
		Timeout:   15 * time.Second, // Increase timeout for CONNECT
		KeepAlive: 30 * time.Second,
	}
	upstreamConn, err := dialer.Dial("tcp", target)
	if err != nil {
		log.Printf("[ERROR] requestID=%s CONNECT dial failed for %s: %v", requestID, target, err)
		errorMsg := fmt.Sprintf("Failed to connect to %s: %v", target, err)
		asyncCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), r, requestID, config)
		return
	}
	defer upstreamConn.Close()

	// For non-MITM mode, async capture of CONNECT line and 200 response
	asyncCapture(reqDump, []byte(established), r, requestID, config)

	// If the server read ahead while parsing CONNECT, forward any buffered
	// client bytes (e.g., TLS ClientHello) to the upstream before piping.
	if n := clientBuf.Reader.Buffered(); n > 0 {
		if _, err := io.CopyN(upstreamConn, clientBuf, int64(n)); err != nil {
			// Non-fatal; the subsequent io.Copy may still surface the error
			log.Printf("[WARN] requestID=%s forwarding buffered client bytes failed for %s: %v", requestID, r.Host, err)
		}
	}

	errc := make(chan error, 2)
	go func() {
		_, e := io.Copy(upstreamConn, clientConn)
		if tcp, ok := upstreamConn.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		errc <- e
	}()
	go func() {
		_, e := io.Copy(clientConn, upstreamConn)
		if tcp, ok := clientConn.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		errc <- e
	}()
	<-errc
}
