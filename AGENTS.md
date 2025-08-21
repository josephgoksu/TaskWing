# Repository Guidelines

## Project Structure & Module Organization

- `cmd/`: Cobra CLI, MCP server, and tools.
- `store/`: `TaskStore` interface + file-backed impl (flock).
- `models/`: Core domain models (e.g., `Task`).
- `types/`: Shared types (config, errors, MCP schemas).
- `llm/`: LLM providers (e.g., OpenAI).
- `prompts/`: Prompt templates and loaders.
- `docs/`, `DOCS.md`, `MCP.md`, `CLAUDE.md`: User/dev docs.
- Runtime data lives under `.taskwing/` in each project.

## Build, Test, and Development Commands

- Build: `go build -o taskwing main.go`
- Run (dev): `go run main.go <command>` (e.g., `list`)
- MCP server: `./taskwing mcp -v`
- Format: `go fmt ./...` (required before PRs)
- Tidy deps: `go mod tidy`
- Tests: `go test ./...` or `go test -v ./...`
- Coverage: `go test -cover ./...`
- Local release: `goreleaser build --snapshot --clean`

## Coding Style & Naming Conventions

- Language: Go 1.24+. Use `gofmt` (tabs, standard imports) and idiomatic Go.
- Packages: lower-case, no underscores; exported names in `CamelCase`.
- Errors: wrap with context; use sentinel errors in `types/errors.go`.
- Storage: never touch files directly—always use `store.TaskStore` (validates + locks).
- Shared shapes in `types/`; use type aliases in `cmd/` if helpful.

## Testing Guidelines

- Framework: standard `testing` with table-driven tests.
- File names: `*_test.go`; test via `store` and `cmd` entry points.
- Focus: validation, locking semantics in `store/`, and critical CLI paths.
- Run locally: `go test -v ./...`; add coverage with `-cover` when useful.

## Commit & Pull Request Guidelines

- Commits: Conventional Commits (e.g., `feat: ...`, `fix: ...`, `docs: ...`).
- Before PR: `go fmt ./... && go mod tidy && go build -o taskwing` and `go test ./...`.
- PRs: include description, linked issues, rationale, relevant CLI output, and any config/migration notes.
- Update docs when behavior or flags change (`DOCS.md`, `MCP.md`, `CLAUDE.md`).

## Security & Configuration Tips

- Config precedence: flags → env (`TASKWING_*`) → project `.taskwing/.taskwing.yaml` → home → defaults.
- Common env: `TASKWING_LLM_APIKEY`, `TASKWING_PROJECT_ROOTDIR`, `TASKWING_DATA_FORMAT`.
- Never commit secrets. Data persists under per-project `.taskwing/`.

## Agent-Specific Instructions

- MCP: run `taskwing mcp` (or `./taskwing mcp -v` in dev). Tools/resources live in `cmd/mcp*.go` and `types/mcp.go`.
- Always access tasks via `store.TaskStore` to respect locking and validation.
