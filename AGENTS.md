# Repository Guidelines

## Project Structure & Module Organization

- `main.go` is the entrypoint; CLI commands live in `cmd/` (Cobra-based).
- Core implementation is under `internal/` (for example `internal/app`, `internal/mcp`, `internal/memory`, `internal/planner`).
- Unit tests are colocated with code as `*_test.go`; integration tests are in `tests/integration/`.
- Documentation is in `docs/`; automation scripts are in `scripts/`; CI pipelines are in `.github/workflows/`.
- Generated/local artifacts such as `.taskwing/`, `test-results/`, and the `taskwing` binary are gitignored.

## CLI Binaries

| Binary | Source | Use for |
|---|---|---|
| `taskwing` | Homebrew (`brew install josephgoksu/tap/taskwing`) | Testing production behavior |
| `./bin/taskwing` | [air](https://github.com/air-verse/air) hot-reload build | Development |

## Build, Test, and Development Commands

- `make dev-setup`: prepares local tooling (deps, golangci-lint), runs `go mod tidy`, and generates code.
- `air`: hot-reload dev server (creates `./bin/taskwing`).
- `make build`: builds the local CLI binary (`./taskwing`).
- `make test`: runs unit, integration, and MCP test targets.
- `make test-unit`: unit tests only.
- `make test-quick`: fast local checks during iteration.
- `go test -v ./internal/bootstrap/...`: tests for one package.
- `make lint`: runs formatting and static analysis (`go fmt`, `go vet`, `staticcheck`, optional `golangci-lint`).
- `make coverage`: test coverage report.
- `go test ./...`: baseline CI-style test run.

## Project Identity

Every TaskWing project is anchored by a `.taskwing.yaml` marker file at the project root. The marker is the SSOT for project identity:

- It freezes the project name and storage slug at init time, so renames or moves don't orphan the data.
- It takes precedence over heuristic markers (`go.mod`, `package.json`, `.git`) during `project.Detect()` - see `internal/project/detect.go:MarkerFileName`.
- Storage lives globally at `~/.taskwing/projects/<slug>/`. The marker file is the only project-local declaration of TaskWing ownership.

| Command | Purpose | Needs LLM |
|---|---|---|
| `taskwing init` | Write the marker, create the global store, generate AI tool integration files (`.claude/settings.json`, `.claude/commands/taskwing/`, etc.) | No |
| `taskwing learn` | Analyze the codebase, populate `~/.taskwing/projects/<slug>/memory.db`. Auto-writes the marker if missing. | Yes |
| `taskwing doctor` | Audit marker presence, store health, AI integration drift | No |
| `taskwing plan --params <json>` | Drive a plan flow (clarify/decompose/expand/generate/finalize) - used by /taskwing:plan | Yes (clarify/decompose/expand/generate) |
| `taskwing task next/current/start/complete` | Task lifecycle - used by /taskwing:next, /taskwing:done | No |
| `taskwing ask "<query>"` | Search project knowledge - used by /taskwing:next | No |
| `taskwing knowledge` | Dump all knowledge nodes - used by /taskwing:context | No |

## How AI tools talk to TaskWing (no MCP)

There is **no MCP server**. AI tools drive TaskWing by invoking the CLI directly:

```
AI tool ──▶ slash command prompt (.claude/commands/taskwing/*.md)
              │
              ▼
        instructs the model to run e.g.
              │
              ▼
        $ taskwing task next --json
        $ taskwing plan --params '{"action":"clarify",...}'
              │
              ▼
        memory.Repository ──▶ ~/.taskwing/projects/<slug>/memory.db
```

Slash commands are pure prompts that tell the model which CLI verbs to call and how to parse the resulting JSON. The skills live in `taskWing-cli/skills/taskwing-*/SKILL.md` (embedded into the binary) and are projected into each project's `.claude/commands/taskwing/` by `taskwing init`.

Why this shape:
- One artifact (the CLI) instead of two (CLI + MCP server)
- No daemon to register, restart, or detect across N AI tools
- Slash commands work in any AI client that can run shell commands
- Easier to debug: every action the model takes is a copy-pasteable shell line

## Architecture Overview

TaskWing is a local-first AI knowledge layer. It extracts architectural decisions, patterns, and constraints from your codebase into local SQLite. AI tools consume that knowledge by invoking the `taskwing` CLI directly through slash commands - there is no MCP server.

### Core Layers

```
cmd/                          # Cobra CLI commands (the public surface)
├── root.go                   # Base command, global flags (--json, --verbose, --preview, --quiet)
├── init.go                   # Write marker, set up AI integration files
├── learn.go                  # LLM-powered knowledge extraction
├── knowledge.go              # View stored project knowledge nodes
├── task.go                   # Task lifecycle (next/current/start/complete)
├── plan.go                   # Plan flow: clarify/decompose/expand/generate/finalize/audit
├── ask.go                    # Search project knowledge
├── doctor.go                 # Diagnostics and integration repair
├── config.go                 # Provider and runtime configuration
├── hook.go                   # Hook handlers used by assistant integrations
└── version.go                # Version output

internal/
├── memory/                   # Data layer
│   ├── store.go              # MemoryStore interface definition
│   ├── sqlite.go             # SQLite implementation (source of truth)
│   └── models.go             # Feature, Decision, Edge types
├── handlers/                 # Plan/task/ask/code/debug action handlers (called from cmd/)
│   ├── handlers.go           # HandlePlanTool, HandleTaskTool, HandleCodeTool, HandleDebugTool
│   ├── presenter.go          # Markdown formatting helpers
│   └── types.go              # *ToolParams / *ToolResult structs (CLI ↔ handler contract)
├── bootstrap/                # Codebase analysis
│   ├── scanner.go            # Heuristic scanner
│   └── llm_analyzer.go       # LLM-powered analysis with streaming
├── llm/
│   └── client.go             # Multi-provider LLM factory (OpenAI, Anthropic, Gemini, Bedrock, Ollama via Eino)
└── knowledge/
    ├── classify.go           # AI classification of knowledge types
    └── embed.go              # Embedding generation
```

### Storage Model

Storage is **global, keyed by per-project slug**. Each project declares itself with a local `.taskwing.yaml` marker; everything else lives under `~/.taskwing/`:

```
~/.taskwing/                                  # global, machine-wide
├── config.yaml                              # global LLM/runtime config
├── projects/
│   ├── index.json                           # registry: slug → root path
│   └── <project-slug>/                      # one dir per project
│       ├── memory.db                        # SQLite: THE source of truth
│       ├── index.json                       # cache: regenerated from SQLite
│       └── features/*.md                    # generated markdown (not canonical)
└── knowledge/                               # cross-project knowledge DB

<project-root>/
└── .taskwing.yaml                           # marker: declares this dir a TaskWing project
```

The slug is `<basename>-<sha256[:6]>` of the resolved project root path (see `config.ProjectSlug`). It's frozen in the marker file at init time.

**Key design principle**: SQLite is the single source of truth. Markdown files are generated snapshots; manual edits may be overwritten. All writes go through CLI commands. The marker file is the only file TaskWing writes inside the project directory; everything else (memory, index, features) is under `~/.taskwing/`.

### Database Schema

| Table | Columns |
|---|---|
| `features` | id, name, one_liner, status, tags (JSON), file_path, decision_count |
| `decisions` | id, feature_id, title, summary, reasoning, tradeoffs |
| `edges` | from_feature, to_feature, edge_type (depends_on, extends, replaces, related) |

Graph traversal uses recursive CTEs for GetDependencies/GetDependents.

### LLM Integration

Uses CloudWeGo Eino for multi-provider support:

| Provider | Env |
|---|---|
| OpenAI | `OPENAI_API_KEY` or `TASKWING_LLM_APIKEY` |
| Anthropic | `ANTHROPIC_API_KEY`, `TASKWING_LLM_PROVIDER=anthropic` |
| Gemini | `GOOGLE_API_KEY`, `TASKWING_LLM_PROVIDER=gemini` |
| Bedrock | `BEDROCK_API_KEY`, `TASKWING_LLM_PROVIDER=bedrock`, `TASKWING_LLM_BEDROCK_REGION=<region>` |
| Ollama | `TASKWING_LLM_PROVIDER=ollama`, `TASKWING_LLM_MODEL=<model>` |

**Bootstrap requires an LLM API key by default** to analyze architecture. Use `--skip-analyze` for CI/testing without LLM (hidden flag, deterministic mode only).

### Task Context Binding

Tasks receive architectural context via a **hybrid early+late binding** approach:

1. **Early binding** (at task creation): `TaskEnricher` executes ask queries and embeds results in `Task.ContextSummary`
2. **Late binding** (at display): `FormatRichContext()` uses early-bound context or fetches fresh context as fallback

This keeps relevant architecture context on every task while staying backward compatible with older tasks.

| File | Role |
|---|---|
| `internal/app/plan.go` | TaskEnricher executes queries at creation time |
| `internal/task/presentation.go` | FormatRichContext() handles display with fallback |
| `internal/task/scope_config.go` | Configurable scope keywords for context matching |

Design rationale: `docs/architecture/ADR_CONTEXT_BINDING.md`.

### Autonomous Task Execution (Hooks)

Hook commands and circuit breakers are in the managed TaskWing section at the end of this file. Full docs: `docs/development/AUTONOMOUS_HOOKS.md`.

## Key Patterns

- **Write-through**: CreateFeature() writes to SQLite → generates markdown → invalidates index cache
- **Global flags**: All commands support `--json`, `--verbose`, `--quiet`, `--preview`
- **Config layering** (low → high priority): defaults → `~/.taskwing/config.yaml` → profile → per-project `~/.taskwing/projects/<slug>/config.yaml` → env vars → flags
- **Path resolution**: `config.GetMemoryBasePath()` in `internal/config/paths.go` returns `~/.taskwing/projects/<slug>/`. `GetMemoryBasePathOrGlobal()` is the only fallback path and is reserved for hooks and non-project commands.
- **Project detection**: `project.Detect()` walks up from cwd. `.taskwing.yaml` short-circuits and wins; otherwise the highest-priority manifest (go.mod, package.json, …) wins; .git is the lowest-priority fallback.

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

- Use Conventional Commit style: `type(scope): description`. Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`.
- Keep commit subjects concise, imperative, and focused on what changed and why.
- **No "Claude Code" or AI assistant references in commit messages. No `Co-Authored-By` headers referencing AI.**
- PRs should include: purpose, linked issue (if applicable), test evidence, and docs updates for user-facing changes.
- Ensure CI passes (`lint`, `test`, docs consistency) before requesting review.

## Versioning (Conservative SemVer)

[SemVer](https://semver.org/) `MAJOR.MINOR.PATCH`, but **err on the side of NOT bumping**.

**Golden rule: batch changes.** Do NOT bump for every small fix. Accumulate related fixes, then bump ONCE. Multiple bug fixes in one session = ONE patch bump.

| Bump | When | NOT this | Examples |
|---|---|---|---|
| PATCH (x.x.X), most changes | Bug fixes, internal refactors (no user-visible change), perf, internal utilities/helpers/error handling, docs fixes, linter/staticcheck fixes, retry/reliability logic, agent coverage reporting, JSON parsing | | JSON repair logic, retry improvements, file tracking fixes |
| MINOR (x.X.0) | New CLI command, new flag, new output format, new provider/model users can configure | Internal refactors, new internal modules, code reorganization | New `taskwing config` subcommand, new `--format` flag, adding Gemini provider |
| MAJOR (X.0.0) | Removing commands/flags/config options, incompatible behavior change, config/database format change without migration | | |

Decision checklist:
1. Visible to users running CLI commands? No → probably no bump yet
2. Breaks existing workflows? → MAJOR
3. Adds user-facing capability? → MINOR
4. Everything else → PATCH (batched)

**Default stance**: when unsure, don't bump.

## Release Process

**CRITICAL: Do NOT release without explicit user approval.**

### AI-assisted release (preferred)

When the user says "let's release", "create a release", or similar:

1. **Analyze changes** since last tag: `git log $(git describe --tags --abbrev=0)..HEAD --oneline`
2. **Generate release notes**: new features (feat:), bug fixes (fix:), breaking changes (if any)
3. **Suggest version bump**: PATCH (fixes, refactors, internal), MINOR (new user-facing features), MAJOR (breaking)
4. **Get user approval** before proceeding
5. **Execute release**:

   ```bash
   # Create annotated tag with release notes (no source file changes needed)
   git tag -a vX.Y.Z -m "Release notes here..."
   # Push tag to trigger CI/CD
   git push origin vX.Y.Z
   ```

   Version is injected via ldflags at build time from the git tag. No need to edit `cmd/root.go`; GoReleaser handles versioning.

### Manual release

`make release`: interactive script that prompts for version, opens editor for notes, creates tag, and pushes.

### Rules

- Never release without explicit user request
- Never bump version autonomously
- Always show release notes for approval before tagging
- GoReleaser + GitHub Actions handle the rest after tag push
<!-- TASKWING_DOCS_START -->

## TaskWing Integration

This project uses TaskWing for architectural knowledge management. You have access to TaskWing MCP tools.

### TaskWing Workflow Contract v1
1. No implementation before a clarified and approved plan/task checkpoint.
2. No completion claim without fresh verification evidence.
3. No debug fix proposal without root-cause evidence.

### MCP Tools (use directly, no skill needed)
- `ask` -- Search project knowledge before modifying unfamiliar code.
- `remember` -- Persist a decision or pattern for future sessions.
- `code` -- Find symbols, explain call graphs, analyze impact, simplify code.
- `debug` -- Diagnose issues with root-cause analysis.
- `task` with action=current -- Check current task status.

**When to use TaskWing MCP tools:**
- Before modifying unfamiliar code: call `ask` to check for relevant decisions, constraints, and patterns
- Before planning multi-step work: call `plan` with action=clarify to get a structured plan
- When asked about architecture, tech stack, or "why" questions: call `ask` with answer=true
- After making an architectural decision: call `remember` to persist it for future sessions
- To understand a symbol's role and callers: call `code` with action=explain

**Do not** grep or read files to answer architecture questions when TaskWing MCP is available. The knowledge graph has pre-extracted, verified decisions with evidence.

### Slash Commands
- /taskwing:plan - Use when you need to clarify a goal and build an approved execution plan.
- /taskwing:next - Use when you are ready to start the next approved TaskWing task with full context.
- /taskwing:done - Use when implementation is verified and you are ready to complete the current task.
- /taskwing:context - Use when you need the full project knowledge dump for complete architectural context.

### Core Commands

<!-- TASKWING_COMMANDS_START -->
- taskwing learn
- taskwing ask "<query>"
- taskwing task
- taskwing mcp
- taskwing doctor
- taskwing config
- taskwing start
<!-- TASKWING_COMMANDS_END -->

### MCP Tools (Canonical Contract)

<!-- TASKWING_MCP_TOOLS_START -->
| Tool | Description |
|------|-------------|
| ask | Search project knowledge (decisions, patterns, constraints) |
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
