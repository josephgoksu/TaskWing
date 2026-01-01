# TaskWing Context for Gemini

## Project Overview

**TaskWing** is an AI-native task management CLI that gives AI tools "project memory." It extracts architectural decisions, patterns, and constraints from your codebase and makes them queryable by AI assistants (like Gemini, Claude, Cursor) via the Model Context Protocol (MCP).

**Core Value Proposition:**
*   **Auto-extraction:** Uses LLM inference to extract architecture from code.
*   **Semantic Search:** Query decisions and trade-offs.
*   **MCP Integration:** Exposes knowledge to AI agents.

## Tech Stack

*   **Language:** Go 1.24+
*   **CLI Framework:** Cobra
*   **Database:** SQLite (modernc.org/sqlite) - *Single source of truth*
*   **LLM Orchestration:** CloudWeGo Eino (OpenAI, Ollama support)
*   **Frontend (Dashboard):**
    *   React 19
    *   Vite 7
    *   Tailwind CSS 4
    *   Shadcn/UI
    *   Bun (likely runtime/package manager)

## Architecture

The system is composed of a CLI tool with an embedded MCP server and a web dashboard.

### Core Layers (`internal/`)
*   **Memory (`internal/memory`):** Repository pattern. Encapsulates SQLite (Source of Truth) and Markdown (Snapshot).
*   **Bootstrap (`internal/bootstrap`):** Analyzes codebases.
    *   `scanner.go`: Heuristic analysis (fast, basic).
    *   `llm_analyzer.go`: Deep analysis using LLMs.
*   **Knowledge (`internal/knowledge`):** `KnowledgeService` centralizes intelligence (RAG, Embeddings, Search).
*   **LLM (`internal/llm`):** Interface for AI providers via Eino (Factory pattern).

### Storage Model (`.taskwing/memory/`)
1.  **`memory.db`**: SQLite database. **The canonical source of truth.**
2.  **`index.json`**: Cached index for fast retrieval.
3.  **`features/*.md`**: Human-readable snapshots (generated via Repository). Do not edit manually.

### Directory Structure

```
/
├── cmd/                  # CLI entry points (root, bootstrap, mcp_server, etc.)
├── internal/             # Private application code
│   ├── agents/           # Specialized agents (code, doc, git_deps)
│   ├── bootstrap/        # Codebase analysis logic
│   ├── knowledge/        # Vector search & classification
│   ├── llm/              # LLM client factories
│   ├── memory/           # SQLite storage implementation
│   └── ui/               # TUI components (Bubble Tea)
├── dashboard/            # React/Vite web frontend
├── docs/                 # Documentation (MCP, Roadmap, etc.)
└── Makefile              # Build & Test automation
```

## Key Commands

### Backend / CLI

| Command | Description |
| :--- | :--- |
| `make build` | Build the `taskwing` binary |
| `make test` | Run all tests (Unit, Integration, MCP) |
| `make test-unit` | Run only unit tests |
| `make test-mcp` | Run MCP protocol tests |
| `make lint` | Run formatters and `golangci-lint` |
| `make dev-setup` | Install dev dependencies |

### CLI Commands

| Command | Description |
| :--- | :--- |
| `tw bootstrap` | Initialize project memory |
| `tw plan` | Manage development plans |
| `tw task` | Manage atomic tasks |
| `tw start` | Start working on a task |
| `tw eval` | Run evaluation benchmarks |
| `tw context` | Query project knowledge |

### Frontend (`dashboard/`)

| Command | Description |
| :--- | :--- |
| `bun dev` / `npm run dev` | Start Vite development server |
| `bun build` | Build for production |

## Development Conventions

1.  **Source of Truth:** Always treat SQLite as the source of truth. The `Repository` handles synchronization.
2.  **Global Flags:** CLI commands should respect global flags like `--json`, `--verbose`, `--preview`.
3.  **Testing:**
    *   Use `make test-quick` for rapid iteration.
    *   Ensure MCP tests pass if modifying server logic.
    *   **New:** Unit tests for `internal/knowledge` and `internal/memory` are required.
4.  **Style:** Follow standard Go idioms. Use `make lint` to enforce.
5.  **LLM Integration:** Use the `internal/llm` client factory to support multiple providers (OpenAI, Ollama) agnostic of the specific API.

## MCP Integration

TaskWing exposes a `project-context` tool. When working on this feature:
*   Ensure responses stay within token budgets (500-1000 tokens).
*   Test with `tw mcp` locally or use `make test-mcp`.
