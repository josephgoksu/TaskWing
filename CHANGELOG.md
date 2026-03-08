# Changelog

All notable changes to TaskWing will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Focused `taskwing goal "<goal>"` command for one-shot clarify -> generate -> activate flow.
- Hard-break CLI surface reduction to core execution workflow commands.
- Local-only default server bind and strict CORS allowlist behavior.
- Workflow contract documentation (`docs/WORKFLOW_CONTRACT_V1.md`) with hard-gate refusal language and KPIs.
- Workflow operations docs for activation and feedback loops (`docs/WORKFLOW_PACK.md`, `docs/PROMPT_FAILURES_LOG.md`).
- Prompt reliability tests for slash command contracts and cross-assistant command description parity.

### Changed

- Updated product messaging to the focused motto:
  - "TaskWing helps turn a goal into executed tasks with persistent context across AI sessions."
- Updated slash and MCP prompt contracts to unified `task` and `plan` action-based interfaces.
- Purged stale/outdated architecture documentation that no longer matches shipped behavior.
- Reworked `/taskwing:plan`, `/taskwing:next`, `/taskwing:done`, and `/taskwing:debug` prompts as explicit process contracts with hard gates and refusal fallbacks.
- Updated slash command descriptions to trigger-focused "Use when ..." phrasing across assistant command generation.
- Session initialization output now injects TaskWing Workflow Contract v1 for hook-enabled assistants.

### Fixed

- **RootPath resolution**: Reject `MarkerNone` contexts in `GetMemoryBasePath` to prevent accidental writes to `~/.taskwing/memory.db`. Also reject `.taskwing` markers above multi-repo workspaces during detection walk-up. (`TestRootPathResolution`, `TestBootstrapRepro_RootPathResolvesToHome`)
- **FK constraint failures**: `LinkNodes` now pre-checks node existence before INSERT to avoid SQLite error 787. Duplicate edges handled gracefully. (`TestKnowledgeLinking_NoFK`)
- **IsMonorepo misclassification**: `Detect()` now checks `hasNestedProjects()` in the `MarkerNone` fallback, so multi-repo workspaces are correctly classified. Resolves disagreement between `Detect()` and `DetectWorkspace()`. (`TestIsMonorepoDetection`, `TestBootstrapRepro_IsMonorepoMisclassification`)
- **Zero docs loaded**: Added `LoadForServices` to `DocLoader` for multi-repo workspaces. Wired into `RunDeterministicBootstrap` via workspace auto-detection. (`TestDocIngestion`, `TestSubrepoMetadataExtraction`)
- **Sub-repo metadata**: Verified per-repo workspace context in node storage with proper isolation and cross-workspace linking. (`TestSubrepoMetadataPresent`)
- **Claude MCP drift**: Added filesystem-based drift detection tests with evidence traceability and Gate 3 consent enforcement for global mutations. (`TestClaudeDriftDetection`)
- **Hallucinated findings**: Gate 3 enforcement in `NewFindingWithEvidence` — findings without evidence start as "skipped". Added `HasEvidence()` and `NeedsHumanVerification()` to `Finding`. (`TestGate3_Enforcement`, `TestParseJSONResponse_Hallucination`)
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
