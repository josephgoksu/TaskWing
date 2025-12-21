# TaskWing v2 — Architecture & Roadmap

> **Created:** 2025-12-15
> **Updated:** 2025-12-17
> **Status:** Active Development

---

## Vision

**Institutional Knowledge Layer for Engineering Teams.**

TaskWing captures the decisions, context, and rationale behind your codebase—making it queryable by humans and AI.

**Problem:** The codebase shows WHAT exists. Nothing shows WHY it exists or HOW it evolved.

**Solution:** A structured, AI-queryable record of decisions, features, and relationships.

### Why Decisions Are the Moat

| What Others Build | What TaskWing Builds |
|-------------------|----------------------|
| Feature lists (Notion, Linear) | **Decision history** with rationale |
| Static docs (CLAUDE.md) | **Living context** that evolves |
| Single-user tools | **Team knowledge** that persists |

Features come and go. **Decisions explain WHY.** That's what new team members, AI tools, and auditors need.

---

## Strategic Roadmap

See [ROADMAP.md](./ROADMAP.md) for full version planning (v2.0 → v5.0).

**v2.0 Current Scope:**

- `taskwing add "text"` — Add knowledge (AI classifies)
- `taskwing list` — View nodes by type
- `taskwing context "query"` — Semantic search
- `taskwing bootstrap` — Auto-generate from repo
- `taskwing mcp` — AI integration

---

## Context Retrieval Strategy

MCP must return **relevant** context to AI tools, not everything. Strategy is phased:

| Phase | Strategy | Implementation | Validation Gate |
|-------|----------|----------------|-----------------|
| **v2.0** | Graph only | Use `GetRelated()` with edge traversal | Ship, get user feedback |
| **v2.1** | Add FTS5 | SQLite full-text search on decisions | Only if graph proves insufficient |
| **v2.2** | Embeddings | Vector similarity on features + decisions | Only if FTS5 fails semantic matching |

### Token Budget (not bytes)

MCP output should target **~500-1000 tokens** for optimal AI context:

| Content | Approx Tokens |
|---------|---------------|
| 1 feature + decisions | ~200-400 tokens |
| 3 related features | ~600-1200 tokens |
| Full dump (6 features) | ~2000+ tokens |

### v2.0 Implementation

```go
// MCP tool accepts optional scope parameter
func projectContext(scope string) Context {
    if scope != "" {
        feature := store.FindFeature(scope)
        related := store.GetRelated(feature.ID, depth=2)
        return loadContext(related)  // ~500 tokens
    }
    return store.GetIndex()  // Summary only
}
```

---

## System Overview

```
                              ┌─────────────────────────────────────┐
                              │           USER INTERFACES           │
                              └─────────────────────────────────────┘
                                              │
                 ┌──────────────────────────────────────────────────────────┐
                 │                          │                               │
                 ▼                          ▼                               ▼
    ┌────────────────────┐     ┌────────────────────┐          ┌────────────────────┐
    │    CLI (Go)        │     │   MCP Server (Go)  │          │   Web UI (Future)  │
    │                    │     │                    │          │                    │
    │  taskwing add      │     │  project-context   │          │  Vite + React +    │
    │  taskwing context  │     │       tool         │          │  shadcn/ui         │
    │  taskwing bootstrap│     │                    │          │                    │
    └─────────┬──────────┘     └─────────┬──────────┘          └─────────┬──────────┘
              │                          │                               │
              └──────────────────────────┼───────────────────────────────┘
                                         │
                                         ▼
                        ┌─────────────────────────────────────┐
                        │          MEMORY STORE               │
                        │       (Unified Interface)           │
                        │                                     │
                        │  • CreateFeature()    • Link()      │
                        │  • AddDecision()      • Unlink()    │
                        │  • GetDependencies()  • Check()     │
                        └─────────────────┬───────────────────┘
                                          │
           ┌──────────────────────────────┼──────────────────────────────┐
           │                              │                              │
           ▼                              ▼                              ▼
┌───────────────────────┐    ┌───────────────────────┐    ┌───────────────────────┐
│     memory.db         │    │   features/*.md       │    │    index.json         │
│     (SQLite)          │    │    (Markdown)         │    │     (Cache)           │
│                       │    │                       │    │                       │
│ ┌───────────────────┐ │    │  ┌─────────────────┐  │    │  {                    │
│ │ features          │ │    │  │ # Auth          │  │    │    "features": [...], │
│ │ decisions         │◄┼────┼──│                 │  │    │    "lastUpdated": ... │
│ │ edges             │ │    │  │ ## Decisions    │  │    │  }                    │
│ └───────────────────┘ │    │  │ - Use JWT...    │  │    │                       │
│                       │    │  └─────────────────┘  │    │  Regenerated from     │
│  SOURCE OF TRUTH      │    │  Human-readable      │    │  SQLite on demand     │
└───────────────────────┘    └───────────────────────┘    └───────────────────────┘



                         ┌─────────────────────────────────────┐
                         │         BOOTSTRAP SCANNER           │
                         └─────────────────────────────────────┘
                                          │
         ┌────────────────────────────────┼────────────────────────────────┐
         │                                │                                │
         ▼                                ▼                                ▼
┌─────────────────┐            ┌─────────────────┐            ┌─────────────────┐
│   Git History   │            │   Directories   │            │   ADR Files     │
│                 │            │                 │            │                 │
│  feat: commits  │            │  src/features/  │            │  docs/decisions │
│  git tags       │            │  packages/      │            │  CHANGELOG.md   │
└─────────────────┘            └─────────────────┘            └─────────────────┘



                              DATA FLOW (Context Loading)
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                                                                 │
│   1. AI asks: "I want to add payment webhooks"                                  │
│                              │                                                  │
│                              ▼                                                  │
│   2. Load index.json (~500 bytes) ─────────────────────────────────────────┐    │
│                              │                                             │    │
│                              ▼                                             │    │
│   3. Query SQLite: GetRelated("payments") → ["users", "orders"]            │    │
│                              │                                             │    │
│                              ▼                                             │    │
│   4. Load features/payments.md + features/users.md                         │    │
│                              │                                             │    │
│                              ▼                                             │    │
│   5. Return combined context (~3KB) ◄──────────────────────────────────────┘    │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Storage

> **Source of truth:** [DATA_MODEL.md](DATA_MODEL.md)

---

## CLI Commands

### Setup

```bash
taskwing bootstrap               # Initialize + auto-generate from repo (LLM-powered if OPENAI_API_KEY set)
taskwing bootstrap --preview     # Preview without saving
taskwing bootstrap --basic       # Heuristic scan only (no LLM calls)
```

### Knowledge

```bash
taskwing add "We chose Go for performance"          # AI classifies as decision
taskwing add "Auth handles OAuth2 and sessions"    # AI classifies as feature
taskwing add "TODO: implement retry logic"         # AI classifies as plan
taskwing list                                       # View all nodes
taskwing list decision                              # Filter by type
taskwing context "error handling"                  # Semantic search
```

### Maintenance

```bash
taskwing memory check            # Validate integrity
taskwing memory repair           # Fix issues
taskwing memory rebuild-index    # Regenerate cache
```

### MCP

```bash
taskwing mcp                     # Start MCP server (default: stdio transport)
taskwing mcp --port 3000         # (Planned) SSE transport on port 3000
```

### Spec & Planning

```bash
taskwing spec "Add OAuth"          # Create a feature spec with AI agents
taskwing spec list                 # List specifications
taskwing task "Implement login"    # Create a dev task from a spec
taskwing task list                 # List tasks
```

---

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--quiet` | Minimal output |
| `--verbose` | Detailed output |
| `--preview` | Dry run (no changes) |

---

## Core Interface: MemoryStore

> See [DATA_MODEL.md](DATA_MODEL.md) for full implementation.

```go
type MemoryStore interface {
    // Features
    CreateFeature(f Feature) error
    UpdateFeature(f Feature) error
    DeleteFeature(id string) error
    GetFeature(id string) (*Feature, error)
    ListFeatures() ([]FeatureSummary, error)

    // Relationships (renamed from edges)
    Link(from, to, relationType string) error
    Unlink(from, to, relationType string) error
    GetDependencies(featureID string) ([]string, error)
    GetDependents(featureID string) ([]string, error)
    GetRelated(featureID string, maxDepth int) ([]string, error)

    // Decisions
    AddDecision(featureID string, d Decision) error
    GetDecisions(featureID string) ([]Decision, error)
    DeleteDecision(id string) error

    // Maintenance
    RebuildIndex() error
    Check() ([]Issue, error)
    Repair() error
}
```

---

## Bootstrap Scanner

> See [BOOTSTRAP.md](BOOTSTRAP.md)

```go
type BootstrapScanner interface {
    Preview() (*BootstrapResult, error)  // Dry run
    Execute() error                       // Actually write
}
```

---

## MCP Interface

```go
{
    "name": "project-context",
    "description": "Get project memory for AI context"
}
```

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| CLI | Go + Cobra |
| Storage | SQLite (`modernc.org/sqlite`) |
| LLM | CloudWeGo Eino (multi-provider: OpenAI, Ollama) |
| MCP | `mcp-go-sdk` |
| Web UI | Vite + React + TS + Tailwind v4 + shadcn/ui |

---

## Package Structure

```
internal/
├── memory/
│   ├── store.go          # MemoryStore implementation
│   ├── sqlite.go         # SQLite operations
│   ├── markdown.go       # Markdown read/write
│   └── index.go          # Cache management
├── spec/                 # Spec & Task management
│   ├── models.go         # Spec/Task data models
│   └── store.go          # JSON/Markdown storage
├── agents/               # AI Agents (ReAct, Doc, Git)
│   ├── agent_graph.go    # Parallel execution graph
│   └── react_code_agent.go # Dynamic code analysis
├── knowledge/            # Semantic search & embeddings
├── server/               # HTTP API server
├── telemetry/            # Anonymous usage metrics
├── bootstrap/
│   ├── api_runner.go     # Bootstrap implementation
│   └── llm_analyzer.go   # LLM-powered analysis
└── llm/
    └── client.go         # Multi-provider ChatModel factory (OpenAI, Ollama)

cmd/
├── root.go
├── config.go             # Configuration and defaults
├── init.go
├── bootstrap.go
├── add.go                # Add knowledge (AI classifies)
├── list.go               # List nodes by type
├── context.go            # Semantic search
├── memory.go             # memory check/repair
└── mcp_server.go         # MCP server
```

---

## Related Docs

| Document | Purpose |
|----------|---------|
| [GETTING_STARTED.md](GETTING_STARTED.md) | Quick start guide |
| [DATA_MODEL.md](DATA_MODEL.md) | Storage schema |
| [BOOTSTRAP.md](BOOTSTRAP.md) | Repo scanning |
| [ERRORS.md](ERRORS.md) | Error messages |
