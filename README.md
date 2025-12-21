# TaskWing

> **Knowledge Graph for Engineering Teams** — Give your AI tools project memory.

TaskWing captures architectural decisions, features, and trade-offs from your codebase and makes them queryable by AI assistants like Claude Code, Cursor, Codex, and Gemini.

## Why TaskWing?

AI coding assistants are great — but they don't know **why** your project is built the way it is. TaskWing fixes that by:

- 🧠 **Auto-extracting architecture** from your codebase with LLM inference
- 🔍 **Semantic search** across decisions, features, and trade-offs
- 🤖 **MCP integration** — works with Claude Code, Cursor, Gemini, and any MCP-compatible tool

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/josephgoksu/TaskWing/main/install.sh | bash

# Bootstrap your project (auto-generates knowledge from code)
cd your-project
tw bootstrap

# Query your knowledge
tw context "How is the backend deployed?" --answer

# Start MCP server for AI tools
tw mcp
```

## Commands

| Command | Description |
|---------|-------------|
| `tw bootstrap` | Auto-generate knowledge from your codebase |
| `tw add "text"` | Add knowledge (AI classifies type) |
| `tw list` | View all knowledge nodes |
| `tw context "query"` | Semantic search |
| `tw context "query" --answer` | Get AI-generated answer |
| `tw delete <id>` | Delete a knowledge node |
| `tw mcp` | Start MCP server for AI tools |

## AI Tool Integration

TaskWing works with any MCP-compatible AI tool:

| Tool | Config File | Guide |
|------|-------------|-------|
| Claude Code | `~/.claude/mcp.json` | [Setup Guide](docs/MCP_INTEGRATION.md#claude-code) |
| Cursor | `.cursor/mcp.json` | [Setup Guide](docs/MCP_INTEGRATION.md#cursor) |
| Gemini | `~/.gemini/settings.json` | [Setup Guide](docs/MCP_INTEGRATION.md#gemini) |
| Codex | Native MCP support | [Setup Guide](docs/MCP_INTEGRATION.md#codex) |

See the full [MCP Integration Guide](docs/MCP_INTEGRATION.md) for setup instructions.

## How It Works

1. **Bootstrap** analyzes your codebase (directory structure, git history, README, Dockerfile) and uses LLM inference to extract:
   - Features with **why** they exist
   - Decisions with **trade-offs**
   - Relationships between components

2. **Knowledge Graph** stores everything in SQLite with vector embeddings for semantic search

3. **MCP Server** exposes your knowledge to AI tools via the standard Model Context Protocol

## Example Output

```bash
$ tw bootstrap --preview

🧠 TaskWing Bootstrap — Architectural Intelligence

## 1. Backend API (Go + chi)
   Monolithic Go service using chi router for HTTP handling.

   **Decisions:**
   • Use go-chi router, standard library style [high]
     Why: Minimal dependencies, good performance, familiar patterns
     Trade-offs: Less magic than full frameworks like Gin

## 2. Database (MongoDB + LanceDB)
   Hybrid storage: MongoDB for metadata, LanceDB for vectors.

   **Decisions:**
   • Embed LanceDB directly into Go backend [high]
     Why: Avoid separate vector service, single binary deployment
     Trade-offs: CGO dependency, must manage native libraries
```

## Configuration

Create `~/.taskwing.yaml` or `.taskwing.yaml` in your project:

```yaml
llm:
  provider: openai          # or ollama
  model: gpt-5-mini-2025-08-07
  apiKey: ""                # or use OPENAI_API_KEY env var
```

## Documentation

- [Getting Started](docs/GETTING_STARTED.md) — Installation and first steps
- [MCP Integration](docs/MCP_INTEGRATION.md) — AI tool setup
- [Error Reference](docs/ERRORS.md) — Troubleshooting common issues

## Requirements

- Go 1.21+ (for building from source)
- OpenAI API key (for LLM-powered bootstrap)
- Git (for commit history analysis)

## License

MIT
