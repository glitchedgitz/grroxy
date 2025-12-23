# API Documentation

## All endpoints

```http
# Proxy
POST /api/proxy/start
POST /api/proxy/stop
POST /api/proxy/restart
GET  /api/proxy/list

# Intercept
POST      /api/intercept/action

# Playground
POST      /api/playground/new
POST      /api/playground/add
POST      /api/playground/delete

# Templates
GET       /api/templates/list
POST      /api/templates/new
DELETE    /api/templates/:template

# File Operations
POST      /api/readfile
POST      /api/savefile

# Cook Engine
POST      /api/cook/generate
POST      /api/cook/apply
POST      /api/cook/search

# Labels
POST      /api/label/new
POST      /api/label/delete
POST      /api/label/attach

# Regex
POST      /api/regex

# Sitemap
POST      /api/sitemap/new
POST      /api/sitemap/fetch

# Commands
POST      /api/runcommand

# Tools
GET       /api/tool

# Fuzzer
POST      /api/fuzzer/start
POST      /api/fuzzer/stop
GET       /api/fuzzer/results/:id

# Repeater
POST      /api/repeater/send

# Certificates
GET       /cacert.crt
```

---

## Proxy

### Start Proxy

Starts a new proxy instance and creates a proxy record in the database. Each proxy has its own intercept setting stored in the `_proxies` collection and filter settings stored separately in the `_ui` collection.

```http
POST /api/proxy/start
```

_Request Body:_

```json
{
  "http": "127.0.0.1:8080",
  "browser": "chrome|firefox|safari|terminal|proxy",
  "name": "My Proxy Instance"
}
```

_Fields:_

- `http` (string, optional): The listen address for the proxy (e.g., "127.0.0.1:8080"). If not provided with a browser, defaults to "127.0.0.1:9797". If the port is unavailable, an available port will be suggested.
- `browser` (string, optional): Browser to launch with this proxy. Options: "chrome", "firefox", "safari", "terminal", "proxy" (no browser). Leave empty for proxy only.
- `name` (string, optional): Custom name for the proxy instance. If not provided, a name will be auto-generated based on browser type and count (e.g., "chrome 1", "firefox 2").

_Response:_

```json
{
  "id": "______________1",
  "listenAddr": "127.0.0.1:8080",
  "label": "chrome 1",
  "browser": "chrome"
}
```

_Response Fields:_

- `id` (string): The unique proxy ID (15 chars: underscores + index number, e.g., "**\*\***\_\_**\*\***1")
- `listenAddr` (string): The address the proxy is listening on (e.g., "127.0.0.1:8080")
- `label` (string): The display name/label for this proxy instance
- `browser` (string): The browser type launched with this proxy

_Error Response:_

```json
{
  "error": "port not available",
  "availableHost": "127.0.0.1:8081"
}
```

---

### Stop Proxy

Stops a running proxy instance and removes it from the manager. Also terminates any associated browser or terminal process.

```http
POST /api/proxy/stop
```

_Request Body:_

```json
{
  "id": "______________1"
}
```

_Fields:_

- `id` (string, optional): The unique proxy ID to stop (e.g., "**\*\***\_\_**\*\***1"). If not provided or empty, stops all running proxies.

_Response:_

```json
{
  "message": "Proxy stopped"
}
```

_Notes:_

- If no `id` field is provided or the request body is empty, all running proxies will be stopped
- The proxy record in the `_proxies` collection is NOT deleted, only the runtime instance is stopped
- When a proxy is stopped, its `state` field in the database is set to empty (`""`)

---

### Restart Proxy

Restarts a previously stopped proxy using its existing configuration from the database. This is useful for resuming a proxy that was stopped without having to reconfigure it.

```http
POST /api/proxy/restart
```

_Request Body:_

```json
{
  "id": "______________1"
}
```

_Fields:_

- `id` (string, required): The unique proxy ID to restart (e.g., "**\*\***\_\_**\*\***1")

_Response:_

```json
{
  "id": "______________1",
  "listenAddr": "127.0.0.1:8080",
  "label": "chrome 1",
  "browser": "chrome"
}
```

## Intercept

### Handle Intercept Action

Processes an intercept action (forward or drop) for a pending request/response. Optionally includes edited request or response data as raw HTTP strings.

```http
POST /api/intercept/action
```

_Request Body:_

```json
{
  "id": "string",
  "action": "forward|drop",
  "is_req_edited": false,
  "is_resp_edited": false,
  "req_edited": "GET /api/endpoint HTTP/1.1\r\nHost: example.com\r\n\r\n",
  "resp_edited": "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n<html>...</html>"
}
```

_Fields:_

- `id` (string, required): The intercept ID
- `action` (string, required): Either "forward" or "drop"
- `is_req_edited` (boolean, optional): Whether the request was edited
- `is_resp_edited` (boolean, optional): Whether the response was edited
- `req_edited` (string, optional): Raw HTTP request string (only if `is_req_edited` is true)
- `resp_edited` (string, optional): Raw HTTP response string (only if `is_resp_edited` is true)

_Response:_

```json
{
  "success": true,
  "message": "Intercept action processed successfully"
}
```

_Error Responses:_

- 400 Bad Request - Invalid action or missing ID
- 403 Forbidden - Unauthorized
- 404 Not Found - Intercept ID not found
- 500 Internal Server Error - Failed to update intercept or save edited data

---

## Playground

### New Playground

Creates a new playground item with specified name, type, and parent ID.

```http
POST /api/playground/new
```

```json
{
  "name": "string",
  "parent_id": "string",
  "type": "string",
  "expanded": false
}
```

```json
{
  "id": "string",
  "name": "string",
  "type": "string",
  "parent_id": "string",
  "sort_order": 0,
  "expanded": false
}
```

---

### Add Playground Items

Adds one or more items to a playground, supporting different types like repeater and fuzzer.

```http
POST /api/playground/add
```

```json
{
  "parent_id": "string",
  "items": [
    {
      "name": "string",
      "original_id": "string",
      "type": "repeater|fuzzer",
      "tool_data": {
        "url": "string",
        "req": "string",
        "resp": "string",
        "data": "string",
        "extra": { "variables": [] }
      }
    }
  ]
}
```

```json
{ "success": true }
```

---

### Delete Playground

Deletes a playground item by its ID.

```http
POST /api/playground/delete
```

```json
{
  "id": "string"
}
```

```json
{ "success": true, "id": "..." }
```

---

## Repeater

### New Repeater

Creates a new repeater record and associated tab collection.

```http
POST /api/repeater/new
```

```json
{
  "url": "string",
  "data": "string",
  "req": "string",
  "resp": "string",
  "extra": { "variables": [] }
}
```

```json
// JSON record of the created repeater.
```

---

### Delete Repeater

Deletes a repeater record and its associated tab collection by its ID.

```http
POST /api/repeater/delete
```

```json
{
  "id": "string"
}
```

```json
{ "success": true, "id": "..." }
```

---

### Send Repeater Request

Sends a raw HTTP request through the repeater and saves both the request and response to the backend. This endpoint combines the functionality of sending a raw HTTP request and automatically storing the transaction in the database.

```http
POST /api/repeater/send
```

_Request Body:_

```json
{
  "host": "example.com",
  "port": "443",
  "tls": true,
  "request": "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n",
  "timeout": 10,
  "http2": false,
  "index": 1.0,
  "url": "https://example.com"
}
```

_Fields:_

- `host` (string, required): The target hostname (without protocol prefix)
- `port` (string, optional): The target port (defaults to 443 for TLS, 80 for non-TLS)
- `tls` (boolean, required): Whether to use TLS/HTTPS
- `request` (string, required): The raw HTTP request to send
- `timeout` (float64, required): Timeout in seconds for the request
- `http2` (boolean, optional): Whether to use HTTP/2 protocol (default: false)
- `index` (float64, required): Primary index for organizing requests
- `url` (string, optional): The full URL for reference

**Note:** `index_minor` is automatically calculated in `SaveRequestToBackend()` using the counter system (counter key format: `"row:23"`, `"row:45"`, etc.). Each `index` has its own counter that increments for each request saved with that index.

_Response:_

```json
{
  "response": "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n...",
  "time": "123ms",
  "userdata": {
    "id": "_______________",
    "index": 1.0,
    "index_minor": 0.0,
    "is_https": true,
    "host": "https://example.com",
    "port": "443",
    "has_resp": true,
    "req_json": { ... },
    "resp_json": { ... },
    "generated_by": "repeater"
  }
}
```

_Response Fields:_

- `response` (string): The raw HTTP response received from the server
- `time` (string): The time taken to send the request and receive the response
- `userdata` (object): The complete user data object stored in the database, including request and response metadata

_Features:_

- **Auto-increments `index_minor`** in `SaveRequestToBackend()` using the counter system (counter key format: `"row:23"`, `"row:45"`, etc.)
- Automatically saves the request to `_req` collection
- Automatically saves the response to `_resp` collection
- Creates entries in `_data` and `_attached` collections
- Updates sitemap based on the request path
- Supports both HTTP/1.1 and HTTP/2
- Configurable timeout
- Marks all saved data with `generated_by: "repeater"`
- Each `index` maintains its own counter for sequential minor indexes
- Counter logic is centralized in `SaveRequestToBackend()` for reusability

---

### Send Raw Request

Sends a raw HTTP request and returns the response. Supports both HTTP/1.1 and HTTP/2.

```http
POST /api/sendrawrequest
```

```json
{
  "host": "string",
  "port": "string",
  "req": "string",
  "tls": true,
  "timeout": 10,
  "httpversion": 1
}
```

```json
{
  "resp": "string",
  "time": "string"
}
```

---

## Intruder

### New Intruder

Creates a new intruder tab collection and inserts a row.

```http
POST /api/intruder/new
```

```json
{
  "id": "string",
  "url": "string",
  "req": "string",
  "payload": "string"
}
```

```json
{ "success": true, "id": "..." }
```

---

### Delete Intruder

Deletes an intruder tab collection by its ID.

```http
POST /api/intruder/delete
```

```json
{
  "id": "string"
}
```

```json
{ "success": true, "id": "..." }
```

---

## Templates

### List Templates

Lists all available templates.

```http
GET /api/templates/list
```

```json
// No request body
```

```json
{
  "list": [{ "name": "string", "path": "string", "is_dir": true }]
}
```

---

### Create New Template

Creates a new template file.

```http
POST /api/templates/new
```

```json
{
  "name": "string",
  "content": "string"
}
```

```json
{ "filepath": "string" }
```

---

### Delete Template

Deletes a template file.

```http
DELETE /api/templates/:template
```

```json
// No request body
```

_Response:_

- 200 OK on success, 500 on error.

---

## File Operations

### Read File

Reads a file from a specified folder.

```http
POST /api/readfile
```

```json
{
  "fileName": "string",
  "folder": "string"
}
```

```json
{ "filecontent": "string" }
```

---

### Save File

Saves a file to a specified folder.

```http
POST /api/savefile
```

```json
{
  "fileName": "string",
  "fileData": "string",
  "folder": "string"
}
```

```json
{ "filepath": "string" }
```

---

## Cook Engine

### Generate Patterns

Generates patterns using the cook engine.

```http
POST /api/cook/generate
```

```json
{ "pattern": ["string", ...] }
```

```json
{ "results": ["string", ...] }
```

---

### Apply Methods

Applies methods to strings using the cook engine.

```http
POST /api/cook/apply
```

```json
{
  "strings": ["string", ...],
  "methods": ["string", ...]
}
```

```json
{ "results": ["string", ...] }
```

---

### Search Patterns

Searches for patterns using the cook engine.

```http
POST /api/cook/search
```

```json
{ "search": "string" }
```

```json
{
  "search": "string",
  "results": ["string", ...]
}
```

---

## Labels

### Create Label

Creates a new label.

```http
POST /api/label/new
```

```json
{
  "name": "string",
  "color": "string",
  "type": "string"
}
```

_Response:_

- 200 OK on success.

---

### Delete Label

Deletes a label by ID or name.

```http
POST /api/label/delete
```

```json
{
  "id": "string" // or "name": "string"
}
```

_Response:_

- 200 OK on success.

---

### Attach Label

Attaches a label to a record.

```http
POST /api/label/attach
```

```json
{
  "id": "string",
  "name": "string"
}
```

_Response:_

- 200 OK on success.

---

## Regex

### Search Regex

Checks if a regex matches a response body.

```http
POST /api/regex
```

```json
{
  "regex": "string",
  "responseBody": "string"
}
```

```json
{
  "matched": true
}
```

---

## Sitemap

### New Sitemap

Creates a new sitemap collection and inserts endpoint data.

```http
POST /api/sitemap/new
```

```json
{
  "host": "string",
  "data": "string",
  "path": "string",
  "query": "string",
  "fragment": "string",
  "type": "string",
  "ext": "string"
}
```

_Response:_

- 200 OK on success.

---

### Fetch Sitemap

Fetches sitemap data for a host and path.

```http
POST /api/sitemap/fetch
```

```json
{
  "host": "string",
  "path": "string"
}
```

```json
[
  {
    "host": "string",
    "path": "string",
    "type": "string",
    "title": "string",
    "ext": "string",
    "query": "string"
  }
]
```

---

## Commands

### Run Command

Executes a command and saves the output to a collection or file.

```http
POST /api/runcommand
```

```json
{
  "command": "string",
  "data": "any",
  "saveTo": "collection|file",
  "collection": "string",
  "filename": "string"
}
```

```json
{
  "id": "string"
}
```

---

## Tools

### Tool Server

Starts a tool server for a specific path.

```http
GET /api/tool
```

Query Parameters:

- `path`: Path to the tool directory

_Response:_

- 200 OK with host address on success
- 500 Internal Server Error on failure

---

## Fuzzer

### Start Fuzzer

Starts a new fuzzer instance with the specified configuration. The fuzzer will send requests with different payloads based on the markers and wordlists provided.

```http
POST /api/fuzzer/start
```

_Request Body:_

```json
{
  "request": "GET /test?param=FUZZ HTTP/1.1\r\nHost: example.com\r\n\r\n",
  "host": "example.com",
  "port": "80",
  "useTLS": false,
  "markers": {
    "FUZZ": "/path/to/wordlist.txt"
  },
  "mode": "cluster_bomb" | "pitch_fork",
  "concurrency": 40,
  "timeout": 10
}
```

_Fields:_

- `request` (string, required): The raw HTTP request template with markers (e.g., "FUZZ") that will be replaced with words from wordlists
- `host` (string, required): The target hostname (http:// or https:// prefix will be stripped automatically)
- `port` (string, optional): The target port. Defaults to 80 for HTTP or 443 for HTTPS
- `useTLS` (boolean, optional): Whether to use TLS/HTTPS. Defaults to false
- `markers` (object, required): Map of marker names to wordlist file paths. Each marker in the request will be replaced with words from its corresponding wordlist
- `mode` (string, optional): Fuzzing mode. Options: "cluster_bomb" (all combinations) or "pitch_fork" (synchronized). Defaults to "cluster_bomb"
- `concurrency` (integer, optional): Number of concurrent requests. Defaults to 40
- `timeout` (float, optional): Request timeout in seconds. Defaults to 10

_Response:_

```json
{
  "id": "______________1"
}
```

_Response Fields:_

- `id` (string): The unique fuzzer ID used to track and stop the fuzzer

_Error Responses:_

- 400 Bad Request - Missing required fields or invalid configuration
- 403 Forbidden - Unauthorized

---

### Stop Fuzzer

Stops a running fuzzer instance.

```http
POST /api/fuzzer/stop
```

_Request Body:_

```json
{
  "id": "______________1"
}
```

_Fields:_

- `id` (string, required): The unique fuzzer ID to stop

_Response:_

```json
{
  "status": "stopped"
}
```

_Error Responses:_

- 400 Bad Request - Missing ID
- 403 Forbidden - Unauthorized
- 404 Not Found - Fuzzer not found

---

### Get Fuzzer Results

Streams fuzzer results in real-time using Server-Sent Events (SSE). Each result contains the request sent, response received, timing information, and the markers used.

```http
GET /api/fuzzer/results/:id
```

_Path Parameters:_

- `id` (string, required): The unique fuzzer ID

_Response:_

Server-Sent Events stream with Content-Type: `text/event-stream`. Each event contains a JSON object:

```json
{
  "request": "GET /test?param=value HTTP/1.1\r\nHost: example.com\r\n\r\n",
  "response": "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n<html>...</html>",
  "time": 1234567890,
  "markers": {
    "FUZZ": "value"
  }
}
```

_Response Fields:_

- `request` (string): The actual HTTP request that was sent
- `response` (string): The raw HTTP response received
- `time` (integer): Response time in nanoseconds
- `markers` (object): Map of marker names to the values that were used in this request

_Error Responses:_

- 400 Bad Request - Missing ID
- 403 Forbidden - Unauthorized
- 404 Not Found - Fuzzer not found

_Notes:_

- The connection will remain open until the fuzzer completes or is stopped
- Results are streamed as they are generated
- The stream will automatically close when the fuzzer finishes

---

## Certificates

### Download CA Certificate

Downloads the CA certificate for HTTPS interception.

```http
GET /cacert.crt
```

_Response:_

- File download of the CA certificate
