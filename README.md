# TaskWing

> TaskWing helps me turn a goal into executed tasks with persistent context across AI sessions.

[![Go Report Card](https://goreportcard.com/badge/github.com/josephgoksu/TaskWing)](https://goreportcard.com/report/github.com/josephgoksu/TaskWing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Focused Workflow

```bash
# 1) Bootstrap project memory
cd your-project
taskwing bootstrap

# 2) Create and activate a plan from one goal
taskwing goal "Add Stripe billing"

# 3) Execute from your AI assistant
/tw-next
# ...work...
/tw-done
```

## What TaskWing Does

- Stores architecture decisions, constraints, and patterns in local project memory.
- Generates executable tasks from a goal using that memory.
- Exposes context and task lifecycle tools to AI assistants via MCP.

## Core Commands

- `taskwing bootstrap`
- `taskwing goal "<goal>"`
- `taskwing plan status`
- `taskwing task list`
- `taskwing slash next`
- `taskwing mcp`
- `taskwing start`
- `taskwing doctor`
- `taskwing config`

## MCP Setup (Claude/Codex)

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

## Docs

- [Getting Started](docs/TUTORIAL.md)
- [Product Vision](docs/PRODUCT_VISION.md)
- [Architecture](docs/architecture/)

## License

MIT
