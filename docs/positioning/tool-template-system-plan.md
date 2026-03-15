# Tool Template System & New MCP Tools - Implementation Plan

**Date:** 2026-03-15
**Status:** Planned

## Overview

Refactor TaskWing's MCP tools and slash commands from a 6-file-per-tool manual
registry system to a single-file self-registering pattern. Then add new tools
using the pattern.

## Naming Convention

No change. Keep current convention:
- MCP tools: short names (`ask`, `plan`, `code`)
- Slash commands: namespaced (`/taskwing:ask`, `/taskwing:plan`)

## Final Tool Roster

### MCP Tools (9 total, under 12 cap)

| # | Tool | Actions | Status |
|---|------|---------|--------|
| 1 | `ask` | search (default), explain (new) | Existing + expand |
| 2 | `task` | next, current, start, complete, session (new) | Existing + expand |
| 3 | `plan` | clarify, decompose, expand, generate, finalize, audit, goal (new) | Existing + expand |
| 4 | `code` | find, search, explain, callers, impact, simplify, depends (new) | Existing + expand |
| 5 | `debug` | (unchanged) | Existing |
| 6 | `remember` | (unchanged) | Existing |
| 7 | `health` | coverage, contradictions, stale, summary | New (v1.22.0) |
| 8 | `onboard` | overview, walkthrough, stack | New (v1.23.0) |
| 9 | `review` | diff, patterns, compliance | New (v1.23.0) |

### Slash Commands (18 total)

**Architecture & Knowledge:**
- `/taskwing:ask` -> ask search
- `/taskwing:explain` -> ask explain
- `/taskwing:remember` -> remember
- `/taskwing:health` -> health summary

**Planning & Tasks:**
- `/taskwing:goal` -> plan goal
- `/taskwing:plan` -> plan clarify
- `/taskwing:next` -> task next
- `/taskwing:done` -> task complete
- `/taskwing:status` -> task current

**Code Intelligence:**
- `/taskwing:code` -> code find
- `/taskwing:simplify` -> code simplify
- `/taskwing:impact` -> code depends
- `/taskwing:review` -> review diff

**Debugging:**
- `/taskwing:debug` -> debug

**Onboarding:**
- `/taskwing:onboard` -> onboard overview
- `/taskwing:tour` -> onboard walkthrough
- `/taskwing:stack` -> onboard stack

**Session:**
- `/taskwing:session` -> task session

## Architecture

### Package Structure

```
internal/tools/
    definition.go          -- Tool, Action, SlashMapping, Request, Response
    registry.go            -- Register(), All(), Get(), AllSlashCommands()
    dispatch.go            -- Action resolution, input parsing
    ask/ask.go
    task/task.go
    plan/plan.go
    code/code.go           -- may split into code.go + find.go + impact.go etc
    debug/debug.go
    remember/remember.go
    health/health.go       -- NEW
    onboard/onboard.go     -- NEW (v1.23.0)
    review/review.go       -- NEW (v1.23.0)
```

### How It Works

1. Each tool file has init() that calls tools.Register()
2. cmd/mcp_server.go imports tool packages for side effects
3. MCP server loops tools.All() to register handlers
4. Slash generator uses tools.AllSlashCommands()
5. Doc generator uses tools.AllMCPTools()

Adding a new tool = create one file + add one import line.

### Key Types

```go
type Tool struct {
    Name          string
    Description   string
    InputSchema   json.RawMessage
    Actions       []Action
    SlashCommands []SlashMapping
    Handle        func(ctx context.Context, req Request) (Response, error)
}
```

## Migration Steps

| Step | What | Effort | Risk |
|------|------|--------|------|
| 0 | Create framework (types + registry + dispatch) | 3h | None (additive) |
| 1 | Migrate `remember` (simplest tool) | 2h | Low |
| 2 | Wire slash/doc generation to registry | 2h | Medium |
| 3 | Migrate `ask` + add `explain` action | 4h | Medium |
| 4 | Migrate `debug` | 1h | Low |
| 5 | Add `health` tool (new) | 3h | Low |
| 6 | Migrate `task` + add `session` action | 4h | Medium |
| 7 | Migrate `plan` + add `goal` action | 4h | Medium |
| 8 | Migrate `code` + add `depends` action | 6h | High |
| 9 | Delete old registries and handlers | 2h | Low |
| 10 | Add `onboard` + `review` tools | 4h each | Low |

Total: ~5 days for migration + 3 new tools.

## Metrics

| Before | After |
|--------|-------|
| 6 files to add a tool | 2 files (tool file + import) |
| 3 registries to sync | 1 registry (tools.All()) |
| ~2 hours to add a tool | ~30 min |
| High doc drift risk | Zero (derived from registry) |
