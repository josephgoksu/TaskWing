# ADR: Hybrid Context Binding for Task Enrichment

> **Status**: Accepted
> **Date**: 2025-01-18
> **Decision**: Use hybrid early+late binding for task context

---

## Context

TaskWing tasks need architectural context from the knowledge graph to help AI assistants work effectively. The system must decide WHEN to attach this context:

1. **Early binding**: Embed context at task creation time
2. **Late binding**: Fetch context at task consumption/display time

Both approaches have trade-offs, and different use cases benefit from different strategies.

---

## Decision

We implement a **hybrid approach** that combines both strategies:

### 1. Early Binding (Primary)

At task creation time (`PlanApp.parseTasksFromMetadata`):

```go
// internal/app/plan.go
t.EnrichAIFields()  // Generates SuggestedRecallQueries

// Execute recall queries and embed context
if a.TaskEnricher != nil && len(t.SuggestedRecallQueries) > 0 {
    if contextSummary, err := a.TaskEnricher(ctx, t.SuggestedRecallQueries); err == nil {
        t.ContextSummary = contextSummary  // Embedded in task record
    }
}
```

**Result**: `Task.ContextSummary` is populated with architectural context when the task is created.

### 2. Late Binding (Fallback)

At task presentation time (`FormatRichContext`):

```go
// internal/task/presentation.go
if t.ContextSummary != "" {
    // Use pre-computed early-bound context (preferred)
    recallContext = "\n" + t.ContextSummary
} else if len(t.SuggestedRecallQueries) > 0 && searchFn != nil {
    // Fallback: Fetch context dynamically
    for _, query := range t.SuggestedRecallQueries {
        results, _ := searchFn(ctx, query, 3)
        // ... aggregate results
    }
}
```

**Result**: Tasks always have context, even if created before this feature existed (backward compatibility).

---

## Data Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        TASK CREATION (Early Binding)                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. PlanningAgent generates task with title, description, keywords       │
│                          │                                               │
│                          ▼                                               │
│  2. EnrichAIFields() generates:                                          │
│     - Scope (inferred from keywords)                                     │
│     - SuggestedRecallQueries (3 queries)                                 │
│                          │                                               │
│                          ▼                                               │
│  3. TaskEnricher executes ALL recall queries                             │
│     - Calls RecallApp.Query() for each query                             │
│     - Aggregates results (deduped by summary)                            │
│     - Truncates content to 200 chars                                     │
│                          │                                               │
│                          ▼                                               │
│  4. ContextSummary embedded in Task record                               │
│     - Stored in SQLite with task                                         │
│     - Persisted across sessions                                          │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                      TASK CONSUMPTION (Late Binding)                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. FormatRichContext() called by hook/MCP                               │
│                          │                                               │
│                          ▼                                               │
│  2. Check: Does Task.ContextSummary exist?                               │
│     ├── YES: Use it directly (fast path)                                 │
│     │                                                                    │
│     └── NO: Execute SuggestedRecallQueries (fallback)                    │
│             - Fetch fresh context from knowledge graph                   │
│             - Deduplicate by summary                                     │
│             - Truncate content to 300 chars for display                  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Trade-offs

### Early Binding

| Pros | Cons |
|------|------|
| **Reliable**: Context always available | **Staleness**: May not reflect latest knowledge |
| **Fast**: No runtime queries needed | **Storage**: Increases task record size |
| **Offline-capable**: Works without recall service | **One-time**: Context frozen at creation time |

### Late Binding

| Pros | Cons |
|------|------|
| **Fresh**: Always reflects current knowledge | **Service dependency**: Requires recall service |
| **Lighter storage**: Queries stored, not results | **Slower**: N queries per task display |
| **Adaptable**: Queries can evolve | **Unreliable**: May fail if service down |

### Hybrid Approach

| Benefit | Mechanism |
|---------|-----------|
| **Best of both worlds** | Early for reliability, late for freshness |
| **Backward compatible** | Old tasks still get context via late binding |
| **Graceful degradation** | Falls back if early binding unavailable |
| **Extensible** | Late binding can fetch additional context |

---

## Consequences

### Positive

1. **Tasks created with embedded context** - AI assistants receive relevant architectural decisions immediately
2. **Backward compatible** - Old tasks (without `ContextSummary`) still work via late binding
3. **Resilient** - System works even if recall service is temporarily unavailable (uses cached context)
4. **Efficient** - Avoids repeated recall queries during task execution

### Negative

1. **Context may become stale** - If knowledge graph is updated, early-bound context doesn't reflect changes
2. **Increased storage** - Task records are larger with embedded context
3. **Complexity** - Two code paths for context resolution

### Mitigations

1. **Staleness**: Late binding fallback can be used if context needs refresh (future enhancement)
2. **Storage**: Content truncated to reasonable limits (200 chars in storage, 300 chars in display)
3. **Complexity**: Clear separation - `models.go` for enrichment, `presentation.go` for display

---

## Implementation Notes

### Key Files

| File | Responsibility |
|------|----------------|
| `internal/task/models.go` | `EnrichAIFields()` - scope inference, keyword extraction, query generation |
| `internal/task/scope_config.go` | Configurable scope keywords via viper |
| `internal/app/plan.go` | `TaskEnricher` - executes recall queries at creation time |
| `internal/task/presentation.go` | `FormatRichContext()` - early binding display with late binding fallback |

### Configuration

Scope keywords are configurable via `.taskwing.yaml`:

```yaml
task:
  scopes:
    custom_domain:
      - keyword1
      - keyword2
  maxKeywords: 15   # default: 10
  minWordLength: 4  # default: 3
```

### Testing

- `internal/app/plan_enrichment_test.go` - Tests for context aggregation
- `internal/task/scope_config_test.go` - Tests for configurable scopes

---

## Related Documents

- [PLANNING_FORENSIC_DOCUMENTATION.md](./PLANNING_FORENSIC_DOCUMENTATION.md) - Detailed planning pipeline analysis
- [SYSTEM_DESIGN.md](./SYSTEM_DESIGN.md) - Overall system architecture
