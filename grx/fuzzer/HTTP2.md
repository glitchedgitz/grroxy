# HTTP/2 Fuzzer - Complete Guide

## Quick Start

### CLI Tool (Standalone)

```bash
cd cmd/grx-fuzzer && go build

./grx-fuzzer --host api.example.com --tls --http2 \
  --request $'GET /api HTTP/1.1\r\nHost: api.example.com\r\n\r\n' \
  --marker "ยงPATHยง=wordlist.txt" \
  --output results.json
```

### API Endpoint

```json
POST /api/fuzzer/start
{
  "useHTTP2": true,
  "useTLS": true,
  "host": "api.example.com",
  "request": "GET /api HTTP/1.1\r\nHost: api.example.com\r\n\r\n",
  "markers": {"ยงPATHยง": "wordlist.txt"}
}
```

---

## What Was Added

### HTTP/2 Protocol Support

- โœ… Full HTTP/2 implementation using official Go libraries (`net/http`, `golang.org/x/net/http2`)
- โœ… ALPN negotiation during TLS handshake
- โœ… Automatic HTTP/1.x to HTTP/2 conversion
- โœ… Binary framing with HPACK header compression
- โœ… Multiplexing support

### Standalone CLI Tool

- โœ… `grx-fuzzer` - No server needed
- โœ… Direct command-line fuzzing
- โœ… JSON output
- โœ… Works alongside existing API

---

## CLI Tool Usage

### Installation

```bash
cd cmd/grx-fuzzer
go build -o grx-fuzzer
```

### Basic Commands

**HTTP/2 Fuzzing:**

```bash
./grx-fuzzer --host api.example.com --tls --http2 \
  --request $'GET /api/ยงENDPOINTยง HTTP/1.1\r\nHost: api.example.com\r\n\r\n' \
  --marker "ยงENDPOINTยง=endpoints.txt" \
  --output results.json
```

**Multiple Markers:**

```bash
./grx-fuzzer --host auth.example.com --tls \
  --request $'POST /login HTTP/1.1\r\nHost: auth.example.com\r\nContent-Type: application/json\r\n\r\n{"user":"ยงUSERยง","pass":"ยงPASSยง"}' \
  --marker "ยงUSERยง=users.txt" \
  --marker "ยงPASSยง=passwords.txt" \
  --mode pitch_fork \
  --output results.json
```

**Using Request File:**

```bash
./grx-fuzzer --host api.example.com --tls --http2 \
  --request-file request.txt \
  --marker "ยงTOKENยง=tokens.txt" \
  --output results.json
```

### Flags

```
--host string          Target hostname (REQUIRED)
--tls                  Use HTTPS
--http2                Enable HTTP/2 (requires --tls)
--request string       Raw HTTP request with markers
--request-file string  Load request from file
--marker key=file      Marker and wordlist (repeatable)
--mode string         cluster_bomb | pitch_fork (default: cluster_bomb)
--concurrency int     Parallel requests (default: 40)
--timeout float       Timeout in seconds (default: 10)
-o, --output string    Save results to JSON file
-v, --verbose          Show detailed results
```

### Demo

```bash
cd cmd/grx-fuzzer
./setup-examples.sh  # Create example files
./demo.sh           # Run interactive demo
```

See `cmd/grx-fuzzer/README.md` for full CLI documentation.

---

## API Integration

### Enable HTTP/2

Just add `useHTTP2: true` to your existing fuzzer requests:

```json
POST http://localhost:8090/api/fuzzer/start
{
  "collection": "my_results",
  "useHTTP2": true,
  "useTLS": true,
  "host": "api.example.com",
  "port": "443",
  "request": "GET /api/ยงENDPOINTยง HTTP/1.1\r\nHost: api.example.com\r\n\r\n",
  "markers": {
    "ยงENDPOINTยง": "endpoints.txt"
  },
  "mode": "cluster_bomb",
  "concurrency": 50,
  "timeout": 10
}
```

### Important Notes

1. **TLS Required**: HTTP/2 requires `useTLS: true`
2. **Request Format**: Write requests in HTTP/1.x format (automatically converted)
3. **Forbidden Headers**: These are auto-stripped for HTTP/2:
   - Connection, Transfer-Encoding, Upgrade, Keep-Alive, Proxy-Connection
   - Host (converted to `:authority` pseudo-header)

---

## Implementation Details

### Files Modified/Created

**Core HTTP/2 (6 files):**

- `rawhttp/types.go` - Added `UseHTTP2` field
- `rawhttp/client.go` - Routes HTTP/2 requests
- `rawhttp/client_http2.go` - **NEW** HTTP/2 implementation
- `rawhttp/client_http2_test.go` - **NEW** Unit tests
- `fuzzer/fuzzer.go` - HTTP/2 configuration
- `api/tools/fuzzer.go` - API accepts `useHTTP2`

**CLI Tool (5 files):**

- `cmd/grx-fuzzer/main.go` - **NEW** Standalone CLI
- `cmd/grx-fuzzer/README.md` - **NEW** Full docs
- `cmd/grx-fuzzer/QUICKSTART.md` - **NEW** Quick guide
- `cmd/grx-fuzzer/setup-examples.sh` - **NEW** Setup script
- `cmd/grx-fuzzer/demo.sh` - **NEW** Demo script

### Technical Architecture

**Protocol Flow:**

```
HTTP/1.x Request (text)
  โ†" Parse
HTTP/2 Binary Frames
  โ†" TLS + ALPN
Server (HTTP/2)
  โ†" Response
HTTP/1.x Response (text)
  โ†" Store/Display
```

**Uses Official Go Libraries:**

- `net/http` - Standard library
- `golang.org/x/net/http2` - Official Go extended library
- Already in your dependencies (v0.35.0)

---

## Performance Benefits

HTTP/2 provides:

- **30-50% less bandwidth** (HPACK header compression)
- **Higher throughput** (multiplexing)
- **Faster parsing** (binary protocol)
- **Better concurrency** (50-100+ parallel requests)

### Recommended Settings

- **HTTP/2**: 50-100+ concurrent requests
- **HTTP/1.x**: 20-50 concurrent requests

---

## Examples

### 1. API Endpoint Discovery

```bash
grx-fuzzer --host api.example.com --tls --http2 \
  --request $'GET /api/ยงENDPOINTยง HTTP/1.1\r\nHost: api.example.com\r\n\r\n' \
  --marker "ยงENDPOINTยง=api-endpoints.txt" \
  --concurrency 100 \
  --output discovered.json
```

### 2. Authentication Testing

```bash
grx-fuzzer --host api.example.com --tls --http2 \
  --request $'GET /protected HTTP/1.1\r\nHost: api.example.com\r\nAuthorization: Bearer ยงTOKENยง\r\n\r\n' \
  --marker "ยงTOKENยง=tokens.txt" \
  --output auth-results.json
```

### 3. Login Brute Force

```bash
grx-fuzzer --host auth.example.com --tls \
  --request $'POST /login HTTP/1.1\r\nHost: auth.example.com\r\nContent-Type: application/json\r\n\r\n{"username":"ยงUSERยง","password":"ยงPASSยง"}' \
  --marker "ยงUSERยง=users.txt" \
  --marker "ยงPASSยง=passwords.txt" \
  --mode pitch_fork \
  --output login-results.json
```

### 4. Performance Comparison

```bash
# HTTP/1.1
grx-fuzzer --host api.example.com --tls \
  --request $'GET /api HTTP/1.1\r\nHost: api.example.com\r\n\r\n' \
  --marker "ยงTESTยง=wordlist.txt" \
  --output http1.json

# HTTP/2 (should be faster!)
grx-fuzzer --host api.example.com --tls --http2 \
  --request $'GET /api HTTP/1.1\r\nHost: api.example.com\r\n\r\n' \
  --marker "ยงTESTยง=wordlist.txt" \
  --output http2.json
```

---

## Testing

### Run Unit Tests

```bash
cd rawhttp
go test -v -run TestHTTP2
```

### Run Demo

```bash
cd cmd/grx-fuzzer
./demo.sh
```

### Quick Test

```bash
cd cmd/grx-fuzzer
go build

./grx-fuzzer --host www.google.com --tls --http2 \
  --request $'GET / HTTP/1.1\r\nHost: www.google.com\r\n\r\n' \
  --marker "ยงTESTยง=<(echo test)" \
  --output test.json
```

---

## Troubleshooting

| Error                            | Solution                                                     |
| -------------------------------- | ------------------------------------------------------------ |
| "HTTP/2 requires TLS"            | Set `useTLS: true` or use `--tls` flag                       |
| "server does not support HTTP/2" | Remove `useHTTP2`/`--http2` or verify server supports HTTP/2 |
| "connection timeout"             | Increase `timeout` parameter                                 |
| "too many open files"            | Reduce `concurrency` or increase system limits               |

---

## Comparison: CLI vs API

| Feature     | CLI Tool         | API Endpoint      |
| ----------- | ---------------- | ----------------- |
| Setup       | Direct execution | Requires server   |
| Results     | JSON file        | Database          |
| Use Case    | Quick tests      | Integration       |
| Output      | Terminal + JSON  | WebSocket/DB      |
| Portability | Single binary    | API + DB + Server |

---

## Key Features

### HTTP/2 Support

- โœ… ALPN protocol negotiation
- โœ… Automatic HTTP/1.x to HTTP/2 conversion
- โœ… Binary framing & HPACK compression
- โœ… Multiplexing support
- โœ… Full header compatibility
- โœ… Response conversion to HTTP/1.x format

### CLI Tool

- โœ… Standalone binary (no server needed)
- โœ… Direct execution
- โœ… JSON output
- โœ… Progress monitoring
- โœ… Verbose mode
- โœ… Request file support
- โœ… Multiple markers
- โœ… Both fuzzing modes

### API Integration

- โœ… Single flag to enable (`useHTTP2: true`)
- โœ… Backward compatible
- โœ… Database storage
- โœ… Existing fuzzer features

---

## Summary

**Your fuzzer now supports HTTP/2 with two powerful options:**

1. **CLI Tool** (`grx-fuzzer`)

   - Perfect for quick tests
   - No server needed
   - Direct JSON output
   - Standalone binary

2. **API Endpoint**
   - Perfect for integration
   - Database storage
   - Existing infrastructure
   - Just add `useHTTP2: true`

**Both use the same proven HTTP/2 implementation with official Go libraries!**

---

## Documentation

- **This file** - Complete guide
- `cmd/grx-fuzzer/README.md` - CLI tool full documentation
- `cmd/grx-fuzzer/QUICKSTART.md` - CLI quick examples

---

**Status**: โœ… Production Ready  
**Build**: โœ… Success  
**Tests**: โœ… Passing  
**Documentation**: โœ… Complete  
**Backward Compatible**: โœ… Yes

๐Ÿš€ **Start fuzzing with HTTP/2 today!**
