# DebugLogger Design Specification

> **Package**: `internal/bootstrap/debug`
> **Status**: Design
> **Author**: TaskWing Team
> **Created**: 2026-01-20

---

## Overview

The DebugLogger provides comprehensive debug logging for `taskwing bootstrap --debug`. It writes structured JSONL logs to both stderr (for immediate visibility) and a timestamped file (for later analysis).

## Goals

1. **Dual Output**: Write to stderr AND `.taskwing/logs/debug-{timestamp}.log`
2. **Structured Logs**: JSONL format for easy parsing and grep
3. **Security**: Automatically redact sensitive values (API keys, tokens, secrets)
4. **Timing**: Track phase durations with `StartPhase()` helper
5. **Retention**: Keep last 5 log files, auto-prune older ones
6. **Thread-Safe**: Safe for concurrent use from multiple goroutines

---

## API Specification

### Constructor

```go
func NewDebugLogger(opts Options) (*DebugLogger, error)
```

Creates a new DebugLogger. Returns error if log directory cannot be created or file cannot be opened.

### Options Struct

```go
type Options struct {
    // OutputDir is the directory for log files.
    // Default: ".taskwing/logs"
    OutputDir string

    // EnableStderr controls whether logs are also written to stderr.
    // Default: true
    EnableStderr bool

    // RetentionCount is the number of log files to keep.
    // Default: 5
    RetentionCount int

    // Component is the default component name for log entries.
    // Default: "bootstrap"
    Component string

    // StderrFormat controls stderr output format.
    // "json" = JSONL (same as file), "text" = human-readable colored
    // Default: "text"
    StderrFormat string
}
```

### Core Methods

```go
// Debug logs a debug-level message
func (l *DebugLogger) Debug(event, message string, metadata map[string]any)

// Info logs an info-level message
func (l *DebugLogger) Info(event, message string, metadata map[string]any)

// Warn logs a warning-level message
func (l *DebugLogger) Warn(event, message string, metadata map[string]any)

// Error logs an error-level message
func (l *DebugLogger) Error(event, message string, metadata map[string]any)

// StartPhase begins timing a phase and returns a stopper function.
// The stopper logs duration_ms when called.
func (l *DebugLogger) StartPhase(phase string, metadata map[string]any) func(err error)

// Close flushes buffers, closes the file, and runs retention pruning.
func (l *DebugLogger) Close() error

// LogPath returns the path to the current log file.
func (l *DebugLogger) LogPath() string
```

### Utility Functions

```go
// SanitizeEnvMap redacts sensitive values from an environment map.
// Keys matching redaction patterns have values replaced with "***REDACTED***".
func SanitizeEnvMap(env map[string]string) map[string]string

// SanitizeMetadata recursively redacts sensitive values in metadata.
func SanitizeMetadata(metadata map[string]any) map[string]any
```

---

## JSONL Schema

Each log entry is a single JSON line with these fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `timestamp` | string | Yes | RFC3339Nano UTC timestamp |
| `level` | string | Yes | One of: `debug`, `info`, `warn`, `error` |
| `component` | string | Yes | Source component (e.g., `bootstrap`, `llm`, `agent`) |
| `event` | string | Yes | Event identifier (e.g., `phase_start`, `config_loaded`) |
| `message` | string | Yes | Human-readable message |
| `duration_ms` | int | No | Duration in milliseconds (for phase_end events) |
| `metadata` | object | No | Additional structured data |
| `agent` | string | No | Agent name (when relevant) |
| `phase` | string | No | Phase name (for phase events) |
| `error` | string | No | Error message (for error events) |

### Example Log Entries

```jsonl
{"timestamp":"2026-01-20T17:38:06.123456789Z","level":"info","component":"bootstrap","event":"debug_start","message":"Debug logging enabled","metadata":{"version":"v1.15.2","log_file":".taskwing/logs/debug-20260120T173806Z.log"}}
{"timestamp":"2026-01-20T17:38:06.124000000Z","level":"debug","component":"bootstrap","event":"phase_start","message":"Starting project detection","phase":"project_detection"}
{"timestamp":"2026-01-20T17:38:06.130000000Z","level":"info","component":"bootstrap","event":"project_detected","message":"Project root detected","metadata":{"root_path":"/Users/dev/myproject","git_root":"/Users/dev/myproject","marker_type":".git","is_monorepo":false}}
{"timestamp":"2026-01-20T17:38:06.131000000Z","level":"debug","component":"bootstrap","event":"phase_end","message":"Project detection complete","phase":"project_detection","duration_ms":7}
{"timestamp":"2026-01-20T17:38:06.135000000Z","level":"info","component":"llm","event":"config_loaded","message":"LLM configuration loaded","metadata":{"provider":"ollama","model":"gpt-oss:20b-cloud","has_api_key":false,"config_file":"~/.taskwing/config.yaml"}}
{"timestamp":"2026-01-20T17:38:06.140000000Z","level":"info","component":"agent","event":"agent_start","message":"Starting agent","agent":"git","metadata":{"base_path":"/Users/dev/myproject"}}
{"timestamp":"2026-01-20T17:38:08.500000000Z","level":"info","component":"agent","event":"agent_end","message":"Agent completed","agent":"git","duration_ms":2360,"metadata":{"findings_count":16}}
```

---

## Redaction Rules

### Pattern Matching

Redaction applies to keys (case-insensitive) matching:
- Contains: `KEY`, `TOKEN`, `SECRET`, `PASSWORD`, `CREDENTIAL`, `AUTH`
- Exact matches: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `TASKWING_LLM_APIKEY`
- HTTP headers: `Authorization`, `X-Api-Key`

### Regex Pattern

```go
var redactPattern = regexp.MustCompile(`(?i)(key|token|secret|password|credential|auth)`)
```

### Redaction Behavior

- Matching values are replaced with: `***REDACTED***`
- Redaction is applied recursively to nested maps and slices
- Original data is never modified; a sanitized copy is created

### Example: SanitizeEnvMap

**Input:**
```go
map[string]string{
    "TASKWING_LLM_PROVIDER": "ollama",
    "TASKWING_LLM_MODEL":    "llama3.2",
    "OPENAI_API_KEY":        "sk-abc123xyz",
    "HOME":                  "/Users/dev",
}
```

**Output:**
```go
map[string]string{
    "TASKWING_LLM_PROVIDER": "ollama",
    "TASKWING_LLM_MODEL":    "llama3.2",
    "OPENAI_API_KEY":        "***REDACTED***",
    "HOME":                  "/Users/dev",
}
```

---

## Retention Policy

### File Naming

Log files follow the pattern: `debug-{timestamp}.log`

Timestamp format: `20060102T150405Z` (UTC)

Example: `debug-20260120T173806Z.log`

### Symlink

A symlink `debug-latest.log` points to the most recent log file for convenience:
```bash
.taskwing/logs/debug-latest.log -> debug-20260120T173806Z.log
```

### Pruning Algorithm

On `Close()`:
1. List all files matching `debug-*.log` in OutputDir
2. Sort by filename (timestamp) descending (newest first)
3. Keep the first `RetentionCount` files
4. Delete remaining files
5. Update `debug-latest.log` symlink

---

## Concurrency Model

- **Thread-Safe**: All public methods are safe for concurrent use
- **Mutex Protection**: Internal mutex guards file writes
- **Buffered Writes**: File writes are buffered for performance
- **Immediate Stderr**: Stderr writes flush immediately for interactive use

### Implementation Notes

```go
type DebugLogger struct {
    mu         sync.Mutex
    file       *os.File
    writer     *bufio.Writer
    opts       Options
    startTime  time.Time
    logPath    string
    closed     bool
}
```

---

## Integration Points

### 1. cmd/bootstrap.go

```go
// In runBootstrap(), after flag parsing:
if flags.Debug {
    debugLogger, err := debug.NewDebugLogger(debug.Options{
        OutputDir:    filepath.Join(cwd, ".taskwing", "logs"),
        EnableStderr: true,
        Component:    "bootstrap",
    })
    if err != nil {
        return fmt.Errorf("create debug logger: %w", err)
    }
    defer debugLogger.Close()

    debugLogger.Info("debug_start", "Debug logging enabled", map[string]any{
        "version":  version,
        "log_file": debugLogger.LogPath(),
    })

    // Pass logger via context or parameter to downstream functions
}
```

### 2. internal/bootstrap/planner.go

```go
// In ProbeEnvironment():
if debugLogger != nil {
    stop := debugLogger.StartPhase("probe_environment", nil)
    defer func() { stop(nil) }()
}

// After detection:
if debugLogger != nil {
    debugLogger.Info("project_detected", "Project root detected", map[string]any{
        "root_path":   ctx.RootPath,
        "git_root":    ctx.GitRoot,
        "marker_type": ctx.MarkerType.String(),
        "is_monorepo": ctx.IsMonorepo,
    })
}
```

### 3. internal/config/llm_loader.go

```go
// In LoadLLMConfig():
if debugLogger != nil {
    // Log env vars (sanitized)
    envMap := map[string]string{
        "TASKWING_LLM_PROVIDER": os.Getenv("TASKWING_LLM_PROVIDER"),
        "TASKWING_LLM_MODEL":    os.Getenv("TASKWING_LLM_MODEL"),
        "OPENAI_API_KEY":        os.Getenv("OPENAI_API_KEY"),
    }
    debugLogger.Info("env_vars", "Environment variables", map[string]any{
        "env": debug.SanitizeEnvMap(envMap),
    })

    // Log resolved config
    debugLogger.Info("config_loaded", "LLM configuration loaded", map[string]any{
        "provider":    string(cfg.Provider),
        "model":       cfg.Model,
        "has_api_key": cfg.APIKey != "",
        "config_file": viper.ConfigFileUsed(),
    })
}
```

### 4. internal/ui/bootstrap_tui.go

```go
// In runAgent():
if debugLogger != nil {
    debugLogger.Info("agent_start", "Starting agent", map[string]any{
        "agent":     agent.Name(),
        "base_path": input.BasePath,
    })

    stop := debugLogger.StartPhase("agent_"+agent.Name(), map[string]any{
        "agent": agent.Name(),
    })
    defer func() { stop(err) }()
}

// After agent completes:
if debugLogger != nil {
    debugLogger.Info("agent_end", "Agent completed", map[string]any{
        "agent":          agent.Name(),
        "findings_count": len(output.Findings),
        "error":          errStr,
    })
}
```

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Cannot create logs directory | Return error from NewDebugLogger |
| Cannot create log file | Return error from NewDebugLogger |
| Write to file fails | Log to stderr, continue operation |
| Stderr write fails | Ignore, continue operation |
| Retention pruning fails | Log warning, continue operation |

---

## Testing Strategy

### Unit Tests

1. **JSONL Format**: Verify log entries match schema
2. **Redaction**: Test all redaction patterns, nested values
3. **Concurrency**: Parallel writes don't corrupt output
4. **Retention**: Correct files are pruned
5. **StartPhase**: Duration calculation accuracy

### Integration Tests

1. **End-to-End**: Run `tw bootstrap --debug` and verify:
   - Log file created at expected path
   - No secrets in file contents
   - Stderr output present
   - Phase timing logged

---

## File Structure

```
internal/bootstrap/debug/
├── design.md          # This specification
├── logger.go          # DebugLogger implementation
├── logger_test.go     # Unit tests
├── redact.go          # Redaction utilities
├── redact_test.go     # Redaction tests
└── retention.go       # File retention/pruning
```

---

## References

- [BOOTSTRAP_INTERNALS.md](../../../docs/development/BOOTSTRAP_INTERNALS.md) - Bootstrap architecture
- [TELEMETRY_POLICY.md](../../../docs/TELEMETRY_POLICY.md) - Data privacy requirements
