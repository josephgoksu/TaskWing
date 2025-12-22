# Agents Package

LLM-powered agents for codebase analysis. Each agent focuses on a specific aspect (docs, git history, dependencies, code patterns).

## Adding a New Agent

### Step 1: Create Your Agent File

Create `internal/agents/my_agent.go`:

```go
package agents

import (
    "context"
    "time"

    "github.com/cloudwego/eino/schema"
    "github.com/josephgoksu/TaskWing/internal/llm"
)

type MyAgent struct {
    BaseAgent // Embed for shared functionality
}

func NewMyAgent(cfg llm.Config) *MyAgent {
    return &MyAgent{
        BaseAgent: NewBaseAgent("my_agent", "Description of what it does", cfg),
    }
}

func (a *MyAgent) Run(ctx context.Context, input Input) (Output, error) {
    start := time.Now()

    // 1. Gather context (read files, run commands, etc.)
    prompt := buildPrompt(...)

    // 2. Call LLM using BaseAgent.Generate()
    messages := []*schema.Message{schema.UserMessage(prompt)}
    rawOutput, err := a.Generate(ctx, messages)
    if err != nil {
        return Output{}, err
    }

    // 3. Parse response using shared helper
    parsed, err := ParseJSONResponse[myResponseType](rawOutput)
    if err != nil {
        return Output{}, err
    }

    // 4. Convert to findings
    var findings []Finding
    for _, item := range parsed.Items {
        findings = append(findings, Finding{
            Type:        FindingTypeDecision, // or Feature, Pattern, etc.
            Title:       item.Title,
            Description: item.Description,
            SourceAgent: a.Name(),
        })
    }

    return BuildOutput(a.Name(), findings, rawOutput, time.Since(start)), nil
}
```

### Step 2: Register the Agent

Add to `registry.go` init():

```go
func init() {
    // ...existing registrations...
    RegisterAgentFactory("my_agent", func(cfg llm.Config) Agent {
        return NewMyAgent(cfg)
    })
}
```

That's it. The agent is now available via `CreateAgent("my_agent", cfg)`.

---

## BaseAgent Helpers

| Method | Purpose |
|--------|---------|
| `Name()` | Returns agent ID |
| `Description()` | Returns agent description |
| `CreateChatModel(ctx)` | Creates LLM client |
| `Generate(ctx, messages)` | Sends prompt, returns response string |
| `ParseJSONResponse[T](response)` | Extracts JSON from LLM response |
| `BuildOutput(name, findings, raw, duration)` | Creates Output struct |

---

## File Overview

| File | Purpose |
|------|---------|
| `agent.go` | Core interfaces: `Agent`, `Input`, `Output`, `Finding` |
| `base_agent.go` | Shared LLM/JSON functionality |
| `registry.go` | Factory pattern for agent creation |
| `doc_agent.go` | Analyzes markdown documentation |
| `git_deps_agent.go` | Analyzes git history + dependencies |
| `react_code_agent.go` | ReAct pattern for code exploration |
| `prompt_agent.go` | Generic configurable agent |
| `eino_tools.go` | Eino-compatible tools for ReAct agents |
