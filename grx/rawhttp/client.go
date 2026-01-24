package rawhttp

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	utls "github.com/refraction-networking/utls"
)

// Client is a raw HTTP client that sends requests with minimal validation
type Client struct {
	config Config
}

// NewClient creates a new raw HTTP client with the given configuration
// Always uses browser TLS fingerprint to bypass Cloudflare and other CDN bot detection
func NewClient(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.TLSMinVersion == 0 {
		config.TLSMinVersion = tls.VersionTLS12
	}
	// Always use browser fingerprint (Chrome by default)
	config.UseBrowserFingerprint = true
	if config.BrowserFingerprint == "" {
		config.BrowserFingerprint = "chrome"
	}
	return &Client{config: config}
}

// DefaultClient returns a client with sensible defaults
// Uses browser TLS fingerprint to bypass Cloudflare
func DefaultClient() *Client {
	return NewClient(Config{
		Timeout:            30 * time.Second,
		InsecureSkipVerify: true,
	})
}

// getUTLSClientHelloID returns the uTLS ClientHelloID based on config
func (c *Client) getUTLSClientHelloID() utls.ClientHelloID {
	switch strings.ToLower(c.config.BrowserFingerprint) {
	case "firefox":
		return utls.HelloFirefox_Auto
	case "safari":
		return utls.HelloSafari_Auto
	case "edge":
		return utls.HelloEdge_Auto
	case "random":
		return utls.HelloRandomized
	case "chrome", "":
		return utls.HelloChrome_Auto
	default:
		return utls.HelloChrome_Auto
	}
}

// dialUTLS creates a TLS connection using uTLS with browser fingerprint
func (c *Client) dialUTLS(addr, serverName string) (net.Conn, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	// Dial TCP connection
	dialer := &net.Dialer{
		Timeout: c.config.Timeout,
	}
	tcpConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial TCP %s: %w", addr, err)
	}

	// Create uTLS config
	// Note: NextProtos here is just for the config, but browser fingerprints
	// override this with their own ALPN extensions
	config := &utls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: c.config.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
	}

	// Create uTLS connection with browser fingerprint
	utlsConn := utls.UClient(tcpConn, config, c.getUTLSClientHelloID())

	// Build handshake state first so we can modify ALPN extension
	// Browser fingerprints include their own ALPN (typically ["h2", "http/1.1"])
	// which would cause server to negotiate HTTP/2. We must override to force HTTP/1.1.
	if err := utlsConn.BuildHandshakeState(); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("failed to build handshake state for %s: %w", serverName, err)
	}

	// Override ALPN extension to force HTTP/1.1 only
	// This is critical: without this, servers negotiate HTTP/2 and send binary frames
	for _, ext := range utlsConn.Extensions {
		if alpnExt, ok := ext.(*utls.ALPNExtension); ok {
			alpnExt.AlpnProtocols = []string{"http/1.1"}
			break
		}
	}

	// Perform TLS handshake with the modified ALPN
	if err := utlsConn.HandshakeContext(ctx); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("uTLS handshake failed for %s: %w", serverName, err)
	}

	return utlsConn, nil
}

// dialUTLSForHTTP2 creates a TLS connection using uTLS with browser fingerprint for HTTP/2
func (c *Client) dialUTLSForHTTP2(ctx context.Context, network, addr, serverName string) (net.Conn, error) {
	// Dial TCP connection
	dialer := &net.Dialer{
		Timeout: c.config.Timeout,
	}
	tcpConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial TCP %s: %w", addr, err)
	}

	// Create uTLS config for HTTP/2
	config := &utls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: c.config.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		NextProtos:         []string{"h2"}, // HTTP/2 ALPN
	}

	// Create uTLS connection with browser fingerprint
	utlsConn := utls.UClient(tcpConn, config, c.getUTLSClientHelloID())

	// Perform TLS handshake
	if err := utlsConn.HandshakeContext(ctx); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("uTLS handshake failed for %s: %w", serverName, err)
	}

	return utlsConn, nil
}

// Send sends a raw HTTP request and returns the raw response.
// This function performs minimal validation - only what's necessary for TCP/TLS connection.
// All malformed headers, formatting issues, and protocol violations are preserved and sent as-is.
// If UseHTTP2 is true, the request will be sent using HTTP/2 protocol.
func (c *Client) Send(req Request) (*Response, error) {
	// Route to HTTP/2 handler if requested
	if req.UseHTTP2 {
		return c.SendHTTP2(req)
	}

	// Determine port
	port := req.Port
	if port == "" {
		if req.UseTLS {
			port = "443"
		} else {
			port = "80"
		}
	}

	// Build address
	addr := net.JoinHostPort(req.Host, port)

	// Establish connection
	var conn net.Conn
	var err error

	if req.UseTLS {
		if c.config.UseBrowserFingerprint {
			// Use uTLS to mimic browser TLS fingerprint (bypasses Cloudflare)
			conn, err = c.dialUTLS(addr, req.Host)
		} else {
			// Use standard TLS with minimal validation
			dialer := &net.Dialer{
				Timeout: c.config.Timeout,
			}
			tlsConfig := &tls.Config{
				InsecureSkipVerify: c.config.InsecureSkipVerify,
				MinVersion:         c.config.TLSMinVersion,
				ServerName:         req.Host, // Optional, may be empty for malformed requests
			}
			conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
		}
	} else {
		// Plain TCP connection
		conn, err = net.DialTimeout("tcp", addr, c.config.Timeout)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()

	// Set write deadline
	if err := conn.SetWriteDeadline(time.Now().Add(c.config.Timeout)); err != nil {
		return nil, fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Send raw request bytes as-is (no validation, no modification)
	requestStartTime := time.Now()
	if _, err := conn.Write(req.RawBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Set read deadline using configured timeout (only as safety, not for blocking)
	if err := conn.SetReadDeadline(time.Now().Add(c.config.Timeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read response using buffered reader
	reader := bufio.NewReader(conn)

	// Read headers first (until \r\n\r\n) - optimized approach
	headerBytes := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	headerEnd := false
	var responseTime time.Duration

	for !headerEnd {
		n, err := reader.Read(buf)
		if n > 0 {
			headerBytes = append(headerBytes, buf[:n]...)

			// Check for \r\n\r\n (most common) - this means headers are complete!
			if idx := bytes.Index(headerBytes, []byte("\r\n\r\n")); idx >= 0 {
				headerEnd = true
				// Record time when we received the complete headers
				responseTime = time.Since(requestStartTime)
				// Break immediately - headers are done!
				break
			} else if idx := bytes.Index(headerBytes, []byte("\n\n")); idx >= 0 {
				// Check for \n\n (alternative) - headers are complete!
				headerEnd = true
				// Record time when we received the complete headers
				responseTime = time.Since(requestStartTime)
				// Break immediately - headers are done!
				break
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read response headers: %w", err)
		}
	}

	// Find where headers end in the buffer
	var headerEndIdx int
	if idx := bytes.Index(headerBytes, []byte("\r\n\r\n")); idx >= 0 {
		headerEndIdx = idx + 4 // Include \r\n\r\n
	} else if idx := bytes.Index(headerBytes, []byte("\n\n")); idx >= 0 {
		headerEndIdx = idx + 2 // Include \n\n
	} else {
		headerEndIdx = len(headerBytes)
	}

	// Check if there's already body data in the buffer after headers
	bodyBytesAlreadyRead := headerBytes[headerEndIdx:]
	headerBytes = headerBytes[:headerEndIdx]

	// Parse headers to find Content-Length, Transfer-Encoding, and Content-Encoding - optimized parsing
	contentLength := -1
	chunked := false
	contentEncoding := ""

	// Find Content-Length header efficiently
	headerLower := strings.ToLower(string(headerBytes))
	if idx := strings.Index(headerLower, "content-length:"); idx >= 0 {
		// Extract value after colon
		start := idx + len("content-length:")
		end := start
		for end < len(headerLower) && headerLower[end] != '\r' && headerLower[end] != '\n' {
			end++
		}
		if cl, err := strconv.Atoi(strings.TrimSpace(headerLower[start:end])); err == nil {
			contentLength = cl
		}
	}

	// Check for chunked encoding
	if strings.Contains(headerLower, "transfer-encoding:") && strings.Contains(headerLower, "chunked") {
		chunked = true
	}

	// Find Content-Encoding header efficiently
	if idx := strings.Index(headerLower, "content-encoding:"); idx >= 0 {
		// Extract value after colon
		start := idx + len("content-encoding:")
		end := start
		for end < len(headerLower) && headerLower[end] != '\r' && headerLower[end] != '\n' {
			end++
		}
		contentEncoding = strings.TrimSpace(headerLower[start:end])
		// Handle multiple encodings (e.g., "gzip, deflate" - take first one)
		if commaIdx := strings.IndexByte(contentEncoding, ','); commaIdx >= 0 {
			contentEncoding = strings.TrimSpace(contentEncoding[:commaIdx])
		}
	}

	responseBytes := headerBytes
	responseBytes = append(responseBytes, bodyBytesAlreadyRead...)

	// Read body based on Content-Length or chunked encoding
	if contentLength > 0 {
		// Read exact number of bytes - calculate how much we still need
		alreadyRead := len(bodyBytesAlreadyRead)
		remaining := contentLength - alreadyRead
		if remaining > 0 {
			// Set deadline as safety, but read immediately if available
			conn.SetReadDeadline(time.Now().Add(c.config.Timeout))
			body := make([]byte, remaining)
			n, err := io.ReadFull(reader, body)
			if err != nil && err != io.ErrUnexpectedEOF {
				// If we can't read full body, include what we got
				if n > 0 {
					responseBytes = append(responseBytes, body[:n]...)
				}
			} else {
				responseBytes = append(responseBytes, body...)
			}
		}
	} else if chunked {
		// Read chunked encoding - read until we find the terminating 0\r\n\r\n
		// Start with what we already have
		conn.SetReadDeadline(time.Now().Add(c.config.Timeout))
		buf := make([]byte, 4096)
		for {
			// Check if we already have the chunked termination in what we've read
			if bytes.Contains(responseBytes, []byte("\r\n0\r\n\r\n")) || bytes.Contains(responseBytes, []byte("\n0\n\n")) {
				break
			}
			n, err := reader.Read(buf)
			if n > 0 {
				responseBytes = append(responseBytes, buf[:n]...)
				// Check again after reading
				if bytes.Contains(responseBytes, []byte("\r\n0\r\n\r\n")) || bytes.Contains(responseBytes, []byte("\n0\n\n")) {
					break
				}
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				if err, ok := err.(net.Error); ok && err.Timeout() {
					// Timeout reached, return what we have
					break
				}
				break
			}
		}
	} else {
		// No Content-Length and not chunked - read until EOF (connection closes)
		// This is HTTP/1.0 behavior or Connection: close
		// For HTTP/1.1 keep-alive, if no data is immediately available, return headers only
		// Check if there's buffered data first
		if reader.Buffered() > 0 {
			// There's data in the buffer, read it
			buf := make([]byte, reader.Buffered())
			n, _ := reader.Read(buf)
			if n > 0 {
				responseBytes = append(responseBytes, buf[:n]...)
			}
		}

		// Now try to read more with a short timeout to check if connection is closing
		// Use a short deadline to detect if server is sending more data
		conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		buf := make([]byte, 4096)
		n, err := reader.Read(buf)
		if n > 0 {
			responseBytes = append(responseBytes, buf[:n]...)
			// Got data, continue reading until EOF
			conn.SetReadDeadline(time.Now().Add(c.config.Timeout))
			for {
				n, err := reader.Read(buf)
				if n > 0 {
					responseBytes = append(responseBytes, buf[:n]...)
				}
				if err != nil {
					if err == io.EOF {
						break
					}
					if err, ok := err.(net.Error); ok && err.Timeout() {
						break
					}
					break
				}
			}
		} else if err != nil {
			// No data immediately available - likely keep-alive with no body
			// Return headers only (this is correct for HTTP/1.1 keep-alive)
		}
	}

	// Decode chunked encoding if present (must be done before decompression)
	chunkedDecoded := false
	if chunked && len(responseBytes) > headerEndIdx {
		chunkedBody := responseBytes[headerEndIdx:]
		decodedBody, err := decodeChunkedBody(chunkedBody)
		if err == nil {
			// Replace chunked body with decoded body
			responseBytes = append(responseBytes[:headerEndIdx], decodedBody...)
			chunked = false // Mark as no longer chunked since we decoded it
			chunkedDecoded = true
		}
		// If decoding fails, keep original chunked body
	}

	// Decompress body if Content-Encoding is present
	// Now we can decompress even if it was originally chunked (since we decoded it above)
	if contentEncoding != "" && len(responseBytes) > headerEndIdx {
		bodyBytes := responseBytes[headerEndIdx:]
		if len(bodyBytes) > 0 {
			decompressedBody, err := decompressBodyByEncoding(bodyBytes, contentEncoding)
			if err == nil && len(decompressedBody) > 0 {
				// Rebuild response with decompressed body
				// Update headers: remove Content-Encoding and update Content-Length
				headerBytesStr := string(headerBytes)
				headerLines := splitHeaderLines(headerBytesStr)

				// Extract status line (first line of headers)
				statusLine := ""
				if idx := strings.Index(headerBytesStr, "\r\n"); idx >= 0 {
					statusLine = headerBytesStr[:idx]
				} else if idx := strings.Index(headerBytesStr, "\n"); idx >= 0 {
					statusLine = headerBytesStr[:idx]
				} else {
					statusLine = headerBytesStr
				}

				var newHeaderLines []string
				hasContentLength := false
				lineBreak := detectLineBreakFromHeaders(headerBytes)

				// Start with status line
				newHeaderLines = append(newHeaderLines, statusLine)

				// Process header lines
				for _, line := range headerLines {
					lineLower := strings.ToLower(line)
					// Skip Content-Encoding header
					if strings.HasPrefix(lineLower, "content-encoding:") {
						continue
					}
					// Skip Transfer-Encoding header (we decoded chunked encoding)
					if strings.HasPrefix(lineLower, "transfer-encoding:") {
						continue
					}
					// Update Content-Length with decompressed size
					// if strings.HasPrefix(lineLower, "content-length:") {
					// 	newHeaderLines = append(newHeaderLines, fmt.Sprintf("Content-Length: %d", len(decompressedBody)))
					// 	hasContentLength = true
					// 	continue
					// }
					newHeaderLines = append(newHeaderLines, line)
				}

				// Add Content-Length if it didn't exist
				if !hasContentLength {
					newHeaderLines = append(newHeaderLines, fmt.Sprintf("Content-Length: %d", len(decompressedBody)))
				}

				// Rebuild response with new headers and decompressed body
				newHeaders := strings.Join(newHeaderLines, lineBreak) + lineBreak + lineBreak
				responseBytes = []byte(newHeaders)
				responseBytes = append(responseBytes, decompressedBody...)
			}
		}
	} else if chunkedDecoded && len(responseBytes) > headerEndIdx {
		// We decoded chunked encoding but there's no content-encoding
		// Still need to update headers to remove Transfer-Encoding
		bodyBytes := responseBytes[headerEndIdx:]
		headerBytesStr := string(headerBytes)
		headerLines := splitHeaderLines(headerBytesStr)

		// Extract status line (first line of headers)
		statusLine := ""
		if idx := strings.Index(headerBytesStr, "\r\n"); idx >= 0 {
			statusLine = headerBytesStr[:idx]
		} else if idx := strings.Index(headerBytesStr, "\n"); idx >= 0 {
			statusLine = headerBytesStr[:idx]
		} else {
			statusLine = headerBytesStr
		}

		var newHeaderLines []string
		hasContentLength := false
		lineBreak := detectLineBreakFromHeaders(headerBytes)

		// Start with status line
		newHeaderLines = append(newHeaderLines, statusLine)

		// Process header lines
		for _, line := range headerLines {
			lineLower := strings.ToLower(line)
			// Skip Transfer-Encoding header (we decoded chunked encoding)
			if strings.HasPrefix(lineLower, "transfer-encoding:") {
				continue
			}
			// Check if Content-Length exists
			if strings.HasPrefix(lineLower, "content-length:") {
				hasContentLength = true
			}
			newHeaderLines = append(newHeaderLines, line)
		}

		// Add Content-Length if it didn't exist
		if !hasContentLength && len(bodyBytes) > 0 {
			newHeaderLines = append(newHeaderLines, fmt.Sprintf("Content-Length: %d", len(bodyBytes)))
		}

		// Rebuild response with new headers and decoded body
		newHeaders := strings.Join(newHeaderLines, lineBreak) + lineBreak + lineBreak
		responseBytes = []byte(newHeaders)
		responseBytes = append(responseBytes, bodyBytes...)
	}

	// Try to parse status code (optional, for convenience)
	statusCode, status := parseStatusLine(responseBytes)

	return &Response{
		RawBytes:     responseBytes,
		StatusCode:   statusCode,
		Status:       status,
		ResponseTime: responseTime,
	}, nil
}

// SendString is a convenience method that sends a raw HTTP request from a string
func (c *Client) SendString(rawRequest string, host string, port string, useTLS bool) (*Response, error) {
	req := Request{
		RawBytes: []byte(rawRequest),
		Host:     host,
		Port:     port,
		UseTLS:   useTLS,
		Timeout:  c.config.Timeout,
	}
	return c.Send(req)
}

// SendBytes is a convenience method that sends raw HTTP request bytes
func (c *Client) SendBytes(rawRequest []byte, host string, port string, useTLS bool) (*Response, error) {
	req := Request{
		RawBytes: rawRequest,
		Host:     host,
		Port:     port,
		UseTLS:   useTLS,
		Timeout:  c.config.Timeout,
	}
	return c.Send(req)
}

// parseStatusLine attempts to parse the HTTP status line from raw response bytes.
// This is a minimal parser that only extracts the status code if possible.
// Returns 0 and empty string if parsing fails (malformed response).
func parseStatusLine(responseBytes []byte) (int, string) {
	if len(responseBytes) == 0 {
		return 0, ""
	}

	// Find first line (status line) - look for \r\n or \n
	firstLineEnd := len(responseBytes)
	for i, b := range responseBytes {
		if b == '\r' || b == '\n' {
			firstLineEnd = i
			break
		}
	}

	firstLine := string(responseBytes[:firstLineEnd])

	// Try to find status code (3 digits after HTTP version or at start)
	// Very minimal parsing - just look for pattern like "200" or "HTTP/1.1 200"
	for i := 0; i <= len(firstLine)-3; i++ {
		if firstLine[i] >= '1' && firstLine[i] <= '5' &&
			firstLine[i+1] >= '0' && firstLine[i+1] <= '9' &&
			firstLine[i+2] >= '0' && firstLine[i+2] <= '9' {
			// Found potential status code
			var code int
			fmt.Sscanf(firstLine[i:i+3], "%d", &code)
			if code >= 100 && code <= 599 {
				return code, firstLine
			}
		}
	}

	return 0, firstLine
}

// decompressBodyByEncoding decompresses the body based on Content-Encoding header
func decompressBodyByEncoding(bodyBytes []byte, contentEncoding string) ([]byte, error) {
	if len(bodyBytes) == 0 {
		return bodyBytes, nil
	}

	var reader io.Reader
	var err error

	switch strings.TrimSpace(strings.ToLower(contentEncoding)) {
	case "gzip", "x-gzip":
		gzReader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			return bodyBytes, err
		}
		defer gzReader.Close()
		reader = gzReader
	case "br", "brotli":
		reader = brotli.NewReader(bytes.NewReader(bodyBytes))
	case "deflate":
		zlibReader, err := zlib.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			return bodyBytes, err
		}
		defer zlibReader.Close()
		reader = zlibReader
	default:
		// Unknown encoding, return original
		return bodyBytes, nil
	}

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return bodyBytes, err
	}

	return decompressed, nil
}

// decodeChunkedBody decodes a chunked HTTP body according to RFC 7230
// Chunked format: chunk-size\r\nchunk-data\r\n...0\r\n\r\n
func decodeChunkedBody(chunkedBody []byte) ([]byte, error) {
	if len(chunkedBody) == 0 {
		return chunkedBody, nil
	}

	var decoded []byte
	pos := 0
	lineBreak := "\r\n"

	// Detect line break style
	if !bytes.Contains(chunkedBody, []byte("\r\n")) {
		lineBreak = "\n"
	}

	for pos < len(chunkedBody) {
		// Find the end of the chunk size line
		var lineEnd int
		if lineBreak == "\r\n" {
			lineEnd = bytes.Index(chunkedBody[pos:], []byte("\r\n"))
		} else {
			lineEnd = bytes.Index(chunkedBody[pos:], []byte("\n"))
		}

		if lineEnd == -1 {
			// Malformed chunked body
			break
		}

		lineEnd += pos

		// Parse chunk size (hex number, may have chunk extensions after semicolon)
		chunkSizeLine := string(chunkedBody[pos:lineEnd])
		chunkSizeStr := chunkSizeLine
		if semicolonIdx := strings.IndexByte(chunkSizeStr, ';'); semicolonIdx >= 0 {
			chunkSizeStr = chunkSizeStr[:semicolonIdx]
		}
		chunkSizeStr = strings.TrimSpace(chunkSizeStr)

		// Parse hex chunk size
		chunkSize, err := strconv.ParseInt(chunkSizeStr, 16, 64)
		if err != nil {
			// Malformed chunk size
			break
		}

		// Move past the chunk size line
		pos = lineEnd + len(lineBreak)

		// If chunk size is 0, we've reached the end
		if chunkSize == 0 {
			// Should be followed by \r\n\r\n or \n\n
			break
		}

		// Read chunk data
		if pos+int(chunkSize) > len(chunkedBody) {
			// Not enough data, return what we have
			break
		}

		chunkData := chunkedBody[pos : pos+int(chunkSize)]
		decoded = append(decoded, chunkData...)

		// Move past chunk data
		pos += int(chunkSize)

		// Skip the trailing \r\n after chunk data
		if pos < len(chunkedBody) {
			if lineBreak == "\r\n" && pos+2 <= len(chunkedBody) &&
				chunkedBody[pos] == '\r' && chunkedBody[pos+1] == '\n' {
				pos += 2
			} else if lineBreak == "\n" && pos < len(chunkedBody) && chunkedBody[pos] == '\n' {
				pos++
			}
		}
	}

	return decoded, nil
}

// splitHeaderLines splits header text into individual header lines
func splitHeaderLines(headerText string) []string {
	var lines []string
	if strings.Contains(headerText, "\r\n") {
		lines = strings.Split(headerText, "\r\n")
	} else {
		lines = strings.Split(headerText, "\n")
	}
	// Filter out empty lines and status line (first line)
	var headerLines []string
	for i, line := range lines {
		if i == 0 {
			// Skip status line
			continue
		}
		if strings.TrimSpace(line) != "" {
			headerLines = append(headerLines, line)
		}
	}
	return headerLines
}

// detectLineBreakFromHeaders detects the line break style from headers
// It finds the first \n and checks if the previous byte is \r
func detectLineBreakFromHeaders(headerBytes []byte) string {
	for i := 0; i < len(headerBytes); i++ {
		if headerBytes[i] == '\n' {
			// Check previous byte if it exists
			if i > 0 && headerBytes[i-1] == '\r' {
				return "\r\n"
			}
			return "\n"
		}
	}
	// Default to \n if no line break found
	return "\n"
}

// SendFile is a convenience method that reads a request from a file and sends it
func (c *Client) SendFile(filepath string, host string, port string, useTLS bool) (*Response, error) {
	rawBytes, err := ReadFromFile(filepath)
	if err != nil {
		return nil, err
	}

	req := Request{
		RawBytes: rawBytes,
		Host:     host,
		Port:     port,
		UseTLS:   useTLS,
		Timeout:  c.config.Timeout,
	}

	return c.Send(req)
}
