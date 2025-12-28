# TaskWing

> **Knowledge Graph for Engineering Teams** — Give your AI tools project memory.

**[Why TaskWing?](docs/WHY_TASKWING.md)** — See how TaskWing compares to using AI tools alone.

```bash
# Install
curl -fsSL https://taskwing.app/install.sh | bash

# Bootstrap (auto-extract architecture)
tw bootstrap

# Query
tw context "how does auth work" --answer

# Plan development tasks
tw plan new "Add OAuth2 support"

# Expose to AI tools via MCP
tw mcp
```

## Core Commands

| Command | Description |
|---------|-------------|
| `tw bootstrap` | Auto-generate knowledge from codebase |
| `tw add "..."` | Add knowledge (AI-classified) |
| `tw context "query"` | Semantic search |
| `tw plan new "goal"` | Generate development plan with tasks |
| `tw plan update <id>` | Update plan goal/status |
| `tw task list` | List tasks (grouped by plan) |
| `tw task update <id> --status` | Update task status |
| `tw node show <id>` | Show a knowledge node |
| `tw mcp` | Start MCP server for AI tools |
| `tw eval` | Evaluate model outputs against repo constraints |

## AI Tool Integration

Works with any MCP-compatible tool:

| Tool | Config |
|------|--------|
| Claude Code | `~/.claude/mcp.json` |
| Cursor | `.cursor/mcp.json` |
| Gemini | `~/.gemini/settings.json` |

See [MCP Integration Guide](docs/MCP_INTEGRATION.md) for setup.

## How It Works

1. **Bootstrap** → LLM analyzes code, extracts features + decisions + trade-offs
2. **Knowledge Graph** → SQLite + vector embeddings for semantic search
3. **MCP Server** → Exposes context to AI assistants

## Eval (Model Comparison)

```bash
tw eval init
tw eval run --model openai:gpt-5-mini-2025-08-07 --model openai:gpt-4.1-mini
```

`tw eval` generates a repo-local harness under `.taskwing/eval/` with tasks, prompts, and hard-fail rules.

## Config

```yaml
# ~/.taskwing.yaml
llm:
  provider: openai
  model: gpt-5-mini
```

## License

MIT
