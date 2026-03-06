# rawproxy

An intercepting forward proxy library in Go that saves raw HTTP requests and responses to files, with support for WebSocket connections.

**Package**:

```
github.com/glitchedgitz/grroxy/grx/rawproxy
```

## Quick Start

```go
package main

import (
    "log"
    "github.com/glitchedgitz/grroxy/grx/rawproxy"
)

func main() {
    proxy, err := rawproxy.New(&rawproxy.Config{
        ListenAddr: ":8080",
        OutputDir:  "./captures",
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(proxy.Start())
}
```

## API Reference

### Proxy Instance

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

### Config Structure

```go
type Config struct {
    ConfigFolder string        // Certificate folder (ca.crt, ca.key)
    ListenAddr   string        // Listen address (default: ":8080")
    OutputDir    string        // Output directory (default: "captures")
    WebSocketDir string        // WebSocket captures (default: "<OutputDir>/websockets")
    ReadTimeout  time.Duration // HTTP read timeout (default: 30s)
    WriteTimeout time.Duration // HTTP write timeout (default: 60s)
    IdleTimeout  time.Duration // HTTP idle timeout (default: 60s)
    MITM         *MitmCA       // MITM CA (auto-generated if nil)
    CertPath     string        // CA cert path (default: "<ConfigFolder>/ca.crt")
    KeyPath      string        // CA key path (default: "<ConfigFolder>/ca.key")
    OnRequestHandler  OnRequestHandler
    OnResponseHandler OnResponseHandler
    ReqCounter   *atomic.Uint64 // Request counter (auto-created if nil)
}
```

### Handler Functions

```go
type RequestData struct {
    RequestID string      // Unique ID (format: "req-00000001")
    Data      interface{} // Custom data passed between handlers
}

type OnRequestHandler func(reqData *RequestData, req *http.Request) (*http.Request, error)
type OnResponseHandler func(reqData *RequestData, resp *http.Response, req *http.Request) (*http.Response, error)
```

**Handler Behavior:**

- Return modified request/response to continue processing
- Return `nil, error` to block the request/response
- Use `reqData.Data` to pass custom data from request to response handler

### CA Management

```go
// Generate new CA certificate
func GenerateMITMCA(dir string) (*MitmCA, string, string, error)

// Load existing CA certificate
func LoadMITMCA(certPath, keyPath string) (*MitmCA, error)

// Check if file exists
func FileExists(path string) bool
```

## Features

- HTTP/HTTPS proxying with MITM support
- WebSocket connection handling (ws:// and wss://)
- Request/response capture to files
- Custom request/response handlers
- Automatic HTTP/2 and HTTP/1.1 negotiation
- Unique request ID tracking
- Async capture (non-blocking writes)
- Auto-generation of CA certificates

## Output Format

If saved to directory
**File naming**: `TIMESTAMP-SEQUENCE-METHOD-HOST.txt`  
**Example**: `20250101-120001.123456789-000001-GET-example.com_443.txt`

## HTTPS MITM

Certificates are auto-generated on first run if they don't exist:

- Default location: `cert/ca.crt` and `cert/ca.key`
- Or use `ConfigFolder` to specify custom location

Install `ca.crt` into your system trust store to intercept HTTPS traffic.

**Platform-specific installation:**

- **macOS**: `sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain <path-to-ca.crt>`
- **Ubuntu/Debian**: `sudo cp <path-to-ca.crt> /usr/local/share/ca-certificates/proxy-ca.crt && sudo update-ca-certificates`
- **Fedora/RHEL**: `sudo cp <path-to-ca.crt> /etc/pki/ca-trust/source/anchors/ && sudo update-ca-trust`
- **Firefox**: Import via Preferences > Privacy & Security > Certificates

## WebSocket Support

WebSocket connections are automatically detected and handled:

- Supports both `ws://` and `wss://` protocols
- Captures individual WebSocket messages with metadata
- Upgrade handshake captured like regular HTTP traffic
- Frame types: text, binary, close, ping, pong

## Example: Custom Handlers

```go
proxy.SetRequestHandler(func(reqData *rawproxy.RequestData, req *http.Request) (*http.Request, error) {
    log.Printf("Request %s: %s", reqData.RequestID, req.URL.String())
    // Block specific domains
    if strings.Contains(req.URL.Host, "blocked.com") {
        return nil, fmt.Errorf("domain blocked")
    }
    return req, nil
})

proxy.SetResponseHandler(func(reqData *rawproxy.RequestData, resp *http.Response, req *http.Request) (*http.Response, error) {
    log.Printf("Response %s: %d", reqData.RequestID, resp.StatusCode)
    // Add security headers
    resp.Header.Set("X-Frame-Options", "DENY")
    return resp, nil
})
```

## Request ID Format

- Pattern: `req-00000001`, `req-00000002`, ... (8-digit zero-padded)
- Maximum: `req-99999999`
- Access: `reqData.RequestID` in handlers
- Thread-safe atomic counter

## Configuration Notes

- If `MITM` is `nil`, certificates are auto-generated/loaded by `New()`
- Default certificate paths: `cert/ca.crt` and `cert/ca.key`
- WebSocket captures go to `<OutputDir>/websockets` by default
- All timeouts default to reasonable values if not specified
