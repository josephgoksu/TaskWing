# Changelog

All notable changes to TaskWing will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Focused `taskwing goal "<goal>"` command for one-shot clarify -> generate -> activate flow.
- Hard-break CLI surface reduction to core execution workflow commands.
- Local-only default server bind and strict CORS allowlist behavior.

### Changed

- Updated product messaging to the focused motto:
  - "TaskWing helps me turn a goal into executed tasks with persistent context across AI sessions."
- Updated slash and MCP prompt contracts to unified `task` and `plan` action-based interfaces.
- Purged stale/outdated architecture documentation that no longer matches shipped behavior.

### Fixed

- Priority scheduling semantics corrected (lower numeric priority executes first).
- Unknown slash subcommands now fail explicitly instead of silently falling back.
- MCP plan action descriptions aligned with implemented behavior.

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
