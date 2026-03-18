# Changelog

All notable changes to TaskWing will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `/taskwing:context` slash command for full project knowledge dump.
- Security hardening: freshness validation, stricter input sanitization.
- Prompt reliability tests for slash command contracts.
- Kill tables and operating principles in skill prompts.
- Workflow contract injection via SessionStart hook.

### Changed

- Consolidated slash commands from 8 to 4: `plan`, `next`, `done`, `context`.
- Planning is now MCP-tool-only (removed `taskwing plan` and `taskwing goal` CLI commands).
- Unified context API replaces separate status/ask workflows.
- Updated slash command and MCP prompt contracts to match reduced surface.
- Product messaging focused: "TaskWing helps turn a goal into executed tasks with persistent context across AI sessions."

### Removed

- `taskwing goal` and `taskwing plan` CLI commands (use `/taskwing:plan` or `plan` MCP tool).
- Slash commands: `/taskwing:ask`, `/taskwing:remember`, `/taskwing:status`, `/taskwing:debug`, `/taskwing:explain`, `/taskwing:simplify`.
- Interactive plan TUI (`internal/ui/plan_tui.go`).
- Net reduction of ~1,100 lines.

### Fixed

- RootPath resolution: reject `MarkerNone` contexts to prevent writes to `~/.taskwing/memory.db`.
- FK constraint failures: `LinkNodes` pre-checks node existence before INSERT.
- IsMonorepo misclassification in `MarkerNone` fallback.
- Zero docs loaded for multi-repo workspaces.
- Claude MCP drift detection with evidence traceability.
- Hallucinated findings: Gate 3 enforcement requires evidence.
- Priority scheduling semantics (lower numeric = execute first).
- Unknown slash subcommands fail explicitly instead of silent fallback.

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
