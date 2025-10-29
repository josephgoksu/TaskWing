# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TaskWing is an AI-native CLI task manager built in Go 1.24+ that integrates with Claude Code, Cursor, and other AI tools via the Model Context Protocol (MCP). It provides local-first task management with **15 optimized MCP tools** for intelligent task operations.

**Key Technologies:**
- Go 1.24+ with Cobra CLI framework
- Model Context Protocol (MCP) for AI integration via `github.com/modelcontextprotocol/go-sdk`
- File-based storage (JSON/YAML/TOML) with file locking via `gofrs/flock`
- Viper for configuration management

**MCP Tool Optimization:**
- **Total Tools Available:** 40 tools in codebase
- **Enabled:** 15 essential tools (~9.6k tokens)
- **Disabled:** 25 advanced/rarely-used tools
- **Token Savings:** 71% reduction (33.2k → 9.6k tokens)
- **See:** `MCP_TOOL_REDUCTION_SUMMARY.md` for details

## Build and Test Commands

```bash
# Build
make build              # Compile to ./taskwing
make clean              # Remove build artifacts

# Testing
make test               # Run all tests (unit, integration, MCP)
make test-quick         # Fast tests for development
make test-unit          # Unit tests only
make test-integration   # Integration tests
make test-mcp           # MCP protocol tests
make test-all           # Comprehensive test suite with coverage

# To run a single test:
go test -v ./cmd -run TestMCPProtocolStdio

# Quality
make lint               # Format and lint code
make coverage           # Generate coverage report to test-results/

# Development
make dev-setup          # Install dev tools (golangci-lint)
./taskwing init         # Initialize in a project
./taskwing mcp -v       # Start MCP server with verbose logging
```

**Important:** Test results and logs are saved to `test-results/` directory.

## Architecture

### Directory Structure

```
cmd/                    # CLI commands (add.go, list.go, done.go, etc.)
├── root.go            # Root command with categorized help
├── mcp_server.go      # MCP server implementation
├── llm_utils.go       # LLM integration helpers
└── utils.go           # Shared utilities

mcp/                    # MCP protocol implementation
├── tools_basic.go     # Basic CRUD tools (add, list, update, delete)
├── tools_intelligent.go # AI-powered tools (suggest, smart-transition)
├── tools_bulk.go      # Batch operations (bulk-tasks, board-reconcile)
├── tools_plan.go      # Planning tools (generate-plan, iterate-plan-step)
├── tools_workflow.go  # Workflow tools (workflow-status, board-snapshot)
├── tools_resolution.go # Reference resolution (find-task, resolve-task-reference)
└── protocol_test.go   # MCP protocol integration tests

store/                  # Data persistence layer
├── interface.go       # TaskStore interface contract
├── file_store.go      # File-based implementation with locking
└── archive_store.go   # Archive management

types/                  # Shared type definitions
├── config.go          # Configuration types
├── mcp.go             # MCP parameter/response types
└── context.go         # Context helpers

models/                 # Domain models
├── task.go            # Task model with validation
└── archive.go         # Archive entry model

llm/                    # LLM provider abstraction
├── provider.go        # LLM provider interface
├── factory.go         # Provider factory
└── openai.go          # OpenAI implementation

prompts/                # LLM prompts
└── prompts.go         # Centralized prompt definitions
```

### Key Architectural Patterns

**1. Store Interface Pattern**
- All data access goes through `store.TaskStore` interface (store/interface.go)
- Primary implementation is `FileTaskStore` with file locking
- Initialize store via `GetStore()` in cmd/root.go
- Store location: `.taskwing/` in project root (configurable via `.taskwing.yaml`)

**2. MCP Tool Registration**
- MCP server defined in cmd/mcp_server.go
- Tools organized by category in mcp/ directory
- Each tool handler follows pattern: `func(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[ParamsType, ResponseType]`
- Tool parameters defined in types/mcp.go with `mcp` struct tags for descriptions

**3. Error Handling**
- Use `HandleFatalError()` for initialization errors (cmd/errors.go)
- MCP errors use `types.NewMCPError()` with error codes and context
- Wrap errors with context: `fmt.Errorf("context: %w", err)`

**4. Configuration Management**
- Viper-based config with layered precedence: flags > env vars > config file
- Config initialization in cmd/config.go via `InitConfig()`
- Primary config type: `types.AppConfig`
- Default config file: `.taskwing.yaml` in project root or `$HOME`

**5. Task Status Workflow**
- Canonical statuses: `todo`, `doing`, `review`, `done`
- Legacy status mapping in mcp/helpers.go via `normalizeStatusString()`
- Status transitions tracked with timestamps

**6. Reference Resolution**
- Tasks can be referenced by full ID, partial ID, or fuzzy title match
- Resolution logic in mcp/tools_resolution.go
- `find-task` and `resolve-task-reference` MCP tools for AI-friendly lookup

## Working with MCP Tools

### Adding a New MCP Tool

1. Define parameter and response types in `types/mcp.go`:
```go
type MyToolParams struct {
    Field string `json:"field" mcp:"Description for AI"`
}
```

2. Implement handler in appropriate `mcp/tools_*.go` file:
```go
func myToolHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.MyToolParams, types.MyResponse] {
    return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.MyToolParams]) (*mcpsdk.CallToolResultFor[types.MyResponse], error) {
        args := params.Arguments
        logToolCall("my-tool", args)

        // Implement logic

        return mcpsdk.NewToolResultSuccess(response), nil
    }
}
```

3. Register in `cmd/mcp_server.go` within `registerMCPTools()`:
```go
mcpsdk.RegistrationFor("my-tool", myToolHandler(taskStore),
    "Tool description",
    mcpsdk.WithPriority(mcpsdk.PriorityNormal)),
```

4. Add tests in corresponding `mcp/*_test.go` file

### Testing MCP Tools

MCP protocol tests use stdio JSON-RPC communication:
```go
// See mcp/protocol_test.go for examples
func TestMCPMyTool(t *testing.T) {
    testDir := setupTestDir(t)
    defer os.RemoveAll(testDir)

    client := startMCPServer(t, testDir)
    defer client.Close()

    // Call tool via JSON-RPC
    result := callToolSimple[types.MyToolParams, types.MyResponse](
        t, client, "my-tool",
        types.MyToolParams{Field: "value"})

    // Assert results
}
```

## LLM Integration

### Using LLM Providers

LLM access is abstracted via `llm.Provider` interface (llm/provider.go):
```go
factory := llm.NewLLMFactory()
provider, err := factory.CreateProvider("openai", config)
messages := []llm.Message{{Role: "user", Content: "prompt"}}
response, err := provider.GenerateResponse(messages)
```

Configuration via environment variables:
```bash
OPENAI_API_KEY=sk-...
TASKWING_LLM_PROVIDER=openai
TASKWING_LLM_MODELNAME=gpt-4
```

Prompts are centralized in `prompts/prompts.go` for consistency.

## Testing Guidelines

- Tests must be self-contained and use temporary directories
- Use `setupTestDir(t)` helper from cmd/test_helpers.go for CLI tests
- Use `createTempProjectDir(t)` helper from mcp/test_helpers_test.go for MCP tests
- Clean up with `defer os.RemoveAll(testDir)`
- MCP tests verify JSON-RPC stdio communication
- Integration tests build the binary first: `make test-integration`

## Configuration Files

**`.taskwing.yaml`** - Project configuration:
```yaml
project:
  root_dir: .
  tasks_dir: .taskwing
  current_task_id: ""

data:
  file: tasks.json
  format: json

llm:
  provider: openai
  model_name: gpt-4
  temperature: 0.7
  max_tokens: 2000
```

**`.env`** - Environment variables (copy from `example.env`):
```bash
OPENAI_API_KEY=sk-...
TASKWING_LLM_PROVIDER=openai
TASKWING_LLM_MODELNAME=gpt-4
```

## Common Development Tasks

### Adding a CLI Command

1. Create `cmd/mycommand.go` following existing patterns
2. Register in `init()` as child of `rootCmd`
3. Add to `commandCategories` map in cmd/root.go for organized help
4. Implement command logic using `GetStore()` for data access

### Debugging MCP Communication

```bash
# Start MCP server with verbose logging
./taskwing mcp -v

# Server logs show JSON-RPC messages and tool calls
# Use `logToolCall()` helper in handlers for consistent logging
```

### Running the CLI Locally

```bash
make build
./taskwing init                              # Initialize in current directory
./taskwing add "Test task" --priority high   # Create task
./taskwing ls                                # List tasks
./taskwing start <task-id>                   # Start task
./taskwing done <task-id>                    # Complete task
./taskwing improve <task-id> --apply         # AI-enhance task
```

## Code Style

- Format with `go fmt` (enforced by `make lint`)
- Use PascalCase for exported identifiers, camelCase for internal
- Packages are lowercase without underscores
- Follow Conventional Commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`
- Error wrapping: `fmt.Errorf("context: %w", err)`

## Important Notes

- **Never commit secrets**: Use `.env` for local development (copy from `example.env`)
- **File locking**: Store uses `gofrs/flock` to prevent concurrent access issues
- **Task IDs**: Generated via `google/uuid`, 8-character prefixes for CLI display
- **MCP compatibility**: Follow Model Context Protocol specification for tool definitions
- **Local-first**: All data stored in `.taskwing/` directory within project
