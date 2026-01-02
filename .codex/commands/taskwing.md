# Fetch project architecture context

Retrieve codebase knowledge (patterns, decisions, constraints) via the TaskWing MCP server.

## Prerequisites
TaskWing MCP server must be configured. If not set up, run:
```bash
tw mcp install codex
```

## MCP Tool: `recall`
- **Overview mode** (no params): Returns summary of features, decisions, patterns, constraints
- **Search mode**: `{"query": "authentication"}` for semantic search across project memory

## When to Use
- Starting work on an unfamiliar codebase
- Before implementing features (check existing patterns)
- When unsure about architecture decisions
- Finding constraints before making changes

## Fallback (No MCP)
If MCP is unavailable, use the CLI directly:
```bash
tw context              # Overview
tw context -q "search"  # Semantic search
tw context --answer     # AI-generated response
```
