# MCP Integration Guide

> Connect TaskWing to your AI coding assistant

TaskWing implements the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) to provide project knowledge to AI tools.

---

## Quick Setup

```bash
# 1. Get your taskwing binary path
which taskwing
# Example: /usr/local/bin/taskwing

# 2. Bootstrap your project (if not already done)
cd your-project
tw bootstrap

# 3. Test MCP server
tw mcp
# Should show: 🎯 TaskWing MCP Server Starting...
```

---

## Claude Code

Edit `~/.claude/mcp.json`:

```json
{
  "mcpServers": {
    "taskwing": {
      "command": "/usr/local/bin/taskwing",
      "args": ["mcp"],
      "cwd": "/path/to/your/project"
    }
  }
}
```

**Verify:** Open Claude Code and ask "What do you know about this project?"

---

## Cursor

Edit `.cursor/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "taskwing": {
      "command": "/usr/local/bin/taskwing",
      "args": ["mcp"]
    }
  }
}
```

**Verify:** In Cursor, press `Cmd+K` and ask about project architecture.

---

## Gemini (Google AI Studio / Gemini CLI)

Edit `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "taskwing": {
      "command": "/usr/local/bin/taskwing",
      "args": ["mcp"],
      "cwd": "/path/to/your/project"
    }
  }
}
```

---

## Codex (OpenAI)

Codex supports MCP natively. Add TaskWing via the MCP configuration:

```json
{
  "mcp": {
    "servers": {
      "taskwing": {
        "command": "/usr/local/bin/taskwing",
        "args": ["mcp"]
      }
    }
  }
}
```

---

## Zed Editor

Edit `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "taskwing": {
      "command": {
        "path": "/usr/local/bin/taskwing",
        "args": ["mcp"]
      }
    }
  }
}
```

---

## Windsurf

Edit `.windsurf/mcp.json` in your project:

```json
{
  "mcpServers": {
    "taskwing": {
      "command": "/usr/local/bin/taskwing",
      "args": ["mcp"]
    }
  }
}
```

---

## Generic MCP Client

TaskWing's MCP server uses **stdio** transport by default:

```bash
# Start server (reads JSON-RPC from stdin, writes to stdout)
taskwing mcp

# Or use SSE transport on a port
taskwing mcp --port 8080
```

### Available Tools

| Tool | Description |
|------|-------------|
| `project-context` | Get project knowledge. Use `{"query":"search term"}` for semantic search, or omit for summary. |

### Example Request

```json
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"project-context","arguments":{"query":"database"}},"id":1}
```

---

## Troubleshooting

### "No knowledge found"

Run `tw bootstrap` in your project directory first:

```bash
cd your-project
tw bootstrap
tw list  # Should show features and decisions
```

### "Connection refused"

Make sure the MCP server can find your project:

```bash
# Option 1: Set cwd in config
"cwd": "/absolute/path/to/your/project"

# Option 2: Set TASKWING_PROJECT_DIR env var
"env": {
  "TASKWING_PROJECT_DIR": "/path/to/project"
}
```

### "API key not found"

For LLM-powered bootstrap, set your API key:

```bash
export OPENAI_API_KEY=sk-...
tw bootstrap
```

---

## Per-Project Server

If you have multiple projects, configure separate MCP servers per project:

```json
{
  "mcpServers": {
    "taskwing-projectA": {
      "command": "/usr/local/bin/taskwing",
      "args": ["mcp"],
      "cwd": "/path/to/projectA"
    },
    "taskwing-projectB": {
      "command": "/usr/local/bin/taskwing",
      "args": ["mcp"],
      "cwd": "/path/to/projectB"
    }
  }
}
```

---

## What the AI Sees

When an AI tool calls `project-context`, it receives:

```json
{
  "summary": {
    "project": "MyProject",
    "total_nodes": 24,
    "decisions": 18,
    "features": 6
  },
  "nodes": {
    "decision": [
      {
        "id": "n-abc123",
        "summary": "Use PostgreSQL as primary datastore",
        "content": "Why: Reliable, transactional, familiar. Trade-offs: Not optimized for OLAP queries."
      }
    ],
    "feature": [
      {
        "id": "n-def456",
        "summary": "Authentication Module",
        "content": "Handles OAuth2 and session management."
      }
    ]
  }
}
```

This gives the AI **context about your architecture** so it can make better suggestions.
