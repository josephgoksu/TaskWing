# TaskWing: AI-Native Task Management

TaskWing helps me turn a goal into executed tasks with persistent context across AI sessions.

## Vision Statement

**TaskWing is AI-native task management that actually understands your codebase.**

We don't just store tasks — we generate context-aware development plans by analyzing your architecture, patterns, and decisions. No more generic AI suggestions that ignore your constraints.

## The Problem

Traditional task management (Jira, Asana, Linear) treats code as external. You write tasks manually, and AI assistants hallucinate solutions that don't fit your architecture.

**Example:** You ask an AI to "add Stripe billing" and it suggests patterns you don't use, libraries you've banned, and ignores your existing payment infrastructure.

## The Solution

TaskWing extracts architectural knowledge from your codebase and uses it to:
1. **Generate accurate tasks** that reference your actual files and patterns
2. **Enforce constraints** (e.g., "use types.gen.go", "secrets in SSM, not .env")
3. **Provide context to AI tools** via MCP so every response is architecture-aware

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    USER INTERFACE                        │
│  taskwing goal "..."  │  /tw-next  │  /tw-done   │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                   TASK GENERATION                        │
│  Analyze goal → Query knowledge graph → Generate tasks   │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│               KNOWLEDGE GRAPH (The Moat)                 │
│  Features │ Patterns │ Decisions │ Constraints │ Files  │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                    MCP SERVER                            │
│  Claude │ Cursor │ Copilot │ Codex — all get context    │
└─────────────────────────────────────────────────────────┘
```

## Core Commands

| Command | Purpose |
|---------|---------|
| `taskwing bootstrap` | Extract knowledge from codebase (one-time setup) |
| `taskwing goal "<goal>"` | Generate and activate a context-aware plan |
| `/tw-next` | Start the next task from your AI tool |
| `/tw-done` | Complete the current task from your AI tool |
| `taskwing task list` | Inspect generated tasks |
| `taskwing plan status` | Show current plan progress |
| `taskwing mcp` | Start MCP server for AI tool integration |

## Competitive Positioning

| Tool | Approach | Limitation |
|------|----------|------------|
| **Jira/Asana** | Manual task creation | No codebase awareness |
| **Linear** | Fast manual entry | No AI generation |
| **GitHub Issues + Copilot** | AI suggestions | Generic, ignores architecture |
| **TaskWing** | AI + Knowledge Graph | Tasks match YOUR patterns |

## Proof: It Actually Works

> "Found a revenue-impacting bug in 8 minutes. The AI recalled our trial policy mid-investigation, identified the root cause at `service.go:126`, implemented the fix, verified the build, and updated the docs — all in one session, all context-aware."
> — Production debugging session, Jan 2026

**Full workflow in one session:** Debug → Fix → Verify → Document

See the architecture notes in `docs/architecture/` for implementation details.

---

## Success Metrics

1. **Task Accuracy**: Generated tasks reference correct files/patterns (target: 80%+)
2. **Developer Adoption**: Daily active users running `taskwing goal`
3. **Context Utilization**: MCP queries per plan execution
4. **Time-to-Root-Cause**: Bug investigations with TaskWing context vs. without

## Monetization (Future)

| Tier | Price | Features |
|------|-------|----------|
| **Open Source** | Free | Full CLI, local knowledge graph |
| **Team** | $29/seat/mo | Shared knowledge graph, team sync |
| **Enterprise** | Custom | SSO, audit, on-prem |

---

*The knowledge graph is the moat. Task management is the product.*
