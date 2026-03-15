# Query-Time Freshness Validation - Implementation Plan

**Date:** 2026-03-15
**Status:** Planned

## Problem

After bootstrap, TaskWing's knowledge graph goes stale as the codebase changes.
Day 1: 170 accurate nodes. Day 30: half describe code that no longer exists.
No hooks, no daemons, no cron -- validate freshness at query time, the exact
moment it matters.

## Design

Every MCP query validates the findings it's about to return:

```
MCP query -> find matching nodes -> stat() evidence files -> adjust confidence -> return with freshness metadata
```

Three levels:
- **Level 1**: stat() mtime check (always, inline, <1ms per node)
- **Level 2**: re-read changed files, check if snippets still exist (only on stale files, 1-5ms)
- **Level 3**: queue for full re-analysis (async, background)

## Coverage

| Scenario | Covered? |
|----------|----------|
| Uncommitted WIP changes | Yes (mtime changes on save) |
| Mid-session refactoring | Yes (every query re-checks) |
| Overnight team changes | Yes (first query catches it) |
| Branch switches | Yes (files change, mtimes change) |
| No git at all | Yes (uses filesystem, not git) |
| Long sessions | Yes (every query is a checkpoint) |

Estimated coverage: ~95%. Only gap: semantic changes in OTHER files that
invalidate a finding whose own file didn't change.

## Schema Changes

Add to `nodes` table:

```sql
ALTER TABLE nodes ADD COLUMN last_verified_at TEXT;
ALTER TABLE nodes ADD COLUMN last_verified_level INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN original_confidence REAL;
```

Uses existing `migrateAddColumn` pattern. NULL = never checked = stale on first query.

## Type Changes

### Node struct (internal/memory/models.go)

```go
LastVerifiedAt     *time.Time  // When last freshness check ran
LastVerifiedLevel  int         // 0=never, 1=stat, 2=snippet, 3=full
OriginalConfidence *float64    // Confidence at ingest, before decay
```

### NodeResponse (internal/knowledge/response.go)

```go
FreshnessStatus string   // "fresh", "stale", "missing", "no_evidence"
FreshnessAge    string   // "2h ago", "3d ago"
StaleFiles      []string // Which evidence files changed
```

## Integration Point

Single hook in `internal/app/ask.go` at end of `Query()`. All MCP tools
that consume knowledge (ask, code, debug) go through this path.

```go
if a.ctx.BasePath != "" {
    freshness.ValidateNodeResponses(ctx, a.ctx.Repo, a.ctx.BasePath, result.Results)
}
```

## Confidence Adjustment

Multiplicative decay with floor of 0.1:

| Condition | Decay Factor |
|-----------|-------------|
| All evidence files unchanged | 1.0 |
| Some files changed, snippets still present (Level 2) | 0.95 |
| Some files changed, snippets moved | 0.85 |
| Some files changed, snippets gone | 0.7 |
| Evidence file deleted | 0.4 |
| All evidence deleted | 0.2 |

`adjustedConfidence = originalConfidence * decayFactor` (never below 0.1)

## Edge Cases

| Edge Case | Handling |
|-----------|---------|
| Relative paths | Always resolve against project root via filepath.Join(basePath, path) |
| Deleted files | Mark as "missing", decay 0.4. Distinct from "contradicted". |
| Moved files (git mv) | stat() fails. Future: grep_pattern fallback. For now: mark as missing. |
| No evidence | Return "no_evidence" status. Exempt from freshness checks. |
| Build artifacts (dist/, node_modules/) | Skip list of path patterns. Don't decay for generated file changes. |
| Large files (>1MB) | Level 1 always. Level 2 skipped (snippet matching too slow). |
| Permission errors | Return "inaccessible". Don't decay confidence. |
| Network mounts | 100ms timeout per stat(). Return "unknown" on timeout. |
| Clock skew | Compare mtime deltas (current stat vs stored stat), not absolute times. |

## MCP Response Format

```
1. **JWT Authentication** (decision) [verified 2h ago]
   Uses RS256 with key rotation...

2. **Session Middleware** (pattern) [STALE: auth/session.go changed]
   Express middleware for session management...
   Confidence: 0.63 (adjusted from 0.9 for stale evidence)

3. **OAuth2 Provider** (decision) [WARNING: evidence file deleted]
   Google OAuth2 integration...
   Confidence: 0.18 (evidence no longer exists)
```

## New Files

| File | Lines (est.) | Purpose |
|------|-------------|---------|
| `internal/freshness/validate.go` | ~120 | Level 1 + Level 2 validation |
| `internal/freshness/cache.go` | ~60 | In-memory stat cache (60s TTL) |
| `internal/freshness/queue.go` | ~50 | Level 3 reanalysis queue |
| `internal/freshness/skip.go` | ~30 | Build artifact skip patterns |

## Modified Files

| File | Change |
|------|--------|
| `internal/memory/models.go` | Add 3 freshness fields to Node |
| `internal/memory/sqlite.go` | Migration + update SELECT/INSERT/UPDATE |
| `internal/knowledge/response.go` | Add freshness fields to NodeResponse |
| `internal/mcp/presenter.go` | Render freshness in markdown |
| `internal/app/ask.go` | Hook freshness validation into Query() |

## Ship Order

| Level | What | Effort | Ship |
|-------|------|--------|------|
| 1 | stat() mtime check + confidence decay + response metadata | 2 days | v1.22.0 |
| 2 | Snippet re-validation on stale files | 1 day | v1.22.0 |
| 3 | Async reanalysis queue + bootstrap --incremental integration | 2 days | v1.23.0 |

## Performance Budget

| Level | Per-node | 5-node query | When |
|-------|---------|-------------|------|
| 1 | ~100us | ~1.5ms | Always |
| 2 | ~1-5ms | ~5-15ms | Only stale |
| 3 | ~2-10s | N/A | Async/background |

Optimization: 60-second in-memory stat cache to avoid re-statting same files
across multiple queries in a session.
