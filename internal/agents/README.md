# Agents Package

LLM-powered agents for codebase analysis using Eino best practices.

## Architecture

### Bootstrap Agents (Deterministic)

All bootstrap agents use a single LLM call with pre-gathered context:

| Agent | Input | Output |
|-------|-------|--------|
| `DocAgent` | *.md files | Features, Constraints |
| `CodeAgent` | Entry points, handlers, configs | Patterns, Decisions |
| `GitAgent` | git log, shortlog | Milestones, Evolution |
| `DepsAgent` | go.mod, package.json | Tech decisions, Stack |

### Planning Agents

| Agent | Pattern | Purpose |
|-------|---------|---------|
| `ClarifyingAgent` | Deterministic | Refines user goals by asking clarifying questions |
| `PlanningAgent` | Deterministic | Decomposes goals into actionable tasks with dependencies |

## Eino Patterns Used

```go
// Deterministic chain (used by all agents)
chain := core.NewDeterministicChain[ResponseType](ctx, name, model, promptTemplate)
parsed, raw, duration, err := chain.Invoke(ctx, input)
```

## Adding a New Agent

1. Embed `core.BaseAgent`
2. Use `core.NewDeterministicChain` for single-call agents
3. Add prompt template to `config/prompts.go`
4. Register in `init()` with `core.RegisterAgent()`

## File Structure

```
core/
├── agent.go             # Agent interface, Finding types
├── base.go              # BaseAgent implementation
├── eino.go              # DeterministicChain wrapper
├── registry.go          # Agent factory registry
├── types.go             # Shared types
├── parsers.go           # JSON parsing utilities
├── callbacks.go         # LLM callbacks
└── report.go            # Report generation

impl/
├── analysis_code.go            # CodeAgent (bootstrap)
├── analysis_code_deterministic.go  # Deterministic code analysis
├── analysis_deps.go            # DepsAgent
├── analysis_doc.go             # DocAgent
├── analysis_git.go             # GitAgent
├── planning_agents.go          # ClarifyingAgent, PlanningAgent
├── planning_context.go         # Planning context utilities
├── audit.go                    # Audit agent
├── watch_agent.go              # Watch agent
└── watch_activity.go           # Activity tracking

tools/
├── eino.go              # Tools for agents (read_file, grep_search, etc.)
├── context.go           # Context gathering utilities
├── budget.go            # Token budget management
└── symbol_context.go    # Symbol context utilities

verification/
└── agent.go             # Verification agent
```
