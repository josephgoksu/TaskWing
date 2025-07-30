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

## Architecture

### Core Components

- **cmd/**: Cobra-based CLI commands with MCP integration
- **models/**: Core Task model with validation (go-playground/validator)
- **store/**: TaskStore interface with file-based implementation
- **llm/**: AI integration layer for task generation
- **prompts/**: System prompts for LLM interactions

### Key Data Flow

1. **CLI Commands** → **TaskStore Interface** → **File Storage** (JSON/YAML/TOML)
2. **MCP Server** → **MCP Tools/Resources/Prompts** → **TaskStore Interface**
3. **AI Tools** ↔ **MCP Protocol** ↔ **TaskWing**

### MCP Integration Architecture

TaskWing implements a full MCP server with:

- **6 Tools**: add-task, list-tasks, update-task, delete-task, mark-done, get-task
- **2 Resources**: taskwing://tasks (JSON data), taskwing://config (settings)
- **2 Prompts**: task-generation, task-breakdown

MCP implementation is split across:

- `cmd/mcp.go`: Server setup and tool registration
- `cmd/mcp_tools.go`: Tool handlers (CRUD operations)
- `cmd/mcp_resources.go`: Resource handlers (data access)
- `cmd/mcp_prompts.go`: Prompt handlers (AI assistance)

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

### Error Handling

- Use `ErrNoTasksFound` for interactive selection scenarios
- Wrap errors with context using `fmt.Errorf`
- Validate all structs using validator tags

### MCP Tool Implementation

- All tools use typed parameters with `omitempty` JSON tags for optional fields
- Tools return structured content with text descriptions
- Error responses use `isError: true` in CallToolResult, not JSON-RPC errors

### Task Dependencies

- Circular dependency validation prevents invalid relationships
- Dependents are managed automatically when dependencies are set
- Delete operations check for dependents before allowing removal

### Configuration Access

- Use `GetConfig()` function in cmd package for unified config access
- Configuration is loaded once during command initialization
- Environment variables automatically override file settings
