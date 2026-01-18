# Proposal: Extended Agent Types

> Beyond planning - agents for debugging, triage, brainstorming, and more.

## Current State

TaskWing has two planning agents:
- **ClarifyingAgent**: Refines goals via questions → `enriched_goal`
- **PlanningAgent**: Decomposes goals into tasks

All agents implement `core.Agent` interface with `Run(ctx, Input) → Output`.

## Proposed New Agents

### 1. DebugAgent
**Purpose**: Help developers diagnose issues systematically.

```
Input: error message, stack trace, or symptom description
Output:
  - Hypotheses ranked by likelihood
  - Suggested investigation steps
  - Relevant code locations from memory
```

**Use case**: "My API returns 500 on /users endpoint"

### 2. TriageAgent
**Purpose**: Assess and prioritize incoming issues/bugs.

```
Input: issue description, affected area
Output:
  - Severity assessment (critical/high/medium/low)
  - Affected components (from architecture memory)
  - Suggested owner/assignee
  - Quick fix vs proper fix recommendation
```

**Use case**: "User reports slow checkout" → Routes to perf vs bug vs infra

### 3. BrainstormAgent
**Purpose**: Generate solution alternatives for a problem.

```
Input: problem statement, constraints
Output:
  - 3-5 distinct approaches
  - Pros/cons of each
  - Architecture alignment score (fits patterns?)
  - Recommendation with reasoning
```

**Use case**: "How should we implement caching?"

### 4. ReviewAgent
**Purpose**: Pre-review code changes against architecture.

```
Input: diff or file list
Output:
  - Pattern violations
  - Missing tests/docs
  - Architectural concerns
  - Suggested improvements
```

**Use case**: Before PR submission, catch issues early

### 5. SimplifyAgent
**Purpose**: Reduce complexity and line count in code.

```
Input: file path or code snippet
Output:
  - Simplified version
  - Lines removed (before/after count)
  - What was removed (dead code, over-abstraction, unnecessary wrappers)
  - Risk assessment (behavior changes?)
```

**Use case**: "This 200-line file feels bloated" → Outputs 80-line version

**Targets**:
- Premature abstractions (helpers used once)
- Defensive code for impossible cases
- Verbose error handling that could be consolidated
- Unused parameters, re-exports, compatibility shims

### 6. ExplainAgent
**Purpose**: Deep-dive explanation of code/concepts.

```
Input: symbol, file, or concept
Output:
  - What it does
  - Why it exists (from decisions memory)
  - How it connects to other components
  - Common pitfalls
```

**Use case**: "Explain the authentication flow"

## Implementation Pattern

All agents follow the same structure:

```go
type XxxAgent struct {
    core.BaseAgent
    chain       *core.DeterministicChain[XxxOutput]
    modelCloser io.Closer
}

type XxxOutput struct {
    // Structured output fields
}

func (a *XxxAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
    // 1. Extract input from ExistingContext
    // 2. Build chainInput map
    // 3. Invoke chain
    // 4. Return BuildOutput with findings
}

func init() {
    core.RegisterAgent("xxx", factory, "Display Name", "Description")
}
```

## Integration Points

| Agent | MCP Tool | Slash Command | Priority |
|-------|----------|---------------|----------|
| SimplifyAgent | `simplify` | `/tw-simplify` | **HIGH** |
| ExplainAgent | `explain` | `/tw-explain` | **HIGH** |
| DebugAgent | `debug` | `/tw-debug` | **MEDIUM** |
| TriageAgent | `triage` | `/tw-triage` | LOW |
| BrainstormAgent | `brainstorm` | `/tw-ideas` | LOW |
| ReviewAgent | `review` | `/tw-review` | LOW |

## Questions to Consider

1. **Memory usage**: Should agents write findings back to memory?
2. **Chaining**: Can agents call each other? (Debug → Triage → Plan)
3. **Streaming**: Real-time output for long-running agents?

## Implementation Status

### Completed ✅
- [x] **SimplifyAgent**: Implemented with MCP action (`code simplify`) and slash command (`/tw-simplify`)
- [x] **ExplainAgent**: Implemented (base agent, uses existing `code explain` action)
- [x] **DebugAgent**: Implemented with standalone MCP tool (`debug`) and slash command (`/tw-debug`)
- [x] System prompts defined in `internal/config/prompts.go`
- [x] MCP handlers in `internal/mcp/handlers.go`
- [x] Formatters in `internal/mcp/presenter.go`
- [x] QA audit completed - fixed type assertion bug in formatters (JSON `[]interface{}` handling)
- [x] Presenter tests added for FormatDebugResult and FormatSimplifyResult

### Remaining
- [ ] TriageAgent
- [ ] BrainstormAgent
- [ ] ReviewAgent
- [ ] Integration tests with real LLM calls
