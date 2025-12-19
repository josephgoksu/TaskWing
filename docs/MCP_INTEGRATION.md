# MCP Integration Guide

> Connect TaskWing to your AI coding assistant

TaskWing implements the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) to provide project knowledge to AI tools.

---

## Quick Setup

The easiest way to connect TaskWing is to use the automatic installer:

```bash
# 1. Bootstrap your project (if not already done)
cd your-project
tw bootstrap

# 2. Install MCP integration for your editor (installs locally in project)
tw mcp install cursor     # For Cursor
tw mcp install claude     # For Claude Code
tw mcp install windsurf   # For Windsurf
tw mcp install gemini     # For Gemini
tw mcp install all        # Install for all supported editors

# Use --global to install in home directory instead
tw mcp install claude --global   # Installs to ~/.claude/mcp.json
```

This command will automatically detect your project path and binary location, and write the necessary configuration files.

---

## Claude Code

**Automatic (local - recommended):**
```bash
tw mcp install claude
```
This creates `.claude/mcp.json` in your project directory.

**Automatic (global):**
```bash
tw mcp install claude --global
```
This adds a project-specific server entry (e.g., `taskwing-myproject`) to `~/.claude/mcp.json`.

**Manual:**
Edit `.claude/mcp.json` in your project (or `~/.claude/mcp.json` for global):

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

**Verify:** Open Claude Code and ask "What do you know about this project?"

---

## Cursor

**Automatic:**
```bash
tw mcp install cursor
```
This creates or updates `.cursor/mcp.json` in your project root.

**Manual:**
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

## Windsurf

**Automatic:**
```bash
tw mcp install windsurf
```
This creates or updates `.windsurf/mcp.json` in your project root.

**Manual:**
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

## Gemini (Google AI Studio / Gemini CLI)

**Automatic (local - recommended):**
```bash
tw mcp install gemini
```
This creates `.gemini/settings.json` in your project directory.

**Automatic (global):**
```bash
tw mcp install gemini --global
```
This adds a project-specific server entry to `~/.gemini/settings.json`.

**Manual:**
Edit `.gemini/settings.json` in your project (or `~/.gemini/settings.json` for global):

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

The `mcp install` command automatically handles multiple projects by adding unique server names (e.g., `taskwing-projectA`) to global configurations like Claude's.

Manual configuration for multiple projects:

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