# TaskWing: AI-Native Task Management

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
│  tw plan new "..."  │  tw plan start  │  tw plan done   │
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
| `tw bootstrap` | Extract knowledge from codebase (one-time setup) |
| `tw plan new "<goal>"` | Generate context-aware task plan |
| `tw plan list` | Show all plans with status |
| `tw plan start <name>` | Set active plan, update MCP context |
| `tw plan status` | Show current plan progress |
| `tw plan done <task>` | Mark task complete |
| `tw mcp` | Start MCP server for AI tool integration |

## Competitive Positioning

| Tool | Approach | Limitation |
|------|----------|------------|
| **Jira/Asana** | Manual task creation | No codebase awareness |
| **Linear** | Fast manual entry | No AI generation |
| **GitHub Issues + Copilot** | AI suggestions | Generic, ignores architecture |
| **TaskWing** | AI + Knowledge Graph | Tasks match YOUR patterns |

## Success Metrics

1. **Task Accuracy**: Generated tasks reference correct files/patterns (target: 80%+)
2. **Developer Adoption**: Daily active users running `tw plan`
3. **Context Utilization**: MCP queries per plan execution

## Monetization (Future)

| Tier | Price | Features |
|------|-------|----------|
| **Open Source** | Free | Full CLI, local knowledge graph |
| **Team** | $29/seat/mo | Shared knowledge graph, team sync |
| **Enterprise** | Custom | SSO, audit, on-prem |

---

*The knowledge graph is the moat. Task management is the product.*
