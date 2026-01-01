# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build
make build                    # Build the taskwing binary

# Test
make test                     # Run all tests (unit, integration, MCP)
make test-unit                # Run unit tests only
make test-quick               # Quick tests for development
go test -v ./internal/bootstrap/...  # Run tests for a specific package

# Quality
make lint                     # Format code and run golangci-lint
make coverage                 # Generate test coverage report

# Development
make dev-setup                # Install dependencies and golangci-lint
```

## Architecture Overview

TaskWing is an AI-native task management CLI that extracts architectural decisions and context from codebases, making them queryable by AI tools via MCP (Model Context Protocol).

### Core Layers

```
cmd/                          # Cobra CLI commands
├── root.go                   # Base command, global flags (--json, --verbose, --preview, --quiet)
├── bootstrap.go              # Auto-generate knowledge from repo
├── add.go                    # Add knowledge (AI classifies type)
├── context.go                # Semantic search with --answer for AI responses
├── list.go                   # View knowledge by type
├── memory.go                 # Maintenance: check/repair/rebuild-index
├── mcp_server.go             # MCP server for AI tool integration
├── plan.go                   # Plan management (new/list/start)
├── task.go                   # Atomic task management
└── eval.go                   # Evaluation benchmarks

internal/
├── memory/                   # Data layer
│   ├── store.go              # MemoryStore interface definition
│   ├── sqlite.go             # SQLite implementation (source of truth)
│   └── models.go             # Feature, Decision, Edge types
├── bootstrap/                # Codebase analysis
│   ├── scanner.go            # Heuristic scanner
│   └── llm_analyzer.go       # LLM-powered analysis with streaming
├── llm/
│   └── client.go             # Multi-provider LLM factory (OpenAI, Ollama via Eino)
└── knowledge/
    ├── classify.go           # AI classification of knowledge types
    └── embed.go              # Embedding generation
```

### Storage Model

```
.taskwing/memory/
├── memory.db                 # SQLite: THE source of truth
├── index.json                # Cache: regenerated from SQLite
└── features/*.md             # Generated markdown (human-readable, not canonical)
```

**Key design principle**: SQLite is the single source of truth. Markdown files are generated snapshots; manual edits may be overwritten. All writes go through CLI commands.

### Database Schema

Three main tables in SQLite:
- `features`: id, name, one_liner, status, tags (JSON), file_path, decision_count
- `decisions`: id, feature_id, title, summary, reasoning, tradeoffs
- `edges`: from_feature, to_feature, edge_type (depends_on, extends, replaces, related)

Graph traversal uses recursive CTEs for GetDependencies/GetDependents.

### LLM Integration

Uses CloudWeGo Eino for multi-provider support:
- OpenAI: Set `OPENAI_API_KEY` or `TASKWING_LLM_APIKEY`
- Ollama: Set `TASKWING_LLM_PROVIDER=ollama` and `TASKWING_LLM_MODELNAME=<model>`

Bootstrap runs LLM analysis by default.

### MCP Server

`tw mcp` starts a JSON-RPC stdio server exposing `project-context` tool for AI assistants. Target token budget: 500-1000 tokens per context response.

## Key Patterns

- **Write-through**: CreateFeature() writes to SQLite → generates markdown → invalidates index cache
- **Global flags**: All commands support `--json`, `--verbose`, `--quiet`, `--preview`
- **Config**: `~/.taskwing.yaml` or `.taskwing.yaml` in project root
- **GetMemoryBasePath()** in `cmd/root.go` resolves `.taskwing/memory` path
