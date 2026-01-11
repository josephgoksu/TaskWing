---
title: I Built a Tool That Made Claude 122% Better at Understanding My Codebase
published: false
description: Your AI coding assistant forgets everything between sessions. Here is how I fixed it.
tags: ai, devtools, opensource, productivity
---

## The Problem That Was Driving Me Insane

Every Claude Code session:

```
Me: "Add authentication to the API"
Claude: *suggests Express.js*
Me: "We use Gin. I told you this yesterday."
Claude: "I apologize for the confusion..."
```

I timed it. **10+ minutes per session** re-explaining:
- Why we chose PostgreSQL over MongoDB
- Our file structure conventions
- That we use Zustand, not Redux
- The deployment constraints

My AI assistant had amnesia. Every. Single. Time.

## The Fix: Give AI Permanent Memory

I built [TaskWing](https://github.com/josephgoksu/TaskWing) — a CLI that extracts your architectural decisions and exposes them to AI via MCP.

```bash
brew install josephgoksu/tap/taskwing

taskwing bootstrap   # Scans your codebase
taskwing mcp         # Exposes to Claude/Cursor
```

That's it. Your AI now remembers your architecture permanently.

## The Results (Measured, Not Claimed)

I ran the same 5 architecture questions against Claude with and without TaskWing:

| Metric | Without | With TaskWing |
|--------|---------|---------------|
| Avg Score | 3.6/10 | 8.0/10 |
| Pass Rate | 0% | 100% |

**+122% improvement.** [Full methodology here](https://github.com/josephgoksu/TaskWing/blob/main/docs/development/EVALUATION.md).

> **Note**: 122% = (8.0 - 3.6) / 3.6. The score more than doubled from baseline.

## What It Captures

```
.taskwing/memory/
├── memory.db          # SQLite: decisions, patterns, constraints
└── index.json         # Searchable cache
```

Examples of what gets extracted:
- **Decisions**: "Chose Gin over Echo for middleware ecosystem"
- **Patterns**: "All handlers follow `internal/handlers/{domain}/` structure"
- **Constraints**: "No ORM - raw SQL only for performance"

## Connect to Claude Code

Add to your MCP config:

```json
{
  "mcpServers": {
    "taskwing": {
      "command": "taskwing",
      "args": ["mcp"]
    }
  }
}
```

Now when you ask Claude anything, it queries your architecture first.

## Privacy

Your code never leaves your machine. Everything is local SQLite. No cloud, no telemetry, no API calls to store your data.

---

**GitHub**: [github.com/josephgoksu/TaskWing](https://github.com/josephgoksu/TaskWing)

Built this because I was mass frustrated. If you're mass frustrated too, try it and let me know what breaks.
