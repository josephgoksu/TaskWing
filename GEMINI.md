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

TaskWing exposes a `recall` tool. When working on this feature:
*   Ensure responses stay within token budgets (500-1000 tokens).
*   Test with `tw mcp` locally or use `make test-mcp`.


### Autonomous Task Execution (Hooks)

TaskWing integrates with Claude Code's hook system for autonomous plan execution:

```bash
taskwing hook session-init      # Initialize session tracking (SessionStart hook)
taskwing hook continue-check    # Check if should continue to next task (Stop hook)
taskwing hook session-end       # Cleanup session (SessionEnd hook)
taskwing hook status            # View current session state
```

**Circuit breakers** prevent runaway execution:
- `--max-tasks=5` - Stop after N tasks for human review
- `--max-minutes=30` - Stop after N minutes

Configuration in `.claude/settings.json` enables auto-continuation through plans.

## CLI Binaries

- **`taskwing`**: Production binary installed via Homebrew (`brew install josephgoksu/tap/taskwing`)
- **`tw`**: Local development binary generated by [air](https://github.com/air-verse/air) for hot-reloading

Use `tw` during development, `taskwing` for testing production behavior.

## Release Process

**CRITICAL: Do NOT release without explicit user approval.**

### AI-Assisted Release (Preferred)

When user says "let's release", "create a release", or similar:

1. **Analyze changes** since last tag:
   ```bash
   git log $(git describe --tags --abbrev=0)..HEAD --oneline
   ```

2. **Generate release notes** summarizing:
   - New features (feat:)
   - Bug fixes (fix:)
   - Breaking changes (if any)

3. **Suggest version bump** based on changes:
   - PATCH: bug fixes, refactors, internal improvements
   - MINOR: new user-facing features
   - MAJOR: breaking changes

4. **Get user approval** before proceeding

5. **Execute release**:
   ```bash
   # Update version in cmd/root.go
   # Commit: "chore: bump version to vX.Y.Z"
   # Create annotated tag with release notes
   git tag -a vX.Y.Z -m "Release notes here..."
   # Push commit and tag
   git push origin main && git push origin vX.Y.Z
   ```

### Manual Release (Standalone)

```bash
make release
```

Interactive script that prompts for version, opens editor for notes, and pushes.

### Rules

- Never release without explicit user request
- Never bump version autonomously
- Always show release notes for approval before tagging
- GoReleaser + GitHub Actions handle the rest after tag push
<!-- TASKWING_DOCS_START -->

## TaskWing Integration

TaskWing provides project memory for AI assistants via MCP tools and slash commands.

### Slash Commands
- `/taskwing` - Fetch full project context (decisions, patterns, constraints)
- `/tw-next` - Start next task with architecture context
- `/tw-done` - Complete current task with summary
- `/tw-plan` - Create development plan from goal
- `/tw-status` - Show current task status
- `/tw-debug` - Get systematic debugging help for issues
- `/tw-explain` - Get deep-dive explanation of a code symbol
- `/tw-simplify` - Simplify code while preserving behavior

### MCP Tools
| Tool | Description |
|------|-------------|
| `recall` | Retrieve project knowledge (decisions, patterns, constraints) |
| `task` | Unified task lifecycle (next, current, start, complete) |
| `plan` | Plan management (clarify, generate, audit) |
| `code` | Code intelligence (find, search, explain, callers, impact, simplify) |
| `debug` | Diagnose issues systematically with AI-powered analysis |
| `remember` | Store knowledge in project memory |

### CLI Commands
```bash
tw bootstrap        # Initialize project memory (first-time setup)
tw context "query"  # Search knowledge semantically
tw add "content"    # Add knowledge to memory
tw plan new "goal"  # Create development plan
tw task list        # List tasks from active plan
```

### Autonomous Task Execution (Hooks)

TaskWing integrates with Claude Code's hook system for autonomous plan execution:

```bash
taskwing hook session-init      # Initialize session tracking (SessionStart hook)
taskwing hook continue-check    # Check if should continue to next task (Stop hook)
taskwing hook session-end       # Cleanup session (SessionEnd hook)
taskwing hook status            # View current session state
```

**Circuit breakers** prevent runaway execution:
- `--max-tasks=5` - Stop after N tasks for human review
- `--max-minutes=30` - Stop after N minutes

Configuration in `.claude/settings.json` enables auto-continuation through plans.

<!-- TASKWING_DOCS_END -->