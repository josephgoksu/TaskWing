# Getting Started with TaskWing v2

> **AI-Native Task Management**

---

## Installation

```bash
# One-liner install (Mac/Linux)
curl -fsSL https://taskwing.app/install.sh | bash

# Or with Go (Requires Go 1.24+)
go install github.com/josephgoksu/TaskWing@latest
```

> **Note:** The binary is installed as `taskwing`. We recommend aliasing it to `tw` for brevity:
> `alias tw=taskwing`

---

## Quick Start

### Bootstrap Your Project

```bash
cd your-project

# Required: set your LLM API key (provider-specific)
export OPENAI_API_KEY=...

# Preview what TaskWing will detect
taskwing bootstrap --preview

# Run bootstrap to auto-generate project memory
taskwing bootstrap

# Alternative: use local Ollama models (no API key needed)
export TASKWING_LLM_PROVIDER=ollama
export TASKWING_LLM_MODEL=llama2
taskwing bootstrap --preview
```

---

## The Complete Workflow

### Step 1: Bootstrap Your Project

```bash
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

💡 This was a preview. Run 'taskwing bootstrap' to save to memory.
```

If satisfied:
```bash
taskwing bootstrap

✓ Bootstrap complete:
  • Features created: 6
  • Decisions created: 24
  • Relationships created: 12
  • Knowledge nodes created: 30
```

### Step 2: Add Knowledge Manually

```bash
# Add any text — AI classifies it automatically
tw add "We chose Go because it's fast and deploys as a single binary"
# ✓ Added [decision]: We chose Go because it's fast...

tw add "The auth module handles OAuth2 and session management"
# ✓ Added [feature]: The auth module handles...

tw add "TODO: implement webhook retry with exponential backoff"
# ✓ Added [plan]: TODO: implement webhook retry...
```

### Step 3: View Your Knowledge

```bash
# List all knowledge nodes
tw list

## 🎯 Decision (15)
  • Use Go for backend
  • Use LanceDB for vector search
  ...

## 📦 Feature (6)
  • Backend API
  • Chrome Extension
  ...

## 📋 Plan (3)
  • Implement webhook retry
  ...

Total: 24 nodes

# Filter by type
tw list decision
tw list plan
```

### Step 4: Search Semantically

```bash
# Find relevant knowledge
tw context "error handling"

Context for: "error handling"

1. 🎯 [decision] (85% match)
   Use structured error types for API responses
   ID: n-abc123

2. 📦 [feature] (72% match)
   Backend API: Primary server providing error handling...
   ID: n-def456
```

### Step 5: Use with AI Tools

```bash
# Start MCP server for Claude Code, Cursor, etc.
taskwing mcp

🎯 TaskWing MCP Server Starting...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
AI-Native Task Management
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ MCP connection established
```

Now when you ask AI about your code, it has full project context with semantic search.

---

## Command Reference

### Knowledge Commands

| Command | Description |
|---------|-------------|
| `tw add "text"` | Add knowledge (AI classifies type) |
| `tw add "text" --type decision` | Add with manual type |
| `tw list` | List all nodes |
| `tw list <type>` | List nodes by type (decision/feature/plan/note) |
| `tw context "query"` | Semantic search |
| `tw node show <id>` | Show a node |
| `tw node update <id> --summary/--type` | Update node fields |
| `tw node delete --type <type>` | Bulk delete nodes by type (safe prompt) |

### Planning & Tasks

| Command | Description |
|---------|-------------|
| `tw plan new "goal"` | Create a plan from a goal |
| `tw plan list` | List all plans |
| `tw plan start <id>` | Set active plan for current work |
| `tw plan status` | Show active plan progress |
| `tw plan show <id>` | Show a plan |
| `tw plan update <id> --goal/--status` | Update plan fields |
| `tw plan archive <id>` | Archive a plan |
| `tw plan unarchive <id>` | Unarchive a plan |
| `tw task list` | List tasks grouped by plan |
| `tw task show <id>` | Show a task |
| `tw task update <id> --status` | Update task status |
| `tw task done <id>` | Mark task as completed |
| `tw task delete <id>` | Delete a task |

### Bootstrap & Maintenance

| Command | Description |
|---------|-------------|
| `tw bootstrap` | Auto-generate from repo with LLM |
| `tw bootstrap --preview` | Preview without saving |
| `tw memory check` | Validate integrity |
| `tw memory repair` | Fix integrity issues |

### MCP Server

| Command | Description |
|---------|-------------|
| `tw mcp` | Start MCP server (stdio) |

### Output Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--quiet` | Minimal output |
| `--verbose` | Detailed output |

---

## What Gets Created

```
.taskwing/
├── memory/
│   ├── memory.db            # SQLite with nodes, node_edges tables
│   ├── index.json           # Cache for quick access
│   └── features/            # Legacy markdown snapshots
```

---

## Next Steps

- [Bootstrap Internals](BOOTSTRAP_INTERNALS.md) — How repo scanning works
- [Data Model](../architecture/DATA_MODEL.md) — Storage format details
- [System Design](../architecture/SYSTEM_DESIGN.md) — Technical design
