# TaskWing v2 — Bootstrap

> **Purpose:** Auto-generate project memory from existing repositories

---

## The Problem

Users adopt TaskWing on **existing projects** with months/years of history.
Empty memory = useless AI context. Manual entry = too much friction.

**Solution:** `taskwing bootstrap` scans the repo and auto-generates memories.

---

## CLI Command

```bash
taskwing bootstrap                   # Execute (LLM-powered if OPENAI_API_KEY set)
taskwing bootstrap --preview         # Preview without saving
taskwing bootstrap --preview --json  # Machine-readable output
taskwing bootstrap --trace           # JSON event stream to file (.taskwing/logs/...)
taskwing bootstrap --trace --trace-stdout  # JSON event stream to stderr

taskwing bootstrap --basic           # Execute heuristic scan only (no LLM calls)
taskwing bootstrap --basic --preview # Preview heuristic scan only
```

Runs LLM-powered analysis when an API key is available (set `OPENAI_API_KEY` or `TASKWING_LLM_APIKEY`).
Writes to `.taskwing/memory/`.

---

## Data Sources (Currently Implemented)

| Source | What We Extract |
|--------|-----------------|
| Directory structure | Feature candidates from common module folders |
| Git conventional commits | Decision candidates from recent conventional commits |
| LLM inference (default) | Feature descriptions + decision reasoning + trade-offs + **relationships** |
| Documentation files | AGENTS.md, ARCHITECTURE.md, docs/*.md content |
| Deployment config | Dockerfile, docker-compose.yaml (service topology) |
| Entry points | main.go, index.ts, cmd/* (initialization patterns) |
| Config files | .env.example, vite.config.ts, Makefile, tsconfig.json |
| Import statements | Go internal package dependency graph |


## LLM Provider Configuration

TaskWing uses [CloudWeGo Eino](https://github.com/cloudwego/eino) for multi-provider LLM support.

### OpenAI (default)

```bash
export OPENAI_API_KEY=your-key
taskwing bootstrap --preview
```

### Ollama (local models)

```bash
# Start Ollama with a model
ollama pull llama2

# Configure TaskWing to use Ollama
export TASKWING_LLM_PROVIDER=ollama
export TASKWING_LLM_MODELNAME=llama2
taskwing bootstrap --preview
```

### Config Options

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `OPENAI_API_KEY` or `TASKWING_LLM_APIKEY` | — | API key for OpenAI |
| `TASKWING_LLM_PROVIDER` | `openai` | Provider: `openai` or `ollama` |
| `TASKWING_LLM_MODELNAME` | `gpt-5-mini-2025-08-07` | Model name |
| `TASKWING_LLM_BASEURL` | `http://localhost:11434` | Base URL (for Ollama) |

---

## Example Output

```
$ taskwing bootstrap --preview

┌──────────────────────────────────────────────────────────────┐
│  🧠 TaskWing Bootstrap — Architectural Intelligence          │
└──────────────────────────────────────────────────────────────┘

  This will analyze your codebase and generate:
  • Features with WHY they exist (not just what)
  • Key decisions with trade-offs and reasoning
  • Relationships between components

  ⚡ Using: gpt-5-mini-2025-08-07 (openai)

  Gathering context...
   📁 Scanning directory structure... (45 entries)
   📦 Reading package files... package.json, go.mod
   📄 Reading README... README.md (1500 chars)
   🔍 Analyzing git history... 234 commits

   Streaming: ........................... (2994 chunks)

🤖 LLM Architecture Analysis
═══════════════════════════════════════════════════════

## 1. Backend API (Go)
   Primary application API and business logic

   **Why it exists:**
   Provide a scalable, typed server to manage users, bookmarks, spaces...

   **Decisions:**
   • Use Go for core API server [high]
     Why: Go was chosen for benefits of a compiled, efficient language...
     Trade-offs: Sacrificed rapid prototyping speed...

   **Depends On:** Vector Search, Infrastructure
   **Related To:** Admin Dashboard, Chrome Extension

═══════════════════════════════════════════════════════

💡 This was a preview. Run 'taskwing bootstrap' to save to memory.
```

---

## Observability

TaskWing streams agent execution events into the Bootstrap TUI.

**Modes:**
- **Compact (default):** Per-agent status with current activity.
- **Details (toggle `t`):** Shows the last ~12 events for the selected agent.
- **Cycle agent (tab):** Switch the details pane between agents.
- **Trace (`--trace`):** Writes JSON event stream to `.taskwing/logs/bootstrap.trace.jsonl`.
- **Trace to stderr (`--trace-stdout`):** Emits JSON events to stderr (can interleave with TUI).

**Event fields (trace JSON):**
- `type` — event type (`agent_start`, `node_start`, `tool_call`, etc.)
- `timestamp` — RFC3339 timestamp
- `agent` — agent name
- `content` — human-friendly summary
- `metadata` — structured info (node type, tool args, durations)

### Bootstrap Summary

```
✓ Bootstrap complete:
  • Features created: 6
  • Decisions created: 24
  • Relationships created: 12
```

---

## Feature Detection

### From Directory Structure

```
src/features/auth/      → Feature: Auth
src/modules/payments/   → Feature: Payments
packages/api/           → Feature: API
```


---

## Decision Extraction

### From Conventional Commits (Subject Line)

TaskWing reads recent git history and extracts decision candidates from conventional commits:

```bash
git log --oneline | grep -E '^[a-f0-9]+ (feat|fix|refactor|perf)(\\(.+\\))?:'
```

Examples:
- `feat(auth): add OAuth2 login` → Decision under feature `Auth`
- `feat: implement user profiles` → Decision under feature `General`

---

## Edge Cases

| Scenario | Handling |
|----------|----------|
| No git history | Directory structure only |
| No conventional commits | Use folder names |
| Monorepo | Scope to current directory |
| Very old repo | All history included |

---

## What Gets Created

```
.taskwing/
├── memory/
│   ├── index.json         # Feature summaries
│   ├── memory.db          # Graph relationships
│   └── features/
│       ├── auth.md        # Auto-generated feature doc
│       ├── users.md
│       └── payments.md
```

Each feature file is pre-populated with:
- Feature name + one-liner
- Decisions (if any) with summary/why/trade-offs/date
- Notes placeholder

---

## .gitignore Updates

TaskWing creates `.taskwing/memory/.gitignore` (via `taskwing bootstrap`) to ignore generated/cache files:

```gitignore
memory.db-journal
memory.db-wal
memory.db-shm
index.json
```

**Why this split?**

| Storage | Contains | Why |
|---------|----------|-----|
| `memory.db` | Features, decisions, relationships | Canonical data store in v2.0 |
| `features/*.md` | Human-readable snapshot | Generated from SQLite |
| `index.json` | Summary cache | Quick MCP context loading (rebuildable) |

> **Note:** TaskWing v2.0 does not import markdown back into SQLite; manual edits in `features/*.md` may be overwritten.

---

## Sharing Between Machines (v2.0)

If you want TaskWing memory to persist across machines in v2.0, you have two options:

```bash
# Option A (recommended for now): commit .taskwing/memory/memory.db
git add .taskwing/memory/memory.db

# Option B: re-run bootstrap on each machine (manual decisions won't carry over)
taskwing bootstrap
```

For future/roadmap items, see ROADMAP.md.
