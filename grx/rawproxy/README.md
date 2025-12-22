# proxy

An intercepting forward proxy in Go that saves raw HTTP requests and responses to files, with support for WebSocket connections.

## Build

```bash
go build -o proxy
```

## Run

**Simple usage:**

```bash
./proxy
# Uses defaults: :8080, captures/, cert/ca.crt, cert/ca.key
```

**With custom certificate folder:**

```bash
./proxy --config ~/.proxy-certs
# Certificates will be at ~/.proxy-certs/ca.crt and ~/.proxy-certs/ca.key
# Captures still go to ./captures/
```

**Full customization:**

```bash
./proxy --listen :8080 --out ./captures --config ./mycerts
# Or specify certificate paths directly
./proxy --cert ./path/ca.crt --key ./path/ca.key --out ./output
```

**Available flags:**

- `--config` - Folder for certificate files (ca.crt, ca.key)
- `--listen` - Address to listen on (default: `:8080`)
- `--out` - Output directory for captures (default: `captures`)
- `--wsout` - WebSocket captures directory (default: `<out>/websockets`)
- `--cert` - Path to CA certificate (default: `<config>/ca.crt` or `cert/ca.crt`)
- `--key` - Path to CA key (default: `<config>/ca.key` or `cert/ca.key`)
- `--log` - Log file path (default: `<out>/proxy.log`)

Set your client (curl, browser, code) to use `http://127.0.0.1:8080` as an HTTP/HTTPS proxy.

Examples:

```bash
# HTTP request via proxy
http_proxy=http://127.0.0.1:8080 curl -i http://example.com/

# HTTPS request via proxy
https_proxy=http://127.0.0.1:8080 curl -i https://example.com/

# WebSocket connections are automatically detected and proxied
# Supports both ws:// and wss:// protocols
```

## Using as a Library

You can use this proxy in your own Go projects. The core functionality is available in the `rawproxy` package.

### Installation

```bash
go get github.com/glitchedgitz/proxy/rawproxy
```

### Basic Example

See `main.go` for a complete working example. Here's a minimal setup:

```go
package main

import (
    "log"
    "net/http"
    "path/filepath"

    "github.com/glitchedgitz/proxy/rawproxy"
)

func main() {
    // Load or generate CA for MITM
    caDir := "cert"
    certPath := filepath.Join(caDir, "ca.crt")
    keyPath := filepath.Join(caDir, "ca.key")

    var mitm *rawproxy.MitmCA
    var err error
    if rawproxy.FileExists(certPath) && rawproxy.FileExists(keyPath) {
        mitm, err = rawproxy.LoadMITMCA(certPath, keyPath)
    } else {
        mitm, _, _, err = rawproxy.GenerateMITMCA(caDir)
    }
    if err != nil {
        log.Fatal(err)
    }

    // Create proxy with configuration
    proxy, err := rawproxy.New(&rawproxy.Config{
        ListenAddr: ":8080",
        OutputDir:  "captures",
        MITM:       mitm,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Optional: Add custom request handler
    proxy.SetRequestHandler(func(requestID string, req *http.Request) (*http.Request, error) {
        log.Printf("[REQUEST] %s %s", requestID, req.URL.String())
        return req, nil
    })

    // Optional: Add custom response handler
    proxy.SetResponseHandler(func(requestID string, resp *http.Response, req *http.Request) (*http.Response, error) {
        log.Printf("[RESPONSE] %s %d", requestID, resp.StatusCode)
        return resp, nil
    })

    // Start proxy (blocking)
    log.Println("Starting proxy on :8080")
    if err := proxy.Start(); err != nil {
        log.Fatal(err)
    }
}
```

**Even simpler** - with ConfigFolder (certificates only):

```go
package main

import (
    "log"
    "github.com/glitchedgitz/proxy/rawproxy"
)

func main() {
    // ConfigFolder for certificates, captures/ for output
    proxy, err := rawproxy.New(&rawproxy.Config{
        ConfigFolder: ".mycerts",  // Certificates: .mycerts/ca.crt, .mycerts/ca.key
        // Captures still go to ./captures/ by default
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Fatal(proxy.Start())
}
```

**Minimal** - use all defaults:

```go
package main

import (
    "log"
    "github.com/glitchedgitz/proxy/rawproxy"
)

func main() {
    // Uses: :8080, cert/ca.crt, cert/ca.key, captures/
    proxy, _ := rawproxy.New(&rawproxy.Config{})
    log.Fatal(proxy.Start())
}
```

### Library API

#### Proxy Instance

```go
// Create a new proxy instance
func New(config *Config) (*Proxy, error)

// Start the proxy server (blocking)
func (p *Proxy) Start() error

// Stop gracefully shuts down the proxy server
func (p *Proxy) Stop(ctx context.Context) error

// Set request handler
func (p *Proxy) SetRequestHandler(handler OnRequestHandler)

// Set response handler
func (p *Proxy) SetResponseHandler(handler OnResponseHandler)

// Get current configuration
func (p *Proxy) GetConfig() *Config
```

#### Config Structure

```go
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
    CertPath string  // Path to CA cert (default: "<ConfigFolder>/ca.crt" or "cert/ca.crt")
    KeyPath  string  // Path to CA key (default: "<ConfigFolder>/ca.key" or "cert/ca.key")

    // Handlers (optional)
    OnRequestHandler  OnRequestHandler  // Custom request handler
    OnResponseHandler OnResponseHandler // Custom response handler

    // Internal (optional - will be created if nil)
    ReqCounter *atomic.Uint64 // Request counter for unique IDs
}
```

#### Handler Functions

```go
type OnRequestHandler func(requestID string, req *http.Request) (*http.Request, error)
type OnResponseHandler func(requestID string, resp *http.Response, req *http.Request) (*http.Response, error)
```

- Return modified request/response to continue processing
- Return `nil, error` to block the request/response
- Each request gets a unique `requestID` for correlation

#### CA Management Functions

```go
// Generate new CA certificate
func GenerateMITMCA(dir string) (*MitmCA, string, string, error)

// Load existing CA certificate
func LoadMITMCA(certPath, keyPath string) (*MitmCA, error)

// Check if file exists
func FileExists(path string) bool
```

### Features Available in Library Mode

- ✅ HTTP/HTTPS proxying with MITM support
- ✅ WebSocket connection handling (ws:// and wss://)
- ✅ Request/response capture to files
- ✅ Custom request/response handlers for modification
- ✅ Automatic HTTP/2 and HTTP/1.1 negotiation
- ✅ Unique request ID tracking
- ✅ Async capture (non-blocking writes)
- ✅ CA certificate management

## Output format

Each request/response pair is written to a single file in the output directory.
WebSocket connections are saved to a separate directory (default: `<out>/websockets/`) for better organization.
The layout is:

```
[RAW REQUEST ]
-------------------|-------------------
[RAW RESPONSE]
```

File names include timestamp, a sequence number, method, and host, e.g.:

```
20250101-120001.123456789-000001-GET-example.com_443.txt
```

Notes:

- Hop-by-hop proxy headers are stripped before forwarding (except for WebSocket upgrade requests).
- WebSocket connections are captured as upgrade request/response pairs, then tunneled transparently.

## HTTPS capture (MITM)

The proxy uses a local CA under `<certpath>/cert/` for HTTPS capture. On first run, it auto-generates `ca.crt` and `ca.key`; on subsequent runs, it reuses them.

Install `<certpath>/cert/ca.crt` into your client/system trust store so HTTPS works via the proxy.

Example with curl using the proxy:

```bash
https_proxy=http://127.0.0.1:8080 curl -i https://example.com/
```

### Trusting the CA

macOS (System Keychain):

```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ./cert/ca.crt
```

Ubuntu/Debian:

```bash
sudo cp ./cert/ca.crt /usr/local/share/ca-certificates/proxy-ca.crt
sudo update-ca-certificates
```

Fedora/RHEL (p11-kit):

```bash
sudo cp ./cert/ca.crt /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust
```

Firefox:

- Firefox uses its own trust store by default. Import `cert/ca.crt` in Preferences > Privacy & Security > Certificates > View Certificates > Authorities > Import.

### Usage examples

- Run with auto-generated CA (default):

```bash
./proxy --listen :8080 --out ./captures
```

- Run with your own CA:

```bash
./proxy --listen :8080 --out ./captures \
  --mitm-ca ./ca.crt --mitm-key ./ca.key
```

- Test HTTP/HTTPS through proxy with curl:

```bash
http_proxy=http://127.0.0.1:8080 curl -i http://example.com/
https_proxy=http://127.0.0.1:8080 curl -i https://example.com/
```

### Capture files

- Saved under your `--out` directory (default `captures/`).
- One file per request, format:

```
[RAW REQUEST ]
-------------------|-------------------
[RAW RESPONSE]
```

- File naming includes timestamp, sequence number, method, and host.

## WebSocket Support

The proxy automatically detects and handles WebSocket connections:

### Features

- **Automatic Detection**: WebSocket upgrade requests are automatically detected by checking for `Upgrade: websocket` and `Connection: Upgrade` headers
- **Protocol Support**: Supports both `ws://` (WebSocket) and `wss://` (WebSocket Secure) protocols
- **Real-time Frame Capture**: Parses and captures individual WebSocket messages as they flow through the proxy
- **Message Type Support**: Handles text, binary, and control frames (close, ping, pong)
- **Individual Message Files**: Each WebSocket message is saved to a separate file with metadata
- **Request/Response Capture**: The initial WebSocket upgrade request and response are captured like regular HTTP traffic
- **Handler Support**: WebSocket upgrade requests can be processed by custom request/response handlers
- **Bidirectional Capture**: Captures messages in both directions (client-to-server and server-to-client)
- **Frame Parsing**: Properly handles WebSocket frame format including masking and fragmentation
- **Logging**: Comprehensive logging of WebSocket connection establishment, message flow, and connection termination

### How it Works

1. **Detection**: The proxy detects WebSocket upgrade requests by examining HTTP headers
2. **Upgrade Handling**: The initial HTTP upgrade request is forwarded to the target server
3. **Connection Hijacking**: Both client and server connections are hijacked for direct frame forwarding
4. **Frame Parsing**: WebSocket frames are parsed in real-time as data flows through the proxy
5. **Message Capture**: Individual messages are extracted and saved with metadata (direction, type, timestamp)
6. **Bidirectional Tunneling**: WebSocket frames are copied bidirectionally between client and server
7. **Capture**: Both the upgrade handshake and individual messages are captured to separate files

### WebSocket Logging

WebSocket connections generate detailed logs with unique request IDs:

```
[WEBSOCKET] requestID=req-00000123 WebSocket upgrade request to ws://example.com/ws
[WEBSOCKET] requestID=req-00000123 Established WebSocket tunnel to ws://example.com/ws
[WS-MSG] requestID=req-00000123 client-to-server text message (13 bytes) saved to 20240115-103045.123456789-ws-req-00000123-client-to-server-text.txt
[WS-MSG] requestID=req-00000123 server-to-client text message (21 bytes) saved to 20240115-103045.234567890-ws-req-00000123-server-to-client-text.txt
[WS-FRAME] requestID=req-00000123 client-to-server ping frame (0 bytes)
[WS-FRAME] requestID=req-00000123 server-to-client pong frame (0 bytes)
[WEBSOCKET] requestID=req-00000123 Client->Server copy ended normally
[WEBSOCKET] requestID=req-00000123 Server->Client copy ended normally
[WEBSOCKET] requestID=req-00000123 WebSocket tunnel closed
```

### Example Usage

```bash
# Start the proxy
./proxy --listen :8080 --out ./captures

# Start with separate WebSocket directory
./proxy --listen :8080 --out ./captures --wsout ./websocket_captures

# WebSocket connections through the proxy work automatically
# Configure your WebSocket client to use http://127.0.0.1:8080 as HTTP proxy
```

The WebSocket upgrade handshake will be captured in the output files, showing the initial HTTP request and the `101 Switching Protocols` response.

### WebSocket Message File Format

Individual WebSocket messages are saved with detailed metadata:

**File naming**: `TIMESTAMP-ws-REQUESTID-DIRECTION-FRAMETYPE.txt`

**Example text message file** (`20240115-103045.123456789-ws-req-00000123-client-to-server-text.txt`):

```
WebSocket Message
================
RequestID: req-00000123
Direction: client-to-server
Frame Type: text
Timestamp: 2024-01-15T10:30:45.123456789Z
Payload Length: 13 bytes

--- Message Content ---
Hello Server!
```

**Example binary message file** (`20240115-103046.234567890-ws-req-00000123-server-to-client-binary.txt`):

```
WebSocket Message
================
RequestID: req-00000123
Direction: server-to-client
Frame Type: binary
Timestamp: 2024-01-15T10:30:46.234567890Z
Payload Length: 8 bytes

--- Message Content ---
Binary data (hex):
00000000: 01 02 03 04 05 06 07 08
```

**Supported frame types**:

- `text` - UTF-8 text messages
- `binary` - Binary data (saved as hex dump)
- `close` - Connection close frames with reason codes
- `ping` / `pong` - Control frames (logged but not saved to files)
- `continuation` - Fragmented message parts

## Request and Response Handlers

The proxy supports custom handlers for processing requests and responses. This allows you to:

- Log and monitor traffic
- Block or filter requests
- Modify request/response headers
- Filter response content
- Implement custom authentication
- Add security headers

### Handler Functions

Two types of handlers are available:

```go
type OnRequestHandler func(requestID string, req *http.Request) (*http.Request, error)
type OnResponseHandler func(requestID string, resp *http.Response, req *http.Request) (*http.Response, error)
```

Each handler receives a unique `requestID` that allows you to correlate requests with their corresponding responses.

### Default Handlers

The proxy includes default handlers that:

- Log all requests and responses with request ID, method, URL, and status codes
- Add `X-Proxy-Processed` and `X-Request-ID` headers to requests
- Add `X-Proxy-Response-Processed` and `X-Request-ID` headers to responses
- Log content types for HTML and JSON responses with request ID correlation

### Custom Handlers

You can set custom handlers using:

```go
SetOnRequestHandler(func(requestID string, req *http.Request) (*http.Request, error) {
    // Your request processing logic
    log.Printf("Processing request %s: %s", requestID, req.URL.String())
    return req, nil
})

SetOnResponseHandler(func(requestID string, resp *http.Response, req *http.Request) (*http.Response, error) {
    // Your response processing logic
    log.Printf("Processing response %s: %d", requestID, resp.StatusCode)
    return resp, nil
})
```

### Example Use Cases

1. **Request Blocking:**

```go
SetOnRequestHandler(func(requestID string, req *http.Request) (*http.Request, error) {
    if strings.Contains(req.URL.Host, "blocked-domain.com") {
        log.Printf("Blocking request %s to %s", requestID, req.URL.Host)
        return nil, fmt.Errorf("domain blocked")
    }
    return req, nil
})
```

2. **Adding Authentication:**

```go
SetOnRequestHandler(func(requestID string, req *http.Request) (*http.Request, error) {
    log.Printf("Adding auth to request %s", requestID)
    req.Header.Set("Authorization", "Bearer your-token")
    return req, nil
})
```

3. **Security Headers:**

```go
SetOnResponseHandler(func(requestID string, resp *http.Response, req *http.Request) (*http.Response, error) {
    log.Printf("Adding security headers to response %s", requestID)
    resp.Header.Set("X-Frame-Options", "DENY")
    resp.Header.Set("X-Content-Type-Options", "nosniff")
    resp.Header.Set("X-Request-ID", requestID)  // Include request ID in response
    return resp, nil
})
```

4. **Content Filtering:**

```go
SetOnResponseHandler(func(requestID string, resp *http.Response, req *http.Request) (*http.Response, error) {
    if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
        log.Printf("Filtering HTML content for request %s", requestID)
        // Read and modify HTML content
        body, _ := io.ReadAll(resp.Body)
        modifiedBody := strings.ReplaceAll(string(body), "unwanted", "filtered")
        resp.Body = io.NopCloser(strings.NewReader(modifiedBody))
        resp.ContentLength = int64(len(modifiedBody))
    }
    return resp, nil
})
```

See `examples.go` for more detailed examples of handler implementations.

### Handler Behavior

- Handlers are called for both regular HTTP proxy requests and MITM HTTPS requests
- Each request gets a unique request ID (format: `req-########`) for correlation
- For HTTPS MITM, each HTTP request within the connection gets a sub-request ID (format: `req-########-sub-#`)
- If a handler returns an error, the request/response will be rejected with an appropriate HTTP error code
- If a handler returns `nil` instead of a request/response, the operation will be aborted
- Handlers can modify headers, body content, and other request/response properties
- All modifications are captured in the output files along with the original traffic
- Request IDs are included in all log messages for easy debugging and correlation

### Request ID Usage

Request IDs allow you to:

- Correlate requests with their responses in logs
- Track request flow through multiple handlers
- Debug specific requests by their ID
- Add request tracing headers to responses
- Implement per-request caching or rate limiting

# Proxy Architecture

## Overview

This is a high-performance HTTP/HTTPS/WebSocket proxy with MITM capabilities, automatic HTTP/2 and HTTP/1.1 negotiation, and request/response interception.

## Project Structure

```
proxy/
├── main.go           # Entry point, CLI flags, CA certificate management
├── proxy.go          # Core HTTP/HTTPS proxy handler logic
├── mitm.go           # Man-in-the-middle TLS interception (HTTP/2 + HTTP/1.1)
├── websocket.go      # WebSocket upgrade handling and frame capture
├── transport.go      # HTTP transport configuration and utilities
├── handlers.go       # Request/response handler types and utilities
├── capture.go        # Request/response capture and file writing
└── go.mod            # Dependencies
```

## File Responsibilities

### `main.go` (201 lines)

**Purpose:** Application entry point and setup

- Command-line flag parsing
- Output directory creation
- Logging configuration
- CA certificate loading/generation
- HTTP server initialization
- Default request/response handlers

### `proxy.go` (290 lines)

**Purpose:** Core proxy routing and HTTP request handling

- `proxyHandler()` - Main HTTP proxy handler
- `handleConnect()` - HTTPS CONNECT tunnel setup
- Regular HTTP/HTTPS request forwarding
- Request/response handler invocation
- Error handling and TLS fallback logic

### `mitm.go` (327 lines)

**Purpose:** HTTPS interception with protocol negotiation

- `mitmHTTPS()` - MITM server with HTTP/2 and HTTP/1.1 support
- `mitmHandler` - HTTP handler for intercepted requests
- `mitmCA` - Certificate authority management
- Dynamic certificate generation per hostname
- Automatic protocol negotiation via ALPN

### `websocket.go` (545 lines)

**Purpose:** WebSocket protocol handling

- `handleWebSocketUpgrade()` - WebSocket upgrade request handling
- `handleWebSocketConnect()` - WebSocket CONNECT tunnel
- `parseWebSocketFrame()` - WebSocket frame parsing
- `websocketCapturingCopy()` - Frame capture while proxying
- `saveWebSocketMessage()` - Frame logging to files
- Frame types: text, binary, close, ping, pong

### `transport.go` (40 lines)

**Purpose:** HTTP transport configuration

- `sharedTransport` - Shared HTTP transport with connection pooling
- HTTP/2 enabled with automatic negotiation
- TLS 1.2/1.3 configuration
- Connection timeouts and keep-alive settings
- `copyHeader()` - HTTP header copying utility

### `handlers.go` (56 lines)

**Purpose:** Request/response handler framework

- `OnRequestHandler` - Request handler function type
- `OnResponseHandler` - Response handler function type
- `generateRequestID()` - Unique request ID generation
- `cloneResponseMeta()` - Response cloning utility
- Global handler registration functions

### `capture.go` (135 lines)

**Purpose:** Async request/response capture

- `captureWriter()` - Async capture worker goroutine
- `asyncCapture()` - Non-blocking capture queuing
- `asyncWebSocketCapture()` - WebSocket-specific capture
- `writeCaptureToDir()` - File writing with timestamps
- Separate directories for HTTP and WebSocket captures

## Key Features

### ✅ Automatic Protocol Negotiation

- **HTTP/2 and HTTP/1.1** automatically negotiated via ALPN
- Client chooses protocol during TLS handshake
- Transparent - no forced protocol versions
- Works in both MITM and passthrough modes

### ✅ MITM Capabilities

- Full HTTPS decryption and inspection
- Dynamic certificate generation per domain
- Request/response modification support
- HTTP/2 support in MITM mode (using `http2.Server`)

### ✅ WebSocket Support

- WebSocket upgrade request interception
- Frame-level capture (text, binary, control frames)
- Bidirectional frame logging
- Both `ws://` and `wss://` support

### ✅ Async Capture

- Non-blocking request/response capture
- Queued writes to prevent proxy slowdown
- Separate directories for HTTP and WebSocket traffic
- Timestamped filenames with request IDs

### ✅ Request/Response Handlers

- Modify requests before forwarding
- Modify responses before returning to client
- Add custom headers, block domains, etc.
- Works with HTTP, HTTPS, and WebSocket traffic

## Data Flow

### Regular HTTP Request

```
Client → proxyHandler() → onRequestHandler → sharedTransport.RoundTrip()
       → onResponseHandler → asyncCapture() → Client
```

### HTTPS CONNECT (MITM)

```
Client → handleConnect() → mitmHTTPS() → http.Server (HTTP/2 or HTTP/1.1)
       → mitmHandler.ServeHTTP() → onRequestHandler → sharedTransport.RoundTrip()
       → onResponseHandler → asyncCapture() → Client
```

### WebSocket Upgrade

```
Client → proxyHandler() → handleWebSocketUpgrade() → Hijack Connection
       → Connect to upstream → Forward upgrade handshake
       → startWebSocketTunnel() → websocketCapturingCopy()
       → parseWebSocketFrame() → saveWebSocketMessage()
```

## Configuration

### Command-Line Flags

```bash
-listen :8080              # Listen address
-out captures              # Output directory for HTTP captures
-wsout captures/websockets # WebSocket capture directory
-certpath .                # CA certificate directory
-log captures/proxy.log    # Log file path
```

### HTTP Transport Settings

- **ForceAttemptHTTP2:** `true` - Enables HTTP/2 negotiation
- **MaxIdleConns:** 100
- **IdleConnTimeout:** 90s
- **TLSHandshakeTimeout:** 15s
- **TLS Versions:** TLS 1.2 - TLS 1.3

### MITM TLS Config

- **NextProtos:** `["h2", "http/1.1"]` - Advertises both protocols
- **MinVersion:** TLS 1.2
- **MaxVersion:** TLS 1.3

## Request ID Format

- Pattern: `req-00000001`, `req-00000002`, ...
- 8-digit zero-padded counter
- Used for correlating requests, responses, and logs
- Sub-requests in MITM: `req-00000001-sub-1`

## Capture File Format

### HTTP/HTTPS Captures

```
[RAW REQUEST ]
GET /path HTTP/1.1
Host: example.com
...

-------------------|-------------------
[RAW RESPONSE]
HTTP/1.1 200 OK
Content-Type: application/json
...
```

### WebSocket Captures

```
WebSocket Message
================
RequestID: req-00000001
Direction: client-to-server
Frame Type: text
Timestamp: 2025-10-13T01:23:45.123456789Z
Payload Length: 42 bytes

--- Message Content ---
{"type":"message","data":"hello"}
```

## Performance Considerations

- **Connection Pooling:** Shared transport reuses connections
- **Async Capture:** Non-blocking writes to disk
- **HTTP/2 Multiplexing:** Multiple requests over single connection
- **Keep-Alive:** Long-lived connections reduce overhead

## Security Notes

⚠️ **MITM Mode Requirements:**

- CA certificate must be installed and trusted on client
- Generated at: `cert/ca.crt`
- Private key at: `cert/ca.key`
- Leaf certificates generated dynamically per hostname

⚠️ **TLS Verification:**

- Upstream servers are verified by default
- No `InsecureSkipVerify` in production code

## Future Enhancements

Potential improvements:

- [ ] HTTP/3 (QUIC) support
- [ ] Connection filtering by domain/IP
- [ ] Request/response body modification
- [ ] Plugin system for custom handlers
- [ ] Web UI for viewing captures
- [ ] Traffic replay capabilities

## Building & Running

```bash
# Build
go build -o proxy .

# Run with defaults
./proxy

# Run with custom settings
./proxy -listen :9090 -out /tmp/captures

# Test with curl
curl -x http://localhost:8080 http://example.com
curl -x http://localhost:8080 -k https://example.com  # MITM
```

## Testing

The proxy has been tested with:

- ✅ Regular HTTP requests
- ✅ HTTPS MITM (HTTP/2 and HTTP/1.1)
- ✅ WebSocket upgrades (ws:// and wss://)
- ✅ Modern browsers (Firefox, Chrome)
- ✅ curl, wget, and other HTTP clients
