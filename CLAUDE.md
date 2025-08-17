# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TaskWing is an AI-assisted CLI task manager for developers built in Go. It provides comprehensive task management with Model Context Protocol (MCP) integration for seamless AI tool interaction.

## Development Commands

### Building and Testing

```bash
# Build the binary
go build -o taskwing main.go

# Run tests (standard Go testing)
go test ./...

# Run with development setup
./taskwing init
./taskwing add
```

### Key CLI Commands for Testing

```bash
# Initialize TaskWing in current directory
taskwing init

# Core task operations
taskwing add                    # Interactive task creation
taskwing list [filters]         # List with optional filtering
taskwing update [task_id]       # Update existing task
taskwing delete [task_id]       # Delete task (checks dependencies)
taskwing done [task_id]         # Mark task completed
taskwing show [task_id]         # Show detailed task info

# MCP server for AI integration
taskwing mcp                    # Start MCP server
taskwing mcp -v                 # Start with verbose logging

# Configuration management
taskwing config [key] [value]   # Manage configuration
```

### MCP Development Workflow

```bash
# Start MCP server for development
./taskwing mcp

# Test MCP functionality through Claude Code
# MCP tools available: add-task, list-tasks, update-task, delete-task, mark-done, get-task
# Advanced tools: batch-create-tasks, bulk-tasks, search-tasks, task-summary
```

## Architecture

### Core Components

- **cmd/**: Cobra-based CLI commands with MCP integration
- **models/**: Core Task model with validation (go-playground/validator)
- **store/**: TaskStore interface with file-based implementation
- **llm/**: AI integration layer for task generation
- **prompts/**: System prompts for LLM interactions
- **types/**: Unified type definitions shared across CLI and MCP (eliminates duplication)

### Key Data Flow

1. **CLI Commands** → **TaskStore Interface** → **File Storage** (JSON/YAML/TOML)
2. **MCP Server** → **MCP Tools/Resources/Prompts** → **TaskStore Interface**
3. **AI Tools** ↔ **MCP Protocol** ↔ **TaskWing**

### MCP Integration Architecture

TaskWing implements a full MCP server with:

- **9 Tools**: add-task, list-tasks, update-task, delete-task, mark-done, get-task, batch-create-tasks, bulk-tasks, search-tasks
- **2 Resources**: taskwing://tasks (JSON data), taskwing://config (settings)
- **2 Prompts**: task-generation, task-breakdown

MCP implementation is split across:

- `cmd/mcp.go`: Server setup and tool registration
- `cmd/mcp_tools.go`: Tool handlers (CRUD operations)
- `cmd/mcp_advanced_tools.go`: Advanced tool handlers (batch operations, search)
- `cmd/mcp_resources.go`: Resource handlers (data access)
- `cmd/mcp_prompts.go`: Prompt handlers (AI assistance)
- `cmd/mcp_context.go`: Context and metrics for intelligent responses
- `cmd/mcp_errors.go`: Structured error handling for MCP responses

### Task Model

Tasks have comprehensive metadata:

- **Core Fields**: ID (UUID), Title, Description, AcceptanceCriteria
- **Status**: pending, in-progress, completed, cancelled, on-hold, blocked, needs-review
- **Priority**: low, medium, high, urgent
- **Relationships**: ParentID, SubtaskIDs, Dependencies, Dependents
- **Timestamps**: CreatedAt, UpdatedAt, CompletedAt

### Configuration System

Uses Viper with hierarchical configuration:

1. Project: `.taskwing/.taskwing.yaml`
2. Directory: `./.taskwing.yaml`
3. Home: `$HOME/.taskwing.yaml`

Environment variables with `TASKWING_` prefix override file settings.

Key configuration options:

- `project.rootDir`: Base directory (default: `.taskwing`)
- `project.tasksDir`: Tasks directory (default: `tasks`)
- `data.file`: Data file name (default: `tasks.json`)
- `data.format`: Storage format (json/yaml/toml)

### Store Interface

All persistence goes through `store.TaskStore` interface:

- **CRUD Operations**: CreateTask, GetTask, UpdateTask, DeleteTask
- **Querying**: ListTasks with filtering and sorting
- **Dependencies**: GetTaskWithDescendents for hierarchy
- **Data Integrity**: File locking (gofrs/flock) and checksum validation
- **Batch Operations**: DeleteTasks, DeleteAllTasks

### Interactive UI Patterns

Uses promptui for consistent interactive experiences:

- Task selection with search functionality
- Custom templates for task display
- Error handling with `ErrNoTasksFound` for empty selections

### Dependencies and Key Libraries

**CLI Framework**: spf13/cobra + spf13/viper
**UI**: manifoldco/promptui for interactive prompts
**Validation**: go-playground/validator with struct tags
**Display**: jedib0t/go-pretty for table formatting
**MCP**: modelcontextprotocol/go-sdk for AI integration
**Storage**: File-based with gofrs/flock for concurrency safety

## Code Patterns

### Unified Type System

**Critical**: All shared types are defined in the `types/` package to eliminate duplication:

- `types/mcp.go`: MCP tool parameters and responses
- `types/config.go`: Configuration structures
- `types/context.go`: Task context and metrics
- `types/errors.go`: MCP error handling
- `types/llm.go`: LLM-specific types

**Usage**: Import `types` package and use type aliases in cmd packages:

```go
type AddTaskParams = types.AddTaskParams
```

### Error Handling

- Use `ErrNoTasksFound` for interactive selection scenarios
- Wrap errors with context using `fmt.Errorf`
- Validate all structs using validator tags
- MCP errors use structured `types.MCPError` with codes and details

### MCP Tool Implementation

- All tools use typed parameters with `omitempty` JSON tags for optional fields
- Tools return structured content with text descriptions
- Error responses use `isError: true` in CallToolResult, not JSON-RPC errors
- **Subtask Support**: `parentId` parameter creates parent-child relationships
- **Context Enrichment**: All responses include project health and metrics

### Task Dependencies

- Circular dependency validation prevents invalid relationships
- Dependents are managed automatically when dependencies are set
- Delete operations check for dependents before allowing removal
- **Parent-Child Relationships**: Separate from dependencies, managed via `ParentID`/`SubtaskIDs`

### Configuration Access

- Use `GetConfig()` function in cmd package for unified config access
- Configuration is loaded once during command initialization
- Environment variables automatically override file settings
- Returns `*types.AppConfig` for type safety

## Key Implementation Details

### MCP Tool Development

When adding new MCP tools:

1. **Define types** in `types/mcp.go` (parameters and responses)
2. **Implement handler** in appropriate mcp file (`cmd/mcp_tools.go` for basic CRUD, `cmd/mcp_advanced_tools.go` for complex operations)
3. **Register tool** in `cmd/mcp.go` with descriptive name and schema
4. **Add type aliases** in relevant cmd files for backward compatibility

### Task Store Pattern

All data operations go through the `store.TaskStore` interface:

```go
type TaskStore interface {
    CreateTask(task models.Task) (models.Task, error)
    GetTask(id string) (models.Task, error)
    UpdateTask(id string, updates map[string]interface{}) (models.Task, error)
    DeleteTask(id string) error
    ListTasks(filterFn func(models.Task) bool, sortFn func([]models.Task) []models.Task) ([]models.Task, error)
    // ... additional methods
}
```

This pattern ensures:

- **Consistent data handling** across CLI and MCP
- **File locking** for concurrent access safety
- **Validation** through struct tags
- **Dependency management** with circular detection

### Interactive UI Consistency

Use `manifoldco/promptui` patterns for all interactive commands:

```go
// Task selection with search
selectedTask, err := selectTaskInteractive(taskStore, filterFn, "Select a task")
if err == promptui.ErrInterrupt {
    // Handle graceful cancellation
}
```

### Configuration Hierarchy

Configuration loading follows strict precedence:

1. **Command flags** (highest priority)
2. **Environment variables** (`TASKWING_*` prefix)
3. **Project config** (`.taskwing/.taskwing.yaml`)
4. **Legacy config** (`./.taskwing.yaml`)
5. **Global config** (`$HOME/.taskwing.yaml`)
6. **Built-in defaults** (lowest priority)

Environment variables use dot-to-underscore mapping: `project.rootDir` → `TASKWING_PROJECT_ROOTDIR`
