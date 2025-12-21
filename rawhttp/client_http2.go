package rawhttp

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

// SendHTTP2 sends a raw HTTP request using HTTP/2 protocol
func (c *Client) SendHTTP2(req Request) (*Response, error) {
	// HTTP/2 requires TLS
	if !req.UseTLS {
		return nil, fmt.Errorf("HTTP/2 requires TLS (UseTLS must be true)")
	}

	// Determine port
	port := req.Port
	if port == "" {
		port = "443"
	}

	// Parse the raw HTTP/1.x request to extract components
	parsedReq := ParseRequest(req.RawBytes)
	if parsedReq.Method == "" {
		return nil, fmt.Errorf("failed to parse request method")
	}

	// Build the request URL
	url := parsedReq.URL
	if url == "" {
		url = "/"
	}
	// Ensure URL starts with /
	if url[0] != '/' {
		url = "/" + url
	}
	fullURL := fmt.Sprintf("https://%s:%s%s", req.Host, port, url)

	// Create TLS config with ALPN for HTTP/2
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.config.InsecureSkipVerify,
		MinVersion:         c.config.TLSMinVersion,
		ServerName:         req.Host,
		NextProtos:         []string{"h2"}, // HTTP/2 over TLS
	}

	// Create HTTP/2 transport
	transport := &http2.Transport{
		TLSClientConfig: tlsConfig,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout: c.config.Timeout,
			}
			return tls.DialWithDialer(dialer, network, addr, cfg)
		},
	}

	// Create HTTP request
	var bodyReader io.Reader
	if len(parsedReq.Body) > 0 {
		bodyReader = bytes.NewReader([]byte(parsedReq.Body))
	}

	httpReq, err := http.NewRequest(parsedReq.Method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers from parsed request
	// Headers format is [][]string where each entry is [key, value]
	for _, header := range parsedReq.Headers {
		if len(header) < 2 {
			continue
		}
		key := strings.TrimSuffix(header[0], ":") // Remove trailing colon from key
		value := strings.TrimSpace(header[1])

		// lowerKey := strings.ToLower(key)

		// Note: HTTP/2 spec forbids certain headers, but we allow them for testing
		// Uncomment the block below to filter forbidden headers automatically:
		/*
			// Skip headers that are forbidden in HTTP/2 or handled automatically
			if lowerKey == "host" || lowerKey == "connection" ||
				lowerKey == "transfer-encoding" || lowerKey == "upgrade" ||
				lowerKey == "keep-alive" || lowerKey == "proxy-connection" {
				continue
			}
		*/

		// For security testing, we send all headers and let the server/library decide
		httpReq.Header.Add(key, value)
	}

	// Set Host header explicitly
	httpReq.Host = req.Host

	// Send request and measure time
	requestStartTime := time.Now()
	httpResp, err := transport.RoundTrip(httpReq)
	responseTime := time.Since(requestStartTime)

	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP/2 request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Convert to raw HTTP/1.x format for consistency with the rest of the codebase
	return convertHTTP2ToRaw(httpResp, respBody, responseTime), nil
}

// convertHTTP2ToRaw converts an HTTP/2 response to raw HTTP/1.x format
func convertHTTP2ToRaw(resp *http.Response, body []byte, responseTime time.Duration) *Response {
	var rawResponse bytes.Buffer

	// Build status line (resp.Status already contains "200 OK", so just prepend HTTP/2.0)
	statusLine := fmt.Sprintf("HTTP/2.0 %s", resp.Status)
	rawResponse.WriteString(statusLine)
	rawResponse.WriteString("\r\n")

	// Write headers
	for key, values := range resp.Header {
		for _, value := range values {
			rawResponse.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	// Add Content-Length header if not present and body exists
	if resp.Header.Get("Content-Length") == "" && len(body) > 0 {
		rawResponse.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(body)))
	}

	// End of headers
	rawResponse.WriteString("\r\n")

	// Write body
	if len(body) > 0 {
		rawResponse.Write(body)
	}

	return &Response{
		RawBytes:     rawResponse.Bytes(),
		StatusCode:   resp.StatusCode,
		Status:       statusLine,
		ResponseTime: responseTime,
	}
}
