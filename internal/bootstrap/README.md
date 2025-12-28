# internal/bootstrap

Package `bootstrap` provides the factory and runner for TaskWing's parallel agent architecture during project analysis.

## Overview

This package creates and coordinates the default set of analysis agents used during `tw bootstrap` to extract project knowledge:
- **DocAgent**: Analyzes documentation files (README, ARCHITECTURE, etc.)
- **CodeAgent**: Analyzes source code patterns and structure
- **GitAgent**: Extracts decisions from git history and conventional commits
- **DepsAgent**: Analyzes dependencies and technology stack

## Key Functions

| Function | Description |
|----------|-------------|
| `NewDefaultAgents(cfg, projectPath)` | Returns the standard agent set for bootstrap |

## Usage

```go
agents := bootstrap.NewDefaultAgents(llmCfg, "/path/to/project")
for _, agent := range agents {
    output, err := agent.Run(ctx, input)
    // Process findings...
}
```

## Architecture

Agents run in parallel during bootstrap. Each agent implements `core.Agent` and returns `core.Output` containing:
- Findings (features, decisions, patterns, constraints)
- Relationships between discovered entities
