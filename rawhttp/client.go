package rawhttp

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Client is a raw HTTP client that sends requests with minimal validation
type Client struct {
	config Config
}

// NewClient creates a new raw HTTP client with the given configuration
func NewClient(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.TLSMinVersion == 0 {
		// Default to TLS 1.0 for maximum compatibility with older servers
		config.TLSMinVersion = tls.VersionTLS10
	}
	return &Client{config: config}
}

// DefaultClient returns a client with sensible defaults
func DefaultClient() *Client {
	return NewClient(Config{
		Timeout:            30 * time.Second,
		InsecureSkipVerify: true,
		TLSMinVersion:      tls.VersionTLS10,
	})
}

// Send sends a raw HTTP request and returns the raw response.
// This function performs minimal validation - only what's necessary for TCP/TLS connection.
// All malformed headers, formatting issues, and protocol violations are preserved and sent as-is.
func (c *Client) Send(req Request) (*Response, error) {
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
		// Use TLS with minimal validation
		dialer := &net.Dialer{
			Timeout: c.config.Timeout,
		}
		tlsConfig := &tls.Config{
			InsecureSkipVerify: c.config.InsecureSkipVerify,
			MinVersion:         c.config.TLSMinVersion,
			ServerName:         req.Host, // Optional, may be empty for malformed requests
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
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

	// Parse headers to find Content-Length - optimized parsing
	contentLength := -1
	chunked := false

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
