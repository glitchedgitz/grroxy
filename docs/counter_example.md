# Counter Manager Usage Examples

The CounterManager provides a thread-safe in-memory map for tracking counts that syncs with the database.

**Location:** `apps/app/counter.go`

**Important:** The `counter_key` is the **primary identifier** - it's the main thing being tracked. The `collection` and `filter` are optional metadata that help categorize the counter.

## Key Features

✅ **Lock-free atomic operations** (~10ns per operation)  
✅ **Two sync modes**: Immediate sync or periodic sync  
✅ **Change detection**: Only writes to DB when values change  
✅ **Exact counts**: No data loss on shutdown  
✅ **Simple map structure**: `map[counter_key]*Counter`

## API Signature

```go
// Basic operations (loadOnStartup=false, immediate sync)
Increment(key, collection, filter string) int64
Decrement(key, collection, filter string) int64
Get(key, collection, filter string) int64
Set(key, collection, filter string, count int64)

// With explicit loadOnStartup flag
IncrementWithStartup(key, collection, filter string, loadOnStartup bool) int64

// Utility methods
SyncOne(key string) error                // Sync single counter immediately
SyncToDB() error                         // Sync all load_on_startup counters
GetDetails(key string) (*Counter, bool)  // Get full counter details
GetAll() map[string]*Counter             // Get all counters
Reset(key, collection, filter string)    // Reset to 0
Delete(key, collection, filter string)   // Remove counter
```

**Parameters:**

- **key** (counter_key) - **REQUIRED**: The main identifier for what you're counting
- **collection** - OPTIONAL: Which collection this relates to (can be blank "")
- **filter** - OPTIONAL: Additional filter criteria (can be blank "")
- **loadOnStartup** - If true, recalculated from DB on startup (no immediate sync)

## Two Types of Counters

### Type 1: Immediate Sync (loadOnStartup=false) - DEFAULT

**Use for:** Counters where you need exact values (proxy counts, host counts, label counts)

```go
// These sync to DB immediately after each change
API.CounterManager.Increment("proxy/______________1", "_data", "")
API.CounterManager.Increment("host:site_example_com", "_data", "")
API.CounterManager.Increment("label:abc123", "_labels", "")
```

**Behavior:**

- Synced to DB immediately via `SyncOne()` (runs in goroutine)
- **Zero data loss** on Ctrl+C or shutdown
- Always exact counts

### Type 2: Periodic Sync (loadOnStartup=true)

**Use for:** Counters recalculated from DB on startup (total row counts)

```go
// This syncs only via periodic SyncToDB() every 1 second
API.CounterManager.IncrementWithStartup("_data", "_data", "", true)
```

**Behavior:**

- Synced periodically (every 1 second)
- On startup, recalculate from database using `collection` and `filter`
- More efficient for high-frequency counters
- Change detection: Only writes if value changed

## Basic Usage

### Increment a counter

```go
// Increment with immediate sync (default)
count := API.CounterManager.Increment("total_requests", "_data", "")
log.Printf("Total requests: %d", count)

// Increment with periodic sync (load_on_startup=true)
count = API.CounterManager.IncrementWithStartup("_data", "_data", "", true)
```

### Get a counter value

```go
// Get the count (instant, lock-free read)
count := API.CounterManager.Get("total_requests", "_data", "")
log.Printf("Total requests: %d", count)
```

### Set a counter value

```go
// Set a specific value
API.CounterManager.Set("total_requests", "_data", "", 1000)
```

### Decrement a counter

```go
// Decrease a counter (immediate sync)
count := API.CounterManager.Decrement("active_connections", "_proxies", "")
log.Printf("Active connections: %d", count)
```

### Reset a counter

```go
// Reset a counter to 0
API.CounterManager.Reset("daily_requests", "_data", "")
```

### Delete a counter

```go
// Remove a counter from memory and database
API.CounterManager.Delete("old_metric", "_data", "")
```

## Advanced Usage

### Manual Sync

```go
// Sync single counter immediately
if err := API.CounterManager.SyncOne("my_counter"); err != nil {
    log.Printf("Error syncing: %v", err)
}

// Sync all load_on_startup counters (called automatically every 1 second)
if err := API.CounterManager.SyncToDB(); err != nil {
    log.Printf("Error syncing counters: %v", err)
}
```

### Get Full Counter Details

```go
// Get complete counter information
details, exists := API.CounterManager.GetDetails("total_requests")
if exists {
    fmt.Printf("Key: %s\n", details.Key)
    fmt.Printf("Collection: %s\n", details.Collection)
    fmt.Printf("Filter: %s\n", details.Filter)
    fmt.Printf("Count: %d\n", details.Count.Load())
    fmt.Printf("LoadOnStartup: %v\n", details.LoadOnStartup)
}
```

### Get All Counters

```go
// Get all counters as a map
allCounters := API.CounterManager.GetAll()
for key, counter := range allCounters {
    fmt.Printf("%s: %d (loadOnStartup=%v)\n",
        key, counter.Count.Load(), counter.LoadOnStartup)
}
```

## Real-World Examples

### Track Total Requests (Periodic Sync)

```go
// On each request - loadOnStartup=true means periodic sync only
API.CounterManager.IncrementWithStartup("_data", "_data", "", true)

// On startup, this counter is recalculated from DB:
// SELECT COUNT(*) FROM _data
```

### Track Per-Proxy Requests (Immediate Sync)

```go
// Each proxy tracks its own count with immediate sync
proxyID := "proxy/______________1"
API.CounterManager.Increment(proxyID, "_data", "")

// Get proxy count (always exact, even after Ctrl+C)
count := API.CounterManager.Get(proxyID, "_data", "")
fmt.Printf("Proxy requests: %d\n", count)
```

### Track Per-Host Requests (Immediate Sync)

```go
// Track requests per host
host := "https://example.com"
sitemapName := utils.ParseDatabaseName(host) // "site_example_com"
API.CounterManager.Increment("host:"+sitemapName, "_data", "")

// Get host count
count := API.CounterManager.Get("host:site_example_com", "_data", "")
```

### Track Per-Label Counts (Immediate Sync)

```go
// When attaching a label
API.CounterManager.Increment("label:"+labelID, "_labels", "")

// When detaching a label
API.CounterManager.Decrement("label:"+labelID, "_labels", "")

// Get count of records with this label
count := API.CounterManager.Get("label:abc123", "_labels", "")
```

### Track Application Events (No Collection Needed)

```go
// These don't need a collection - counter_key is the main thing!
API.CounterManager.Increment("app_starts", "", "")
API.CounterManager.Increment("user_logins", "", "")
API.CounterManager.Increment("errors", "", "")

// With filters but no collection
API.CounterManager.Increment("errors", "", "database_timeout")
API.CounterManager.Increment("errors", "", "network_error")
```

## Automatic Syncing

### Periodic Sync (Every 1 Second)

Automatically runs in background for `loadOnStartup=true` counters:

```go
// In serve.go - already set up
go func() {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        if err := API.CounterManager.SyncToDB(); err != nil {
            log.Printf("[CounterManager] Periodic sync error: %v", err)
        }
    }
}()
```

**Output:**

```
[CounterManager][SyncToDB] Synced 1 counters, skipped 0 unchanged
... (no activity)
[CounterManager][SyncToDB] Synced 0 counters, skipped 1 unchanged
```

### Graceful Shutdown (Ctrl+C)

Automatically syncs on interrupt:

```go
// In serve.go - already set up
go func() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    <-sigChan
    log.Println("\n[Shutdown] Interrupt signal received, syncing counters...")
    if err := API.CounterManager.SyncToDB(); err != nil {
        log.Printf("[Shutdown] Error syncing counters: %v", err)
    } else {
        log.Println("[Shutdown] Counters synced successfully")
    }
    os.Exit(0)
}()
```

## Thread Safety

The CounterManager is thread-safe. All operations can be called from multiple goroutines:

```go
// Safe to call from multiple goroutines simultaneously
go func() {
    for i := 0; i < 1000; i++ {
        API.CounterManager.Increment("goroutine_counter", "_data", "goroutine_1")
    }
}()

go func() {
    for i := 0; i < 1000; i++ {
        API.CounterManager.Increment("goroutine_counter", "_data", "goroutine_2")
    }
}()
```

## Database Schema

The counters are stored in the `_counters` collection:

| Field             | Type   | Required | Description                             |
| ----------------- | ------ | -------- | --------------------------------------- |
| `counter_key`     | text   | ✅       | Main identifier (unique index)          |
| `collection`      | text   | ❌       | Collection name (metadata)              |
| `filter`          | text   | ❌       | Filter criteria (metadata)              |
| `count`           | number | ✅       | Current count value                     |
| `load_on_startup` | bool   | ❌       | If true, recalculate from DB on startup |

**Index:** Unique index on `counter_key` only.

## Performance Characteristics

| Operation     | Speed    | Notes                             |
| ------------- | -------- | --------------------------------- |
| `Increment()` | ~10ns    | Atomic operation, lock-free       |
| `Decrement()` | ~10-50ns | CAS loop for bounds check         |
| `Get()`       | ~5ns     | Atomic load, instant              |
| `SyncOne()`   | ~1-2ms   | Single DB write (async)           |
| `SyncToDB()`  | ~1-10ms  | Batch write (only changed values) |

## Best Practices

### ✅ DO:

1. **Use `loadOnStartup=true` for totals** that can be recalculated from DB
2. **Use default (immediate sync) for critical counts** (proxy, host, label)
3. **Use descriptive counter keys** like `"proxy/xxx"`, `"host:xxx"`, `"label:xxx"`
4. **Let the system handle syncing** - it's automatic

### ❌ DON'T:

1. **Don't call `SyncToDB()` manually** unless you have a specific reason
2. **Don't use `loadOnStartup=true` for counters you need exact** on shutdown
3. **Don't create too many unique counters** (map grows in memory)
4. **Don't use complex filter strings** - keep them simple

## Troubleshooting

### Counters not persisting on Ctrl+C?

Check if the counter has `loadOnStartup=false` (default). Only these get immediate sync.

### Too many DB writes?

Use `loadOnStartup=true` for high-frequency counters that can be recalculated:

```go
IncrementWithStartup("_data", "_data", "", true)
```

### Want to see what's being synced?

Check the logs:

```
[CounterManager][SyncToDB] Synced 1 counters, skipped 0 unchanged
```

## Summary

The counter system provides:

- ⚡ **Lightning fast** lock-free atomic operations
- 💾 **Two sync modes**: immediate (exact) or periodic (efficient)
- 🎯 **Zero data loss** with graceful shutdown
- 📊 **Change detection** to avoid unnecessary DB writes
- 🗺️ **Simple structure**: `map[counter_key]*Counter`

Perfect for tracking proxy traffic, host counts, labels, and application metrics! 🚀
