# Why TaskWing?

> **TL;DR**: TaskWing turns AI assistants from "smart interns who need constant guidance" into "team members who understand your architecture."

---

## The Problem

AI coding assistants (Claude Code, Cursor, Copilot, Codex, Gemini) are powerful but:

1. **No persistent memory** — They re-discover your architecture every session
2. **No constraint awareness** — They might violate your "MUST follow" rules
3. **No historical context** — They don't know WHY decisions were made
4. **High token usage** — They re-read files repeatedly
5. **Hallucination risk** — They make claims without evidence

---

## The Solution

TaskWing creates a **persistent knowledge graph** of your codebase that AI assistants can query via MCP (Model Context Protocol).

```
┌─────────────────────────────────────────────────────────────────┐
│                    WITHOUT TASKWING                              │
│                                                                  │
│   Developer ──► AI Tool ──► Scans files ──► Generic response    │
│                    │                                             │
│                    └── No memory, no constraints, no history     │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     WITH TASKWING                                │
│                                                                  │
│   Developer ──► AI Tool ──► TaskWing MCP ──► Targeted context   │
│                                   │                              │
│                    ┌──────────────┴──────────────┐              │
│                    │  • Decisions + WHY          │              │
│                    │  • Constraints (MUST rules) │              │
│                    │  • Patterns + evidence      │              │
│                    │  • Semantic relationships   │              │
│                    └─────────────────────────────┘              │
└─────────────────────────────────────────────────────────────────┘
```

---

## Comparison Tables

### Context & Knowledge

| Aspect | Without TaskWing | With TaskWing |
|--------|------------------|---------------|
| **Architectural knowledge** | AI re-discovers every session | Persisted in knowledge graph |
| **WHY decisions were made** | Lost unless in comments | Captured with reasoning + tradeoffs |
| **Constraints/Rules** | AI might violate them | Surfaced as MUST-follow rules |
| **Evidence for claims** | AI can hallucinate | File paths + line numbers verified |
| **Cross-session memory** | None (starts fresh) | Persistent `memory.db` |
| **Team knowledge sharing** | Copy-paste conversations | Commit `.taskwing/` to repo |

### Developer Experience

| Aspect | Without TaskWing | With TaskWing |
|--------|------------------|---------------|
| **Onboarding new dev** | "Read the codebase" | `tw list` shows architecture instantly |
| **"How does auth work?"** | AI scans files each time | `tw context --answer "auth"` with cached context |
| **Planning a feature** | AI guesses at constraints | `tw plan new` surfaces relevant rules |
| **Code review context** | Reviewer must know history | AI sees related decisions |
| **Finding related code** | Grep + manual search | Semantic search across knowledge |

### AI Assistant Efficiency

| Metric | Without TaskWing | With TaskWing |
|--------|------------------|---------------|
| **Context per query** | Full file contents | Targeted 500-1000 tokens |
| **Token usage** | High (re-reads files) | Low (pre-computed embeddings) |
| **Hallucination risk** | Higher | Lower (evidence-backed) |
| **Constraint violations** | Common | Prevented (constraints surfaced) |
| **Response relevance** | Generic | Project-specific |

---

## Real-World Example: "Add a new API endpoint"

| Step | Without TaskWing | With TaskWing |
|------|------------------|---------------|
| 1 | AI scans random files | AI gets: "HTTP handlers MUST use types from `types.gen.go`" |
| 2 | AI might create local types | AI follows OpenAPI-first constraint |
| 3 | AI might skip validation | AI knows: "Update `openapi.yaml` first, run codegen" |
| 4 | AI might use wrong router | AI knows: "Chi router pattern at `internal/api/`" |
| 5 | AI might miss tracing | AI knows: "OpenTelemetry instrumentation required" |

---

## More Scenarios

| Scenario | Without TaskWing | With TaskWing |
|----------|------------------|---------------|
| **New team member asks "why MongoDB?"** | "Ask someone who was here" | `tw context "database choice"` returns reasoning |
| **AI suggests adding Redis** | No context on existing decisions | Knows LanceDB already handles caching/vector |
| **PR review on auth changes** | Manual review of all auth code | AI sees JWT + refresh token decision |
| **Debugging production issue** | "What changed recently?" | `tw list` shows recent milestones |
| **Planning refactor** | Guess at dependencies | Knowledge graph shows component relationships |

---

## Cost & Efficiency

| Metric | Without TaskWing | With TaskWing |
|--------|------------------|---------------|
| **Bootstrap analysis** | N/A (no equivalent) | One-time ~$0.10 |
| **Per-query context tokens** | 10-50K tokens | 500-1K tokens |
| **Monthly AI cost (active dev)** | Higher | ~50-70% lower |
| **Onboarding time** | Days-weeks | Hours |
| **Knowledge loss on turnover** | High | Low (persisted) |

---

## How It Works

### 1. Bootstrap (one-time)

```bash
tw bootstrap
```

TaskWing analyzes your codebase with 4 parallel agents:

| Agent | Analyzes | Extracts |
|-------|----------|----------|
| **DocAgent** | `*.md` files | Features, Constraints |
| **CodeAgent** | Source code | Patterns, Decisions |
| **GitAgent** | Git history | Milestones, Evolution |
| **DepsAgent** | `package.json`, `go.mod` | Tech stack decisions |

### 2. Query (anytime)

```bash
tw list                           # See all knowledge
tw context "authentication"       # Semantic search
tw context --answer "how auth"    # AI-powered answer
```

### 3. MCP Integration (automatic)

AI assistants query TaskWing via MCP for relevant context before responding.

---

## What Gets Captured

```
.taskwing/
├── memory/
│   ├── memory.db          # SQLite knowledge graph
│   ├── index.json         # Quick-load cache
│   └── features/          # Human-readable snapshots
│       ├── auth.md
│       ├── payments.md
│       └── ...
```

### Knowledge Types

| Type | Icon | Example |
|------|------|---------|
| **Decision** | 🎯 | "MongoDB as primary datastore — chosen for flexible schema" |
| **Feature** | 📦 | "Semantic Search — vector-based bookmark search" |
| **Constraint** | ⚠️ | "MUST use types from `types.gen.go` for API handlers" |
| **Pattern** | 🧩 | "Contract-first API with OpenAPI code generation" |

---

## Getting Started

```bash
# Install
go install github.com/josephgoksu/TaskWing@latest

# Initialize + analyze your project
cd your-project
tw bootstrap

# See what was discovered
tw list

# Query specific topics
tw context "database"
```

See [GETTING_STARTED.md](../development/GETTING_STARTED.md) for detailed setup instructions.
