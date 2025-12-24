# TaskWing

> **Knowledge Graph for Engineering Teams** — Give your AI tools project memory.

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
| `tw mcp` | Start MCP server for AI tools |

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

## Config

```yaml
# ~/.taskwing.yaml
llm:
  provider: openai
  model: gpt-5-mini
```

## License

MIT
