package rawhttp

import (
	"time"
)

// Request represents a raw HTTP request to be sent
type Request struct {
	// RawBytes contains the complete raw HTTP request as bytes
	// This preserves all malformations, whitespace, and formatting
	RawBytes []byte

	// Host is the target hostname (e.g., "example.com")
	Host string

	// Port is the target port (e.g., "80", "443")
	// If empty, defaults to 80 for HTTP and 443 for HTTPS
	Port string

	// UseTLS determines whether to use TLS/HTTPS
	UseTLS bool

	// UseHTTP2 determines whether to use HTTP/2 protocol
	// If true, the request will be sent using HTTP/2
	// Note: HTTP/2 requires TLS (UseTLS must also be true)
	UseHTTP2 bool

	// Timeout specifies the connection and read timeout
	Timeout time.Duration
}

// Response represents the raw HTTP response received
type Response struct {
	// RawBytes contains the complete raw HTTP response as bytes
	RawBytes []byte

	// StatusCode is the HTTP status code (if parseable)
	StatusCode int

	// Status is the status line (if parseable)
	Status string

	// ResponseTime is the time taken to receive the response headers (not including body)
	ResponseTime time.Duration
}

// Config holds configuration for the raw HTTP client
type Config struct {
	// Timeout for connection and read operations
	Timeout time.Duration

	// InsecureSkipVerify skips TLS certificate verification
	InsecureSkipVerify bool

	// TLSMinVersion sets the minimum TLS version (default: TLS 1.0 for compatibility)
	TLSMinVersion uint16

	// UseBrowserFingerprint enables uTLS to mimic browser TLS fingerprint
	// This helps bypass Cloudflare and other CDN bot detection that use JA3/JA4 fingerprinting
	UseBrowserFingerprint bool

	// BrowserFingerprint specifies which browser to mimic (default: Chrome)
	// Options: "chrome", "firefox", "safari", "edge", "random"
	BrowserFingerprint string
}
