# TaskWing Agent Evolution Plan

> **Created**: 2025-12-20
> **Status**: Planning
> **Owner**: Joseph Goksu

## Executive Summary

This document outlines the evolution of TaskWing's agent architecture from **batch analysis** (bootstrap once) to **continuous intelligence** (always watching, always learning). The plan leverages the CloudWeGo Eino framework to implement ReAct agents, graph orchestration, and real-time file watching.

---

## Current State Analysis

### Problem: Agents Don't Explore (SOLVED)

*   **Legacy Static Analysis**: The old `internal/bootstrap` package (static, hardcoded analysis) has been **deleted**.
*   **New ReAct Agents**: `internal/agents/react_code_agent.go` now drives analysis using dynamic tool exploration (`list_dir`, `read_file`, `grep_search`).

### Tool Usage

The tools defined in `tools.go` are now **actively used** by the `ReactCodeAgent`.

| Tool | Capability | Usage |
|------|-----------|-------|
| `ReadFileTool` | Read any file dynamically | **Active (ReAct)** |
| `GrepTool` | Search patterns across codebase | **Active (ReAct)** |
| `ListDirTool` | Explore directory structure | **Active (ReAct)** |
| `ExecCommandTool` | Run git, cat, head, tail, wc | **Active (ReAct)** |

---

## Architecture Vision

```
┌─────────────────────────────────────────────────────────────────┐
│                     CONTINUOUS INTELLIGENCE                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────┐    ┌─────────────────────────────────────────┐   │
│   │  Watch   │───▶│           Agent Dispatcher              │   │
│   │  Agent   │    │  ┌─────────┬─────────┬─────────────┐    │   │
│   └──────────┘    │  │DocAgent │CodeAgent│ DepsAgent   │    │   │
│        │          │  │         │         │             │    │   │
│   fsnotify        │  └────┬────┴────┬────┴──────┬──────┘    │   │
│        │          │       │         │           │           │   │
│   Debouncer       │       ▼         ▼           ▼           │   │
│        │          │  ┌─────────────────────────────────┐    │   │
│   Categorizer     │  │         Synthesizer             │    │   │
│                   │  │   (Cross-ref, Dedupe, Conflict) │    │   │
│                   │  └────────────────┬────────────────┘    │   │
│                   └───────────────────┼─────────────────────┘   │
│                                       ▼                          │
│                              ┌────────────────┐                  │
│                              │  Memory (DB)   │                  │
│                              └───────┬────────┘                  │
│                                      ▼                           │
│                              ┌────────────────┐                  │
│                              │  MCP Server    │───▶ AI Clients   │
│                              └────────────────┘                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Tool Conversion + ReAct Foundation

**Goal**: Enable agents to dynamically explore the codebase instead of using hardcoded context.

### 1.1 Convert Tools to Eino Format

| Task | Description | Status |
|------|-------------|--------|
| Convert `ReadFileTool` | Implement `tool.BaseTool` interface | TODO |
| Convert `GrepTool` | Implement `tool.BaseTool` interface | TODO |
| Convert `ListDirTool` | Implement `tool.BaseTool` interface | TODO |
| Create `EinoExecTool` | Wrap ExecCommandTool for Eino | TODO |

### 1.2 Build ReAct CodeAgent

Replace static analysis with ReAct (Reasoning + Acting) loop:

```go
// LLM decides what to explore
systemPrompt := `You are analyzing a codebase for architectural patterns.

Tools available:
- list_dir: Explore directory structure
- read_file: Read file contents
- grep_search: Find patterns across files

PROCESS:
1. Start with list_dir to understand structure
2. Read key files based on what you discover
3. Search for patterns to understand connections
4. When confident, provide structured findings`
```

### 1.3 Validation

- Compare ReAct agent findings vs current static approach
- Measure: files examined, patterns discovered, accuracy

---

## Phase 2: Graph Orchestration + Streaming

**Goal**: Enable agents to work together and provide real-time feedback.

### 2.1 Agent DAG

```
Input (Changes) ──┬── DocAgent ───┐
                  ├── CodeAgent ──┼── Synthesizer ── Output
                  └── DepsAgent ──┘
```

### 2.2 Synthesizer Agent

- Cross-reference findings from multiple agents
- Deduplicate similar discoveries
- Flag contradictions (e.g., docs say PostgreSQL, code uses SQLite)

### 2.3 Streaming + Callbacks

- Real-time token output during analysis
- OnStart/OnEnd/OnError hooks for observability
- Progress indicators for CLI

---

## Phase 3: Watch Agent (Continuous Intelligence)

**Goal**: Transform from "run once" to "always watching, always learning."

### 3.1 Core Components

| Component | Responsibility |
|-----------|----------------|
| `WatchAgent` | Monitor filesystem via fsnotify |
| `Debouncer` | Batch rapid changes (500ms-5s by category) |
| `Categorizer` | Route changes to appropriate agents |
| `Dispatcher` | Trigger agents with incremental context |

### 3.2 Routing Rules

| File Pattern | Agent | Debounce |
|-------------|-------|----------|
| `*.md`, `docs/**` | DocAgent | 1s |
| `*.go`, `*.ts`, `*.js` | CodeAgent | 500ms |
| `go.mod`, `package.json` | DepsAgent | 2s |
| `Dockerfile*`, `compose*` | DeployAgent | 1s |
| `.git/HEAD` (commit) | GitAgent | 5s |

### 3.3 Inter-Agent Communication

```go
type AgentMessage struct {
    FromAgent string
    ToAgent   string
    Type      string  // "request_analysis", "share_finding", "flag_conflict"
    Payload   any
}
```

### 3.4 Incremental Analysis

- Only analyze changed files (not full re-bootstrap)
- Track import graph to find affected files
- Mark findings as "stale" when source changes

### 3.5 CLI Integration

```bash
tw watch                    # Foreground mode
tw watch --daemon           # Background daemon
tw watch --verbose          # Detailed output
tw watch --mcp              # Stream to MCP clients
tw watch --include "src/**" # Filter paths
```

---

## Phase 4: Production Hardening

### 4.1 MCP Live Updates

- Push finding updates to connected AI clients
- Invalidate cached context on changes

### 4.2 Reliability

- Graceful shutdown on SIGTERM
- Crash recovery (resume from last state)
- Rate limiting for rapid file changes

### 4.3 Observability

- Metrics: files processed, agents triggered, findings generated
- Tracing: OpenTelemetry integration
- Logging: structured JSON logs

---

## Timeline

| Phase | Duration | Key Deliverable |
|-------|----------|-----------------|
| Phase 1 | 2 weeks | ReAct CodeAgent with dynamic exploration |
| Phase 2 | 2 weeks | Agent DAG with Synthesizer |
| Phase 3 | 3 weeks | Watch Agent with incremental analysis |
| Phase 4 | 2 weeks | Production-ready continuous mode |

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Files analyzed per run | ~15 (hardcoded) | 50+ (dynamic) |
| LLM calls per agent | 1 | 5-15 (ReAct iterations) |
| Cross-reference findings | 0 | 3-5 per run |
| Update latency | Manual run | <2s (real-time) |
| Time to first insight | 30s+ | <2s (streaming) |

---

## Open Questions

1. **Git Commits**: Should `tw watch` trigger on git commits in addition to file saves?
2. **Stale Findings**: Auto-remove after N days, or keep until manually resolved?
3. **Daemon Approach**: systemd, launchd, or simple nohup?

---

## References

- [Eino Documentation](https://www.cloudwego.io/docs/eino/overview/)
- [Eino GitHub](https://github.com/cloudwego/eino)
- [fsnotify](https://github.com/fsnotify/fsnotify)
