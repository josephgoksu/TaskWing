# Repository Guidelines

## Project Structure & Module Organization

Core CLI entry lives in `main.go`, with commands under `cmd/` (e.g., `root.go`, task and MCP subcommands). Shared contracts sit in `types/`, domain models in `models/`, and persistence code in `store/`. LLM providers and prompts belong in `llm/` and `prompts/` respectively. Docs and design references are under `docs/`, while runtime data is written to `.taskwing/`. Test artifacts such as logs and coverage land in `test-results/`.

## Build, Test, and Development Commands

Use `make build` to compile the CLI to `./taskwing`. `make test` runs unit, integration, and MCP suites and streams logs to `test-results/`. For a faster pass, run `make test-quick`; scope to integration or MCP with `make test-integration` or `make test-mcp`. Format and lint via `make lint`, and prepare a fresh dev environment with `make dev-setup`. Run the CLI locally with `./taskwing init` followed by commands like `./taskwing add "Fix login"` or `./taskwing mcp -v` for verbose MCP sessions.

## Coding Style & Naming Conventions

Target Go 1.24+, keeping packages lowercase without underscores. Exported identifiers use PascalCase, while internals use camelCase. Always format with `go fmt` (enforced through `make lint`) and wrap errors using `fmt.Errorf("context: %w", err)`. Place commands in files that mirror their feature names (for example, `cmd/list.go`).

## Testing Guidelines

Tests live beside their sources as `*_test.go` and rely on the standard `go test` framework. Name tests with `TestXxx` and prefer focused coverage per package. Generate coverage using `make coverage`, which also writes `test-results/coverage.html`. When debugging a specific path, run targeted commands such as `go test -v ./cmd -run TestMCPProtocolStdio`.

## Commit & Pull Request Guidelines

Adopt Conventional Commits prefixes like `feat:` and `fix:` to align with repository history. Pull requests should include a clear summary, linked issues, and a test plan detailing executed commands with relevant snippets from `test-results/`. Update CLI help or `EXAMPLES.md` whenever behavior changes.

## Security & Configuration Tips

Never commit secrets; copy `example.env` to `.env` for local overrides and configure `OPENAI_API_KEY`, `TASKWING_LLM_PROVIDER`, and `TASKWING_LLM_MODELNAME`. Use `.taskwing.yaml` or the `--config` flag for project-specific settings. Treat `.taskwing/` artifacts as generated and exclude them from version control.
