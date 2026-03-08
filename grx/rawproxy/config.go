package rawproxy

import (
	"net/http"
	"sync/atomic"
	"time"
)

// RequestData holds data that can be passed from request handler to response handler
type RequestData struct {
	RequestID string      // Unique request ID
	HttpProto string      // Actual upstream protocol used (e.g. "HTTP/1.1", "HTTP/2.0")
	Data      interface{} // Custom data (e.g., UserData, metadata, etc.)
}

type OnRequestHandler func(reqData *RequestData, req *http.Request) (*http.Request, error)
type OnResponseHandler func(reqData *RequestData, resp *http.Response, req *http.Request) (*http.Response, error)

// WebSocketMessage contains all context for a captured WebSocket message
type WebSocketMessage struct {
	RequestID string    // Proxy request ID (e.g., req-00000001)
	Index     int       // Message sequence number within connection
	Host      string    // WebSocket server host
	Path      string    // WebSocket endpoint path
	URL       string    // Full WebSocket URL
	Direction string    // "send" (client→server) or "recv" (server→client)
	Type      string    // Frame type: text, binary, close, ping, pong
	IsBinary  bool      // Quick check for binary content
	Payload   []byte    // Message content
	Timestamp time.Time // When captured
}

type OnWebSocketMessageHandler func(msg *WebSocketMessage) error

// Config holds the proxy configuration
type Config struct {
	// Certificate folder (optional - only for cert files)
	ConfigFolder string // Folder for certificate files only (ca.crt, ca.key)

	// Server settings (optional - defaults provided)
	ListenAddr   string        // Address to listen on (default: ":8080")
	ReadTimeout  time.Duration // HTTP read timeout (default: 10m, high to support intercept)
	WriteTimeout time.Duration // HTTP write timeout (default: 10m, high to support intercept)
	IdleTimeout  time.Duration // HTTP idle timeout (default: 10m, high to support intercept)

	// Output settings (optional - defaults provided)
	OutputDir    string // Directory for HTTP/HTTPS captures (default: "captures")
	WebSocketDir string // Directory for WebSocket captures (default: "<OutputDir>/websockets")

	// MITM settings (optional - if nil, HTTPS will be tunneled without inspection)
	MITM     *MitmCA // MITM CA certificate
	CertPath string  // Path to CA certificate (default: "<ConfigFolder>/ca.crt" or "cert/ca.crt")
	KeyPath  string  // Path to CA key (default: "<ConfigFolder>/ca.key" or "cert/ca.key")

	// Handlers (optional)
	OnRequestHandler          OnRequestHandler          // Custom request handler
	OnResponseHandler         OnResponseHandler         // Custom response handler
	OnWebSocketMessageHandler OnWebSocketMessageHandler // Custom websocket message handler

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
