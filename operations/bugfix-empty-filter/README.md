# Bug Fix: Empty Filter in mcp_tools.go

## Status: `done`

## Problem

In `apps/app/mcp_tools.go`, when `filter == ""`, the code was using `dao.FindRecordsByFilter()` which doesn't handle empty filters correctly. It should use `dao.FindRecordsByExpr()` instead.

## Root Cause

Line 463 in `listHostsHandler` used `FindRecordsByFilter("_hosts", filter, "", 0, 0)` for the total count query without checking if `filter` was empty. The main query (lines 423-427) already had the correct pattern.

## Fix

Added a conditional check before the total count query:

```go
var total []*models.Record
if filter == "" {
    total, _ = dao.FindRecordsByExpr("_hosts")
} else {
    total, _ = dao.FindRecordsByFilter("_hosts", filter, "", 0, 0)
}
```

## Files Changed

- `apps/app/mcp_tools.go` - Line 463: Added empty filter guard for total count query
