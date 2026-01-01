# Changelog

All notable changes to TaskWing will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Stricter LLM Judge**: Responses with wrong tech stack (e.g., TypeScript when repo uses Go) now score ≤3 regardless of structure
- **Failure Details in Reports**: `tw eval report` now shows LLM judge reasoning for failed tasks
- **Eval Comparison Script**: `run-eval-comparison.sh` runs parallel with/without context benchmarks
- **Auto-bootstrap in Eval**: Script automatically bootstraps project memory if missing

### Changed

- Eval judge prompt now explicitly penalizes wrong programming language, file paths, and frameworks
- Improved scoring rubric documentation with tech stack correctness requirements

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
