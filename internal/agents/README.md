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

### Interactive Agent (ReAct)

| Agent | Pattern | Purpose |
|-------|---------|---------|
| `ReactAgent` | `react.NewAgent` | Dynamic tool exploration for `tw context --answer` |

## Eino Patterns Used

```go
// Deterministic chain (used by bootstrap agents)
chain := core.NewDeterministicChain[ResponseType](ctx, name, model, promptTemplate)
parsed, raw, duration, err := chain.Invoke(ctx, input)

// ReAct agent with tool calling (used by ReactAgent)
agent, _ := react.NewAgent(ctx, &react.AgentConfig{
    ToolCallingModel: model,
    ToolsConfig:      compose.ToolsNodeConfig{Tools: tools},
    MessageModifier:  func(ctx, msgs) []*schema.Message { ... },
})
result, _ := agent.Generate(ctx, messages)
```

## Adding a New Agent

1. Embed `core.BaseAgent`
2. Use `core.NewDeterministicChain` for single-call agents
3. Add prompt template to `config/prompts.go`
4. Register in `init()` with `core.RegisterAgentFactory()`

## File Structure

```
analysis/
├── code.go              # ReactAgent (interactive exploration)
├── code_deterministic.go # CodeAgent (bootstrap)
├── doc.go               # DocAgent
├── git.go               # GitAgent
└── deps.go              # DepsAgent

tools/
├── eino.go              # Tools for ReactAgent (read_file, grep_search, etc.)
└── context.go           # Context gathering utilities

core/
├── agent.go             # Agent interface, Finding types
├── base.go              # BaseAgent implementation
├── eino.go              # DeterministicChain wrapper
└── registry.go          # Agent factory registry
```
