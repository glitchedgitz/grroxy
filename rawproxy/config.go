package rawproxy

import (
	"net/http"
	"sync/atomic"
	"time"
)

// Handler function types for request and response processing
type OnRequestHandler func(requestID string, req *http.Request) (*http.Request, error)
type OnResponseHandler func(requestID string, resp *http.Response, req *http.Request) (*http.Response, error)

// Config holds the proxy configuration
type Config struct {
	// Certificate folder (optional - only for cert files)
	ConfigFolder string // Folder for certificate files only (ca.crt, ca.key)

	// Server settings (optional - defaults provided)
	ListenAddr   string        // Address to listen on (default: ":8080")
	ReadTimeout  time.Duration // HTTP read timeout (default: 30s)
	WriteTimeout time.Duration // HTTP write timeout (default: 60s)
	IdleTimeout  time.Duration // HTTP idle timeout (default: 60s)

	// Output settings (optional - defaults provided)
	OutputDir    string // Directory for HTTP/HTTPS captures (default: "captures")
	WebSocketDir string // Directory for WebSocket captures (default: "<OutputDir>/websockets")

	// MITM settings (optional - if nil, HTTPS will be tunneled without inspection)
	MITM     *MitmCA // MITM CA certificate
	CertPath string  // Path to CA certificate (default: "<ConfigFolder>/ca.crt" or "cert/ca.crt")
	KeyPath  string  // Path to CA key (default: "<ConfigFolder>/ca.key" or "cert/ca.key")

	// Handlers (optional)
	OnRequestHandler  OnRequestHandler  // Custom request handler
	OnResponseHandler OnResponseHandler // Custom response handler

	// Internal (optional - will be created if nil)
	ReqCounter *atomic.Uint64 // Request counter for unique IDs
}

// Request ID counter for correlating requests and responses
var requestIDCounter atomic.Uint64

// generateRequestID creates a unique ID for each request
func generateRequestID() string {
	id := requestIDCounter.Add(1)
	// Format as req-00000001, req-00000002, etc.
	if id < 100000000 {
		digits := make([]byte, 8)
		tempID := id
		for i := 7; i >= 0; i-- {
			digits[i] = byte('0' + (tempID % 10))
			tempID /= 10
		}
		return "req-" + string(digits)
	}
	return "req-99999999"
}
