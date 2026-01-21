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
# Codex uses manual config (see Codex section below)
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

TaskWing's MCP server uses **stdio** transport:

```bash
# Start server (reads JSON-RPC from stdin, writes to stdout)
taskwing mcp
```

### Available Tools

| Tool | Description |
|------|-------------|
| `recall` | Retrieve codebase architecture knowledge. Use `{"query":"search term"}` for semantic search, or omit for summary. |
| `remember` | Add knowledge to project memory. Content is auto-classified by type. |
| `code` | Unified code intelligence: find, search, explain, callers, impact, simplify. |
| `task` | Task lifecycle: next, current, start, complete. |
| `plan` | Plan management: clarify, generate, audit. |
| `debug` | Diagnose issues systematically with AI-powered analysis. |
| `policy` | Evaluate code changes against OPA policies. |

### Recall Tool Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | Search query (omit for project summary) |
| `answer` | boolean | Generate RAG answer using LLM (default: false) |
| `workspace` | string | Filter by workspace name (e.g., `"api"`, `"web"`). Includes root nodes by default. |
| `all` | boolean | Search all workspaces (ignores workspace filter) |

### Example Requests

```json
// Basic search
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"recall","arguments":{"query":"database"}},"id":1}

// Workspace-scoped search (api + root nodes)
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"recall","arguments":{"query":"authentication","workspace":"api"}},"id":2}

// Explicit all-workspace search
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"recall","arguments":{"query":"patterns","all":true}},"id":3}
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

For LLM-powered bootstrap, set your provider API key:

```bash
export OPENAI_API_KEY=sk-...
# or: ANTHROPIC_API_KEY / GEMINI_API_KEY
tw bootstrap
```

---

## Per-Project Server

The `mcp install` command uses a consistent server name (`taskwing-mcp`) across all projects. The AI tools differentiate projects by:

- **CLI-based tools** (Claude Code, Codex, Gemini): Associate the server with the project directory internally
- **File-based configs** (Cursor, Copilot): Store config in project's local directory (`.cursor/mcp.json`, `.vscode/mcp.json`)
- **Claude Desktop**: Uses a global config; last installed project takes precedence

For manual multi-project setup with Claude Desktop, use unique server names:

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

When an AI tool calls `recall`, it receives:

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

---

## Development & Testing

When developing or testing MCP features, use the local development binary instead of the production Homebrew binary:

| MCP Server | Binary | Use Case |
|------------|--------|----------|
| `taskwing-mcp` | Production (`/usr/local/bin/taskwing`) | Stable features, production testing |
| `taskwing-local-dev-mcp` | Development (`./bin/taskwing`) | Testing new/changed features |

**Important:** The production MCP server uses the installed Homebrew binary. Code changes are NOT reflected until you rebuild and reinstall.

For development testing:
1. Run `air` to start hot-reload dev server (creates `./bin/taskwing`)
2. Use the `taskwing-local-dev-mcp` tools in Claude Code
3. Verify with `go run . mcp` for direct testing

See `CLAUDE.md` in the repository for the complete development workflow.
