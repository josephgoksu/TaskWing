# Phase 3: Copy Rewrite Recommendations

**Date:** 2026-03-14
**Status:** Complete
**Depends on:** Phase 1 (messaging framework), Phase 2 (market validation)

## Key Decision: Dual-Track Confirmed

Phase 2 validates the dual-track approach:
- **Developer copy**: Lead with speed, embed local-first as the "how"
- **Enterprise copy**: Lead with sovereignty, back with architecture

## README Rewrite

### Current Hero
```
Your AI tools start every session from zero. They don't know your stack,
your patterns, or why you chose PostgreSQL over MongoDB.

TaskWing fixes this. One command extracts your architecture into a local
database. Every AI session after that just knows.
```

### Proposed Hero (Option A -- Recommended)
```
Your AI tools start every session from zero -- and every session, your
code context flows through someone else's cloud.

TaskWing takes the opposite approach. One command extracts your architecture
into a local knowledge base on your machine. No cloud. No account. Every AI
session after that just knows -- without your codebase leaving your infrastructure.
```

### Proposed Hero (Option B -- Shorter)
```
Your AI tools forget everything between sessions. TaskWing doesn't.

One command extracts your architecture into a local SQLite database.
Every AI tool gets instant context -- without your code leaving your machine.
```

### Current Tagline
"Give your AI tools a brain."

### Proposed Taglines (ranked)
1. "The local-first knowledge layer for AI development."
2. "AI context that stays on your machine."
3. "Your architecture. Your machine. Every AI tool."
4. "Give your AI tools a brain." (keep as secondary/playful -- still good, just not differentiating)

### Current Subtitle
"Memory, planning, task execution, and project intelligence -- the control plane for AI-native development."

### Proposed Subtitle
"Local-first architectural knowledge for AI coding tools. Private by architecture."

## New Section: How Your Data Stays Local

Add after Quick Start, before Works With:

```markdown
## Private by Architecture

TaskWing keeps your knowledge base on your machine. No cloud database,
no account, no sync.

How it works:
1. **Bootstrap** -- your code is analyzed by your chosen LLM provider
   (or fully local via Ollama)
2. **Store** -- extracted knowledge lives in local SQLite on your filesystem
3. **Query** -- AI tools connect via local MCP (stdio). Nothing leaves your machine.

Want full air-gap? Use Ollama. Zero network calls, zero external dependencies.
```

## Landing Page Recommendations

### Hero Section
- **Headline**: "AI context that stays on your machine."
- **Subhead**: "One command extracts your architecture. Every AI tool gets instant context. Your code never touches our servers -- we don't have any."
- **CTA**: `brew install josephgoksu/tap/taskwing`

### Benefits Grid (replace capability table)

| Current | Proposed |
|---------|----------|
| Memory | **Local knowledge** -- Decisions, patterns, constraints in local SQLite |
| Planning | **Goal to tasks** -- Decompose goals into executable plans |
| Task Execution | **AI-driven lifecycle** -- next, start, complete, verify |
| Code Intelligence | **Code analysis** -- symbols, call graphs, impact analysis |
| MCP Integration | **Works everywhere** -- Claude, Cursor, Copilot, Gemini, Codex, OpenCode |
| Debugging | **Root cause first** -- AI diagnosis before fixes |

### Social Proof / Trust Section
- "Open source (MIT) -- audit every line"
- "No account. No cloud. No telemetry by default."
- "Air-gappable with Ollama for regulated environments"
- Stars count, if significant

### Comparison Section (new)

```
                    TaskWing    Cursor    Copilot   Cline
Knowledge stored    Local       Cloud     Cloud     None
Persistent context  Yes         Limited   No        No
Multi-tool support  6+ tools    1         1         1
Self-hosted         Yes         No        No        Yes
Air-gappable        Yes         No        No        Yes
Open source         MIT         No        No        Apache 2.0
```

## Words to Update Across All Copy

| Find | Replace with |
|------|-------------|
| "persistent memory" | "local knowledge base" or "local-first knowledge" |
| "AI task manager" | "AI development workflow" or omit |
| "control plane" | "knowledge layer" |
| "the brain" (when primary) | "local-first knowledge layer" |
| "extracts into a database" | "extracts into a local SQLite database on your machine" |
| "MCP integration" | "works with [tool names] via local MCP" |

## Enterprise/Sovereignty Page (Future)

Dedicated `/local-first` or `/enterprise` page:

- **Headline**: "AI-assisted development without surrendering your intellectual property."
- **Subhead**: "TaskWing stores architectural knowledge in local SQLite. No cloud. No account. Fully auditable. Air-gappable with Ollama."
- Architecture diagram showing data flow
- Comparison with cloud-dependent alternatives
- Compliance-friendly language (without claiming certs you don't have)
- "Private by architecture, not by policy" messaging
- Ollama air-gap instructions
