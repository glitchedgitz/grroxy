# Grroxy Launcher API Documentation

## Overview

The Grroxy Launcher is a project management application that allows you to create, manage, and launch multiple Grroxy instances (projects). Each project is an isolated workspace with its own database and configuration.

**Base URL**: `http://{host}:{port}/api`

---

## Table of Contents

- [Projects](#projects)
- [Templates](#templates)
- [File Operations](#file-operations)
- [Cook Engine](#cook-engine)
- [Commands](#commands)
- [Utilities](#utilities)
- [Update](#update)
- [Certificates](#certificates)

---

## Projects

Project management endpoints for creating and launching isolated Grroxy instances.

### List Projects

Lists all existing projects.

```http
GET /api/project/list
```

**Response:**

```json
[
  {
    "id": "unique_project_id",
    "name": "My Project",
    "path": "/path/to/project",
    "data": {
      "ip": "127.0.0.1:8091",
      "state": "active"
    },
    "version": "1.0",
    "created": "2024-01-01T00:00:00Z",
    "updated": "2024-01-01T00:00:00Z"
  }
]
```

**Response Fields:**

- `id` (string): Unique project identifier
- `name` (string): Project name
- `path` (string): Filesystem path to project directory
- `data` (object): Project state information
  - `ip` (string): Host address where project is running (empty if inactive)
  - `state` (string): "active" or "unactive"
- `version` (string): Project version
- `created` (string): Creation timestamp
- `updated` (string): Last update timestamp

---

### Create New Project

Creates a new project with its own directory and database.

```http
POST /api/project/new
```

**Request Body:**

```json
{
  "name": "My New Project"
}
```

**Fields:**

- `name` (string, required): Project name (cannot be empty or whitespace)

**Response:**

```json
{
  "id": "ckx7y8z9a0001",
  "name": "My New Project",
  "path": "/projects/ckx7y8z9a0001",
  "data": {
    "ip": "127.0.0.1:8091",
    "state": "active"
  }
}
```

**Features:**

- Automatically creates project directory
- Generates unique project ID
- Finds available port for the project
- Launches project immediately after creation
- State automatically set to "active"

**Error Responses:**

- 400 Bad Request - Empty or whitespace project name
- 500 Internal Server Error - Failed to create project

---

### Open Project

Opens (launches) an existing project. If already running, returns existing connection info.

```http
POST /api/project/open
```

**Request Body:**

```json
{
  "project": "My Project"
}
```

OR

```json
{
  "project": "ckx7y8z9a0001"
}
```

**Fields:**

- `project` (string, required): Project name OR project ID

**Response:**

```json
{
  "id": "ckx7y8z9a0001",
  "name": "My Project",
  "path": "/projects/ckx7y8z9a0001",
  "data": {
    "ip": "127.0.0.1:8092",
    "state": "active"
  }
}
```

**Behavior:**

- If project is already active, returns existing host address
- If project is inactive, finds new available port and launches it
- Updates project state to "active" in database
- Launches `grroxy-app` process in background

**Error Responses:**

- 400 Bad Request - Empty project name/ID
- 404 Not Found - Project doesn't exist
- 500 Internal Server Error - Failed to start project

---

## Templates

Template management for creating reusable request/response templates.

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
      "name": "auth-template.yaml",
      "path": "/templates/auth-template.yaml",
      "is_dir": false
    }
  ]
}
```

**Features:**

- Only lists `.yaml` and `.yml` files
- Ignores hidden files (starting with `.`)
- Ignores directories

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
  "content": "request:\n  method: GET\n  path: /api/test"
}
```

**Fields:**

- `name` (string, required): Template filename (should end with .yaml or .yml)
- `content` (string, required): Template content in YAML format

**Response:**

```json
{
  "filepath": "/templates/my-template.yaml"
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

**Example:**

```http
DELETE /api/templates/my-template.yaml
```

**Response:**

- 200 OK - File deleted successfully
- 500 Internal Server Error - Error deleting file

---

## File Operations

File read/write operations for accessing project files.

### Read File

Reads a file from a specified folder location.

```http
POST /api/readfile
```

**Request Body:**

```json
{
  "fileName": "config.json",
  "folder": "config"
}
```

**Fields:**

- `fileName` (string, required): Name of the file to read
- `folder` (string, required): Folder location:
  - `"cache"` - Cache directory
  - `"config"` - Projects/config directory
  - `"cwd"` - Current working directory
  - Or absolute path

**Response:**

```json
{
  "filecontent": "file contents as string"
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
  "folder": "cache"
}
```

**Fields:**

- `fileName` (string, required): Name of the file to save
- `fileData` (string, required): Content to write
- `folder` (string, required): Folder location (same options as readfile)

**Response:**

```json
{
  "filepath": "/cache/output.txt"
}
```

---

## Cook Engine

### Search Patterns

Searches for available Cook transformation patterns and methods.

```http
POST /api/cook/search
```

**Request Body:**

```json
{
  "search": "encode"
}
```

**Fields:**

- `search` (string, required): Search term

**Response (Found):**

```json
{
  "search": "encode",
  "results": ["base64encode", "urlencode", "htmlencode"]
}
```

**Response (Not Found):**

- 404 Not Found

---

## Commands

### Run Command

Executes a shell command and saves output.

```http
POST /api/runcommand
```

**Request Body:**

```json
{
  "command": "ls -la",
  "data": {...},
  "saveTo": "collection|file",
  "collection": "command_output",
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

**Features:**

- Commands run asynchronously in background
- Process state tracked in `_process` collection
- Output saved line-by-line (collection mode) or to file
- Uses bash on Unix/Linux, cmd on Windows

---

## Utilities

### SQL Test

Tests SQL queries against the database.

```http
POST /api/sqltest
```

**Request Body:**

```json
{
  "sql": "SELECT * FROM _projects LIMIT 10"
}
```

**Response:**

```json
{
  "result": [...],
  "rowCount": 10,
  "error": null
}
```

---

### Search Regex

Tests if a regex pattern matches text.

```http
POST /api/regex
```

**Request Body:**

```json
{
  "regex": "\\d{3}-\\d{4}",
  "responseBody": "Call 555-1234 today"
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
  "error": "invalid regex syntax"
}
```

---

### File Watcher

Monitors template directory for changes.

```http
GET /api/filewatcher
```

**Response:**

Server-Sent Events (SSE) stream that notifies when template files change.

**Event Format:**

```
event: file-change
data: {"file": "template.yaml", "action": "modified"}
```

---

## Tools

### Tool Server

Starts a new tool server instance.

```http
GET /api/tool/server
```

**Response:**

```json
{
  "path": "/tools",
  "hostAddress": "127.0.0.1:8090",
  "id": "tool_server_id",
  "name": "server_name",
  "username": "new@example.com",
  "password": "1234567890"
}
```

**Features:**

- Automatically finds available port
- Launches `grroxy-tool` process
- Provides authentication credentials
- Process tracked in `_process` collection

---

### Tools Instance

Starts a PocketBase instance at specified path.

```http
GET /api/tool?path=/path/to/tool
```

**Query Parameters:**

- `path` (string, required): Path to tool directory

**Response:**

- 200 OK with host address
- 500 Internal Server Error on failure

---

## Update

### Check for Update

Checks if a newer version of Grroxy is available.

```http
GET /api/update/check
```

**Response:**

```json
{
  "current_version": "0.22.0",
  "latest_version": "v0.25.0",
  "update_available": true,
  "platform": "darwin/arm64"
}
```

**Response Fields:**

- `current_version` (string): Currently installed backend version
- `latest_version` (string): Latest release tag from GitHub
- `update_available` (boolean): Whether an update is available
- `platform` (string): Current OS and architecture

---

### Perform Update

Downloads and replaces all Grroxy binaries (`grroxy`, `grroxy-app`, `grroxy-tool`) with the latest release.

```http
POST /api/update
```

**Response (Updated):**

```json
{
  "previous_version": "0.22.0",
  "new_version": "v0.25.0",
  "results": [
    {
      "binary": "grroxy",
      "status": "updated",
      "path": "/usr/local/bin/grroxy"
    },
    {
      "binary": "grroxy-app",
      "status": "updated",
      "path": "/usr/local/bin/grroxy-app"
    },
    {
      "binary": "grroxy-tool",
      "status": "skipped",
      "error": "no asset found matching \"grroxy-tool-darwin-arm64\" in release v0.25.0"
    }
  ]
}
```

**Response (Already Up to Date):**

```json
{
  "message": "already up to date",
  "version": "0.25.0"
}
```

**Response Fields:**

- `previous_version` (string): Version before the update
- `new_version` (string): Version after the update
- `results` (array): Per-binary update results
  - `binary` (string): Binary name
  - `status` (string): `"updated"`, `"skipped"`, or `"failed"`
  - `path` (string): Binary path (present when updated)
  - `error` (string): Error message (present when skipped/failed)

**Notes:**

- Uses `GITHUB_TOKEN` or `GH_TOKEN` environment variable for private repo access
- Restart Grroxy after update for changes to take effect

---

## Certificates

### Download CA Certificate

Downloads the CA certificate for HTTPS interception.

```http
GET /cacert.crt
```

**Response:**

- File download of CA certificate (grroxy-ca.crt)
- Content-Type: application/x-x509-ca-cert

**Usage:**

Install this certificate in your system/browser trust store to intercept HTTPS traffic from launched projects.

---

## Project Lifecycle

### How Projects Work

1. **Creation**: `POST /api/project/new` creates a new project directory and database
2. **Launch**: Project automatically launches `grroxy-app` with unique host address
3. **State Tracking**: Project state ("active"/"unactive") tracked in database
4. **Access**: Connect to project at the provided `ip` address
5. **Shutdown**: When `grroxy-app` process terminates, state automatically set to "unactive"
6. **Reopen**: `POST /api/project/open` relaunches inactive projects

### Project States

- **active**: Project is currently running and accessible
- **unactive**: Project exists but is not running

### Project Structure

Each project has:

- Unique ID (e.g., `ckx7y8z9a0001`)
- Dedicated directory at `{projects_dir}/{id}/`
- Isolated database file
- Own configuration
- Separate log files

---

## Error Responses

Standard error response format:

```json
{
  "error": "Error description"
}
```

**Common Status Codes:**

- **400 Bad Request** - Invalid input or parameters
- **403 Forbidden** - Authentication required
- **404 Not Found** - Resource doesn't exist
- **500 Internal Server Error** - Server-side error

---

## Authentication

Most API endpoints require authentication:

- Admin credentials
- Authenticated user record

Unauthenticated requests return `403 Forbidden`.

The static frontend (`/*`) is accessible without authentication.

---

## Data Collections

Main database collections:

- `_projects` - Project records and metadata
- `_process` - Background process tracking
- `_tools` - Tool server instances

---

## Configuration

The launcher uses these environment variables:

- `GRROXY_PROJECTS_DIR` - Root directory for all projects
- `GRROXY_TEMPLATE_DIR` - Directory for template files
- `GRROXY_CONFIG_DIR` - Configuration directory
- `GRROXY_CACHE_DIR` - Cache directory

---

## Version

This documentation is for Grroxy Launcher API v1.0

