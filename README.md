# TaskWing

> Your AI coding assistant forgets everything between sessions. TaskWing gives it permanent memory.

[![Go Report Card](https://goreportcard.com/badge/github.com/josephgoksu/TaskWing)](https://goreportcard.com/report/github.com/josephgoksu/TaskWing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Downloads](https://img.shields.io/github/downloads/josephgoksu/TaskWing/total)

## The Problem

Every session with Claude, Cursor, or Copilot:

- Forgets why you chose PostgreSQL over MongoDB
- Suggests Redux when you picked Zustand
- Ignores your file structure and patterns

**You waste 10+ minutes re-explaining your architecture.**

## The Solution

```bash
brew install --cask josephgoksu/tap/taskwing

taskwing bootstrap   # Extract decisions, patterns, constraints
taskwing mcp         # Expose to AI via MCP
```

> **Upgrading?** If you installed before v1.12, run:
> ```bash
> brew uninstall taskwing && brew install --cask josephgoksu/tap/taskwing
> ```

Your AI now knows your architecture permanently.

## Results

| Without TaskWing | With TaskWing  |
| ---------------- | -------------- |
| Score: 3.6/10    | Score: 8.0/10  |
| 0% pass rate     | 100% pass rate |

**+122% better responses.** [Methodology →](docs/development/EVALUATION.md)

## Connect to Claude Code

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

## What Gets Captured

- **Decisions**: Why PostgreSQL? Why monorepo?
- **Patterns**: File structure, naming conventions
- **Constraints**: No .env in prod, deployment rules

All stored locally in `.taskwing/memory/`. Your code never leaves your machine.

## Docs

- [Getting Started](docs/development/GETTING_STARTED.md)
- [Architecture](docs/architecture/)
- [CLI Reference](docs/reference/)

## License

MIT
