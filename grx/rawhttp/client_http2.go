package rawhttp

import (
	"bytes"
	"context"
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

	// Create HTTP/2 transport with appropriate TLS dialer
	var transport *http2.Transport

	if c.config.UseBrowserFingerprint {
		// Use uTLS to mimic browser TLS fingerprint (bypasses Cloudflare)
		transport = &http2.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return c.dialUTLSForHTTP2(ctx, network, addr, req.Host)
			},
		}
	} else {
		// Create TLS config with ALPN for HTTP/2
		tlsConfig := &tls.Config{
			InsecureSkipVerify: c.config.InsecureSkipVerify,
			MinVersion:         c.config.TLSMinVersion,
			ServerName:         req.Host,
			NextProtos:         []string{"h2"}, // HTTP/2 over TLS
		}

		transport = &http2.Transport{
			TLSClientConfig: tlsConfig,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				dialer := &net.Dialer{
					Timeout: c.config.Timeout,
				}
				return tls.DialWithDialer(dialer, network, addr, cfg)
			},
		}
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

		lowerKey := strings.ToLower(key)

		// Skip headers that are forbidden in HTTP/2 or handled automatically
		// These headers cause INTERNAL_ERROR or PROTOCOL_ERROR from HTTP/2 servers
		if lowerKey == "connection" ||
			lowerKey == "transfer-encoding" ||
			lowerKey == "upgrade" ||
			lowerKey == "keep-alive" ||
			lowerKey == "proxy-connection" ||
			lowerKey == "http2-settings" {
			continue
		}

		// Host header is handled separately via httpReq.Host
		if lowerKey == "host" {
			continue
		}

		// Skip TE header unless its value is "trailers"
		if lowerKey == "te" && !strings.EqualFold(value, "trailers") {
			continue
		}

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

	// Decompress the body if Content-Encoding is present
	contentEncoding := httpResp.Header.Get("Content-Encoding")
	if contentEncoding != "" {
		decompressedBody, err := decompressBodyByEncoding(respBody, contentEncoding)
		if err == nil && len(decompressedBody) > 0 {
			respBody = decompressedBody
			// Remove Content-Encoding header since we've decoded it
			// httpResp.Header.Del("Content-Encoding")
		}
		// If decompression fails, we'll just use the original body
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

	// Write headers (but update Content-Length to match actual body size)
	for key, values := range resp.Header {
		// Skip Content-Length, we'll add it after with the correct value
		if strings.ToLower(key) == "content-length" {
			continue
		}
		for _, value := range values {
			rawResponse.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	// Add Content-Length header with actual body size (after any decompression)
	if len(body) > 0 {
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
