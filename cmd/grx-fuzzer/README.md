# grx-fuzzer - Standalone HTTP/HTTP2 Fuzzer

A standalone command-line fuzzing tool for grroxy that supports both HTTP/1.x and HTTP/2 protocols.

## Installation

```bash
cd cmd/grx-fuzzer
go build -o grx-fuzzer
```

Or install globally:

```bash
go install github.com/glitchedgitz/grroxy/cmd/grx-fuzzer@latest
```

## Quick Start

### Basic HTTP/1.1 Fuzzing

```bash
grx-fuzzer --host example.com \
  --request "GET /ยงPATHยง HTTP/1.1\r\nHost: example.com\r\n\r\n" \
  --marker "ยงPATHยง=wordlist.txt" \
  --output results.json
```

### HTTP/2 Fuzzing

```bash
grx-fuzzer --host api.example.com --tls --http2 \
  --request "GET /api/ยงENDPOINTยง HTTP/1.1\r\nHost: api.example.com\r\n\r\n" \
  --marker "ยงENDPOINTยง=endpoints.txt" \
  --output results.json
```

## Usage

```
grx-fuzzer [flags]

Flags:
  # Request Configuration
  --host string              Target host (required)
  --port string              Target port (default: 80 for HTTP, 443 for HTTPS)
  --request string           Raw HTTP request with markers
  --request-file string      File containing raw HTTP request
  --tls                      Use TLS/HTTPS
  --http2                    Use HTTP/2 protocol (requires --tls)

  # Fuzzer Configuration
  --marker stringToString    Fuzzing markers (format: MARKER=wordlist.txt)
                            Can be specified multiple times
  --mode string             Fuzzing mode: cluster_bomb or pitch_fork (default: cluster_bomb)
  --concurrency int         Number of concurrent requests (default: 40)
  --timeout float           Request timeout in seconds (default: 10)

  # Output Configuration
  -o, --output string        Output file for results (JSON format)
  -v, --verbose              Verbose output (show all results)
  -h, --help                 Help for grx-fuzzer
```

## Examples

### 1. Simple Path Fuzzing

```bash
grx-fuzzer --host example.com \
  --request "GET /ยงPATHยง HTTP/1.1\r\nHost: example.com\r\n\r\n" \
  --marker "ยงPATHยง=paths.txt" \
  --output results.json
```

### 2. Authentication Testing (HTTP/2)

```bash
grx-fuzzer --host api.example.com --tls --http2 \
  --request "GET /api/data HTTP/1.1\r\nHost: api.example.com\r\nAuthorization: Bearer ยงTOKENยง\r\n\r\n" \
  --marker "ยงTOKENยง=tokens.txt" \
  --concurrency 50 \
  --output results.json
```

### 3. Login Brute Force (Multiple Markers)

```bash
grx-fuzzer --host auth.example.com --tls \
  --request "POST /login HTTP/1.1\r\nHost: auth.example.com\r\nContent-Type: application/json\r\n\r\n{\"username\":\"ยงUSERยง\",\"password\":\"ยงPASSยง\"}" \
  --marker "ยงUSERยง=users.txt" \
  --marker "ยงPASSยง=passwords.txt" \
  --mode pitch_fork \
  --output results.json
```

### 4. Using Request File

Create a file `request.txt`:

```
GET /api/ยงENDPOINTยง HTTP/1.1
Host: api.example.com
User-Agent: grx-fuzzer
Authorization: Bearer ยงTOKENยง

```

Then run:

```bash
grx-fuzzer --host api.example.com --tls --http2 \
  --request-file request.txt \
  --marker "ยงENDPOINTยง=endpoints.txt" \
  --marker "ยงTOKENยง=tokens.txt" \
  --output results.json
```

### 5. Verbose Mode

```bash
grx-fuzzer --host example.com \
  --request "GET /ยงPATHยง HTTP/1.1\r\nHost: example.com\r\n\r\n" \
  --marker "ยงPATHยง=paths.txt" \
  --verbose \
  --output results.json
```

### 6. High Concurrency HTTP/2

```bash
grx-fuzzer --host fast-api.example.com --tls --http2 \
  --request "GET /ยงPATHยง HTTP/1.1\r\nHost: fast-api.example.com\r\n\r\n" \
  --marker "ยงPATHยง=large-wordlist.txt" \
  --concurrency 100 \
  --timeout 5 \
  --output results.json
```

## Fuzzing Modes

### Cluster Bomb (default)

Tests all possible combinations of markers.

Example: With 3 users and 3 passwords, makes 9 requests (3 ร— 3).

```bash
--mode cluster_bomb
```

### Pitch Fork

Tests markers in parallel (one-to-one).

Example: With 3 users and 3 passwords, makes 3 requests (user1+pass1, user2+pass2, user3+pass3).

```bash
--mode pitch_fork
```

## Output Format

Results are saved in JSON format:

```json
[
  {
    "request": "GET /admin HTTP/1.1\r\nHost: example.com\r\n\r\n",
    "response": "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n...",
    "response_time_ms": 123.45,
    "markers": {
      "ยงPATHยง": "admin"
    },
    "status_code": 200
  },
  {
    "request": "GET /login HTTP/1.1\r\nHost: example.com\r\n\r\n",
    "response": "HTTP/1.1 404 Not Found\r\n...",
    "response_time_ms": 89.12,
    "markers": {
      "ยงPATHยง": "login"
    },
    "status_code": 404
  }
]
```

## HTTP/2 Support

grx-fuzzer fully supports HTTP/2:

- Use `--tls --http2` flags
- Write requests in HTTP/1.x format (automatically converted)
- Server must support HTTP/2
- Benefits from multiplexing and header compression

### HTTP/2 Example

```bash
# Test Google with HTTP/2
grx-fuzzer --host www.google.com --tls --http2 \
  --request "GET / HTTP/1.1\r\nHost: www.google.com\r\n\r\n" \
  --marker "ยงDUMMYยง=single.txt" \
  --output google-http2.json

# single.txt contains just: test
```

## Tips

### 1. Line Breaks in Requests

Use `\r\n` for line breaks in `--request`:

```bash
--request "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"
```

Or use `--request-file` for complex requests.

### 2. Performance Tuning

- HTTP/2 works well with higher concurrency (50-100+)
- HTTP/1.x typically works best with 20-50 concurrent requests
- Adjust `--timeout` based on server response times

### 3. Wordlists

Create wordlists with one entry per line:

```
admin
api
users
config
```

### 4. Progress Monitoring

- Use `--verbose` to see all results in real-time
- Without verbose, progress updates every 10 requests
- Final summary always shown

### 5. Error Handling

- Errors are counted and reported in summary
- Error details saved in JSON output
- Non-zero exit code on fatal errors

## Comparison with API Approach

| Feature     | grx-fuzzer CLI   | API Endpoint            |
| ----------- | ---------------- | ----------------------- |
| Setup       | Direct execution | Requires server running |
| Results     | JSON file        | Database                |
| Real-time   | Terminal output  | WebSocket/polling       |
| Portability | Single binary    | API + DB                |
| Integration | Shell scripts    | REST API                |

## Troubleshooting

### "HTTP/2 requires TLS"

**Solution**: Add `--tls` flag when using `--http2`

### "server does not support HTTP/2"

**Solution**: Remove `--http2` flag or verify server supports HTTP/2

### "connection timeout"

**Solution**: Increase `--timeout` value

### "too many open files"

**Solution**: Reduce `--concurrency` or increase system limits

## Building from Source

```bash
# From project root
cd cmd/grx-fuzzer
go build -o grx-fuzzer

# Test it
./grx-fuzzer --help
```

## License

Same as grroxy project.
