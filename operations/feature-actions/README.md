# Feature: Enable Template Actions

## Status: `in-progress`

## Goal

The templates/actions system already exists in `grx/templates/` (YAML parsing, conditions, variable interpolation, file watcher, actions). We are wiring it into the proxy pipeline and enabling it for use.

## Use Cases

- Each proxy has its own templates working as settings to modify request/response (search and replace, set/delete headers, etc.)
- Hooks trigger templates at specific points — both automatic (proxy pipeline) and user-triggered (UI buttons)
- Any template can be enabled/disabled independently
- Each task within a template has a `disabled` field (false by default) so individual tasks can be turned off without removing them
- Templates are stored in the database (`_templates` collection on launcher) and can be managed via API
- Templates are scoped to projects via the `projects` relation field

## Architecture

### Template Structure

- **Template** — top-level unit with info, config, hooks, and tasks
- **Task** — a condition + list of actions to execute when matched; has `disabled` field
- **Action** — a single operation (replace, set, delete, create_label, etc.)

### Schema Fields (`_templates` collection on launcher)

name, title, description, author, type, mode, hooks (JSON), tasks (JSON), enabled, global, is_default, archive, projects (relation)

### Hooks

Proxy hooks (automatic):

- `proxy:before_request` — before saving request to DB (synchronous, can modify request)
- `proxy:request` — after saving request to DB (async)
- `proxy:before_response` — before saving response to DB (synchronous)
- `proxy:response` — after saving response to DB (async)

UI hooks (user-triggered):

- `request-action-button` — button shown on request rows in the UI
- `response-action-button` — button shown on response rows in the UI
- `sitemap-action-button` — button shown on sitemap entries in the UI

### Enable/Disable Toggles (3 levels)

1. **Global** — `_configs` on launcher: `key: "settings.templatesEnabled"` — master switch for all projects
2. **Per-project** — `_projects.data.templatesEnabled` on launcher — per project toggle, disabled when global is off
3. **Per-proxy** — `_proxies.data.run_templates` on grroxy-app — per proxy toggle

All three must be enabled for templates to execute on a proxy.

Settings UI shows global and per-project toggles. Per-project toggle is grayed out when global is off. Per-proxy toggle is in the proxy toolbar.

### Data Flow

- Templates live in launcher's `_templates` collection (not in grroxy-app DB)
- Launcher passes `-launcher` flag to grroxy-app with its address
- On startup, grroxy-app fetches templates via `GET http://{launcher}/api/templates/list`
- Launcher watches `_templates` for changes and notifies running projects via `POST /api/templates/reload`
- Launcher watches `_projects.data.templatesEnabled` changes and notifies via `POST /api/templates/global-toggle`
- Launcher watches `_configs` for `settings.templatesEnabled` changes and notifies all projects

### Default Templates

- Embedded in binary via `//go:embed` in `grx/templates/defaults/`
- Seeded into `_templates` on migration with `is_default: true`
- On startup, `SeedDefaultTemplates` updates defaults if `is_default=true`, skips user-modified ones
- Default templates: extensions, mime, paths, path-based-tech, proxy-configs

### Request Modification (before_request)

- `set` and `delete` actions directly modify `http.Request` headers/method/path via `applySetToRequest`/`applyDeleteToRequest`
- No raw string rebuild — original request stays intact, only targeted fields change
- Other actions (create_label, etc.) execute normally

### Label Attachment

- `create_label` action creates/finds the label in `_labels`, then attaches it to the request row via `_attached` record
- Skips if already attached

### Available Actions (`grx/templates/actions/`)

- `set` — Modify request/response fields
- `delete` — Remove request/response fields
- `replace` — Search and replace (string or regex)
- `create_label` — Create a label and attach to the request row
- `send_request` — Send a modified copy of the request (uses `sendRepeaterLogic`)

## API Endpoints

### Launcher (`grroxy`)

- `GET /api/templates/list` — list all templates from `_templates` DB
- `POST /api/templates/new` — create template in DB
- `DELETE /api/templates/:template` — delete template by ID
- `grroxy migrate up/down` — run/revert migrations

### Project App (`grroxy-app`)

- `GET /api/templates/list` — serve templates from in-memory engine
- `POST /api/templates/reload` — re-fetch templates from launcher
- `POST /api/templates/toggle` — per-proxy toggle `{proxy_id, enabled}`
- `POST /api/templates/global-toggle` — set `TemplatesEnabled` `{enabled}`
- `GET /api/templates/global-status` — get current `TemplatesEnabled` state

## Files Changed

- `internal/schemas/templates.go` — DB schema
- `cmd/grroxy/migrations/1774288591_templates.go` — migration + default seed
- `grx/templates/templates.go` — json tags, disabled field, LoadTemplate/RemoveTemplate, paused dir loading
- `grx/templates/defaults/` — embedded default YAML templates
- `apps/app/main.go` — Templates + TemplatesEnabled on Backend
- `apps/app/template_loader.go` — LoadTemplatesFromLauncher, LoadTemplatesEnabledFromLauncher
- `apps/app/template_actions.go` — ExecuteTemplateActions, create_label with row attachment
- `apps/app/template_toggle.go` — toggle endpoints, reload endpoint, hooks
- `apps/app/template_request.go` — applySetToRequest, applyDeleteToRequest
- `apps/app/templates.go` — list/new/delete endpoints (DB-backed)
- `apps/app/proxy_rawproxy.go` — 4 template hooks in proxy pipeline
- `apps/launcher/template_hooks.go` — watch _templates/_projects/_configs, notify projects
- `apps/launcher/templates.go` — list/new/delete endpoints (DB-backed)
- `apps/launcher/projects.go` — pass -launcher flag to grroxy-app
- `cmd/grroxy/main.go` — migrate command, template hooks, conf.HostAddr
- `cmd/grroxy-app/main.go` — LauncherAddress flag
- `cmd/grroxy-app/serve.go` — init templates from launcher on startup
- `internal/config/config.go` — LauncherAddr field
- `cybernetic-ui/.../Actions.svelte` — DB-backed template list, subscription, fresh fetch on click
- `cybernetic-ui/.../ActionsEditor.svelte` — task disable toggle, template enable toggle
- `cybernetic-ui/.../SettingsGeneral.svelte` — global + per-project template toggles
- `cybernetic-ui/.../ProxyManager.svelte` — per-proxy templates toggle button
- `grx/dev/src/lib/app-api.ts` — updated API docs
- `grx/dev/src/lib/launcher-api.ts` — updated API docs

## Bug Fixes Applied

- **Header matching in `modify.go`** — `RequestUpdateKey` and `RequestDeleteKey` now use `strings.TrimRight(h[0], ": ")` to normalize header names. Previously `"User-Agent: "` (rawhttp format) didn't match `"User-Agent:"` check, causing headers to be duplicated instead of updated, and deletes to silently fail.
- **PocketBase JSON field in launcher hooks** — `forEachRunningProject` in `template_hooks.go` now handles `types.JsonRaw`, `string`, and `map[string]any` for the `data` field. Previously the type assertion `data.(map[string]any)` always failed for PocketBase JSON fields, so no running projects were ever notified of template changes.

## Known Issues

### Critical (race conditions — will crash under load)

- **`Templates.Templates` map** — `LoadTemplate`/`RemoveTemplate` write from HTTP handlers while `Run()` reads from proxy goroutines. Fatal map race. Fix: add `sync.RWMutex` to `Templates` struct.
- **`TemplatesEnabled` bool** — written by global-toggle HTTP handler, read by proxy goroutines. Fix: use `atomic.Bool`.
- **`inst.Proxy.RunTemplates`** — written after releasing `RLock`, read concurrently by proxy. Fix: use `atomic.Bool`.
- **Shallow copy of userdata in template goroutines** — `templateData["req"]` and `templateData["resp"]` share nested maps with the main request flow. If `ExecuteTemplateActions` reads while main goroutine writes to `saveResponseToDB`, data race. Fix: deep-copy nested maps before goroutine.
- **Race window during reload** — `Setup()` clears the map, then `LoadTemplatesFromLauncher` repopulates. Concurrent `Run()` calls see empty map in between. Fix: swap atomically or hold write lock for the full reload.

### Medium

- **`TemplatesCheck` route never registered** — endpoint defined in `templates.go` but not added in `serve.go`. Dead code.
- **`LoadTemplatesFromLauncher` swallows errors** — all error paths `return nil`. Reload always reports 200 OK even when launcher is down.
- **`send_request` header/body overrides are TODO stubs** — silently do nothing. No warning logged.
- **`executeCreateLabel` SaveRecord error swallowed** — any error (not just duplicates) is ignored on line 60.
- **`LoadTemplatesEnabledFromLauncher`** — no HTTP status code check before parsing response body.

### Low

- **Go map iteration order** — `Todo []map[string]map[string]any` loses order when looping. YAML defines actions in order but Go maps don't preserve it. Will be handled in the UI.
- **Content-Length not updated on body change** — In `modify.go`, `RequestUpdateKey` updates `requestData["length"]` and the body in `requestData["raw"]`, but does NOT update the `Content-Length` header in `requestData["headers"]`. Same gap in `buildRawRequest`, `userdata.go`'s `RequestUpdateKey`, and `template_request.go`'s `applySetToRequest`.
- **Empty `resp` data in `before_request`** — `templateData["resp"]` is an empty map (status=0, mime=""), could cause false positive condition matches.
- **`forEachRunningProject` fetches all projects** — no filter, iterates all including inactive.
- **Pre-existing typo** — `TempalteDir` should be `TemplateDir` in `templates.go:57`.

## Test Coverage

| Function | Test File | Tests |
|----------|-----------|-------|
| `ParseVariable` | `grx/templates/parse_variable_test.go` | 9 cases |
| `ParseTemplateActions` | `grx/templates/functions_test.go` | 8 cases (disabled, default, mode any/all, condition, interpolation, multiple, empty) |
| `applySetToRequest` | `apps/app/template_request_test.go` | 13 cases (method, path, url, headers, body, query, unknown) |
| `applyDeleteToRequest` | `apps/app/template_request_test.go` | 7 cases (header, wildcard, method, path, url, body, query) |
| `RequestUpdateKey` | `apps/app/modify_test.go` | 8 cases |
| `RequestDeleteKey` | `apps/app/modify_test.go` | 5 cases |
| `RequestReplace` | `apps/app/modify_test.go` | 4 cases (simple, regex, no match, invalid regex) |
| `buildRawRequest` | `apps/app/modify_test.go` | 3 cases |
| `runActions` | `apps/app/modify_test.go` | 6 cases (set, delete, replace, multiple, empty, unknown) |

## TODO

- [ ] Fix race conditions (Critical issues above — `sync.RWMutex` + `atomic.Bool`)
- [ ] Register `TemplatesCheck` endpoint in `serve.go`
- [ ] Implement `send_request` header/body overrides
- [ ] Hook into `on_new_sitemap`
- [ ] Frontend: render action buttons from UI hooks on request/response/sitemap rows
- [ ] Remote sync from `grroxy-templates` repo for updates
- [ ] Test end-to-end: full flow with all 3 toggles enabled
