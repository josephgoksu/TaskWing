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

## Real-World Case Study: Debugging a Revenue-Impacting Bug

> This actually happened. Not a hypothetical.

### The Scenario

A customer was double-charged during subscription signup. Investigation needed to determine:
- Why two subscriptions were created 28 seconds apart
- Why one had a trial ($0) and one was charged ($3.90)
- Root cause in the codebase

### Without TaskWing

```
Developer: "Why did this happen?"
AI: *scans random files* "I'd need to see your subscription logic..."
Developer: *pastes 500 lines of code*
AI: "Can you also show me the Stripe integration?"
Developer: *pastes more code*
AI: "What's your trial policy?"
Developer: *explains manually*
... 45 minutes later, still hunting
```

### With TaskWing (Actual Session)

```
Developer: "Investigate this double-subscription issue"
AI: *calls TaskWing recall* → Gets subscription patterns, constraints, Stripe decisions
AI: "Found it. Bug at service.go:126-128. Code only checks 'active' subscriptions,
     ignores 'trialing'. User subscribed → status was 'trialing' → clicked again
     → code found no 'active' sub → allowed second checkout → charged $3.90."
```

**Time to root cause: 8 minutes**

### What TaskWing Provided Mid-Investigation

| Recall Query | Context Returned |
|--------------|------------------|
| `"subscription stripe checkout duplicate trial"` | Monetization strategy decisions, trial logic constraints |
| Automatic doc lookup | `backend-go/internal/subscriptions/README.md` |

### The Fix (Also Context-Aware)

```go
// BUG: Only checks "active"
Status: stripe.String(string(stripe.SubscriptionStatusActive))

// FIX: Check both "active" AND "trialing"
statuses := []string{
    string(stripe.SubscriptionStatusActive),
    string(stripe.SubscriptionStatusTrialing),
}
```

**Key insight**: The AI knew to check for `trialing` status because TaskWing had captured the decision that "new users get 3-day trials" — context that would otherwise require re-reading docs or asking the developer.

### The Complete Workflow (Same Session)

After identifying the bug, the AI continued to **implement the fix** — still context-aware:

```
1. Recall  → "subscription stripe checkout duplicate trial"
2. Check   → Found backend-go/internal/subscriptions/README.md
3. Implement → Modified service.go:125-188
4. Verify  → go build, go vet (passed)
5. Document → Updated README with new feature
6. Summary → Generated change table for review
```

| Step | What TaskWing Provided |
|------|------------------------|
| Before coding | Architecture: `handler.go → service.go → model.go` |
| During fix | Constraint: "Monetization Strategy Hardening - strict Stripe trials" |
| After fix | Context for documentation update |

**The AI didn't just find the bug — it fixed it, verified it, and documented it, all while respecting the codebase's architectural decisions.**

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

## Key Capability: Mid-Conversation Recall

**This is what makes TaskWing different from "chat with your codebase" tools.**

Traditional AI assistants require you to front-load context. TaskWing enables **dynamic context injection** — the AI can recall relevant knowledge *during* a conversation as new questions arise.

```
┌─────────────────────────────────────────────────────────────────┐
│                    TRADITIONAL AI FLOW                          │
│                                                                  │
│   Start ──► Load context ──► Work ──► Need more? ──► Start over │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    TASKWING FLOW                                 │
│                                                                  │
│   Start ──► Work ──► Need context? ──► Recall ──► Continue      │
│                            │                                     │
│                            └──► AI decides WHEN to fetch context │
└─────────────────────────────────────────────────────────────────┘
```

### Why This Matters

| Scenario | Without Mid-Conversation Recall | With TaskWing |
|----------|--------------------------------|---------------|
| Debugging takes unexpected turn | "Hold on, let me re-read the docs..." | AI recalls relevant constraints automatically |
| Customer issue spans multiple systems | Manually cross-reference | AI queries `"subscription stripe trial"` mid-investigation |
| Planning reveals unknown constraint | Start over with more context | AI pulls constraint on demand |

### Real Example (from production use)

During a subscription bug investigation, the AI:
1. Started investigating Stripe logs
2. **Mid-conversation**: Called `recall("subscription stripe checkout duplicate trial")`
3. Received monetization strategy decisions and trial logic
4. Used that context to identify the root cause

The developer never had to stop and manually provide context. The AI knew when it needed more information and fetched it.

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
