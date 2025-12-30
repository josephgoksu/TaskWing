# TaskWing Development Guide

## Commands
- `make build` - Build the binary
- `make test` - Run all tests (unit, integration, MCP)
- `go test -v ./internal/bootstrap/...` - Run tests for a specific package
- `go test -v ./... -run TestName` - Run a single test by name
- `make lint` - Format code and run golangci-lint
- `make coverage` - Generate test coverage report

## Code Style
- **Imports**: Group stdlib, then third-party, then internal; use `go fmt`/`golangci-lint`
- **Naming**: `camelCase` for variables/functions, `PascalCase` for exported types
- **Error handling**: Wrap errors with context (`fmt.Errorf("action: %w", err)`)
- **Types**: Use concrete types where possible; prefer `struct{}` over empty interface
- **Comments**: Comment exported types/functions; use `//` for single-line, `/**/` for block comments
- **SQLite is source of truth**: Markdown files are generated snapshots; all writes go through CLI commands

## Key Patterns
- Write-through: CreateFeature() writes to SQLite → generates markdown → invalidates cache
- Global flags: `--json`, `--verbose`, `--quiet`, `--preview` supported by all commands
