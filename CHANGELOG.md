# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v2026.4.1] - v0.29.0 - Grroxy Actions

### Added

- **Template Actions Engine** — Hook-based automation on proxy traffic. Templates run on `before_request`, `request`, `response` hooks. Each template has tasks with conditions (dadql) and actions.
- **Actions:** `set`, `delete`, `replace` (string/regex), `create_label`, `send_request`
- **`_templates` collection** — DB-backed template storage on launcher with migration + default seed
- **Default templates** — `extensions`, `mime`, `paths`, `proxy-configs` embedded via `//go:embed`, seeded on first migration
- **3-level enable/disable** — Global (`_configs`), per-project (`_projects.data`), per-proxy (`_proxies.data.run_templates`). New projects and proxies default to enabled.
- **Live reload** — Launcher watches `_templates` for changes, notifies running `grroxy-app` instances via `POST /api/templates/reload`
- **`/api/templates/check`** — Validate template YAML (hooks, actions, required keys) before saving
- **`/api/templates/info`** — Returns all available hooks, actions, keys, descriptions, and full syntax reference
- **`/api/project/delete`** — Delete a project, its DB record, and project folder
- **`/api/request/modify`** — Apply template actions to raw HTTP request strings (set, delete, replace)
- **MCP template tools** — `templateList`, `templateRead`, `templateGetInfo`, `templateCreate`, `templateUpdate` for AI agents (Claude Code, MCP clients)
- **Frontend AI tools** — `templateGetInfo`, `templateList`, `templateRead`, `templateCreate`, `templateUpdate`, `templateDelete`, `templateCheck` in AI SDK
- **Template syntax reference** — Full reference in `grx/templates/reference.go`, served via `/api/templates/info` for AI agents
- **Settings UI** — Global and per-project template toggles in settings, per-proxy toggle in proxy toolbar
- **Frontend** — DB-backed template list with PocketBase subscription, editor with task disable toggles
- **Proxy auto-color** — New proxies get a color from the palette automatically. Migration assigns colors to existing proxies with empty color.
- **Frontend: `send_request` UI** — Template editor now supports `send_request` action with method, headers, and body fields
- **Frontend: `delete` action UI** — Template editor now supports `delete` action (reuses `set` UI)
- **Frontend: `ColorDialog` component** — Shared color picker used in both proxy manager and template label editor
- **Frontend: Editable author** — Template author is now inline-editable (same as title/description)
- **Frontend: Sidebar template status** — Disabled templates show dimmed icon in sidebar
- **E2E test script** — `tests/e2e_test.sh` — full integration test: project creation, template CRUD, proxy traffic, label verification, send_request, toggles, restart persistence (20 tests)
- **Cleanup script** — `tests/cleanup.sh` — removes test templates and projects

### Fixed

- **Header matching in `modify.go`** — `RequestUpdateKey` and `RequestDeleteKey` now use `strings.TrimRight(h[0], ": ")` to handle both `"Header:"` and `"Header: "` formats from rawhttp parser
- **PocketBase JSON field parsing in launcher hooks** — `forEachRunningProject` now handles `types.JsonRaw`, `string`, and `map[string]any` for the `data` field instead of silently skipping all projects
- **Project data preserved on restart** — `ResetProjectStates`, `setProjectStateClose`, and `OpenProject` now merge into existing `data` field instead of overwriting (preserves `templatesEnabled`)
- **New projects default to `templatesEnabled: true`** — `CreateNewProject` sets it in the initial data
- **New proxies default to `run_templates: true`** — `startProxyLogic` sets it in proxy data and on the live instance
- **`send_request` action** — Fixed: strips `http://`/`https://` from host, fetches raw request from `_req` DB record, normalizes `\r\n` line endings, fixes Host header for direct replay, sets `Url` field so host shows in DB

### Known Issues

- Race condition on `Templates.Templates` map (concurrent read/write from proxy goroutines and reload handlers) — needs `sync.RWMutex`
- `TemplatesEnabled` and `RunTemplates` bools need `atomic.Bool` for concurrent access
- `Content-Length` header not updated when body changes via `set` action

---

# Released v2026.3.9

Includes `v0.28.1`

- Using single frontend for electron and binary.
- Apple Developer Signed Binaries
- ZoomSlider, App loading improvements and other misc fixes

# Released v2026.3.8

Includes `v0.28.0` and `v0.27.1`

## v0.28.0 - Frontend Updates

- Added Apple Developer Signed Binaries
- **Frontend: Convert to POST / GET** — Convert requests between POST and GET, moving params between body and query string. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: Duplicate tab** — Duplicate the active data tab including all persisted filters. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: Decoder panel stay minimized** — Decoder panel stays minimized after user manually minimize it. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: Toggle sidebar** — Show/hide the sidebar. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: Custom tab names** — Ability to name tabs. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: Open in new tab** — Open a request in a new tab from proxy or data tabs. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: Auto-remove headers** — Filter to auto-hide unnecessary headers. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: Search bar** — Unified search icon for general search in Data tab. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: New request popup** — A button to create a new request.
- **Frontend: Send request shortcut** — Shortcut to send the request
- **Intercepted landing page** — Browser now opens a custom `intercepted.html` page (served via `file://`) instead of `grroxy.com`. So we don't capture unwanted traffic.
- **`/api/request/parse` endpoint** — Parse raw HTTP request/response into structured breakdown (method, path, query, headers, body). Uses existing `rawhttp.ParseRequest` and `rawhttp.ParseResponse`.
- **Frontend: Filter AddNew and Edit Popup** — Add, edit filters from UI.
- **Frontend: Proxy page** — Improved UI
- **Frontend: Cmd+Enter to send** — Keyboard shortcut to send requests.

### Fixed

- **Proxy Timeout** — There waas a 60sec timeout for intercepting the request. [@Sharo_k_h](https://x.com/Sharo_k_h)
- **Proxy Pastebin** — Proxy pastebin was not working. [@Sharo_k_h](https://x.com/Sharo_k_h)
- **Frontend: Repeater sort** — Fixed repeater index sorting bug. [@Sharo_k_h](https://x.com/Sharo_k_h)
- **Frontend: Proxy pastebin** — Fixed proxy pastebin not working. [@Sharo_k_h](https://x.com/Sharo_k_h)
- **Frontend: Long filter view** — Fixed broken view when filter is long. [@Behi_Sec](https://x.com/Behi_Sec)
- **Frontend: Title bar tooltips** — Tooltips in title bar were positioned at top-left corner.
- **Intercept counter not updating on toggle off** — Per-proxy intercept counter now resets to 0 immediately when intercept is disabled.

## [2026-MAR] - v0.27.1 - MCP Fixes, Proxy Endpoints & HTTP/2 Parsing

### Added

- **Proxy HTTP endpoints** — `/api/proxy/typetext`, `/api/proxy/waitforselector`, `/api/proxy/evaluate` for browser automation via API.
- **MCP tools** — `listHosts`, `getHostInfo`, `getNoteForHost`, `setNoteForHost`, `modifyHostLabels`, `modifyHostNotes`, `interceptToggle`, `interceptPrintRowsInDetails`, `interceptGetRawRequestAndResponse`, `interceptAction`, `proxyList`, `proxyStart`, `proxyStop`, `proxyScreenshot`, `proxyClick`, `proxyElements`, `proxyType`, `proxyEval`, `proxyWaitForSelector`, `proxyListTabs`, `proxyOpenTab`, `proxyNavigateTab`, `proxyActivateTab`, `proxyCloseTab`, `proxyReloadTab`, `proxyGoBack`, `proxyGoForward`.
- **Frontend: AI tools** — `proxyTypeText`, `proxyWaitForSelector`, `proxyEvaluate` tool definitions and handlers in AI tools panel.
- **Frontend: backend API methods** — `proxyTypeText`, `proxyWaitForSelector`, `proxyEvaluate` in `backend_app.ts`.
- **Frontend: MCP tools sorted alphabetically** in HudTerminal.
- **Frontend: CWD File Explorer** — VSCode-style file/folder explorer for browsing, opening, and previewing files from the current working directory.
- **File watcher** — `fsnotify`-based file watcher for CWD explorer live updates.
- **Chrome `GetElements` improved** — Added input/textarea/select to interactive element selectors; unique CSS selector paths using `nth-of-type`.

### Changed

- **`proxyStart` MCP tool** — Removed `browser` and `http` options; hardcoded to Chrome with auto-assigned HTTP port.

### Fixed

- **MCP `interceptToggle` not intercepting** — `dao.SaveRecord()` doesn't trigger `OnRecordAfterUpdateRequest` hooks; now sets `inst.Proxy.Intercept` directly in memory.
- **Edited request parsing: `HTTP/2` rejected** — `http.ReadRequest` requires `major.minor` format; normalizes `HTTP/2` → `HTTP/2.0`.
- **Edited request parsing: unexpected EOF** — Detects linebreak style (`\r\n` vs `\n`) and ensures request ends with double linebreak.

# Released v2026.3.7

Includes `v0.27.0`

## [2026-MAR] - v0.27.0 - MCP (Model Context Protocol) Support

### Added

- **MCP server** — Built-in MCP server using `mcp-go` with SSE transport for AI tool integration.
- **MCP tools** — `version`, `getRequestResponseFromID`, `hostPrintSitemap`, `hostPrintRowsInDetails`, `sendRequest` tools for AI agents to interact with proxy data.
- **MCP endpoints** — `/mcp/start`, `/mcp/stop`, `/mcp/health`, `/mcp/listtools`, `/mcp/sse`, `/mcp/message`, `/mcp/setup/claude`.
- **Claude Code integration** — `/mcp/setup/claude` endpoint writes `.mcp.json` and `CLAUDE.md` auto setup.

---

## [2026-MAR] - v0.26.5 - Dev UI, Xterm Improvements & Proxy Timeout Fix

### Added

- **Dev UI (SvelteKit testing interface)** — Full SvelteKit app (`grx/dev`) for interactive API testing and development.
- **Xterm scrollback replay** — Reconnecting WebSocket clients receive up to 256KB of buffered terminal output, restoring previous output on page reload.
- **Multi-client terminal sessions** — Multiple WebSocket clients can view the same terminal session concurrently (up to 10 per session).
- **Persistent PTY reader** — Single goroutine per session reads PTY output and broadcasts to all connected clients, replacing per-connection readers.
- **Electron build script updated** — Updated `cmd/electron/build.sh` and `package.json`.

### Fixed

- **Electron: orphan child processes on quit** — Closing the Electron app now kills the entire process group (grroxy, grroxy-app, grroxy-tool) instead of only the grroxy process. Uses `detached: true` spawn and `process.kill(-pid)` for group termination.
- **[IMP][PERFORMANCE] Electron: orphan child processes on quit** — Closing the Electron app now kills the entire process group (grroxy, grroxy-app, grroxy-tool) instead of only the grroxy process. Uses `detached: true` spawn and `process.kill(-pid)` for group termination.
- **Proxy timeouts increased to 10 minutes** — Prevents intercepted request connection failures when requests are held for manual review (`rawproxy` config, MITM, and proxy server timeouts all updated).
- **Stale WebSocket cleanup on session close** — All connected WebSocket clients are now closed when a terminal session is closed, preventing hanging goroutines.
- **Scrollback buffer memory compaction** — Buffer uses in-place `copy()` to prevent unbounded underlying array growth.
- **Client limit per session** — Capped at 10 concurrent WebSocket connections per terminal session to prevent resource exhaustion.

---

## [2026-MAR] - v0.26.4 - v2026.3.6

### Added

- **Cook bundled in Electron app** — `cook` binary is now built and embedded in the Electron app alongside grroxy, grroxy-app, and grroxy-tool.

### Fixed

- **Fuzzer** — Fixed fuzzer issues.

---

## [2026-MAR] - v0.26.3 - v2026.3.5 - Desktop App, Update Check & Cross-Platform Build

### Added

- **Electron Desktop App** — Standalone desktop application wrapping the grroxy web UI.
- **`getVersion` IPC** — Frontend can fetch current app version.
- **`openURL` IPC** — Open external URLs in default browser.
- **Cross-platform Electron build** — `./build.sh all` now builds Electron app for mac, linux, and windows.

---

## [2026-MAR] - v0.26.2 - Fix `go install` Error

### Fixed

- **`go install` error** — Removed `replace` directives from `go.mod` that caused `go install` to fail for remote installs.

---

## [2026-MAR] - v0.26.1 - Initial Public Release

### Added

- **Public release of Grroxy**

---

## [2026-MAR] - v0.26.0 - Repository Rename: `grroxy-db` -> `grroxy`

### Changed

- **Repository renamed** from `github.com/glitchedgitz/grroxy-db` to `github.com/glitchedgitz/grroxy`
- **Go module path updated** — All imports changed from `github.com/glitchedgitz/grroxy-db/...` to `github.com/glitchedgitz/grroxy/...`
- **Self-update URL updated** — GitHub releases API URL in `internal/updater/updater.go` now points to the new repo
- **Documentation updated** — All docs and READMEs reference the new repository name

### Migration (for users)

- The old GitHub URL (`github.com/glitchedgitz/grroxy-db`) will redirect to the new one
- Update your git remote: `git remote set-url origin git@github.com:glitchedgitz/grroxy.git`
- If using `go get`: `go get github.com/glitchedgitz/grroxy@latest`

See `docs/RENAME_MIGRATION.md` for full migration details.

---

## [2026-MAR] - Fuzzer: Unified Markers with Inline Payloads

### Added

- **Fuzzer: Inline Payloads via `markers`** — Each marker value can now be either a string (wordlist file path) or an array of strings (inline payloads). Both types can be mixed in the same request. Inline payloads support multi-line values since they are iterated by index, not split by newlines.
- **Fuzzer: `generated_by` field** — Track what generated a fuzzer request (e.g., "manual", "workflow").
- **Fuzzer: `process_data` field** — Attach arbitrary metadata to a fuzzer process.
- **Fuzzer: `Failed` process state** — Fuzzer processes that encounter errors are now marked as "Failed" with error details.
- **Fuzzer: `markerSource` abstraction** — Internal interface (`fileSource`, `sliceSource`) replaces raw `bufio.Reader` for all marker iteration, enabling correct multi-line payload support.

### Changed

- **Fuzzer: Removed separate `payloads` field** — The `markers` field now handles both file paths and inline payloads via type detection. No separate `payloads` field needed.
- **Fuzzer: Improved validation** — Validates marker types (must be string or array), empty values, and provides clearer error messages.

### Fixed

- **Fuzzer: Pitch fork last-item dispatch** — Fixed bug where the last payload in pitch_fork mode was skipped when EOF arrived with the final value.
- **Fuzzer: Cleaned up wordlist initialization** — Removed debug code from fuzzer core.

---

## [2026-FEB] - v0.25.0 - Self-Update, Electron Launch & Proxy Improvements

### Added

- **Self-Update Command** (1dc12df)
  - `grroxy update` - Fetch and replace binaries (`grroxy`, `grroxy-app`, `grroxy-tool`) from GitHub Releases
  - Private repo support via `GITHUB_TOKEN` environment variable
  - Cross-platform binary replacement with `.exe` handling for Windows

- **Update API Endpoints**
  - `GET /api/update/check` - Check if a newer version is available (returns current/latest version and platform info)
  - `POST /api/update` - Perform the update for all binaries from the launcher

- **Electron App Launch Integration** (3a41c75)
  - Electron app now spawns `grroxy start` as a child process on launch
  - Automatic backend startup when opening the desktop app

- **Chrome Browser Test Suite** (31a1186)
  - Comprehensive test cases for Chrome automation (`grx/browser/chrome_test.go`)
  - Multi-tab workflow tests and navigation timeout fixes

### Changed

- **Rawproxy Protocol Handling** (da3c528)
  - Improved protocol detection and handling per target
  - uTLS transport caching per target for better performance

- **Serve Configuration** (b3c5945)
  - Use `.grroxy` directory and `chdir` on launch for cleaner working directory management

- **Frontend Updates** (d702ab7, b621b2c)
  - Frontend fetch improvements

### Fixed

- Host header handling (9ee9aad)
- Chrome navigation timeout in MultiTabWorkflow test (2d2980d)

---

## [2026-FEB] - v0.24.0 - Chrome Automation Refactor & Tab Management

### Added

- **Chrome Tab Management API**
  - `GET /api/proxy/chrome/tabs` - List all open tabs in the attached Chrome instance
  - `POST /api/proxy/chrome/tab/open` - Open a new tab with optional URL
  - `POST /api/proxy/chrome/tab/navigate` - Navigate a specific tab with configurable wait conditions (`load`, `domcontentloaded`, `networkidle`)
  - `POST /api/proxy/chrome/tab/activate` - Switch focus to a specific tab
  - `POST /api/proxy/chrome/tab/close` - Close a specific tab
  - `POST /api/proxy/chrome/tab/reload` - Reload a specific tab with optional cache bypass
  - `POST /api/proxy/chrome/tab/back` - Navigate back in history for a specific tab
  - `POST /api/proxy/chrome/tab/forward` - Navigate forward in history for a specific tab

### Changed

- **Chrome Automation Refactor**
  - Refactored `grx/browser/chrome.go` to use `ChromeRemote` struct for better state management and persistence
  - Improved connection handling and context management for Chrome DevTools Protocol
  - Migrated standalone functions to `ChromeRemote` methods for multi-tab support

### Deprecated

- Standalone browser functions `TakeChromeScreenshot`, `ClickChromeElement`, etc. are now deprecated in favor of `ChromeRemote` methods

## [2026-FEB] - v0.23.0 - Process Management & SDK Integration

### Added

- **Process Management System** (44a3971)
  - Complete process management API for tracking long-running operations (fuzzers, scanners, etc.)
  - `_processes` collection with real-time progress tracking
  - Process states: `In Queue`, `Running`, `Completed`, `Killed`, `Failed`, `Paused`
  - Automatic progress percentage calculation based on completed/total counts
  - Process fields: `parent_id`, `generated_by`, `created_by` for better tracking
  - Database migration for `_processes` collection schema updates

- **SDK for External Tools** (44a3971)
  - `internal/sdk/process.go` - SDK client for external tools to connect to main app
  - SDK authentication via admin email/password
  - Process management functions:
    - `CreateProcess()` - Create new process with metadata
    - `UpdateProcess()` - Update progress with atomic operations
    - `CompleteProcess()` - Mark process as completed
    - `FailProcess()` - Mark process as failed with error message
    - `PauseProcess()` - Pause running process
    - `KillProcess()` - Stop process by user request
  - Environment variable support (`GRROXY_APP_URL`, `GRROXY_ADMIN_EMAIL`, `GRROXY_ADMIN_PASSWORD`)
  - External tools can now update main app's `_processes` collection via HTTP API

- **Fuzzer Improvements** (44a3971, 553f762)
  - Batch database saving for improved performance
  - Atomic progress counters using `atomic.AddInt64()` and `atomic.LoadInt64()` (no mutexes)
  - SDK integration for process tracking in external `grroxy-tools`
  - Periodic progress updates (1-second ticker) instead of per-request updates
  - Process creation with fuzzer configuration and request metadata
  - Automatic process state management (In Queue → Running → Completed/Failed/Killed)

- **Documentation** (44a3971)
  - `docs/PROCESS_MANAGEMENT.md` - Comprehensive guide for process management and SDK integration
  - `examples/sdk_process_example.go` - Working examples for SDK usage
  - API documentation for process management endpoints

### Changed

- **Tools Architecture** (44a3971)
  - `apps/tools/main.go` - Added `AppSDK` field to `Tools` struct for SDK client
  - `apps/tools/fuzzer.go` - Refactored to use SDK for all process operations
  - `grx/fuzzer/fuzzer.go` - Added atomic counters (`totalRequests`, `completedRequests`) for thread-safe progress tracking

- **Process Schema** (44a3971)
  - `internal/schemas/processes.go` - Added `Failed` and `Paused` states
  - Enhanced process input/output structure for better metadata tracking

### Fixed

- Improved fuzzer performance with batch database operations
- Thread-safe progress tracking without mutex contention
- Proper error handling and state management for long-running processes

---

## [2026-JAN] - v0.22.0 - WebSocket Proxying & Capture

### Added

- **WebSocket Proxying & Capture**
  - Full WebSocket proxying support through `/rawproxy` with MITM capabilities
  - `_websockets` collection for storing captured WebSocket messages
  - WebSocket frame parsing and capture (text, binary, close, ping, pong frames)
  - Bidirectional message tracking with direction indicators (send/recv)
  - Message indexing and correlation with HTTP upgrade requests via `proxy_id`
  - Support for both `ws://` (plain) and `wss://` (TLS) WebSocket connections
  - WebSocket message handler callback (`OnWebSocketMessageHandler`)
  - File-based WebSocket message logging with metadata
  - Automatic HTTP/1.1 enforcement for WebSocket upgrades (prevents HTTP/2 conflicts)

---

## [2026-JAN] - v0.21.0 - Browser Automation & Data Extraction

### Added

- **Browser Automation via Chrome DevTools Protocol** (605f41d)
  - `/api/proxy/screenshot` - Capture screenshots (full-page or viewport, optional file save)
  - `/api/proxy/click` - Click elements using CSS selectors
  - `/api/proxy/elements` - Get clickable elements from current page

- **Data Extraction** (5c87dbb)
  - `/api/extract` - Extract fields from database records by host (supports `req.*`, `resp.*`, `req_edited.*`, `resp_edited.*`)

- **Request Modification** (386148b, fc66654)
  - `/api/request/modify` - Modify HTTP requests (set, delete, replace operations)
  - Wildcard header deletion support (fc66654)

- **System Info** (5c87dbb)
  - `/api/info` - Get version, directories, and project info

### Changed

- Enhanced proxy instances with Chrome browser integration (605f41d)
- Improved request parsing and rebuilding (386148b)

### Fixed

- Content-Length header handling (843820b)
- HTTP/1.1 protocol improvements (797e28b)
- TLS browser connection issues (504d8c7)
- InsecureSkipVerify for testing (7b43171)
- Zstd decoder support (2a691f0)

---

## [2026-JAN] - v0.20.1 - Labels Update

- Labels and Notes for hosts
- Tech counter
- Disabling label collection

## [2025-DEC] - v0.20.0 - Xterm Terminal Integration

### Added

- Web-based terminal support using xterm.js frontend and PTY backend
- `/api/xterm/start` - Create new terminal sessions with custom shell, working directory, and environment variables
- `/api/xterm/sessions` - List all active terminal sessions
- `/api/xterm/sessions/:id` - Close terminal sessions via DELETE endpoint
- `/api/xterm/ws/:id` - WebSocket endpoint for bidirectional terminal I/O (input, output, resize, ping/pong)
- Cross-platform terminal support (Linux, macOS, Windows)
- PTY (Pseudo-Terminal) integration for full terminal emulation
- Terminal session management with automatic cleanup on process exit
- Support for interactive terminal applications (vim, htop, etc.)
- Terminal resize functionality
- Comprehensive xterm API documentation

### Changed

- Updated API documentation with xterm endpoints and WebSocket protocol details

---

## [2025-DEC] - v0.19.0 - Counter Table & Refactoring #27

### Added

- Counter table for different hook points and intercept operations
- `/api/filter/check` - New API endpoint for filter validation using dadql
- `/api/repeater/send` - New API endpoint for request replay functionality with automatic database storage
- Counter support for intercept operations
- New columns in `_data` collection: `http` and `proxyid` for better request tracking
- Database unique index logging for better debugging
- Time logging with all log entries
- Comprehensive API documentation (`api_docs.md`) for all three apps (app, launcher, tools)

### Changed

- Merged `grrhttp` package into `rawhttp` for better organization
- Moved packages to `internal` directory for better encapsulation
- Moved packages to `grx` directory for modular organization
- Renamed `api` directory to `apps` for clarity
- Refactored certificate and profile path handling to use `ConfigDirectory`
- Renamed `ProjectDirectory` to `ProjectsDirectory` for consistency
- Updated configuration handling to use `ConfigDirectory` for certificate paths
- Commented out verbose logs for cleaner output

### Fixed

- Fixed rawhttp HTTP/2 error handling
- Fixed config-related issues and path handling
- Fixed Electron preload issues
- Fixed Electron build for Windows
- Fixed macOS error on reopen from dock

### Removed

- Deleted duplicate markdown files
- Removed unused files and `project.go`
- Cleaned up codebase

---

## [2025-DEC] - v0.18.0 - v2025.12.0 Release

### Added

- HTTP/2 support for fuzzer
- Isolated browser profile support
- Dump request functionality
- `/api/request/add` - New parameter `generated_by` added to endpoint
- Enhanced fuzzer with better error handling and logging
- Sitemap depth parameter and children node support

### Changed

- Frontend updated to version 2025.12.0
- Frontend fetch improvements
- Changed `grroxy-tool` current working directory handling
- Parser improvements: no trimming or lowercase conversion of headers
- Rawhttp parser enhancements

### Fixed

- Fixed panic when environment variable is missing
- Fixed rawhttp when response is chunked and encoded
- Fixed rawhttp decompression after send
- Fixed sitemap fetch with path parameter
- Unparse request/response improvements

## [2025-OCT] - Multi Proxy Support #24

### Added

- Multiple proxy instances support with per-proxy configuration
- Per-proxy intercept settings stored in `_proxies` collection
- Per-proxy filter rules stored separately in `_ui` collection (format: `proxy/{proxyID}`)
- Single-click intercepted browser/terminal launch functionality
- Ability to enable/disable intercept for specific proxies independently
- Proxy auto-label generation based on browser type and instance count
- Terminal process launch and management support for proxy instances
- Proxy state persistence and restoration on application startup

### Changed

- Migrated from single proxy instance to multiple concurrent proxy support
- Filter management now scoped per-proxy instead of global settings
- Proxy configuration now stored in database collections instead of runtime-only state

### Fixed

- Fixed proxy state synchronization between database and runtime instances
- Improved terminal process cleanup when proxy is stopped

## [2025-OCT] - Core Update #23

### Added

- Separate relational collections (`_req`, `_resp`, `_req_edited`, `_resp_edited`) with proper indexing
- Direct channel-based communication between API and goroutines for improved performance
- Raw HTTP strings now stored in `raw` field of respective collections
- Retained JSON fields `req_json`, `resp_json`, `req_edited_json`, `resp_edited_json` for backward compatibility

### Changed

- **BREAKING**: Migrated from JSON-based storage to separate relational collections for significant performance improvements
  - **Previously**: Request/response data stored as JSON in `req` and `resp` columns of `_data`
  - **Now**: Separate collections with proper relational structure and indexing
- **BREAKING**: Moved from typed structs to `map[string]any` for direct database operations
  - Directly usable for inserting to database, checking filter and running templates
  - Struct definitions retained for manual reference
- **BREAKING**: Consolidated `_raw` collection into separate typed collections (`_req`, `_resp`, `_req_edited`, `_resp_edited`)
- Improved data flow and operations with direct database access patterns

### Fixed

- Significantly reduced database operations while inserting/modifying new records
- Better type safety and query performance with relational collections
- Faster queries with proper indexing on relational collections
- Reduced data duplication through normalized collection structure

## [2025-OCT] - New Proxy Migration #22

### Added

- Lightweight `/rawproxy` implementation replacing unmaintained `elazarl/goproxy` package
- HTTP tunnel ID tracking for better request correlation
- Direct database integration using PocketBase `dao` instead of `grroxysdk`
- Atomic counter-based indexing with persistence across restarts
- Thread-safe request/response correlation using `RequestData` passing
- `RawProxyWrapper` in `api/app/proxy_rawproxy.go` for rawproxy integration
- `RequestData` struct in rawproxy for passing context between handlers
- Fixed certificate location at `~/.config/grroxy/`
- Certificate generation on application startup in `setConfig()`
- Comprehensive logging system with detailed error tracking

### Changed

- **BREAKING**: Migrated from `proxify(v0.8)` to new lightweight `/rawproxy` implementation as `elazarl/goproxy` was no longer maintained
- **BREAKING**: Unified certificate system using `rawproxy.GenerateMITMCA()`
- Improved certificate serving consistency across all components

### Removed

- `GetStats()` API endpoint (use direct database queries instead)

---

### Technical Notes

#### HTTP vs HTTPS Proxy Behavior

The proxy handles HTTP and HTTPS requests differently as per RFC 7230:

**HTTP (Plain) - Absolute URI:**

- Proxy receives absolute URI in request line
- Proxy transforms to relative path for upstream
- Host header preserved for virtual hosting at origin

**HTTPS (Encrypted) - CONNECT Tunnel:**

- Proxy establishes TCP tunnel with CONNECT
- Client sends relative paths inside encrypted tunnel
- With MITM: Proxy decrypts, processes, and re-encrypts
- Without MITM: Proxy acts as passthrough tunnel

This behavioral difference is standard HTTP proxy protocol. The `RequestURI` field is cleared before forwarding to upstream as Go's `http.Transport` expects it empty.
