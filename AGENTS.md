# Repository Guidelines

## Project Structure & Module Organization
- `cmd/`: Cobra CLI, MCP server, tools, and entry points.
- `store/`: `TaskStore` interface and file-backed impl (flock locking).
- `models/`: Core domain models (e.g., `Task`).
- `types/`: Shared types (config, errors, MCP schemas).
- `llm/`: LLM providers (e.g., OpenAI).
- `prompts/`: Prompt templates and loaders.
- `docs/`, `DOCS.md`, `MCP.md`, `CLAUDE.md`: User/dev docs.
- Runtime data lives under `.taskwing/` per project. Do not write elsewhere.

## Build, Test, and Development Commands
- Build: `go build -o taskwing main.go` — compile CLI.
- Run (dev): `go run main.go <command>` (e.g., `list`).
- MCP server: `./taskwing mcp -v` — start MCP with verbose logs.
- Format: `go fmt ./...` — required before PRs.
- Tidy deps: `go mod tidy` — sync `go.mod/sum`.
- Tests: `go test ./...` or `go test -v ./...` — run unit/integration.
- Coverage: `go test -cover ./...` — quick coverage snapshot.
- Local release: `goreleaser build --snapshot --clean` — local binaries.

## Coding Style & Naming Conventions
- Language: Go 1.24+. Use `gofmt` (tabs, std imports) and idiomatic Go.
- Packages: lower-case, no underscores; exported names in `CamelCase`.
- Errors: wrap with context; use sentinel errors in `types/errors.go`.
- Storage: never touch files directly—always go through `store.TaskStore` (validates + locks).
- Shared shapes live in `types/`; use type aliases in `cmd/` if helpful.

## Testing Guidelines
- Framework: standard `testing`; prefer table-driven tests.
- Files: `*_test.go`; place near code. Exercise via `store` and `cmd` entry points.
- Focus: validation logic, locking semantics in `store/`, and critical CLI paths.
- Run: `go test -v ./...`; add `-cover` when useful.

## Commit & Pull Request Guidelines
- Commits: Conventional Commits (e.g., `feat: ...`, `fix: ...`, `docs: ...`).
- Before PR: `go fmt ./... && go mod tidy && go build -o taskwing && go test ./...`.
- PRs: include description, linked issues, rationale, relevant CLI output, and any config/migration notes. Update docs (`DOCS.md`, `MCP.md`, `CLAUDE.md`) when flags/behavior change.

## Security & Configuration Tips
- Config precedence: flags → env (`TASKWING_*`) → project `.taskwing/.taskwing.yaml` → home → defaults.
- Common env: `TASKWING_LLM_APIKEY`, `TASKWING_PROJECT_ROOTDIR`, `TASKWING_DATA_FORMAT`.
- Never commit secrets. Data persists under per-project `.taskwing/`.

## Agent-Specific Instructions
- MCP: run `taskwing mcp` (or `./taskwing mcp -v` in dev). Tools/resources live in `cmd/mcp*.go` and `types/mcp.go`.
- Always access tasks via `store.TaskStore` to respect locking and validation.

