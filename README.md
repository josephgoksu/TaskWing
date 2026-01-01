# TaskWing

[![Go Report Card](https://goreportcard.com/badge/github.com/josephgoksu/TaskWing)](https://goreportcard.com/report/github.com/josephgoksu/TaskWing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> **Your AI coding assistant forgets everything about your codebase between sessions. TaskWing gives it permanent memory.**

## The Problem

Every time you start a new session with Claude, Cursor, or Copilot:

- It forgets why you chose PostgreSQL over MongoDB
- It suggests Redux when you deliberately picked Zustand
- It generates code that violates your established patterns
- You waste 10+ minutes re-explaining your architecture

**The AI is smart. It just has amnesia.**

## The Solution

TaskWing extracts your architectural decisions, patterns, and constraints—then exposes them to AI assistants via [MCP](https://modelcontextprotocol.io/).

```bash
# Install
brew install josephgoksu/tap/taskwing

# Extract knowledge from your codebase
taskwing bootstrap

# Expose to AI assistants
taskwing mcp
```

Your AI assistant now knows:
- **Why** you made each architectural decision
- **What** patterns to follow
- **Which** constraints to respect

## Before & After

| Without TaskWing | With TaskWing |
|------------------|---------------|
| "Add authentication" → suggests JWT when you use session cookies | Knows you chose sessions for SSR compatibility |
| "Create API endpoint" → wrong file structure | References your exact `internal/api/handlers/` pattern |
| "Add database field" → suggests raw SQL | Follows your ORM conventions with proper migrations |
| Re-explain context every session | AI queries your documented decisions instantly |

## Quick Start

```bash
# Install via Homebrew (macOS/Linux)
brew install josephgoksu/tap/taskwing

# Or download directly
curl -fsSL https://taskwing.app/install.sh | sh
```

```bash
# Navigate to your project
cd /path/to/your/repo

# Extract architectural knowledge (one-time)
taskwing bootstrap

# Start the MCP server (AI assistants connect here)
taskwing mcp
```

### Connect to Claude Code

Add to your Claude Code MCP config:

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

Now Claude can query your project's architectural context automatically.

## Measured Impact

We tested AI responses with and without TaskWing on a production codebase:

| Configuration | Avg Score (0-10) | Pass Rate |
|---------------|------------------|-----------|
| **Baseline** (no context) | 3.6 | 0% |
| **With TaskWing** | **8.0** | **100%** |

**+122% improvement.** Without context, models assumed wrong tech stack and file paths. With TaskWing, they correctly referenced the actual architecture.

> [Full evaluation methodology →](docs/development/EVALUATION.md)

## What Gets Captured

TaskWing analyzes your codebase and extracts:

- **Decisions**: Why PostgreSQL? Why monorepo? Why that auth strategy?
- **Patterns**: Your file structure, naming conventions, API design
- **Constraints**: No `.env` in prod, required code review, deployment rules
- **Dependencies**: What talks to what, integration points

All stored locally in `.taskwing/memory/` (SQLite). Your code never leaves your machine.

## Generate Context-Aware Tasks

Beyond memory, TaskWing generates tasks that match your architecture:

```bash
$ taskwing plan new "Add Stripe billing"

✓ Analyzed codebase (46 nodes, 22 decisions, 12 patterns)
✓ Generated 7 tasks based on your architecture

Plan: stripe-billing
  [ ] T1: Add Stripe SDK to backend-go (see: go.mod, internal/payments/)
  [ ] T2: Create webhook handler (pattern: internal/api/handlers/)
  [ ] T3: Add billing_status to User model (constraint: use types.gen.go)
  [ ] T4: Update OpenAPI spec (workflow: make generate-api)
  [ ] T5: Implement billing page (see: web/src/pages/)
  [ ] T6: Add Stripe keys to SSM (policy: no .env in prod)
  [ ] T7: Update CDK for billing IAM (see: cdk/lib/)
```

Every task references **your actual files, patterns, and constraints**.

## Documentation

| Scope | Directory | Purpose |
|-------|-----------|---------|
| **Architecture** | [`docs/architecture/`](docs/architecture/) | System design, data model, roadmap |
| **Development** | [`docs/development/`](docs/development/) | Contributing, testing, internals |
| **Reference** | [`docs/reference/`](docs/reference/) | CLI commands, configuration, MCP protocol |

## License

MIT. Built for engineers who are tired of re-explaining their codebase.
