# Telemetry Package

This package provides anonymous usage analytics for TaskWing.

## Overview

The telemetry package implements GDPR-compliant, anonymous telemetry with:
- First-run consent prompt (opt-in)
- Anonymous data only (no PII)
- Local consent storage (`~/.taskwing/telemetry.json`)
- Easy opt-out via config or `--no-telemetry` flag

## Usage

Telemetry is automatically initialized via the root command's `PersistentPreRunE`.
Events are automatically flushed when commands complete via `PersistentPostRunE`.

### Manual Tracking (Internal Use)

```go
import "github.com/josephgoksu/TaskWing/internal/telemetry"

// Track a custom event
telemetry.Track(telemetry.Event{
    Name: "custom_event",
    Props: map[string]any{
        "key": "value",
    },
})

// Convenience methods
client := telemetry.GetClient()
client.TrackCommand("bootstrap", 1500, true, "")
client.TrackBootstrap("openai", 25, 100)
client.TrackSearch(5, true)
```

## Files

| File | Purpose |
|------|---------|
| `telemetry.go` | Core client, event tracking, HTTP posting |
| `consent.go` | First-run prompt, consent file management |
| `events.go` | Event type definitions |
| `telemetry_test.go` | Unit tests |

## Configuration

Telemetry can be disabled via:
1. First-run prompt (choose 'n')
2. Global flag: `taskwing --no-telemetry <command>`
3. Config command: `taskwing config telemetry disable`

## What We Collect

- Command names (not arguments)
- Success/failure status
- Error types (not details)
- Duration
- OS/architecture
- CLI version

## What We DON'T Collect

- File paths, project names, or code
- Search query content
- API keys or credentials
- IP addresses (not logged server-side)
- Any personally identifiable information
