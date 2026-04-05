# Grroxy

Cybersecurity proxy toolkit that blends manual web testing with AI agents. Intercepts HTTP/HTTPS traffic, provides request modification, fuzzing, browser automation, and an MCP endpoint for AI-driven security analysis. Built as a multi-binary Go backend with a Svelte frontend.

**Version:** 2026.4.1 (App) / 0.29.0 (Backend & Frontend)

## Tech Stack

- **Backend:** Go 1.24 (toolchain go1.24.1)
- **Database:** PocketBase (forked: `github.com/glitchedgitz/pocketbase`)
- **Frontend:** Svelte 5 + Vite + Tailwind CSS + SvelteKit (dev UI)
- **Desktop:** Electron 36.4.0
- **CLI:** cobra (command framework)
- **HTTP:** echo/v5 (web framework), custom raw HTTP parser, uTLS
- **Browser:** chromedp (Chrome DevTools Protocol)
- **Key libs:** cook/v2 (payload generation), dadql (query language), wappalyzergo (tech detection), mcp-go (Model Context Protocol), fsnotify (file watching)

## Architecture

### Binaries

| Binary        | Entry Point              | Purpose                                                                        |
| ------------- | ------------------------ | ------------------------------------------------------------------------------ |
| `grroxy`      | `cmd/grroxy/main.go`     | **Launcher** - manages projects, starts per-project backends, global templates |
| `grroxy-app`  | `cmd/grroxy-app/main.go` | **Project backend** - proxy, intercept, templates, all per-project APIs        |
| `grroxy-tool` | `cmd/grroxy-tool/`       | Standalone tools server (fuzzer, SDK)                                          |
| `grx-fuzzer`  | `cmd/grx-fuzzer/main.go` | Standalone HTTP/HTTP2 fuzzer CLI                                               |
| `grxp`        | `cmd/grxp/main.go`       | URL parser/prober from stdin with dadql filtering                              |

### Directory Layout

```
apps/
  app/           # grroxy-app backend logic (~40 files: proxy, intercept, templates, MCP, etc.)
  launcher/      # grroxy launcher backend (project management, template distribution)
  tools/         # grroxy-tool backend (fuzzer)
cmd/
  grroxy/        # Main CLI entry + migrations
  grroxy-app/    # Project app entry + migrations
  grroxy-tool/   # Tool server entry + migrations
  electron/      # Electron desktop wrapper
  grx-fuzzer/    # Fuzzer CLI
  grxp/          # URL parser CLI
grx/
  browser/       # Chrome automation (chromedp wrappers)
  dev/           # Developer UI (SvelteKit, served at /dev)
  frontend/      # Main production frontend (Svelte, bundled into binary)
  fuzzer/        # Fuzzing engine (cluster bomb, pitch fork modes)
  rawhttp/       # Raw HTTP/1.1 & HTTP/2 parser and client
  rawproxy/      # MITM proxy with TLS, cert generation, WebSocket support
  templates/     # Template engine with hooks and default configs
  version/       # Version constants
internal/
  config/        # Config struct and initialization
  logflags/      # Log flag setup
  process/       # Process/command management
  save/          # File saving utilities
  schemas/       # PocketBase collection schemas (22 files)
  sdk/           # Client SDK
  types/         # Shared type definitions
  updater/       # Binary self-updater (GitHub Releases)
  utils/         # Utility functions
```

### Data Flow

1. `grroxy start` boots the **launcher** (default `127.0.0.1:8090`), manages projects via PocketBase
2. Opening a project spawns a **`grroxy-app`** subprocess with `--host`, `--path`, `--launcher` flags
3. `grroxy-app` starts its own PocketBase instance in the project directory, boots proxy listeners
4. Proxy intercepts HTTP/HTTPS traffic via MITM, stores requests/responses in PocketBase collections
5. Frontend communicates with both launcher and project APIs
6. Templates with hooks (`on_request`, `on_response`, `on_new_sitemap`) run automatically on proxy traffic

### Config Directories

- **Config:** `~/.config/grroxy/` (CA certs, config)
- **Projects:** `$XDG_CONFIG_DIR/grroxy/` (project databases, launcher DB in `grroxy-main/`)
- **Cache:** `$XDG_CACHE_DIR/grroxy/`
- **Templates:** configurable via `GRROXY_TEMPLATE_DIR` env var

## Build & Run

### Backend

```bash
# Build all binaries
go build ./cmd/grroxy
go build ./cmd/grroxy-app
go build ./cmd/grroxy-tool

# Run launcher
./grroxy start
./grroxy start --host 127.0.0.1:9090  # custom port

# Run project app directly (normally spawned by launcher)
./grroxy-app --host 127.0.0.1:8091 --path /path/to/project --launcher 127.0.0.1:8090

# Run migrations
./grroxy migrate up
```

### Frontend (Dev UI)

```bash
cd grx/dev
npm install
npm run dev       # Dev server
npm run build     # Production build
npm run check     # TypeScript + Svelte check
```

### Frontend (Main)

```bash
cd grx/frontend
npm install
npm run dev
npm run build
```

### Electron Desktop

```bash
cd cmd/electron
npm install
npm start         # Dev mode
npm run build     # Build installers (DMG/NSIS/AppImage)
```

### Release (cross-compile all platforms)

```bash
./release.sh  # Builds darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64
```

## CLI Flags

```
grroxy-app:
  --host      Host address (default: 127.0.0.1:8090)
  --proxy     Proxy address (default: 127.0.0.1:8888)
  --path      Project directory path (required)
  --launcher  Launcher address for template sync
  --log       Enable debug logs (default: false)
```

## Database

Uses PocketBase with migration-based schema management. Each binary has its own migrations in `cmd/<binary>/migrations/`.

### Key Collections

| Collection                    | Purpose                                                                          |
| ----------------------------- | -------------------------------------------------------------------------------- |
| `_proxies`                    | Proxy instances (addr, intercept state, browser config)                          |
| `_data` (Rows)                | HTTP request/response storage (index, host, method, relations to `_req`/`_resp`) |
| `_req`, `_resp`               | Raw request/response data (headers, body, URL parts)                             |
| `_req_edited`, `_resp_edited` | Edited versions for intercept/modify                                             |
| `_intercept`                  | Currently intercepted messages (cleared on startup)                              |
| `_templates`                  | Templates with hooks (on_request, on_response, action buttons)                   |
| `_filters`                    | Filter rules (dadql expressions)                                                 |
| `_sitemap`                    | Discovered endpoints (path, query, fragment, type)                               |
| `_websockets`                 | WebSocket message capture                                                        |
| `_projects`                   | Project metadata (launcher only)                                                 |
| `_settings`                   | Key-value settings                                                               |
| `_tools`                      | Tool server configurations                                                       |

### Schema Files

All in `internal/schemas/` - each file exports a function returning `*models.Collection` with field definitions. Field types: Text, Number, Bool, Json (max 100-500KB), Relation, Date.

## API Routes

Routes registered in `cmd/grroxy-app/serve.go` via `API.App.OnBeforeServe().Add(...)`. Key groups:

- **Proxy:** `/api/proxy/{start,stop,restart,list,screenshot,click,...}`
- **Intercept:** `/api/intercept/action`, `/api/modify/request`
- **Templates:** `/api/templates/{list,new,delete,toggle,global-toggle,reload}`
- **Requests:** `/api/add-request`, `/api/send-repeater`, `/api/send-raw-request`
- **Cook:** `/api/cook/{search,generate,apply-methods}`
- **Files:** `/api/readfile`, `/api/savefile`, `/api/cwd/{content,browse,read-file}`
- **MCP:** `/api/mcp` (Model Context Protocol endpoint)
- **Browser:** `/api/proxy/{open-chrome-tab,navigate,evaluate,...}`
- **System:** `/api/info`, `/cacert.crt`, `/api/check-update`, `/api/do-update`

Launcher routes in `cmd/grroxy/main.go`: project CRUD, tool servers, template management.

## Testing

```bash
go test ./...                    # Run all tests
go test ./grx/rawhttp/...       # Raw HTTP parser tests
go test ./grx/fuzzer/...        # Fuzzer tests
go test ./grx/templates/...     # Template tests
go test ./internal/utils/...    # Utility tests
```

Test data files in `grx/rawhttp/test/` (19+ edge case HTTP request/response samples).

## Code Conventions

- **Route registration:** Each route handler is a method on `Backend` or `Launcher` struct, registered via `OnBeforeServe().Add()` pattern
- **Error handling:** `utils.CheckErr()` for fatal errors during init; `log.Printf` + return nil for non-fatal runtime errors
- **Schema definitions:** One file per collection in `internal/schemas/`, function returns `*models.Collection`
- **Frontend APIs:** Typed API clients in `grx/dev/src/lib/{api,app-api,launcher-api,tool-api}.ts`
- **Logging:** `log.SetFlags(log.LstdFlags | log.Lmicroseconds)` - timestamps with microseconds
- **Migration naming:** `<unix_timestamp>_<description>.go`
- **PocketBase records:** accessed via `API.App.Dao().FindRecordById()`, `FindRecordsByExpr()`, etc.

## Template System

Templates in `grx/templates/` provide hook-based automation on proxy traffic:

- **Hook types:** `on_request`, `on_response`, `on_new_sitemap`, `request-action-button`, `response-action-button`, `sitemap-action-button`
- **Default configs:** `grx/templates/defaults/` contains `mime.yaml`, `paths.yaml`, `extensions.yaml`, `proxy-configs.yaml`
- Templates are managed at the launcher level and synced to project backends via `/api/templates/reload`
- Global toggle enables/disables all template processing

## Important Patterns

- **Startup sequence** (`serve.go`): register routes -> load templates from launcher -> initialize proxy -> clear intercept table -> reset all proxy states -> setup hooks (intercept, templates, filters, counters)
- **Proxy state reset:** On every startup, all proxy records get `intercept=false, state=""` - prevents stale state from crashed sessions
- **Counter manager:** Periodic sync (1s ticker) to batch-write request counts to DB
- **Process management:** Commands run via `CmdChannel` (channel-based), managed by `CommandManager()` goroutine
- **Xterm:** Terminal sessions via WebSocket, registered separately via `API.RegisterXtermRoutes()`
- **MCP integration:** `apps/app/mcp_tools.go` (~1000+ lines) defines AI tool capabilities

## Known Bugs

- **Empty filter in `mcp_tools.go`**: When `filter = ""`, must use `FindRecordByExpr` instead of filter-based query. Applies everywhere in `mcp_tools.go`.

## Gotchas

- `grroxy-app` has a `MIGRATION_MODE` const in `main.go` - must be `false` for production, `true` only when creating new migrations via PocketBase admin
- PocketBase is a **fork** (`github.com/glitchedgitz/pocketbase`), not upstream - don't reference upstream PocketBase docs for API differences
- `grx/frontend/` is marked private (not on main branch for public repo) - the dev UI at `grx/dev/` is the open one
- CA certificates auto-generated on first `grroxy start` in `~/.config/grroxy/`
- Intercept table (`_intercept`) is wiped on every startup - by design
- Proxy address flag exists but is unused (`// removed, we use api now`) - proxy starts are API-driven
- `cook` is another tool by the same author for payload/wordlist generation
- `dadql` is a custom query language for filtering requests

## Current Features

- **Proxy:** MITM HTTP/HTTPS interception with request/response modification
- **Intercept:** Pause and edit requests/responses before forwarding
- **Repeater:** Resend modified requests
- **Fuzzer:** Cluster bomb and pitch fork modes with concurrent execution
- **Browser automation:** Chrome control via DevTools Protocol
- **CWD explorer:** File/folder browser (VSCode-like) in frontend
- **Templates:** Hook-based automation on proxy traffic
- **MCP:** AI agent integration endpoint
- **Cook integration:** Payload generation and transformation
- **Wappalyzer:** Technology detection on proxied sites
- **Self-update:** `grroxy update` pulls from GitHub Releases
