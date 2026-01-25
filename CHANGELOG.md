# Changelog

All notable changes to TaskWing will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **ID Prefix Resolution**: `tw task show` now accepts unique ID prefixes (e.g., `task-abc` instead of full ID)
  - Ambiguous prefixes display candidate IDs with clear error message
  - Auto-prepends `task-` or `plan-` prefix when missing
- **Archived Plan Filtering**: `tw task list` now excludes archived plans by default
  - New `--include-archived` flag to show tasks from archived plans
  - JSON output includes `plan_status` field for each task
- **ID Utilities**: New `internal/util` package with ID helper functions
  - `ShortID(id, n)` for consistent ID truncation
  - `ResolveTaskID` and `ResolvePlanID` for prefix-based ID resolution
  - Repository methods `FindTaskIDsByPrefix` and `FindPlanIDsByPrefix`

### Changed

- Standardized PlanID JSON field to `plan_id` (snake_case) across all types
  - `planId` (camelCase) still accepted as deprecated alias with warning
  - Affects Task model, MCP tool params (TaskToolParams, PlanToolParams, PolicyToolParams)
- Improved CLI ID display using consistent `util.ShortID` formatting
- Enhanced error messages with context wrapping (e.g., "failed to list tasks for plan X: ...")

### Fixed

- **Task Show ID Mismatch**: Fixed truncated 12-char display vs 13-char actual IDs causing "task not found" errors
- **Silent Error Suppression**: `tw task list` now properly propagates errors instead of silently returning empty results
- **Storage Layer**: Added `rows.Err()` checks to 24 database query functions to catch iteration errors
- **TaskStatus Formatting**: Complete coverage for all 8 status values with graceful "unknown" fallback

- **OpenCode Support**: Full integration with OpenCode AI assistant
  - Bootstrap creates `opencode.json` at project root with MCP server configuration
  - Commands directory `.opencode/commands/` with TaskWing slash commands (tw-next, tw-done, tw-brief, etc.)
  - Plugin hooks `.opencode/plugins/taskwing-hooks.js` for autonomous task execution using Bun's ctx.$ API
  - Doctor health checks validate OpenCode configuration (MCP, commands, plugins)
  - Integration tests and CI job for OpenCode-specific validation
  - Documentation in TUTORIAL.md with opencode.json example, command structure, and plugin format
- **Workspace-Aware Knowledge Scoping**: Full monorepo support for knowledge management
  - New `tw workspaces` command to list detected workspaces in a monorepo
  - `--workspace` and `--all` flags for `tw list` and `tw context` commands
  - `workspace` and `all` parameters for MCP `recall` tool
  - Workspace badges in `tw list` output showing `[workspace]` for non-root nodes
  - Auto-detection of current workspace from working directory
  - Agents tag their findings with the appropriate workspace
  - Database migration adds `workspace` column to nodes table
- **Stricter LLM Judge**: Responses with wrong tech stack (e.g., TypeScript when repo uses Go) now score ≤3 regardless of structure
- **Failure Details in Reports**: `tw eval report` now shows LLM judge reasoning for failed tasks
- **Eval Comparison Script**: `run-eval-comparison.sh` runs parallel with/without context benchmarks
- **Auto-bootstrap in Eval**: Script automatically bootstraps project memory if missing

### Changed

- Eval judge prompt now explicitly penalizes wrong programming language, file paths, and frameworks
- Improved scoring rubric documentation with tech stack correctness requirements
- Existing knowledge nodes default to `root` workspace (backward compatible)

### Fixed

- Empty failure details in eval report output
- Baseline scores were too lenient (7.4 → 3.6 with stricter judge)

## [0.9.2] - 2025-08-30

### Changed

- Expanded documentation into `docs/` directory structure
- Added `GETTING_STARTED.md` and `MCP_INTEGRATION.md`
- Enhanced CLI user experience with organized help categories
- Improved installation instructions
- Added professional badges to README

### Removed

- Redundant and outdated documentation references
- Duplicate installation/setup snippets

### Fixed

- Version number updated to 0.9.2 across all references
- Cross-references between documentation files
- Markdown formatting and linting issues

## [Previous Releases]

### Added in Recent Versions

- Model Context Protocol (MCP) server integration
- Current task management with `taskwing current` commands
- Batch task creation and bulk operations
- Advanced search and filtering capabilities
- Task dependencies and parent-child relationships
- Multiple output formats (JSON, YAML, TOML)
- Interactive task selection and management
- Comprehensive AI tool integration

### Core Features

- CLI task management with CRUD operations
- Local file-based storage with integrity checks
- Project-specific task organization
- Task status tracking and priority management
- Configuration system with environment variable support
- Cross-platform compatibility (Windows, macOS, Linux)

---

Note: For detailed history, see `git log` in this repository.
