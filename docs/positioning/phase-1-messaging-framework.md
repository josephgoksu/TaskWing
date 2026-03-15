# Phase 1: Strategic Positioning & Messaging Framework

**Date:** 2026-03-14
**Status:** Complete

## Core Narrative Angle

AI coding tools are powerful but architecturally reckless -- they vacuum up your codebase context and send it to cloud services you don't control. TaskWing takes the opposite approach: extract your architecture once, store it locally, and give every AI tool instant context without surrendering your intellectual property.

Speed and control aren't competing values -- TaskWing delivers both because local queries are inherently faster AND more private.

**One-liner candidates:**

- "AI context that stays on your machine."
- "Your architecture. Your machine. Every AI tool."
- "The local-first knowledge layer for AI development."

## Five Credible Sovereignty Claims

| # | Claim | Scope / Honest Boundary |
|---|-------|------------------------|
| 1 | **Your knowledge base is 100% local** -- stored in SQLite on your filesystem, no cloud sync, no account | Fully true. The extracted knowledge (decisions, patterns, constraints) never leaves the machine. |
| 2 | **No signup, no telemetry by default** -- install and use with zero data collection | True. Telemetry is opt-in with explicit consent prompt. |
| 3 | **AI tools connect locally via MCP** -- context flows over local stdio, not cloud APIs | True. The MCP server runs locally; AI tools query it over stdin/stdout. |
| 4 | **Fully air-gappable with Ollama** -- bootstrap + query with zero external network calls | True but requires Ollama setup. This is the only config where *nothing* leaves the machine. Worth promoting explicitly. |
| 5 | **Open source (MIT)** -- audit the code, fork it, run it on your own terms | True. No "open core" bait-and-switch. |

**Claim you must NOT make:**
"Your code never leaves your machine." During bootstrap with cloud LLM providers, code context IS sent to the API. The correct framing: *"During initial analysis, code context is processed by your chosen LLM provider. After that, your knowledge base is entirely local."*

## Messaging Hierarchy

### Primary Message
**"AI context that never leaves your machine."**

### Secondary Messages

| Message | Audience | When to use |
|---------|----------|-------------|
| "90% fewer tokens. 75% faster. Zero cloud dependency." | Developers | README hero, landing page metrics section |
| "Your architectural knowledge stays on your infrastructure." | Engineering leads, CTOs | Enterprise landing page, sales conversations |
| "Works with Claude, Cursor, Copilot, Gemini -- without sending your codebase to another cloud." | Developers evaluating tools | Comparison pages, "Works With" section |
| "Fully air-gappable with Ollama for classified and regulated environments." | Gov/defense/healthcare | Dedicated sovereignty page, enterprise docs |

### Supporting Proof Points
- Local SQLite storage (no cloud database)
- MIT open source (auditable)
- No account required (no data collection surface)
- MCP over local stdio (no network calls for queries)
- Supports Ollama (full air-gap option)

## Words & Phrases

### Adopt

| Phrase | Why |
|--------|-----|
| "local-first" | Developer community understands this instantly. Signals architectural intent. |
| "your machine" / "your infrastructure" | Concrete. Avoids abstract "sovereignty" jargon. |
| "zero cloud dependency" (for queries) | Specific, verifiable, differentiating. |
| "air-gappable" | Signals seriousness to gov/defense/enterprise. |
| "knowledge layer" | Better than "memory" -- implies structure, not just recall. |
| "private by architecture" | Stronger than "private by policy." Architecture can be audited. |

### Retire

| Phrase | Why |
|--------|-----|
| "persistent memory" | Sounds like a database feature. Undersells sovereignty. |
| "AI task manager" | Commodity framing. Every PM tool claims AI now. |
| "control plane" | Infrastructure jargon meaningless to most developers. |
| "a brain" (as primary) | Vague. Doesn't differentiate. Keep only as secondary/playful. |
| "data sovereignty" (in dev copy) | Too formal for developers. Use "local-first" instead. Reserve for enterprise/gov. |

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Bootstrap sends code to LLM APIs | High | Be transparent upfront. Promote Ollama for full air-gap. |
| "Sovereignty" sounds enterprise-washy | Medium | Use "local-first" and "your machine" for devs. Reserve "sovereignty" for enterprise page. |
| No SOC 2 or compliance certs | Medium | Position as "private by architecture" -- nothing to comply about. |
| Ollama quality gap vs cloud LLMs | Low | Frame Ollama as air-gap option, not recommended default. |
| Competitors add local mode | Medium | Ship the story now. First-mover in positioning matters. |

## Recommendation: Dual-Track (Option C)

- **Developer-facing** (README, landing hero): Lead with productivity, embed local-first in same breath.
- **Enterprise/gov-facing** (dedicated page, LinkedIn, sales): Lead with sovereignty.

### Proposed README Hero

> Your AI tools start every session from zero -- and every session, your code context flows through someone else's cloud.
> TaskWing takes the opposite approach. One command extracts your architecture into a local knowledge base on your machine. No cloud. No account. Every AI session after that just *knows* -- without your codebase leaving your infrastructure.
