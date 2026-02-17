# Repository Guidelines

## Project Structure & Module Organization
- `main.go` is the entrypoint; CLI commands live in `cmd/` (Cobra-based).
- Core implementation is under `internal/` (for example `internal/app`, `internal/mcp`, `internal/memory`, `internal/planner`).
- Unit tests are colocated with code as `*_test.go`; integration tests are in `tests/integration/`.
- Documentation is in `docs/`; automation scripts are in `scripts/`; CI pipelines are in `.github/workflows/`.
- Generated/local artifacts such as `.taskwing/`, `test-results/`, and the `taskwing` binary are gitignored.

## Build, Test, and Development Commands
- `make dev-setup`: prepares local tooling, runs `go mod tidy`, and generates code.
- `make build`: builds the local CLI binary (`./taskwing`).
- `make test`: runs unit, integration, and MCP test targets.
- `make test-quick`: fast local checks during iteration.
- `make lint`: runs formatting and static analysis (`go fmt`, `go vet`, `staticcheck`, optional `golangci-lint`).
- `go test ./...`: baseline CI-style test run.
- `./scripts/check-doc-consistency.sh`: validates Markdown/doc sync rules used by CI.

## Coding Style & Naming Conventions
- Target Go `1.24.x` (see `go.mod` and CI workflow).
- Follow standard Go style: gofmt-managed formatting (tabs), idiomatic naming, lowercase package names.
- Keep CLI wiring in `cmd/` and business logic in `internal/...`.
- Use descriptive lowercase file names; tests must use `_test.go`.
- Do not commit local secrets or machine-specific files (`.env`, local binaries, temp outputs).

## Testing Guidelines
- Prefer table-driven tests for logic-heavy code and use `t.Run(...)` for subcases.
- Name tests with `TestXxx` and keep assertions focused on observable behavior.
- Run `make test-quick` before small commits; run `make test` before opening a PR.
- Add/update integration tests in `tests/integration/` for end-to-end CLI/MCP behavior.
- Use `make coverage` to inspect coverage and avoid reducing coverage in touched packages.

## Commit & Pull Request Guidelines
- Use Conventional Commit style: `feat(scope): ...`, `fix: ...`, `test: ...`, `chore: ...`.
- Keep commit subjects concise, imperative, and focused on what changed.
- PRs should include: purpose, linked issue (if applicable), test evidence, and docs updates for user-facing changes.
- Ensure CI passes (`lint`, `test`, docs consistency) before requesting review.
<!-- TASKWING_DOCS_START -->

## TaskWing Integration

TaskWing helps me turn a goal into executed tasks with persistent context across AI sessions.

### Supported Models

<!-- TASKWING_PROVIDERS_START -->
[![OpenAI](https://img.shields.io/badge/OpenAI-412991?logo=openai&logoColor=white)](https://platform.openai.com/)
[![Anthropic](https://img.shields.io/badge/Anthropic-191919?logo=anthropic&logoColor=white)](https://www.anthropic.com/)
[![Google Gemini](https://img.shields.io/badge/Google_Gemini-4285F4?logo=google&logoColor=white)](https://ai.google.dev/)
[![AWS Bedrock](https://img.shields.io/badge/AWS_Bedrock-OpenAI--Compatible_Beta-FF9900?logo=amazonaws&logoColor=white)](https://docs.aws.amazon.com/bedrock/latest/userguide/inference-chat-completions.html)
[![Ollama](https://img.shields.io/badge/Ollama-Local-000000?logo=ollama&logoColor=white)](https://ollama.com/)
<!-- TASKWING_PROVIDERS_END -->

### Works With

<!-- TASKWING_TOOLS_START -->
[![Claude Code](https://img.shields.io/badge/Claude_Code-191919?logo=anthropic&logoColor=white)](https://www.anthropic.com/claude-code)
[![OpenAI Codex](https://img.shields.io/badge/OpenAI_Codex-412991?logo=openai&logoColor=white)](https://developers.openai.com/codex)
[![Cursor](https://img.shields.io/badge/Cursor-111111?logo=cursor&logoColor=white)](https://cursor.com/)
[![GitHub Copilot](https://img.shields.io/badge/GitHub_Copilot-181717?logo=githubcopilot&logoColor=white)](https://github.com/features/copilot)
[![Gemini CLI](https://img.shields.io/badge/Gemini_CLI-4285F4?logo=google&logoColor=white)](https://github.com/google-gemini/gemini-cli)
[![OpenCode](https://img.shields.io/badge/OpenCode-000000?logo=opencode&logoColor=white)](https://opencode.ai/)
<!-- TASKWING_TOOLS_END -->

<!-- TASKWING_LEGAL_START -->
Brand names and logos are trademarks of their respective owners; usage here indicates compatibility, not endorsement.
<!-- TASKWING_LEGAL_END -->

### Slash Commands
- /tw-brief - Use when you need a compact project brief.
- /tw-next - Use when you are ready to start the next approved task.
- /tw-done - Use when implementation is verified and ready to complete.
- /tw-plan - Use when you need to clarify a goal and build a plan.
- /tw-status - Use when you need current task progress.
- /tw-debug - Use when debugging must start from root-cause evidence.
- /tw-explain - Use when you need a deep symbol explanation.
- /tw-simplify - Use when you want to simplify code without behavior changes.

### Core Commands

<!-- TASKWING_COMMANDS_START -->
- taskwing bootstrap
- taskwing goal "<goal>"
- taskwing task
- taskwing plan status
- taskwing slash
- taskwing mcp
- taskwing doctor
- taskwing config
- taskwing start
<!-- TASKWING_COMMANDS_END -->

### MCP Tools (Canonical Contract)

<!-- TASKWING_MCP_TOOLS_START -->
| Tool | Description |
|------|-------------|
| recall | Retrieve project knowledge (decisions, patterns, constraints) |
| task | Unified task lifecycle (next, current, start, complete) |
| plan | Plan management (clarify, decompose, expand, generate, finalize, audit) |
| code | Code intelligence (find, search, explain, callers, impact, simplify) |
| debug | Diagnose issues systematically with AI-powered analysis |
| remember | Store knowledge in project memory |
<!-- TASKWING_MCP_TOOLS_END -->

### Autonomous Task Execution (Hooks)

TaskWing integrates with Claude Code's hook system for autonomous plan execution:

~~~bash
taskwing hook session-init      # Initialize session tracking (SessionStart hook)
taskwing hook continue-check    # Check if should continue to next task (Stop hook)
taskwing hook session-end       # Cleanup session (SessionEnd hook)
taskwing hook status            # View current session state
~~~

Circuit breakers prevent runaway execution:
- --max-tasks=5 stops after N tasks for human review.
- --max-minutes=30 stops after N minutes.

Configuration in .claude/settings.json enables auto-continuation through plans.
Hook commands prefer $CLAUDE_PROJECT_DIR/bin/taskwing and fall back to taskwing in PATH.
If Claude Code is already running, use /hooks to review or reload hook changes.

<!-- TASKWING_DOCS_END -->