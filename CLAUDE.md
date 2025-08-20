# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

TaskWing is a Go 1.24+ CLI task manager with Model Context Protocol (MCP) server for AI tool integration. The architecture prioritizes local-first storage with no cloud dependencies while enabling sophisticated AI assistance through MCP.

## Development Commands

```bash
# Build the application
go build -o taskwing main.go

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Format code
go fmt ./...

# Tidy dependencies
go mod tidy

# Generate any code (before build)
go generate ./...

# Run the MCP server for testing
./taskwing mcp -v

# Build for release (uses goreleaser)
goreleaser build --snapshot --clean

# Lint code (if golangci-lint is installed)
golangci-lint run
```

## Architecture

### Core Components

- **cmd/**: Cobra-based CLI commands with MCP integration
  - **archive.go**: Project archival and knowledge capture system
  - **patterns.go**: Task pattern library and extraction engine
  - **retrospective.go**: Interactive retrospective generation
- **models/**: Core Task model with validation (go-playground/validator)
- **store/**: TaskStore interface with file-based implementation
- **llm/**: AI integration layer for task generation
- **prompts/**: System prompts for LLM interactions
- **types/**: Unified type definitions shared across CLI and MCP (eliminates duplication)

### Critical Data Flow Paths

1. **CLI → Store → File**: All CLI commands (`cmd/*.go`) interact with `store.TaskStore` interface, never directly with files
2. **MCP → Store → File**: MCP tools use the same TaskStore, ensuring consistency
3. **Config Loading**: Viper loads config hierarchically: flags → env → project → home → defaults
4. **Task Validation**: Input → Validator tags → Store validation → File write with flock
5. **Pattern Learning**: Completed tasks → Archive → Pattern extraction → Knowledge base → AI suggestions

### MCP Integration Architecture

TaskWing implements a full MCP server with:

- **13 Tools**: add-task, list-tasks, update-task, delete-task, mark-done, get-task, set-current-task, get-current-task, clear-current-task, batch-create-tasks, bulk-tasks, search-tasks, task-summary, suggest-patterns
- **4 Resources**: taskwing://tasks (JSON data), taskwing://config (settings), taskwing://archive (historical data), taskwing://knowledge (pattern library)
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

### Configuration

Hierarchical configuration using Viper:

- Project: `.taskwing/.taskwing.yaml`
- Home: `$HOME/.taskwing.yaml`
- Environment: `TASKWING_*` variables

See [Configuration Guide](DOCS.md#configuration) for details.

### TaskStore Interface (Critical)

The `store.TaskStore` interface in `store/store.go` is the ONLY way to interact with task data:

```go
// Key methods that MUST be used:
CreateTask(task models.Task) (models.Task, error)  // Validates & assigns UUID
GetTask(id string) (models.Task, error)            // Returns ErrTaskNotFound if missing
UpdateTask(id string, updates map[string]interface{}) (models.Task, error)
DeleteTask(id string) error                        // Checks dependencies first
ListTasks(filterFn, sortFn) ([]models.Task, error) // Never returns nil
```

**Critical**: The store uses file locking (flock) to prevent corruption during concurrent access. Never bypass this!

### Interactive UI Patterns

Uses promptui for consistent interactive experiences:

- Task selection with search functionality
- Custom templates for task display
- Error handling with `ErrNoTasksFound` for empty selections

### Critical Dependencies

- **spf13/cobra**: CLI framework - all commands in `cmd/` extend cobra.Command
- **spf13/viper**: Config management - hierarchical config loading with env overrides
- **modelcontextprotocol/go-sdk**: MCP server implementation for AI integration
- **gofrs/flock**: File locking to prevent concurrent file corruption
- **go-playground/validator**: Struct validation using tags like `validate:"required,min=3"`
- **manifoldco/promptui**: Interactive prompts - consistent UX patterns

## Critical Code Patterns

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
- **Current Task Management**: `SetCurrentTask()`, `GetCurrentTask()`, `ClearCurrentTask()` persist to config

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

### Interactive Command Patterns

All interactive commands follow this pattern (see `cmd/utils.go`):

```go
// 1. Check for tasks first
tasks, _ := taskStore.ListTasks(nil, nil)
if len(tasks) == 0 {
    return ErrNoTasksFound  // Defined in cmd/errors.go
}

// 2. Use selectTaskInteractive helper
task, err := selectTaskInteractive(taskStore, filterFn, "Select task")
if err == promptui.ErrInterrupt {
    return nil  // User cancelled - don't show error
}
```

### Configuration Loading (Viper)

The config system (`cmd/config.go`) uses Viper with this exact precedence:

1. **Command flags** - Set via cobra commands
2. **Environment variables** - `TASKWING_*` prefix (auto-bound)
3. **Project config** - `.taskwing/.taskwing.yaml` (if exists)
4. **Directory config** - `./.taskwing.yaml` (legacy support)
5. **Home config** - `$HOME/.taskwing.yaml`
6. **Defaults** - Hardcoded in `initConfig()`

**Critical**: The `GetConfig()` function returns a singleton - call it once per command execution

## Knowledge Management

### Components

- **Archive System**: Captures completed project data in `.taskwing/archive/`
- **Pattern Library**: Extracts reusable workflows from archives
- **Knowledge Base**: `KNOWLEDGE.md` stores organizational wisdom

### AI Integration

- Historical data via `taskwing://archive` resource
- Pattern suggestions via `suggest-patterns` tool
- Continuous learning from archived projects

## Current Task Feature

Tracks active task for context-aware AI assistance.

**Implementation**: `SetCurrentTask()`, `GetCurrentTask()`, `ClearCurrentTask()`

**MCP Integration**: Current task context included in all tool responses for better AI assistance.

## Contributing Guidelines

### Development Workflow

1. **Fork and clone** the repository
2. **Create a feature branch**: `git checkout -b feature/your-feature-name`
3. **Make changes** following the code patterns described above
4. **Test thoroughly**: Run tests, build, and test MCP integration
5. **Commit with clear messages**: Follow conventional commit format
6. **Submit a pull request** with detailed description

### Before Committing

```bash
# Required checks
go test ./...
go fmt ./...
go mod tidy
go build -o taskwing main.go

# Test MCP functionality
./taskwing mcp -v

# Optional but recommended
golangci-lint run
```

### Code Standards

- **Follow Go conventions**: Use `gofmt`, proper naming, error handling
- **Add tests**: Include unit tests for new functionality
- **Update documentation**: Keep CLAUDE.md, DOCS.md, and MCP.md current
- **Validate MCP tools**: Test new MCP functionality thoroughly
- **Handle errors gracefully**: Use structured error types and helpful messages

## Important Implementation Notes

### MCP Server Architecture

The MCP server runs as a subprocess communicating via stdin/stdout:

- Tools are registered in `initializeMCPServer()` in `cmd/mcp.go`
- Each tool handler is in `cmd/mcp_tools.go` or `cmd/mcp_advanced_tools.go`
- All tools return `CallToolResult` with optional `_meta` for context
- Errors use `isError: true` flag, not JSON-RPC errors

### Task ID Resolution

TaskWing uses UUID v4 for task IDs but supports partial matching:

- Full UUID: `7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b`
- Short form: `7b3e4f2a` (first 8 chars)
- The `resolveTaskID()` helper in `cmd/utils.go` handles this

### Dependency Management

- Dependencies are task IDs that must complete before a task can start
- Parent/Child relationships are separate from dependencies
- Circular dependency checking happens in `store.validateDependencies()`
- Deleting a task with dependents is blocked

### Current Task Context

The "current task" feature is critical for AI integration:

- Stored in config as `project.currentTaskId`
- Automatically included in all MCP tool responses
- Used by AI to understand user's active work context

### File Storage Details

- Tasks stored in `.taskwing/tasks/tasks.json` (or .yaml/.toml)
- File is locked during reads/writes using `flock`
- Backup created before each write operation
- Empty task list is valid - stored as `[]` in JSON
