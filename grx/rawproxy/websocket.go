package rawproxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WebSocket frame types
const (
	wsOpcodeContinuation = 0x0
	wsOpcodeText         = 0x1
	wsOpcodeBinary       = 0x2
	wsOpcodeClose        = 0x8
	wsOpcodePing         = 0x9
	wsOpcodePong         = 0xa
)

type wsFrame struct {
	fin     bool
	opcode  byte
	masked  bool
	length  uint64
	payload []byte
	raw     []byte
}

// parseWebSocketFrame parses a single WebSocket frame from the given data
func parseWebSocketFrame(data []byte) (*wsFrame, int, error) {
	if len(data) < 2 {
		return nil, 0, fmt.Errorf("insufficient data for WebSocket frame header")
	}

	frame := &wsFrame{}
	frame.raw = data

	// First byte: FIN (1 bit) + RSV (3 bits) + Opcode (4 bits)
	firstByte := data[0]
	frame.fin = (firstByte & 0x80) != 0
	frame.opcode = firstByte & 0x0F

	// Second byte: MASK (1 bit) + Payload Length (7 bits)
	secondByte := data[1]
	frame.masked = (secondByte & 0x80) != 0
	payloadLen := uint64(secondByte & 0x7F)

	offset := 2

	// Extended payload length
	if payloadLen == 126 {
		if len(data) < offset+2 {
			return nil, 0, fmt.Errorf("insufficient data for 16-bit length")
		}
		payloadLen = uint64(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
	} else if payloadLen == 127 {
		if len(data) < offset+8 {
			return nil, 0, fmt.Errorf("insufficient data for 64-bit length")
		}
		payloadLen = binary.BigEndian.Uint64(data[offset : offset+8])
		offset += 8
	}

	frame.length = payloadLen

	// Masking key (if present)
	var maskKey []byte
	if frame.masked {
		if len(data) < offset+4 {
			return nil, 0, fmt.Errorf("insufficient data for mask key")
		}
		maskKey = data[offset : offset+4]
		offset += 4
	}

	// Check if we have the complete payload
	if len(data) < offset+int(payloadLen) {
		return nil, 0, fmt.Errorf("insufficient data for payload")
	}

	// Extract and unmask payload
	frame.payload = make([]byte, payloadLen)
	copy(frame.payload, data[offset:offset+int(payloadLen)])

	if frame.masked && maskKey != nil {
		for i := range frame.payload {
			frame.payload[i] ^= maskKey[i%4]
		}
	}

	return frame, offset + int(payloadLen), nil
}

func getFrameTypeName(opcode byte) string {
	switch opcode {
	case wsOpcodeContinuation:
		return "continuation"
	case wsOpcodeText:
		return "text"
	case wsOpcodeBinary:
		return "binary"
	case wsOpcodeClose:
		return "close"
	case wsOpcodePing:
		return "ping"
	case wsOpcodePong:
		return "pong"
	default:
		return fmt.Sprintf("unknown_%d", opcode)
	}
}

func saveWebSocketMessage(requestID, direction, frameType string, payload []byte, timestamp time.Time, config *Config) {
	// Create filename for WebSocket message
	ts := timestamp.UTC().Format("20060102-150405.000000000")
	fileName := fmt.Sprintf("%s-ws-%s-%s-%s.txt", ts, requestID, direction, frameType)
	filePath := filepath.Join(config.WebSocketDir, fileName)

	// Create the message file
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("[ERROR] Failed to create WebSocket message file %s: %v", filePath, err)
		return
	}
	defer file.Close()

	// Write message content
	fmt.Fprintf(file, "WebSocket Message\n")
	fmt.Fprintf(file, "================\n")
	fmt.Fprintf(file, "RequestID: %s\n", requestID)
	fmt.Fprintf(file, "Direction: %s\n", direction)
	fmt.Fprintf(file, "Frame Type: %s\n", frameType)
	fmt.Fprintf(file, "Timestamp: %s\n", timestamp.Format(time.RFC3339Nano))
	fmt.Fprintf(file, "Payload Length: %d bytes\n", len(payload))
	fmt.Fprintf(file, "\n--- Message Content ---\n")

	if frameType == "text" {
		// For text frames, write as string
		fmt.Fprintf(file, "%s\n", string(payload))
	} else if frameType == "binary" {
		// For binary frames, write as hex dump
		fmt.Fprintf(file, "Binary data (hex):\n")
		for i := 0; i < len(payload); i += 16 {
			end := i + 16
			if end > len(payload) {
				end = len(payload)
			}
			fmt.Fprintf(file, "%08x: ", i)
			for j := i; j < end; j++ {
				fmt.Fprintf(file, "%02x ", payload[j])
			}
			fmt.Fprintf(file, "\n")
		}
	} else {
		// For control frames, write payload as string if printable
		if len(payload) > 0 {
			fmt.Fprintf(file, "%s\n", string(payload))
		}
	}

	log.Printf("[WS-MSG] requestID=%s %s %s message (%d bytes) saved to %s",
		requestID, direction, frameType, len(payload), fileName)
}

// websocketCapturingCopy copies data between connections while capturing WebSocket frames
func websocketCapturingCopy(dst io.Writer, src io.Reader, requestID, direction string, config *Config) error {
	buffer := make([]byte, 4096)
	frameBuffer := make([]byte, 0, 8192) // Buffer for partial frames
	skipHTTPHeaders := true              // Skip HTTP upgrade handshake data

	for {
		n, err := src.Read(buffer)
		if n > 0 {
			// Write data to destination
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}

			// Add to frame buffer for WebSocket parsing
			frameBuffer = append(frameBuffer, buffer[:n]...)

			// Skip HTTP handshake data (until we transition to WebSocket frames)
			if skipHTTPHeaders {
				// Check if this looks like HTTP data (starts with HTTP method or response)
				frameStr := string(frameBuffer)
				if strings.HasPrefix(frameStr, "GET ") ||
					strings.HasPrefix(frameStr, "POST ") ||
					strings.HasPrefix(frameStr, "HTTP/") ||
					strings.Contains(frameStr, "Upgrade: websocket") {

					// Look for end of HTTP headers (\r\n\r\n)
					if headerEnd := bytes.Index(frameBuffer, []byte("\r\n\r\n")); headerEnd != -1 {
						// Remove HTTP headers from buffer
						frameBuffer = frameBuffer[headerEnd+4:]
						skipHTTPHeaders = false
						log.Printf("[WS-DEBUG] requestID=%s %s skipped HTTP handshake data, starting WebSocket frame parsing", requestID, direction)
					} else {
						// Still in HTTP headers, skip parsing
						continue
					}
				} else {
					// No HTTP markers found, assume we're in WebSocket mode
					skipHTTPHeaders = false
					log.Printf("[WS-DEBUG] requestID=%s %s no HTTP headers detected, starting WebSocket frame parsing", requestID, direction)
				}
			}

			// Try to parse WebSocket frames from buffer
			for len(frameBuffer) >= 2 {
				frame, frameLen, parseErr := parseWebSocketFrame(frameBuffer)
				if parseErr != nil {
					// Not enough data for complete frame, wait for more
					break
				}

				// Validate opcode (should be 0-2 or 8-10 for valid WebSocket frames)
				if !isValidWebSocketOpcode(frame.opcode) {
					log.Printf("[WS-WARN] requestID=%s %s invalid opcode %d, skipping frame", requestID, direction, frame.opcode)
					frameBuffer = frameBuffer[1:] // Skip one byte and try again
					continue
				}

				// Save the WebSocket message
				frameType := getFrameTypeName(frame.opcode)
				timestamp := time.Now()

				// Only save meaningful frames (skip ping/pong for now)
				if frame.opcode == wsOpcodeText || frame.opcode == wsOpcodeBinary || frame.opcode == wsOpcodeClose {
					saveWebSocketMessage(requestID, direction, frameType, frame.payload, timestamp, config)
				} else {
					log.Printf("[WS-FRAME] requestID=%s %s %s frame (%d bytes)",
						requestID, direction, frameType, len(frame.payload))
				}

				// Remove processed frame from buffer
				frameBuffer = frameBuffer[frameLen:]
			}
		}

		if err != nil {
			return err
		}
	}
}

func isValidWebSocketOpcode(opcode byte) bool {
	// Valid WebSocket opcodes: 0x0-0x2 (data frames), 0x8-0xA (control frames)
	return opcode <= 0x2 || (opcode >= 0x8 && opcode <= 0xA)
}

func HandleWebSocketUpgrade(w http.ResponseWriter, r *http.Request, requestID string, config *Config) {
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
			log.Printf("[ERROR] requestID=%s WebSocket onRequest handler failed for %s: %v", requestID, r.URL.String(), err)
			http.Error(w, fmt.Sprintf("WebSocket request processing error: %v", err), http.StatusBadRequest)
			return
		}
		if processedRequest == nil {
			log.Printf("[ERROR] requestID=%s WebSocket onRequest handler returned nil request for %s", requestID, r.URL.String())
			http.Error(w, "WebSocket request processing returned nil", http.StatusBadRequest)
			return
		}
	}

	// Capture the WebSocket upgrade request
	reqDump, err := httputil.DumpRequest(processedRequest, false)
	if err != nil {
		log.Printf("[ERROR] requestID=%s Failed to dump WebSocket request: %v", requestID, err)
		reqDump = []byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\n\r\n", processedRequest.URL.Path, processedRequest.Host))
	}

	// Hijack the connection to handle WebSocket upgrade
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket upgrade not supported", http.StatusInternalServerError)
		asyncWebSocketCapture(reqDump, []byte("HTTP/1.1 500 Internal Server Error\r\n\r\nWebSocket upgrade not supported\n"), r, requestID, config)
		log.Printf("[ERROR] requestID=%s WebSocket hijacking not supported for %s", requestID, r.URL.String())
		return
	}

	clientConn, clientBuf, err := hj.Hijack()
	if err != nil {
		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 500 Internal Server Error\r\n\r\n%v\n", err)), r, requestID, config)
		log.Printf("[ERROR] requestID=%s WebSocket hijack failed for %s: %v", requestID, r.URL.String(), err)
		return
	}
	defer clientConn.Close()

	// Determine target URL
	targetURL := *processedRequest.URL
	if targetURL.Scheme == "" {
		if processedRequest.TLS != nil {
			targetURL.Scheme = "wss"
		} else {
			targetURL.Scheme = "ws"
		}
	}
	if targetURL.Host == "" {
		targetURL.Host = processedRequest.Host
	}

	// Establish direct TCP connection to target server
	target := targetURL.Host
	if !strings.Contains(target, ":") {
		if targetURL.Scheme == "wss" {
			target += ":443"
		} else {
			target += ":80"
		}
	}

	log.Printf("[WEBSOCKET] requestID=%s Connecting to %s (%s)", requestID, target, targetURL.Scheme)

	// Establish connection to upstream server (TCP for ws://, TLS for wss://)
	var upstreamConn net.Conn
	if targetURL.Scheme == "wss" {
		// Secure WebSocket - establish TLS connection
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
			ServerName:         targetURL.Hostname(), // Critical for SNI
		}

		var err error
		upstreamConn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: 15 * time.Second},
			"tcp",
			target,
			tlsConfig,
		)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to establish TLS connection to WebSocket server: %v", err)
			log.Printf("[ERROR] requestID=%s %s", requestID, errorMsg)
			asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), r, requestID, config)
			return
		}
		log.Printf("[WEBSOCKET] requestID=%s TLS handshake successful with %s", requestID, target)
	} else {
		// Plain WebSocket - establish TCP connection
		var err error
		upstreamConn, err = net.DialTimeout("tcp", target, 15*time.Second)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to connect to WebSocket server: %v", err)
			log.Printf("[ERROR] requestID=%s %s", requestID, errorMsg)
			asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), r, requestID, config)
			return
		}
	}
	defer upstreamConn.Close()

	log.Printf("[WEBSOCKET] requestID=%s Connected to %s", requestID, target)

	// Forward the WebSocket upgrade request to upstream server
	if err := processedRequest.Write(upstreamConn); err != nil {
		errorMsg := fmt.Sprintf("Failed to send WebSocket upgrade request: %v", err)
		log.Printf("[ERROR] requestID=%s %s", requestID, errorMsg)
		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), r, requestID, config)
		return
	}

	// Read the upgrade response from upstream server
	upstreamReader := bufio.NewReader(upstreamConn)
	resp, err := http.ReadResponse(upstreamReader, processedRequest)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to read WebSocket upgrade response: %v", err)
		log.Printf("[ERROR] requestID=%s %s", requestID, errorMsg)
		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), r, requestID, config)
		return
	}

	// Check if upgrade was successful
	if resp.StatusCode != http.StatusSwitchingProtocols {
		errorMsg := fmt.Sprintf("WebSocket upgrade failed: %s", resp.Status)
		log.Printf("[ERROR] requestID=%s %s", requestID, errorMsg)

		// Forward the error response to client
		respDump, _ := httputil.DumpResponse(resp, true)
		asyncWebSocketCapture(reqDump, respDump, r, requestID, config)

		// Write response to client
		resp.Write(clientConn)
		return
	}

	log.Printf("[WEBSOCKET] requestID=%s WebSocket upgrade successful: %s", requestID, resp.Status)

	// Capture successful WebSocket upgrade
	respDump, _ := httputil.DumpResponse(resp, false)
	asyncWebSocketCapture(reqDump, respDump, r, requestID, config)

	// Apply onResponse handler if configured (use same reqData from onRequest)
	if config.OnResponseHandler != nil {
		processedResponse, err := config.OnResponseHandler(reqData, resp, processedRequest)
		if err != nil {
			log.Printf("[ERROR] requestID=%s WebSocket onResponse handler failed for %s: %v", requestID, r.URL.String(), err)
			return
		}
		if processedResponse != nil {
			resp = processedResponse
		}
	}

	// Forward the successful upgrade response to client
	if err := resp.Write(clientConn); err != nil {
		log.Printf("[ERROR] requestID=%s Failed to send WebSocket upgrade response to client: %v", requestID, err)
		return
	}

	log.Printf("[WEBSOCKET] requestID=%s Established WebSocket tunnel to %s", requestID, targetURL.String())

	// Start bidirectional copying with WebSocket frame logging
	StartWebSocketTunnel(clientConn, upstreamConn, requestID, clientBuf, config)
}

func StartWebSocketTunnel(clientConn, serverConn net.Conn, requestID string, clientBuf *bufio.ReadWriter, config *Config) {
	// Handle any buffered data from client (this is likely residual HTTP data, forward directly)
	if clientBuf != nil && clientBuf.Reader.Buffered() > 0 {
		bufferedBytes := clientBuf.Reader.Buffered()
		log.Printf("[WS-DEBUG] requestID=%s Forwarding %d buffered bytes (likely HTTP handshake residue)", requestID, bufferedBytes)
		if _, err := io.CopyN(serverConn, clientBuf, int64(bufferedBytes)); err != nil {
			log.Printf("[WARN] requestID=%s Error forwarding buffered WebSocket data: %v", requestID, err)
		}
	}

	// Start bidirectional copying
	errc := make(chan error, 2)

	// Client to Server with WebSocket frame capture
	go func() {
		defer func() {
			if tcp, ok := serverConn.(*net.TCPConn); ok {
				tcp.CloseWrite()
			}
		}()
		err := websocketCapturingCopy(serverConn, clientConn, requestID, "client-to-server", config)
		if err != nil {
			log.Printf("[WEBSOCKET] requestID=%s Client->Server copy ended: %v", requestID, err)
		} else {
			log.Printf("[WEBSOCKET] requestID=%s Client->Server copy ended normally", requestID)
		}
		errc <- err
	}()

	// Server to Client with WebSocket frame capture
	go func() {
		defer func() {
			if tcp, ok := clientConn.(*net.TCPConn); ok {
				tcp.CloseWrite()
			}
		}()
		err := websocketCapturingCopy(clientConn, serverConn, requestID, "server-to-client", config)
		if err != nil {
			log.Printf("[WEBSOCKET] requestID=%s Server->Client copy ended: %v", requestID, err)
		} else {
			log.Printf("[WEBSOCKET] requestID=%s Server->Client copy ended normally", requestID)
		}
		errc <- err
	}()

	// Wait for either direction to complete
	<-errc
	log.Printf("[WEBSOCKET] requestID=%s WebSocket tunnel closed", requestID)
}

func HandleWebSocketConnect(clientConn net.Conn, target string, reqDump []byte, requestID string, config *Config) {
	defer clientConn.Close()

	log.Printf("[WEBSOCKET-CONNECT] requestID=%s Establishing TCP connection to %s", requestID, target)

	// Establish direct TCP connection to target (no TLS for port 80)
	upstreamConn, err := net.DialTimeout("tcp", target, 15*time.Second)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to connect to WebSocket server %s: %v", target, err)
		log.Printf("[ERROR] requestID=%s %s", requestID, errorMsg)
		// Create a proper request object for capture
		dummyReq, _ := http.NewRequest("CONNECT", "http://"+target, nil)
		dummyReq.Host = strings.Split(target, ":")[0]
		asyncWebSocketCapture(reqDump, []byte(fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\n\r\n%s\n", errorMsg)), dummyReq, requestID, config)
		return
	}
	defer upstreamConn.Close()

	log.Printf("[WEBSOCKET-CONNECT] requestID=%s Connected to %s, starting tunnel", requestID, target)

	// Async capture of CONNECT and 200 response
	established := "HTTP/1.1 200 Connection Established\r\nProxy-Agent: go-capture-proxy\r\n\r\n"
	// Create a proper request object for capture
	dummyReq, _ := http.NewRequest("CONNECT", "http://"+target, nil)
	dummyReq.Host = strings.Split(target, ":")[0]
	asyncWebSocketCapture(reqDump, []byte(established), dummyReq, requestID, config)

	// Start bidirectional copying for WebSocket tunnel
	errc := make(chan error, 2)

	// Client to Server with WebSocket frame capture
	go func() {
		defer func() {
			if tcp, ok := upstreamConn.(*net.TCPConn); ok {
				tcp.CloseWrite()
			}
		}()
		err := websocketCapturingCopy(upstreamConn, clientConn, requestID, "client-to-server", config)
		if err != nil {
			log.Printf("[WEBSOCKET-CONNECT] requestID=%s Client->Server copy ended: %v", requestID, err)
		} else {
			log.Printf("[WEBSOCKET-CONNECT] requestID=%s Client->Server copy ended normally", requestID)
		}
		errc <- err
	}()

	// Server to Client with WebSocket frame capture
	go func() {
		defer func() {
			if tcp, ok := clientConn.(*net.TCPConn); ok {
				tcp.CloseWrite()
			}
		}()
		err := websocketCapturingCopy(clientConn, upstreamConn, requestID, "server-to-client", config)
		if err != nil {
			log.Printf("[WEBSOCKET-CONNECT] requestID=%s Server->Client copy ended: %v", requestID, err)
		} else {
			log.Printf("[WEBSOCKET-CONNECT] requestID=%s Server->Client copy ended normally", requestID)
		}
		errc <- err
	}()

	// Wait for either direction to complete
	<-errc
	log.Printf("[WEBSOCKET-CONNECT] requestID=%s WebSocket CONNECT tunnel closed", requestID)
}
