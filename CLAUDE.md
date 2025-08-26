# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

TaskWing is a Go 1.24+ CLI task manager with Model Context Protocol (MCP) server for AI tool integration. Local-first storage, no cloud dependencies.

## Development Commands

```bash
# Build
make build                    # Build binary
make release                  # Build release version

# Test
make test                     # Run all tests (unit, integration, MCP)
make test-quick              # Quick tests for development
make test-mcp                # Test MCP protocol (JSON-RPC stdio)
make coverage                # Generate coverage report

# Quality
make lint                    # Format and lint code

# Development
make clean                   # Clean artifacts
make dev-setup              # Install dev tools
make mcp-server             # Run MCP server for testing
```

## Architecture

### Core Structure

- **cmd/**: CLI commands and MCP implementation
  - `mcp_server.go`: Server bootstrap and tool registration
  - `mcp_tools_*.go`: Tool handlers organized by function
  - `mcp_resources_*.go`: Resource handlers
  - `mcp_board.go`: Board visualization tools
  - `mcp_prompts.go`: AI prompt handlers
- **models/**: Task model with validation
- **store/**: TaskStore interface (file-based implementation with flock)
- **types/**: Shared type definitions (MCP params, config, errors)
- **llm/**: AI provider integration

### Critical Patterns

#### 1. Always Use TaskStore Interface

```go
// NEVER bypass this - handles validation, locking, dependencies
store.CreateTask(task)    // Validates & assigns UUID
store.GetTask(id)         // Returns ErrTaskNotFound if missing
store.UpdateTask(id, updates)
store.DeleteTask(id)      // Checks dependencies first
```

#### 2. Type System

All shared types in `types/` package:

```go
import "github.com/josephgoksu/TaskWing/types"
type AddTaskParams = types.AddTaskParams  // Use type aliases
```

#### 3. MCP Tool Pattern

```go
// 1. Define types in types/mcp.go
// 2. Implement handler in cmd/mcp_tools_*.go
// 3. Register in cmd/mcp_server.go
// 4. Return CallToolResult with isError flag for errors
```

#### 4. Configuration Access

```go
config := GetConfig()  // Singleton, loaded once per command
// Hierarchy: flags → env (TASKWING_*) → project → home → defaults
```

### MCP Implementation

**33+ Tools** across categories:

- **Basic**: add-task, get-task, update-task, delete-task, mark-done
- **Bulk**: batch-create-tasks, bulk-tasks, bulk-by-filter, clear-tasks
- **Search**: list-tasks, search-tasks, query-tasks, filter-tasks, find-task
- **Board**: board-snapshot, board-reconcile
- **Context**: set/get/clear-current-task, task-summary
- **Smart**: suggest-tasks, smart-task-transition, dependency-health

**Key Details**:

- Tools communicate via stdin/stdout JSON-RPC
- Errors use `isError: true` in CallToolResult
- All responses include project metrics in `_meta`
- Current task context automatically included

### Task Model

```go
type Task struct {
    ID                 string    // UUID v4
    Title              string    // Required
    Description        string
    AcceptanceCriteria string
    Status             string    // todo|doing|review|done
    Priority           string    // low|medium|high|urgent
    ParentID           string    // Parent task (subtasks)
    Dependencies       []string  // Must complete before this
    CreatedAt          time.Time
    UpdatedAt          time.Time
    CompletedAt        *time.Time
}
```

## Testing

```bash
# Before committing
make test-all           # Comprehensive test suite
make test-quick         # Fast development cycle

# Test types
make test-unit          # Unit tests
make test-integration   # Binary integration
make test-mcp          # MCP protocol tests
```

Test results in `test-results/`:

- `coverage.html`: Interactive coverage
- `*.log`: Test output logs

## Key Implementation Rules

1. **Data Access**: Always through TaskStore interface (never direct file access)
2. **File Locking**: Store uses flock for concurrent safety
3. **Dependencies**: Circular dependency validation, blocks deletion with dependents
4. **Task IDs**: Full UUID or 8-char prefix supported via `resolveTaskID()`
5. **Current Task**: Stored in config as `project.currentTaskId`
6. **Error Handling**: Use `ErrTaskNotFound`, wrap with `fmt.Errorf`
7. **MCP Errors**: Structured `types.MCPError` with codes

## Adding New Features

### New MCP Tool

1. Define types in `types/mcp.go`
2. Implement handler in `cmd/mcp_tools_*.go`
3. Register in `cmd/mcp_server.go`
4. Add tests to `cmd/mcp_protocol_test.go`

### New CLI Command

1. Create command file in `cmd/`
2. Use `GetStore()` for data access
3. Add to `rootCmd` in init()
4. Follow interactive patterns from existing commands

## MCP System Prompt

For AI tools using TaskWing MCP:

```
Use TaskWing MCP tools for all task management. Do not create separate lists.
- First: Call task-summary, then get-current-task
- Status: todo|doing|review|done
- Priority: low|medium|high|urgent
- Create: add-task (single), batch-create-tasks (multiple with TempIDs)
- Find: find-task or query-tasks (not list-tasks unless filtering)
- Update: update-task for fields, mark-done to complete
- Bulk: bulk-tasks or clear-tasks (default: completed=true)
- Search: search-tasks (AND/OR/NOT), filter-tasks (JSONPath)
```
