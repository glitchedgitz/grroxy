# Grroxy App API Documentation

## Overview

This is the main Grroxy application API that provides comprehensive HTTP proxy, security testing, and request manipulation capabilities. All endpoints require authentication (admin or authenticated user) unless otherwise specified.

**Base URL**: `http://{host}:{port}/api`

---

## Table of Contents

- [Proxy Management](#proxy-management)
- [Intercept](#intercept)
- [Playground](#playground)
- [Request Modification](#request-modification)
- [Repeater](#repeater)
- [Filters](#filters)
- [Templates](#templates)
- [File Operations](#file-operations)
- [Cook Engine](#cook-engine)
- [Labels](#labels)
- [Regex](#regex)
- [Sitemap](#sitemap)
- [Commands](#commands)
- [Tools](#tools)
- [Raw HTTP](#raw-http)
- [Certificates](#certificates)
- [Extractor](#extractor)

---

## INFO

Returns information about the running Grroxy App instance and important paths.

### Get Info

```http
GET /api/info
```

**Request:**

- No request body.
- Requires authentication (admin or authenticated user).

**Response:**

```json
{
  "version": "v1.0.0",
  "cwd": "/path/to/projects",
  "cache": "/path/to/cache",
  "config": "/path/to/config",
  "template": "/path/to/templates"
}
```

**Fields:**

- `version` (string): Current backend version (`version.CURRENT_BACKEND_VERSION`).
- `cwd` (string): Projects directory path (where project data is stored).
- `cache` (string): Cache directory path (used for temporary/output files).
- `config` (string): Config directory path.
- `template` (string): Template directory path.

## Proxy Management

### Start Proxy

Starts a new proxy instance and creates a proxy record in the database. Each proxy has its own intercept setting and filter configuration.

```http
POST /api/proxy/start
```

**Request Body:**

```json
{
  "http": "127.0.0.1:8080",
  "browser": "chrome|firefox|safari|terminal|proxy",
  "name": "My Proxy Instance"
}
```

**Fields:**

- `http` (string, optional): The listen address for the proxy (e.g., "127.0.0.1:8080"). Defaults to "127.0.0.1:9797" if not provided with a browser. Auto-adjusts to available port.
- `browser` (string, optional): Browser to launch with this proxy. Options: "chrome", "firefox", "safari", "terminal", "proxy". Leave empty for proxy only.
- `name` (string, optional): Custom name for the proxy instance. Auto-generated if not provided (e.g., "chrome 1", "firefox 2").

**Response (Success):**

```json
{
  "id": "______________1",
  "listenAddr": "127.0.0.1:8080",
  "label": "chrome 1",
  "browser": "chrome"
}
```

**Response Fields:**

- `id` (string): Unique proxy ID (15 chars format)
- `listenAddr` (string): The address the proxy is listening on
- `label` (string): Display name for this proxy instance
- `browser` (string): Browser type launched with this proxy

**Error Response:**

```json
{
  "error": "port not available",
  "availableHost": "127.0.0.1:8081"
}
```

---

### Stop Proxy

Stops a running proxy instance and terminates any associated browser/terminal process.

```http
POST /api/proxy/stop
```

**Request Body:**

```json
{
  "id": "______________1"
}
```

**Fields:**

- `id` (string, optional): The unique proxy ID to stop. If empty or not provided, stops ALL running proxies.

**Response:**

```json
{
  "message": "Proxy stopped"
}
```

**Notes:**

- The proxy record in `_proxies` collection is NOT deleted, only the runtime instance is stopped
- State field is set to empty (`""`) in the database
- Associated browser/terminal process is terminated

---

### Restart Proxy

Restarts a previously stopped proxy using its existing configuration from the database.

```http
POST /api/proxy/restart
```

**Request Body:**

```json
{
  "id": "______________1"
}
```

**Fields:**

- `id` (string, required): The unique proxy ID to restart

**Response:**

```json
{
  "id": "______________1",
  "listenAddr": "127.0.0.1:8080",
  "label": "chrome 1",
  "browser": "chrome"
}
```

**Error Responses:**

- 400 Bad Request - Missing ID
- 404 Not Found - Proxy record not found
- 409 Conflict - Proxy already running or port unavailable

---

### List Proxies

Lists all currently running proxy instances.

```http
GET /api/proxy/list
```

**Response:**

```json
{
  "proxies": [
    {
      "id": "______________1",
      "listenAddr": "127.0.0.1:8080",
      "label": "chrome 1",
      "browser": "chrome",
      "browserPid": 12345
    }
  ],
  "count": 1
}
```

---

### Take Screenshot

Captures a screenshot using the Chrome browser attached to a proxy instance via Chrome DevTools Protocol.

```http
POST /api/proxy/screenshot
```

**Request Body:**

```json
{
  "id": "______________1",
  "url": "https://example.com",
  "fullPage": true,
  "saveFile": false
}
```

**Fields:**

- `id` (string, required): The proxy ID with Chrome browser attached
- `url` (string, optional): URL to navigate to before capturing. If empty, captures the current active tab
- `fullPage` (boolean, optional, default: false): If true, captures the entire page including scrollable content. If false, captures only the visible viewport
- `saveFile` (boolean, optional, default: false): If true, saves the screenshot to disk in the cache directory and returns the file path

**Response (Success):**

```json
{
  "screenshot": "iVBORw0KGgoAAAANSUhEUgAAAAUA...",
  "filePath": "/path/to/cache/screenshot-20260121-103045.png",
  "size": 52480,
  "timestamp": "2026-01-21T10:30:45Z"
}
```

**Fields:**

- `screenshot` (string): Base64-encoded PNG image data
- `filePath` (string, optional): Full path to saved file (only present if `saveFile` was true)
- `size` (number): Size of the screenshot in bytes
- `timestamp` (string): ISO 8601 timestamp when screenshot was captured

**Error Responses:**

- 400 Bad Request - Missing or invalid request body
- 403 Forbidden - Not authenticated
- 404 Not Found - Proxy ID not found
- 500 Internal Server Error - Failed to capture screenshot (see error message for details)

**Error Examples:**

```json
{
  "error": "Proxy ______________1 not found"
}
```

```json
{
  "error": "proxy ______________1 does not have a Chrome browser attached (browser: firefox)"
}
```

```json
{
  "error": "failed to get Chrome debug URL: open /path/to/profile/DevToolsActivePort: no such file or directory"
}
```

**Notes:**

- Only works with proxy instances that have Chrome browser attached (`"browser": "chrome"`)
- Chrome must be launched with `--remote-debugging-port=0` flag (enabled by default)
- If `url` is provided, the browser will navigate to that URL before capturing
- Full page screenshots may take longer for pages with lots of content
- Screenshot is always returned as PNG format
- The Chrome DevTools Protocol connection uses a 30-second timeout

**Requirements:**

- Proxy must be running with Chrome browser
- Chrome process must be alive and responsive
- DevToolsActivePort file must exist in Chrome's profile directory

---

### Click Element

Clicks an element on the page using the Chrome browser attached to a proxy instance via Chrome DevTools Protocol.

```http
POST /api/proxy/click
```

**Request Body:**

```json
{
  "id": "______________1",
  "url": "https://example.com",
  "selector": "#submit-button",
  "waitForNavigation": false
}
```

**Fields:**

- `id` (string, required): The proxy ID with Chrome browser attached
- `url` (string, optional): URL to navigate to before clicking. If empty, operates on the current active page
- `selector` (string, required): CSS selector for the element to click (e.g., "#button-id", ".class-name", "button[type='submit']")
- `waitForNavigation` (boolean, optional, default: false): If true, waits for page navigation after click (useful for form submissions or links)

**Response (Success):**

```json
{
  "success": true,
  "message": "Element clicked successfully",
  "selector": "#submit-button",
  "timestamp": "2026-01-21T10:30:45Z"
}
```

**Fields:**

- `success` (boolean): Always true on success
- `message` (string): Success message
- `selector` (string): The CSS selector that was clicked
- `timestamp` (string): ISO 8601 timestamp when element was clicked

**Error Responses:**

- 400 Bad Request - Missing or invalid request body
- 403 Forbidden - Not authenticated
- 404 Not Found - Proxy ID not found
- 500 Internal Server Error - Failed to click element (see error message for details)

**Error Examples:**

```json
{
  "error": "Proxy ______________1 not found"
}
```

```json
{
  "error": "Selector is required"
}
```

```json
{
  "error": "failed to click element: context deadline exceeded"
}
```

**CSS Selector Examples:**

- `#login-button` - Element with ID "login-button"
- `.submit-btn` - Element with class "submit-btn"
- `button[type='submit']` - Submit button by attribute
- `a[href='/logout']` - Link with specific href
- `input[name='username']` - Input field by name
- `div.container > button:first-child` - Complex selector

**Notes:**

- Only works with proxy instances that have Chrome browser attached (`"browser": "chrome"`)
- Chrome must be launched with `--remote-debugging-port=0` flag (enabled by default)
- Element must be visible on the page before clicking
- If `url` is provided, the browser will navigate to that URL before clicking
- Use `waitForNavigation: true` for elements that trigger page navigation (form submits, links)
- The Chrome DevTools Protocol connection uses a 30-second timeout
- Supports all standard CSS selectors

**Requirements:**

- Proxy must be running with Chrome browser
- Chrome process must be alive and responsive
- DevToolsActivePort file must exist in Chrome's profile directory
- Target element must be visible and clickable

---

### Get Clickable Elements

Extracts information about all clickable elements on the page (buttons, links, inputs) to help identify what can be clicked.

```http
POST /api/proxy/elements
```

**Request Body:**

```json
{
  "id": "______________1",
  "url": "https://example.com"
}
```

**Fields:**

- `id` (string, required): The proxy ID with Chrome browser attached
- `url` (string, optional): URL to navigate to before extracting elements. If empty, analyzes the current active page

**Response (Success):**

```json
{
  "elements": [
    {
      "selector": "#login-button",
      "tagName": "button",
      "id": "login-button",
      "class": "btn btn-primary",
      "text": "Sign In",
      "type": "submit",
      "href": "",
      "name": "",
      "aria": "Login button",
      "placeholder": ""
    },
    {
      "selector": "a.nav-link[href='/about']",
      "tagName": "a",
      "id": "",
      "class": "nav-link",
      "text": "About Us",
      "type": "",
      "href": "https://example.com/about",
      "name": "",
      "aria": "",
      "placeholder": ""
    },
    {
      "selector": "input.search[type='text']",
      "tagName": "input",
      "id": "",
      "class": "search",
      "text": "",
      "type": "text",
      "href": "",
      "name": "q",
      "aria": "Search",
      "placeholder": "Enter search term..."
    }
  ],
  "count": 3,
  "timestamp": "2026-01-21T10:30:45Z"
}
```

**Element Fields:**

- `selector` (string): CSS selector that can be used with `/api/proxy/click` endpoint
- `tagName` (string): HTML tag name (button, a, input, etc.)
- `id` (string): Element ID attribute (empty if not present)
- `class` (string): Element class attribute (empty if not present)
- `text` (string): Visible text content or input value (truncated to 100 chars)
- `type` (string): Input/button type (submit, button, text, etc.)
- `href` (string): Link destination (for anchor tags)
- `name` (string): Name attribute
- `aria` (string): ARIA label for accessibility
- `placeholder` (string): Placeholder text (for input fields)

**Error Responses:**

- 400 Bad Request - Missing proxy ID
- 403 Forbidden - Not authenticated
- 404 Not Found - Proxy ID not found
- 500 Internal Server Error - Failed to extract elements

**Notes:**

- Only works with proxy instances that have Chrome browser attached
- Extracts buttons, links, submit inputs, and elements with onclick handlers
- Hidden elements (width/height = 0) are automatically filtered out
- Selectors are auto-generated: prioritizes ID, then class+tag, then tag+attribute
- Text content is truncated to 100 characters for readability
- Use the returned `selector` field directly with `/api/proxy/click`

**Use Case - AI-Assisted Clicking:**

1. Call `/api/proxy/elements` to get list of clickable elements
2. AI/User reviews the elements and their text/descriptions
3. AI/User identifies the target element by its text or purpose
4. Use the `selector` from that element with `/api/proxy/click`

**Example Workflow:**

```javascript
// Step 1: Get elements
POST /api/proxy/elements
{ "id": "______________1", "url": "https://example.com" }

// Response shows: { "selector": "#login-button", "text": "Sign In", ... }

// Step 2: Click the identified element
POST /api/proxy/click
{ "id": "______________1", "selector": "#login-button" }
```

**Requirements:**

- Proxy must be running with Chrome browser
- Chrome process must be alive and responsive
- DevToolsActivePort file must exist in Chrome's profile directory

---

## Intercept

### Handle Intercept Action

Processes an intercept action (forward or drop) for a pending request/response. Optionally includes edited request or response data.

```http
POST /api/intercept/action
```

**Request Body:**

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

**Fields:**

- `id` (string, required): The intercept ID
- `action` (string, required): Either "forward" or "drop"
- `is_req_edited` (boolean, optional): Whether the request was edited
- `is_resp_edited` (boolean, optional): Whether the response was edited
- `req_edited` (string, optional): Raw HTTP request string (required if `is_req_edited` is true)
- `resp_edited` (string, optional): Raw HTTP response string (required if `is_resp_edited` is true)

**Response:**

```json
{
  "success": true,
  "message": "Intercept action processed successfully"
}
```

**Error Responses:**

- 400 Bad Request - Invalid action or missing ID
- 403 Forbidden - Unauthorized
- 404 Not Found - Intercept ID not found

---

## Playground

Playground provides a hierarchical workspace for organizing security testing tools (repeater, fuzzer) into folders and projects.

### New Playground Item

Creates a new playground item (folder or workspace container).

```http
POST /api/playground/new
```

**Request Body:**

```json
{
  "name": "My Workspace",
  "parent_id": "",
  "type": "playground",
  "expanded": false
}
```

**Fields:**

- `name` (string, optional): Name of the playground item. Defaults to "New Playground".
- `parent_id` (string, optional): Parent item ID for nesting. Empty for root level.
- `type` (string, optional): Type of item. Defaults to "playground".
- `expanded` (boolean, optional): Whether the item is expanded in UI. Defaults to false.

**Response:**

```json
{
  "id": "string",
  "name": "My Workspace",
  "type": "playground",
  "parent_id": "",
  "sort_order": 1000,
  "expanded": false
}
```

---

### Add Playground Items

Adds one or more tool items (repeater, fuzzer) to a playground folder.

```http
POST /api/playground/add
```

**Request Body:**

```json
{
  "parent_id": "string",
  "items": [
    {
      "name": "Test Request",
      "original_id": "string",
      "type": "repeater|fuzzer",
      "tool_data": {
        "url": "https://example.com",
        "req": "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n",
        "resp": "HTTP/1.1 200 OK\r\n...",
        "data": {...},
        "extra": {
          "variables": []
        }
      }
    }
  ]
}
```

**Fields:**

- `parent_id` (string, required): Parent playground item ID
- `items` (array, required): Array of tool items to add
  - `name` (string): Name for the tool item
  - `type` (string): "repeater" or "fuzzer"
  - `tool_data` (object): Tool-specific data (URL, request, response, etc.)

**Response:**

```json
{
  "success": true,
  "items": [...]
}
```

---

### Delete Playground Item

Deletes a playground item and all its children recursively.

```http
POST /api/playground/delete
```

**Request Body:**

```json
{
  "id": "string"
}
```

**Fields:**

- `id` (string, required): The playground item ID to delete

**Response:**

```json
{
  "success": true,
  "id": "string"
}
```

**Notes:**

- Recursively deletes all child items
- Automatically deletes associated collections for repeater/fuzzer items

---

## Request Modification

### Modify Request

Applies a series of transformation actions to an HTTP request without sending it. Useful for testing request modifications before sending.

```http
POST /api/request/modify
```

**Request Body:**

```json
{
  "request": "GET /api/test HTTP/1.1\r\nHost: example.com\r\nUser-Agent: Mozilla/5.0\r\n\r\n",
  "url": "https://example.com/api/test",
  "tasks": [
    {
      "action": "set",
      "key": "req.method",
      "value": "POST"
    },
    {
      "action": "replace",
      "search": "Mozilla",
      "value": "CustomAgent",
      "regex": false
    },
    {
      "action": "delete",
      "key": "req.headers.User-Agent"
    }
  ]
}
```

**Fields:**

- `request` (string, required): Raw HTTP request string
- `url` (string, required): Full URL of the request
- `tasks` (array, required): Array of action objects to apply

**Action Types:**

1. **Set Action** - Sets or updates a specific field:

   ```json
   {
     "action": "set",
     "key": "req.method|req.url|req.path|req.query.{param}|req.headers.{header}|req.body",
     "value": "new value"
   }
   ```

2. **Replace Action** - Replaces text in the entire request:

   ```json
   {
     "action": "replace",
     "search": "search string or regex pattern",
     "value": "replacement value",
     "regex": false
   }
   ```

   - `search` (string, required): Text to search for (or regex pattern if `regex: true`)
   - `value` (string, required): Replacement text
   - `regex` (boolean, optional): Use regex matching (default: false)

3. **Delete Action** - Removes a specific field:
   ```json
   {
     "action": "delete",
     "key": "req.method|req.url|req.path|req.query.{param}|req.headers.{header}|req.body"
   }
   ```

**Supported Keys:**

- `req.method` - HTTP method (GET, POST, etc.)
- `req.url` - Full URL
- `req.path` - URL path
- `req.query.{paramName}` - Specific query parameter
- `req.headers.{headerName}` - Specific header
- `req.body` - Request body

**Response:**

```json
{
  "success": "true",
  "request": "POST /api/test HTTP/1.1\r\nHost: example.com\r\n\r\n"
}
```

**Response Fields:**

- `success` (string): "true" on success
- `request` (string): The modified raw HTTP request

**Features:**

- Actions are applied sequentially in the order provided
- Request is automatically re-parsed after modifications to maintain consistency
- Headers, query parameters, and body are properly updated
- Supports both simple string replacement and regex-based replacement
- All request fields (method, URL, headers, etc.) stay synchronized

**Example Use Cases:**

1. Change request method:

   ```json
   { "action": "set", "key": "req.method", "value": "POST" }
   ```

2. Add/update a header:

   ```json
   {
     "action": "set",
     "key": "req.headers.Authorization",
     "value": "Bearer token123"
   }
   ```

3. Replace session tokens:

   ```json
   {
     "action": "replace",
     "search": "session=[^;]+",
     "value": "session=newsession",
     "regex": true
   }
   ```

4. Remove a query parameter:

   ```json
   { "action": "delete", "key": "req.query.debug" }
   ```

5. Update request body:
   ```json
   { "action": "set", "key": "req.body", "value": "{\"new\":\"data\"}" }
   ```

**Error Responses:**

- 400 Bad Request - Invalid request body or malformed actions
- 403 Forbidden - Unauthorized
- 500 Internal Server Error - Error processing actions

---

## Repeater

The Repeater allows you to send modified HTTP requests and analyze responses.

### Send Repeater Request

Sends a raw HTTP request and saves both request and response to the database.

```http
POST /api/repeater/send
```

**Request Body:**

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

**Fields:**

- `host` (string, required): Target hostname (without protocol)
- `port` (string, optional): Target port (defaults: 443 for TLS, 80 for non-TLS)
- `tls` (boolean, required): Whether to use TLS/HTTPS
- `request` (string, required): Raw HTTP request
- `timeout` (float64, required): Timeout in seconds
- `http2` (boolean, optional): Use HTTP/2 protocol (default: false)
- `index` (float64, required): Primary index for organizing requests
- `url` (string, optional): Full URL for reference

**Response:**

```json
{
  "response": "HTTP/1.1 200 OK\r\n...",
  "time": "123ms",
  "userdata": {
    "id": "_______________",
    "index": 1.0,
    "index_minor": 0.0,
    "is_https": true,
    "host": "https://example.com",
    "port": "443",
    "has_resp": true,
    "req_json": {...},
    "resp_json": {...},
    "generated_by": "repeater"
  }
}
```

**Features:**

- Auto-increments `index_minor` using counter system
- Automatically saves to `_req`, `_resp`, `_data`, and `_attached` collections
- Updates sitemap based on request path
- Supports HTTP/1.1 and HTTP/2
- Marks all data with `generated_by: "repeater"`

---

## Filters

### Check Filter

Evaluates a dadql filter expression against provided data.

```http
POST /api/filter/check
```

**Request Body:**

```json
{
  "filter": "status == 200 && method == 'GET'",
  "columns": {
    "status": 200,
    "method": "GET",
    "path": "/api/test"
  }
}
```

**Fields:**

- `filter` (string, required): dadql filter expression
- `columns` (object, required): Data to evaluate against the filter

**Response (Valid Filter):**

```json
{
  "ok": true,
  "match": true
}
```

**Response (Invalid Filter):**

```json
{
  "ok": false,
  "error": "parse or evaluation error message"
}
```

---

## Extractor

The Extractor lets you export request/response data for a specific host into a JSONL file, using the same field structure as filters and `_data` records.

### Extract Data

Extracts records for a host and writes selected fields to a file, one JSON object per line.

```http
POST /api/extract
```

**Request Body:**

```json
{
  "host": "http://detectportal.firefox.com",
  "fields": [
    "created",
    "host",
    "id",
    "index",
    "index_minor",
    "port",
    "is_req_edited",
    "is_resp_edited",
    "is_https",
    "has_params",
    "has_resp",
    "req.method",
    "req.url",
    "req.path",
    "req.query",
    "req.headers",
    "resp.status",
    "resp.mime",
    "resp.title",
    "resp.headers",
    "req_edited.method",
    "req_edited.url",
    "resp_edited.status",
    "resp_edited.mime",
    "req.raw",
    "resp.raw",
    "req_edited.raw",
    "resp_edited.raw"
  ],
  "outputFile": "/path/to/output.jsonl"
}
```

**Fields:**

- `host` (string, required): Host to match records on. Can be with or without scheme (e.g. `detectportal.firefox.com`, `http://detectportal.firefox.com`).
- `fields` (array|string, optional):
  - Array of field names or a comma-separated string.
  - Supports:
    - Top-level: `created`, `host`, `id`, `index`, `index_minor`, `port`, `is_req_edited`, `is_resp_edited`, `is_https`, `has_params`, `has_resp`, `http`, `proxy_id`, `generated_by`
    - Request JSON: `req.method`, `req.url`, `req.path`, `req.query`, `req.params`, `req.fragment`, `req.ext`, `req.headers`, `req.has_cookies`, `req.has_params`, `req.length`
    - Response JSON: `resp.status`, `resp.mime`, `resp.title`, `resp.headers`, `resp.length`, `resp.has_cookies`, `resp.date`, `resp.time`
    - Edited request JSON: `req_edited.*` (same structure as `req.*`)
    - Edited response JSON: `resp_edited.*` (same structure as `resp.*`)
    - Raw bodies from related collections: `req.raw`, `resp.raw`, `req_edited.raw`, `resp_edited.raw`
- `outputFile` (string, optional): Absolute or relative path for the output file.
  - If omitted, a file is created under the cache directory:  
    `cache/extract_{host}_{timestamp}.jsonl`

If `fields` is omitted, the default is:

```json
["host", "req.method", "req.url", "req.path", "req.params"]
```

**Response (Success):**

```json
{
  "success": true,
  "filePath": "/path/to/output.jsonl",
  "host": "detectportal.firefox.com",
  "fields": ["host", "req.method", "req.url", "req.path", "req.params"],
  "extractedAt": "2025-06-25T20:25:44.136Z"
}
```

Each line in the output file is a JSON object matching the filter-style structure, for example:

```json
{
  "created": "2025-06-25 20:25:44.136Z",
  "host": "http://detectportal.firefox.com",
  "id": "____________1.9",
  "index": 1,
  "index_minor": 9,
  "port": "",
  "is_req_edited": false,
  "is_resp_edited": false,
  "is_https": false,
  "has_params": true,
  "has_resp": true,
  "req": {
    "has_cookies": false,
    "method": "GET",
    "url": "/success.txt?ipv4",
    "length": 3059,
    "query": "alias=getPortfolioProjects",
    "path": "/api/graphql/v1",
    "headers": {
      "Host": "detectportal.firefox.com",
      "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"
    }
  },
  "resp": {
    "length": 8,
    "mime": "text/plain",
    "status": 200,
    "title": "New Page",
    "headers": {
      "Content-Type": "text/plain"
    }
  }
}
```

**Error Responses:**

- 400 Bad Request - Missing `host` or invalid body.
- 403 Forbidden - Unauthorized.
- 500 Internal Server Error - Failed to query or write data.

---

## Templates

### List Templates

Lists all available YAML template files.

```http
GET /api/templates/list
```

**Response:**

```json
{
  "list": [
    {
      "name": "template.yaml",
      "path": "/path/to/template.yaml",
      "is_dir": false
    }
  ]
}
```

---

### Create New Template

Creates a new template file.

```http
POST /api/templates/new
```

**Request Body:**

```json
{
  "name": "my-template.yaml",
  "content": "template: content\nhere: value"
}
```

**Response:**

```json
{
  "filepath": "/path/to/my-template.yaml"
}
```

---

### Delete Template

Deletes a template file.

```http
DELETE /api/templates/:template
```

**URL Parameters:**

- `template` (string): Template filename

**Response:**

- 200 OK on success
- 500 Internal Server Error on failure

---

## File Operations

### Read File

Reads a file from a specified folder location.

```http
POST /api/readfile
```

**Request Body:**

```json
{
  "fileName": "data.txt",
  "folder": "cache|config|cwd"
}
```

**Fields:**

- `fileName` (string, required): Name of the file to read
- `folder` (string, required): Folder location ("cache", "config", "cwd", or full path)

**Response:**

```json
{
  "filecontent": "file contents here"
}
```

---

### Save File

Saves content to a file in a specified folder.

```http
POST /api/savefile
```

**Request Body:**

```json
{
  "fileName": "output.txt",
  "fileData": "content to save",
  "folder": "cache|config|cwd"
}
```

**Fields:**

- `fileName` (string, required): Name of the file to save
- `fileData` (string, required): Content to write to the file
- `folder` (string, required): Folder location

**Response:**

```json
{
  "filepath": "/path/to/output.txt"
}
```

---

## Cook Engine

The Cook Engine provides pattern generation and string manipulation capabilities.

### Generate Patterns

Generates strings from Cook pattern syntax.

```http
POST /api/cook/generate
```

**Request Body:**

```json
{
  "pattern": ["admin{1-3}", "user@{example,test}.com"]
}
```

**Response:**

```json
{
  "results": ["admin1", "admin2", "admin3", "user@example.com", "user@test.com"]
}
```

---

### Apply Methods

Applies transformation methods to strings.

```http
POST /api/cook/apply
```

**Request Body:**

```json
{
  "strings": ["example", "TEST"],
  "methods": ["upper", "lower"]
}
```

**Response:**

```json
{
  "results": ["EXAMPLE", "test"]
}
```

---

### Search Patterns

Searches for available Cook patterns/methods.

```http
POST /api/cook/search
```

**Request Body:**

```json
{
  "search": "encode"
}
```

**Response:**

```json
{
  "search": "encode",
  "results": ["base64encode", "urlencode", "htmlencode"]
}
```

---

## Labels

Labels provide a way to tag and organize requests/responses.

### Create Label

Creates a new label.

```http
POST /api/label/new
```

**Request Body:**

```json
{
  "name": "Important",
  "color": "#FF0000",
  "type": "request"
}
```

**Fields:**

- `name` (string, required): Label name
- `color` (string, required): Color code (hex format)
- `type` (string, optional): Label type

**Response:**

- 200 OK with "Created" message

---

### Delete Label

Deletes a label by ID or name.

```http
POST /api/label/delete
```

**Request Body:**

```json
{
  "id": "string"
}
```

OR

```json
{
  "name": "Important"
}
```

**Response:**

- 200 OK with "Deleted" message

---

### Attach Label

Attaches a label to a request/response record.

```http
POST /api/label/attach
```

**Request Body:**

```json
{
  "id": "record_id",
  "name": "Important",
  "color": "#FF0000"
}
```

**Fields:**

- `id` (string, required): Record ID to attach label to
- `name` (string, required): Label name
- `color` (string, optional): Label color

**Response:**

- 200 OK with "Created" message

---

## Regex

### Search Regex

Tests if a regex pattern matches a response body.

```http
POST /api/regex
```

**Request Body:**

```json
{
  "regex": "\\bpassword\\b",
  "responseBody": "This contains password field"
}
```

**Response:**

```json
{
  "matched": true
}
```

**Error Response:**

```json
{
  "error": "invalid regex pattern"
}
```

---

## Sitemap

### New Sitemap Entry

Creates a new sitemap entry and collection for a host.

```http
POST /api/sitemap/new
```

**Request Body:**

```json
{
  "host": "https://example.com",
  "data": "endpoint_id",
  "path": "/api/users",
  "query": "page=1",
  "fragment": "section",
  "type": "endpoint",
  "ext": "json"
}
```

**Fields:**

- `host` (string, required): Full host URL
- `data` (string, required): Unique endpoint identifier
- `path` (string, required): URL path
- `query` (string, optional): Query string
- `fragment` (string, optional): URL fragment
- `type` (string, optional): Endpoint type
- `ext` (string, optional): File extension

**Response:**

- 200 OK with "Created" message

**Features:**

- Auto-creates host collection if doesn't exist
- Runs technology fingerprinting (Wappalyzer) for new hosts
- Extracts and stores page title
- Stores domain and TLD information

---

### Fetch Sitemap

Fetches sitemap data in hierarchical tree structure.

```http
POST /api/sitemap/fetch
```

**Request Body:**

```json
{
  "host": "https://example.com",
  "path": "/api",
  "depth": 1
}
```

**Fields:**

- `host` (string, required): Host URL
- `path` (string, optional): Base path to fetch from (empty for all)
- `depth` (integer, optional): Tree depth limit (default: 1, -1 for unlimited)

**Response:**

```json
[
  {
    "host": "https://example.com",
    "path": "/api/users",
    "title": "users",
    "type": "endpoint",
    "ext": "json",
    "query": "page=1",
    "children": [...],
    "childrenCount": 3
  }
]
```

---

## Commands

### Run Command

Executes a shell command and saves output to a collection or file.

```http
POST /api/runcommand
```

**Request Body:**

```json
{
  "command": "ls -la",
  "data": "additional_data",
  "saveTo": "collection|file",
  "collection": "command_results",
  "filename": "output.txt"
}
```

**Fields:**

- `command` (string, required): Shell command to execute
- `data` (any, optional): Additional data to store
- `saveTo` (string, required): "collection" or "file"
- `collection` (string, conditional): Required if saveTo is "collection"
- `filename` (string, conditional): Required if saveTo is "file"

**Response:**

```json
{
  "id": "process_id"
}
```

**Notes:**

- Command is executed asynchronously
- Process state tracked in `_process` collection
- Output saved line-by-line to specified collection (if collection mode)
- Uses bash on Unix/Linux, cmd on Windows

---

## Tools

### Tool Server

Starts a tool server instance for a project.

```http
GET /api/tool/server
```

**Response:**

```json
{
  "path": "/path/to/project",
  "hostAddress": "127.0.0.1:8090",
  "id": "process_id",
  "name": "server_name",
  "username": "new@example.com",
  "password": "1234567890"
}
```

---

### Tools Instance

Starts a new PocketBase instance at a specified path.

```http
GET /api/tool?path=/path/to/project
```

**Query Parameters:**

- `path` (string, required): Path to the tool directory

**Response:**

- 200 OK with host address on success
- 500 Internal Server Error on failure

---

## Raw HTTP

### Send Raw Request

Sends a raw HTTP request directly without proxy. Supports HTTP/1.1 and HTTP/2.

```http
POST /api/sendrawrequest
```

**Request Body:**

```json
{
  "host": "example.com",
  "port": "443",
  "req": "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n",
  "tls": true,
  "timeout": 10,
  "httpversion": 1
}
```

**Fields:**

- `host` (string, required): Target hostname (protocol prefix stripped automatically)
- `port` (string, optional): Target port
- `req` (string, required): Raw HTTP request
- `tls` (boolean, required): Use TLS/HTTPS
- `timeout` (float64, required): Timeout in seconds
- `httpversion` (float64, required): HTTP version (1 for HTTP/1.1, 2 for HTTP/2)

**Response:**

```json
{
  "resp": "HTTP/1.1 200 OK\r\n...",
  "time": "123ms"
}
```

---

## Certificates

### Download CA Certificate

Downloads the CA certificate for HTTPS interception. Install this certificate in your system/browser to intercept HTTPS traffic.

```http
GET /cacert.crt
```

**Response:**

- File download of the CA certificate (grroxy-ca.crt)
- Content-Type: application/x-x509-ca-cert

---

## Error Responses

All endpoints may return the following error responses:

- **400 Bad Request** - Invalid request body or parameters
- **403 Forbidden** - Not authenticated
- **404 Not Found** - Resource not found
- **500 Internal Server Error** - Server error

Error response format:

```json
{
  "error": "Error message description"
}
```

---

## Authentication

All API endpoints (except `/cacert.crt`) require authentication. Authentication is handled via:

- Admin credentials
- User authentication record

Unauthenticated requests return `403 Forbidden`.

---

## Data Collections

The app uses the following main database collections:

- `_proxies` - Proxy instances and their configuration
- `_data` - Request/response data records
- `_req` - Raw request data
- `_resp` - Raw response data
- `_attached` - Metadata and relationships
- `_intercept` - Pending intercept requests
- `_playground` - Playground workspace items
- `repeater_{id}` - Repeater tabs for each playground item
- `intruder_{id}` - Fuzzer tabs for each playground item
- `_labels` - Label definitions
- `label_{id}` - Records tagged with each label
- `_hosts` - Discovered hosts and their info
- `_tech` - Technology fingerprints
- `{host_collection}` - Sitemap for each host
- `_process` - Background process tracking

---

## Version

This documentation is for Grroxy App API v1.0
