# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

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
