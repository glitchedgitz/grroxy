# Process Management & SDK Integration Guide

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [SDK Setup for External Tools](#sdk-setup-for-external-tools)
4. [Process Management API](#process-management-api)
5. [Implementation Details](#implementation-details)
6. [Examples](#examples)
7. [Troubleshooting](#troubleshooting)

---

## Overview

This system allows you to manage processes (fuzzers, scanners, etc.) with real-time progress tracking. Processes are stored in the `_processes` collection and visible in the UI.

### Key Features

- ✅ Real-time progress tracking with atomic operations (no mutexes!)
- ✅ Create, update, complete, fail, pause, and kill processes
- ✅ SDK for external tools to connect to main app
- ✅ Automatic progress percentage calculation
- ✅ Error handling and state management

### Architecture

```
┌─────────────────┐                    ┌─────────────────┐
│  grroxy-tools   │   SDK (HTTP)      │   grroxy-app    │
│  (External)     │──────────────────▶│   (Main App)    │
│                 │                    │                 │
│  Fuzzer runs    │   Updates via SDK │  _processes DB  │
│  Results saved  │                    │  UI shows all   │
└─────────────────┘                    └─────────────────┘
```

**Important**: External tools (like `grroxy-tools`) use the SDK to update the main app's `_processes` collection, ensuring all processes are visible in the UI.

---

## Quick Start

### For External Tools (grroxy-tools)

```go
import "github.com/glitchedgitz/grroxy/internal/sdk"

// 1. Initialize SDK client
client := sdk.NewClient(
    "http://localhost:8090",
    sdk.WithAdminEmailPassword("admin@example.com", "password"),
)

// 2. Create a process
processID, _ := client.CreateProcess(sdk.CreateProcessRequest{
    Name: "My Fuzzer",
    Type: "fuzzer",
    Data: map[string]any{"target": "example.com"},
})

// 3. Update progress
for i := 0; i <= 100; i += 10 {
    client.UpdateProcess(processID, sdk.ProgressUpdate{
        Completed: i,
        Total:     100,
        Message:   fmt.Sprintf("Progress: %d%%", i),
    })
    time.Sleep(1 * time.Second)
}

// 4. Complete
client.CompleteProcess(processID, "Done!")
```

### For Internal Use (grroxy-app)

```go
import "github.com/glitchedgitz/grroxy/internal/process"

// Create process
id := process.CreateProcess(app, "Fuzzer", "Fuzzing target", "fuzzer", "running", data, "")

// Update progress
process.UpdateProgress(app, id, process.ProgressUpdate{
    Completed: 50,
    Total:     100,
    Message:   "Halfway there!",
})

// Complete
process.CompleteProcess(app, id, "Success!")
```

---

## SDK Setup for External Tools

### Method 1: Environment Variables (Recommended)

```bash
export GRROXY_APP_URL="http://localhost:8090"
export GRROXY_ADMIN_EMAIL="admin@example.com"
export GRROXY_ADMIN_PASSWORD="your-secure-password"
```

```go
import (
    "os"
    "github.com/glitchedgitz/grroxy/internal/sdk"
)

// Initialize Tools struct
tools := &Tools{
    AppURL: os.Getenv("GRROXY_APP_URL"),
}

// Initialize SDK client
tools.AppSDK = sdk.NewClient(
    tools.AppURL,
    sdk.WithAdminEmailPassword(
        os.Getenv("GRROXY_ADMIN_EMAIL"),
        os.Getenv("GRROXY_ADMIN_PASSWORD"),
    ),
)

// Verify connection
if err := tools.AppSDK.Authorize(); err != nil {
    log.Fatalf("Failed to connect to main app: %v", err)
}
```

### Method 2: Configuration File

Create `~/.config/grroxy/tools.json`:

```json
{
  "app_url": "http://localhost:8090",
  "admin_email": "admin@example.com",
  "admin_password": "your-password"
}
```

Load it:

```go
type ToolsConfig struct {
    AppURL        string `json:"app_url"`
    AdminEmail    string `json:"admin_email"`
    AdminPassword string `json:"admin_password"`
}

func LoadConfig() (*ToolsConfig, error) {
    home, _ := os.UserHomeDir()
    configPath := filepath.Join(home, ".config", "grroxy", "tools.json")
    data, _ := os.ReadFile(configPath)

    var config ToolsConfig
    json.Unmarshal(data, &config)
    return &config, nil
}
```

---

## Process Management API

### Process Structure

```json
{
  "id": "abc123xyz",
  "name": "Fuzzer - example.com",
  "description": "Fuzzing example.com with wordlist",
  "type": "fuzzer",
  "state": "Running",
  "data": {
    "host": "example.com",
    "wordlist": "/path/to/wordlist.txt"
  },
  "input": {
    "completed": 450,
    "total": 1000,
    "progress": 45,
    "message": "Processing request 450/1000",
    "error": ""
  },
  "output": {}
}
```

### Process States

- `"In Queue"` - Waiting to start
- `"Running"` - Currently executing
- `"Completed"` - Finished successfully
- `"Killed"` - Stopped by user
- `"Failed"` - Error occurred
- `"Paused"` - Temporarily paused

### SDK Functions

#### CreateProcess

```go
processID, err := client.CreateProcess(sdk.CreateProcessRequest{
    Name:        "Fuzzer - example.com",
    Description: "Fuzzing example.com",
    Type:        "fuzzer",
    State:       "running", // Optional, defaults to "running"
    Data: map[string]any{
        "host": "example.com",
        "wordlist": "/path/to/wordlist.txt",
    },
    Input: &sdk.ProcessInput{ // Optional
        Completed: 0,
        Total:     1000,
        Message:   "Starting...",
    },
})
```

#### UpdateProcess

```go
err := client.UpdateProcess(processID, sdk.ProgressUpdate{
    Completed: 450,
    Total:     1000,
    Message:   "Processing request 450/1000",
    State:     "running", // Optional
})
```

Progress percentage is automatically calculated: `(completed / total) * 100`

#### CompleteProcess

```go
err := client.CompleteProcess(processID, "Fuzzing completed successfully")
```

#### FailProcess

```go
err := client.FailProcess(processID, "Connection timeout after 30 seconds")
```

#### PauseProcess

```go
err := client.PauseProcess(processID, "User requested pause")
```

#### KillProcess

```go
err := client.KillProcess(processID, "User cancelled operation")
```

---

## Implementation Details

### Atomic Operations (No Mutexes!)

The fuzzer uses atomic operations for high performance:

```go
type Fuzzer struct {
    // Progress tracking using atomic operations
    totalRequests     int64
    completedRequests int64
}

// Increment atomically
atomic.AddInt64(&f.completedRequests, 1)

// Read atomically
completed := atomic.LoadInt64(&f.completedRequests)
total := atomic.LoadInt64(&f.totalRequests)
```

### Periodic Updates

Don't update on every operation - use a ticker:

```go
progressTicker := time.NewTicker(1 * time.Second)
defer progressTicker.Stop()

for {
    select {
    case <-progressTicker.C:
        completed, total := fuzzer.GetProgress()
        client.UpdateProcess(processID, sdk.ProgressUpdate{
            Completed: completed,
            Total:     total,
            Message:   fmt.Sprintf("Processing: %d/%d", completed, total),
        })
    }
}
```

### Error Handling

Always complete or fail processes:

```go
defer func() {
    if err := recover(); err != nil {
        client.FailProcess(processID, fmt.Sprintf("Panic: %v", err))
    }
}()

// Your work here...

if err != nil {
    client.FailProcess(processID, err.Error())
} else {
    client.CompleteProcess(processID, "Success")
}
```

---

## Examples

### Example 1: Simple Fuzzer

```go
// Create process
id, _ := client.CreateProcess(sdk.CreateProcessRequest{
    Name: "Fuzzer - example.com",
    Type: "fuzzer",
    Data: map[string]any{
        "host": "example.com",
        "wordlist": "/path/to/wordlist.txt",
    },
})

// Simulate fuzzing
for i := 0; i <= 1000; i += 100 {
    time.Sleep(1 * time.Second)

    client.UpdateProcess(id, sdk.ProgressUpdate{
        Completed: i,
        Total:     1000,
        Message:   fmt.Sprintf("Processing request %d/1000", i),
    })
}

// Complete
client.CompleteProcess(id, "Fuzzing completed successfully")
```

### Example 2: Error Handling

```go
id, _ := client.CreateProcess(sdk.CreateProcessRequest{
    Name: "Scanner",
    Type: "scanner",
})

// Simulate work
time.Sleep(1 * time.Second)

// Something went wrong
client.FailProcess(id, "Connection timeout after 30 seconds")
```

### Example 3: User Cancellation

```go
id, _ := client.CreateProcess(sdk.CreateProcessRequest{
    Name: "Long Running Task",
    Type: "background",
})

// User clicks stop button
client.KillProcess(id, "User cancelled operation")
```

### Example 4: Fuzzer Integration (Real Implementation)

See `apps/tools/fuzzer.go` for complete example:

```go
// Create process via SDK
id, err := backend.AppSDK.CreateProcess(sdk.CreateProcessRequest{
    Name:        "Fuzzer",
    Description: fmt.Sprintf("Fuzzing %s", body.Host),
    Type:        "fuzzer",
    State:       "In Queue",
    Data: map[string]any{
        "config": config,
        "request": body,
    },
})

// Start fuzzer with progress tracking
go func() {
    progressTicker := time.NewTicker(1 * time.Second)
    defer progressTicker.Stop()

    for {
        select {
        case <-progressTicker.C:
            completed, total := fuzzer.GetProgress()
            backend.AppSDK.UpdateProcess(id, sdk.ProgressUpdate{
                Completed: completed,
                Total:     total,
                Message:   fmt.Sprintf("Processing: %d/%d requests", completed, total),
            })
        }
    }
}()

// On completion
completed, total := fuzzer.GetProgress()
backend.AppSDK.CompleteProcess(id, fmt.Sprintf("Completed: %d/%d requests", completed, total))

// On error
backend.AppSDK.FailProcess(id, fmt.Sprintf("Fuzzer error: %v", err))

// On user stop
backend.AppSDK.UpdateProcess(id, sdk.ProgressUpdate{
    Completed: completed,
    Total:     total,
    Message:   fmt.Sprintf("Stopped by user at %d/%d requests", completed, total),
    State:     "Killed",
})
```

---

## Troubleshooting

### SDK Client Not Initialized

**Error**: `SDK client not initialized`

**Solution**:

```go
if backend.AppSDK == nil {
    log.Fatal("SDK client not initialized. Set GRROXY_APP_URL and credentials.")
}
```

### Authentication Failed

**Error**: `401 Unauthorized`

**Solutions**:

- Verify admin email and password are correct
- Ensure admin account exists in main app
- Check that main app is running

### Connection Refused

**Error**: `connection refused`

**Solutions**:

- Verify main app is running: `curl http://localhost:8090/api/health`
- Check firewall settings
- Ensure correct port (default: 8090)

### Process Not Appearing in UI

**Checklist**:

1. ✓ SDK initialized? Check `backend.AppSDK != nil`
2. ✓ Main app running? Check `http://localhost:8090`
3. ✓ Credentials correct? Test with `client.Authorize()`
4. ✓ Check logs for errors

### Testing SDK Connection

```go
// Test connection
err := client.Authorize()
if err != nil {
    log.Fatalf("❌ Failed to connect: %v", err)
}
log.Println("✅ Successfully connected to main app")

// Test process creation
testID, err := client.CreateProcess(sdk.CreateProcessRequest{
    Name: "Test Process",
    Type: "test",
})
if err != nil {
    log.Fatalf("❌ Failed to create process: %v", err)
}
log.Printf("✅ Created test process: %s", testID)

// Clean up
client.CompleteProcess(testID, "Test completed")
log.Println("✅ Test completed successfully")
```

---

## Security Best Practices

⚠️ **Never commit credentials to version control**  
⚠️ **Use environment variables or secure config files**  
⚠️ **Restrict file permissions**: `chmod 600 ~/.config/grroxy/tools.json`  
⚠️ **Consider using a service account instead of admin**  
⚠️ **Use HTTPS in production**

---

## Files Reference

### Created/Modified Files

**SDK & Process Management:**

- `internal/sdk/process.go` - SDK process management functions
- `internal/process/db.go` - Internal process functions with progress tracking
- `internal/schemas/processes.go` - Process states (added Failed, Paused)

**Integration:**

- `apps/tools/main.go` - Added AppSDK field to Tools struct
- `apps/tools/fuzzer.go` - Updated to use SDK for all process operations

**Examples & Docs:**

- `examples/sdk_process_example.go` - Complete working examples
- `README_PROCESS_MANAGEMENT.md` - This file

**Fuzzer Updates:**

- `grx/fuzzer/fuzzer.go` - Added atomic counters for progress tracking

---

## Summary

### What Was Implemented

1. **SDK Process Management** - Complete API for external tools
2. **Atomic Operations** - High-performance counters (no mutexes)
3. **Progress Tracking** - Real-time updates with auto-calculated percentages
4. **Error Handling** - Proper state management for all scenarios
5. **SDK Integration** - External tools connect to main app's database

### JavaScript vs Go API

| JavaScript          | Go SDK                     |
| ------------------- | -------------------------- |
| `createProcess()`   | `client.CreateProcess()`   |
| `updateProcess()`   | `client.UpdateProcess()`   |
| `completeProcess()` | `client.CompleteProcess()` |
| `failProcess()`     | `client.FailProcess()`     |

### Next Steps

1. ✅ Set up environment variables or config file
2. ✅ Initialize SDK client when starting tools
3. ✅ Test connection before running fuzzers
4. ✅ Monitor processes in main app UI

**You're all set!** External tools will now properly update processes in the main app's database. 🚀
