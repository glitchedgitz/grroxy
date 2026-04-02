# Feature: Integration Test Script

## Status: `planned`

## Goal

End-to-end bash script that tests the full grroxy flow — project creation, template system, proxy traffic, and state persistence across restarts.

## Prerequisites

- `grroxy` and `grroxy-app` binaries built
- Port 8090 (launcher) and 8091+ (project) available
- `curl` and `jq` available

## Test Plan

### Phase 1: Setup

1. **Start `grroxy`** on a test port (e.g. `127.0.0.1:18090`)
2. **Wait for ready** — poll `/api/health` or check stdout
3. **Create project** `"Test-N"` via `POST /api/project/create`
4. **Verify** project appears in `GET /api/project/list`
5. **Verify** `templatesEnabled=true` on the project (should be default for new projects)
6. **Verify** global `settings.templatesEnabled=true` in `_configs`

### Phase 2: Templates

7. **Create template A** — `before_request` hook, `set` action: add header `X-Test: grroxy-test`
   - Filter: `host ~ 'example.com'`
8. **Create template B** — `before_request` hook, `set` action: modify path `/original` → `/modified`
   - Filter: `host ~ 'example.com'`
9. **Create template C** — `request` hook, `send_request` action: send a duplicate request with method `PUT`
   - Filter: `host ~ 'example.com'`
10. **Create template D** — `request` hook, `create_label` action: label `"test-label"` with color `green`
    - Filter: `host ~ 'example.com'`
11. **Verify** all 4 templates appear in `GET /api/templates/list`
12. **Validate** each template via `POST /api/templates/check`

### Phase 3: Proxy & Traffic

13. **Start a test HTTP server** on `127.0.0.1:19999` that echoes back request headers and path
14. **Open project** → `grroxy-app` starts
15. **Start proxy** via `POST /api/proxy/start`
16. **Verify** proxy is running via `GET /api/proxy/list`
17. **Verify** `run_templates=true` on the proxy record
18. **Send request** through proxy to `http://example-test.com:19999/original` (using proxy address)
19. **Wait** for async template actions to complete (1-2s)

### Phase 4: Verify Template Effects

20. **Check request captured** — query `_data` collection, verify request exists with host `example-test.com`
21. **Check `before_request` set header** — verify the request saved to DB has `X-Test: grroxy-test` header
22. **Check `before_request` path modify** — verify the request path was changed from `/original` to `/modified`
23. **Check `create_label`** — verify `test-label` exists in `_labels` collection
24. **Check label attached** — verify the label is attached to the request row via `_attached`
25. **Check `send_request`** — verify a second request exists with `generated_by: "template:send_request"` and method `PUT`

### Phase 5: Template Edit & Reload

26. **Edit template D** — change label name to `"edited-label"` via PocketBase API
27. **Wait** for launcher to notify app (1-2s)
28. **Verify** template reloaded — `GET /api/templates/list` on grroxy-app shows updated template
29. **Send another request** through proxy
30. **Verify** new request has `edited-label` attached (not `test-label`)

### Phase 6: Toggle Tests

31. **Disable template B** — set `enabled=false`
32. **Send request** through proxy
33. **Verify** path is NOT modified (template B disabled), but header still added (template A active)
34. **Disable per-proxy templates** — `POST /api/templates/toggle` with `enabled=false`
35. **Send request** through proxy
36. **Verify** NO templates ran (no labels, no header modification)
37. **Re-enable per-proxy** and **disable per-project** (set `templatesEnabled=false` on `_projects.data`)
38. **Send request** through proxy
39. **Verify** NO templates ran (project-level override)
40. **Re-enable per-project**

### Phase 7: Restart & State Persistence

41. **Stop the project** (kill `grroxy-app` process)
42. **Verify** project state set to `unactive` in launcher DB
43. **Verify** `templatesEnabled` still `true` on the project record (not wiped by close)
44. **Re-open project** via API
45. **Wait** for `grroxy-app` to start and load templates
46. **Verify** templates loaded — `GET /api/templates/list` returns same templates
47. **Verify** proxy `run_templates=true` after restart (from migration)
48. **Start proxy** and **send request** through proxy
49. **Verify** templates still work after restart

### Phase 8: Cleanup

50. **Stop proxy**
51. **Delete all test templates** via API
52. **Delete test project** via API
53. **Stop test HTTP server**
54. **Stop `grroxy`**
55. **Print test summary** — pass/fail count

## Script Structure

```
tests/
  e2e_test.sh          # Main test script
  test_server.go       # Simple echo HTTP server for upstream testing
```

## Output Format

```
[PASS] Project created: Test-1
[PASS] Templates enabled by default
[PASS] Template A created (set header)
[PASS] Template B created (modify path)
[PASS] Template C created (send_request)
[PASS] Template D created (create_label)
[PASS] Proxy started
[PASS] Request captured
[PASS] Header X-Test added by template
[PASS] Path modified by template
[PASS] Label created and attached
[PASS] send_request created duplicate with PUT
[PASS] Template edit triggers reload
[PASS] Edited label applied to new request
[PASS] Disabled template B: path not modified
[PASS] Per-proxy toggle off: no templates ran
[PASS] Per-project toggle off: no templates ran
[PASS] State persists after restart
[PASS] Templates work after restart

Results: 19/19 passed
```

## Notes

- Each `[PASS]`/`[FAIL]` assertion is a single `curl` + `jq` check
- Script exits with non-zero code if any test fails
- Test server echoes headers back in response body so we can verify modifications
- All ports use high numbers (18090+) to avoid conflicts
- Cleanup runs even on failure (trap EXIT)
