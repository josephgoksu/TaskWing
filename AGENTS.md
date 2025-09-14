# Repository Guidelines

## Project Structure & Modules

- `cmd/`: CLI commands and entry points (e.g., `root.go`, task/MCP subcommands).
- `main.go`: binary entry; builds the `taskwing` CLI.
- `types/`: shared types and interfaces (config, MCP, errors, LLM contracts).
- `models/`: domain models (e.g., `task.go`).
- `store/`: persistence layer (file-backed task store).
- `llm/`: LLM providers and helpers (e.g., OpenAI integration).
- `prompts/`: prompt loaders and templates.
- `docs/`: design notes and long-form docs.
- `.taskwing/`: local project data (e.g., `tasks/`); generated at runtime.
- `test-results/`: logs and coverage outputs created by tests.

## Build, Test, and Development

- `make build`: compile CLI to `./taskwing`.
- `make test`: run unit + integration + MCP tests; logs in `test-results/`.
- `make test-quick`: fast local check of core paths.
- `make test-integration` / `make test-mcp`: scoped suites for CLI/MCP.
- `make coverage`: generate `test-results/coverage.html` and summary.
- `make lint`: `go fmt` and optional `golangci-lint` if installed.
- `make dev-setup`: tidy modules, generate, and install linters.
- Run locally: `./taskwing init`, `./taskwing add "Fix login"`, `./taskwing mcp -v`.

## Coding Style & Conventions

- Go 1.24+; format with `go fmt` (enforced via `make lint`).
- Packages: lowercase, no underscores (e.g., `store`, `llm`).
- Exported names: `PascalCase`; unexported: `camelCase`.
- Errors: wrap with context (`fmt.Errorf("...: %w", err)`).
- Files/commands: mirror feature names (e.g., `cmd/list.go`, `cmd/add.go`).

## Testing Guidelines

- Framework: standard `go test`; tests live beside code as `*_test.go` with `TestXxx`.
- Common suites live in `cmd/` (MCP and CLI behavior).
- Run: `make test` or target a file: `go test -v ./cmd -run TestMCPProtocolStdio`.
- Coverage: keep or improve existing totals; review `coverage.html` locally.

## Commits & Pull Requests

- Commits: follow Conventional Commits (`feat:`, `fix:`, `docs:`, `refactor:`) as in repo history.
- PRs must include: clear description, linked issues, test plan (commands run) and relevant logs from `test-results/`.
- If CLI behavior changes, add examples to `EXAMPLES.md` and update help where relevant.

## Security & Configuration

- Use `example.env` → `.env`; never commit secrets. Key vars: `OPENAI_API_KEY`, `TASKWING_LLM_PROVIDER`, `TASKWING_LLM_MODELNAME`.
- Local config: `.taskwing.yaml` or `--config` flag; see `.taskwing.example.yaml`.
