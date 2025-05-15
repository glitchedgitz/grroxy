# API Documentation

## Playground

### New Playground

Creates or finds a folder and playground based on host, port, and url.

```http
POST /api/playground/new
```

```json
{
  "data": {
    "host": "string",
    "port": "string",
    "req": { "url": "string", ... }
  }
}
```

```json
// JSON record of the created/found playground.
```

---

### Initiate Playground with Child

Creates or finds a folder and playground, then creates a child (e.g., repeater, fuzzer) under it.

```http
POST /api/playground/initiate
```

```json
{
  "type": "repeater|fuzzer|...",
  "data": {
    "host": "string",
    "port": "string",
    "req": { "url": "string", ... }
  }
}
```

```json
// JSON record of the created child.
```

---

### Delete Playground

Deletes a playground, folder, or child record by its ID.

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

### Send Raw Request

Sends a raw HTTP request and returns the response.

```http
POST /api/sendrawrequest
```

```json
{
  "host": "string",
  "port": "string",
  "req": "string",
  "tls": true,
  "timeout": 10
}
```

```json
{
  "response": "string",
  "timeTaken": "string"
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

## File

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

## Cook

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

