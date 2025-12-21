# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
