# MCP Dogfooding Notes

> Notes from using TaskWing MCP tools during development.

## Session: 2026-01-18 - Building New Agents

### Note #1: `recall` with `answer=true`
**Tool**: `mcp__taskwing-mcp__recall`
**Query**: "agent implementation pattern deterministic chain system prompt"

**Result**: Excellent context. Returned:
- DeterministicChain pattern explanation with file locations
- System prompt locations (`internal/config/prompts.go:463`, `:513`)
- Code symbols with line numbers
- Related constraint: "Deterministic Agent Patterns"

**Verdict**: High value. Single call gave enough context to understand the architecture.

---

### Note #2: `code search` relevance scores
**Tool**: `mcp__taskwing-mcp__code` with `action=search`
**Query**: "BaseAgent DeterministicChain agent implementation"

**Result**: Lower relevance scores (max 0.30), but still pointed to correct files:
- `internal/agents/impl/audit.go`
- `internal/agents/core/eino.go`
- `internal/agents/core/base.go`

**Verdict**: Useful for discovery, but scores suggest semantic search could be improved. Consider:
- Better embedding model?
- More context in index?

---

### Note #3: `code explain` is the killer feature
**Tool**: `mcp__taskwing-mcp__code` with `action=explain`
**Query**: "DeterministicChain"

**Result**: Comprehensive output including:
- Full source code with line numbers
- Related decisions (CloudWeGo Eino, Bubble Tea, CGO-free SQLite)
- Related patterns (Adding a New Agent, Deterministic Agent Patterns)
- AI-generated explanation of architectural significance
- Impact analysis (callers/callees)

**Verdict**: This is what developers actually need. Deep context in one call.

---

## Improvement Ideas

1. **Search relevance**: 0.30 scores feel low. Investigate embedding quality.
2. **Cross-reference**: `recall` + `code explain` together would be even better.
3. **Streaming**: Long `explain` calls could benefit from streaming output.

---

### Note #4: Context integration for agents
**Use case**: SimplifyAgent needs architectural context to avoid removing patterns that are intentional.

**Approach**: Handler fetches context via `RecallApp.Query()` before invoking agent:
```go
result, err := recallApp.Query(ctx, "patterns and constraints for "+filePath, ...)
kgContext = formatRecallContext(result)
```

**Verdict**: Clean pattern. Agents stay focused on their task, MCP layer handles context fetching. Could be abstracted into a helper.

---

### Note #5: Adding a new action to unified tools
**Task**: Add `simplify` action to the `code` tool.

**Steps**:
1. Add action const to `internal/mcp/types.go`
2. Update `IsValid()` and `ValidCodeActions()`
3. Add handler to `internal/mcp/handlers.go`
4. Add formatter to `internal/mcp/presenter.go`
5. Update MCP tool description in `cmd/mcp_server.go`

**Verdict**: Pattern is clear and easy to follow. Could use a generator or scaffolding tool for new actions.

---

## Summary

| Tool | Use Case | Verdict |
|------|----------|---------|
| `recall` + `answer=true` | Quick architecture questions | Excellent |
| `code search` | Discovery, finding files | Good (needs tuning) |
| `code explain` | Deep dive into symbols | Excellent |
| `code simplify` | Reduce code complexity | New - testing needed |
| `code find` | Not tested yet | TBD |
| `code impact` | Not tested yet | TBD |

### Note #6: Adding a new standalone tool
**Task**: Add `debug` as a new MCP tool.

**Different from unified tools**: Debug is not code intelligence, so separate tool makes sense.

**Pattern**:
1. Add params type to `internal/mcp/types.go`
2. Add handler to `internal/mcp/handlers.go`
3. Add formatter to `internal/mcp/presenter.go`
4. Register tool in `cmd/mcp_server.go`

**Observation**: Handler pattern is consistent - fetch context, invoke agent, format output.

---

### Note #7: Slash commands
**Task**: Add `/tw-simplify` and `/tw-debug` slash commands.

**Pattern**:
1. Add command to `cmd/slash.go` (`slashSimplifyCmd`, etc.)
2. Add content to `cmd/slash_content.go`
3. Content tells AI which MCP tools to call

**Key insight**: Slash commands are just prompts that tell AI how to use MCP tools. They bridge user intent to tool invocation.

---

## Patterns Discovered

1. **Context → Agent → Format**: MCP handlers should fetch context, run agent, format output
2. **Unified tools**: Single tool with `action` param is cleaner than many small tools
3. **Presenter separation**: Format logic separate from business logic is maintainable
4. **Slash commands**: Prompts that teach AI how to combine MCP tools for specific workflows

---

### Note #8: Type Assertion Bug in Formatters
**Issue**: QA audit found critical runtime bug in `FormatDebugResult` and `FormatSimplifyResult`.

**Problem**: Direct type assertions like `f.Metadata["changes"].([]SimplifyChange)` fail at runtime when data has been through JSON serialization/deserialization. JSON unmarshals:
- Slices → `[]interface{}`
- Maps → `map[string]interface{}`
- Numbers → `float64` (not `int`)

**Solution**: Added extraction helper functions that handle both direct types and `[]interface{}`:
```go
func extractSimplifyChanges(raw interface{}) []SimplifyChange {
    // Direct type match (from agent before serialization)
    if typed, ok := raw.([]SimplifyChange); ok {
        return typed
    }
    // Handle []interface{} from JSON
    if arr, ok := raw.([]interface{}); ok {
        result := make([]SimplifyChange, 0, len(arr))
        for _, item := range arr {
            if m, ok := item.(map[string]interface{}); ok {
                c := SimplifyChange{
                    What: getStringField(m, "what"),
                    Why:  getStringField(m, "why"),
                    Risk: getStringField(m, "risk"),
                }
                result = append(result, c)
            }
        }
        return result
    }
    return nil
}
```

**Key insight**: Always assume metadata coming through MCP could be JSON-serialized. Don't trust direct type assertions on complex types.

**Files fixed**:
- `internal/mcp/presenter.go` - Added `extractDebugHypotheses`, `extractDebugSteps`, `extractDebugFixes`, `extractSimplifyChanges`, `getIntFromMetadata`

**Tests added**:
- `TestFormatDebugResult_WithJSONStyleData` - Tests with `[]interface{}` data
- `TestFormatSimplifyResult_WithJSONStyleData` - Tests with `float64` numbers
- `TestExtractSimplifyChanges_DirectType` - Tests direct type handling
- `TestExtractDebugHypotheses_DirectType` - Tests direct type handling
- `TestGetIntFromMetadata` - Tests int extraction from `float64`

---

### Note #9: Path Traversal Vulnerability
**Issue**: QA audit found security vulnerability in `handleCodeSimplify`.

**Problem**: The original code:
```go
basePath, _ := config.GetProjectRoot()
fullPath := filePath
if basePath != "" && !strings.HasPrefix(filePath, "/") {
    fullPath = basePath + "/" + filePath
}
content, err := readFileContent(fullPath)
```

This allowed `../../../etc/passwd` to escape the project root via path traversal.

**Solution**: Added `validateAndResolvePath()` function that:
1. Uses `filepath.Clean()` to normalize the path
2. Rejects any path containing `..` after cleaning
3. Verifies the resolved absolute path starts with the project root
4. Checks that the path is a file, not a directory

**Tests added**:
- `TestValidateAndResolvePath_PathTraversal` - Tests various traversal attacks
- `TestValidateAndResolvePath_ValidPaths` - Tests legitimate paths
- `TestValidateAndResolvePath_DirectoryRejection` - Rejects directories
- `TestHandleCodeTool_SimplifyPathTraversal` - Integration test

**Key insight**: Any file reading handler exposed via MCP needs strict path validation. MCP tools are exposed to potentially untrusted input.

---


from another conversation

⏺ taskwing-mcp - plan (MCP)(action: "clarify", goal: "Implement automatic policy enforcement during task completion - when an AI agent completes a task via /tw-done, the PolicyEnforcer should
                           evaluate the files_modified against loaded OPA policies and block completion if violations are detected")
  ⎿  ## 🔍 Clarification Needed

     **Goal**: Add OPA-based policy enforcement to block task completion on policy violations.
     … +8 lines (ctrl+o to expand)

⏺ Good questions from TaskWing. Let me provide answers and continue clarifying.

⏺ taskwing-mcp - plan (MCP)(action: "clarify", goal: "Implement automatic policy enforcement during task completion - when an AI agent completes a task via /tw-done, the PolicyEnforcer should
                           evaluate the files_modified against loaded OPA policies and block completion if violations are detected", history: "Q1: Policies already reside in
                           .taskwing/policies/*.rego - we have a default policy created via 'taskwing policy init'. Q2: No bypass mechanism - if policies are violated, completion should fail and
                            task should remain in_progress. The user can update the policy if needed. Q3: Pass task ID, task title, files_modified, files_created (empty for now), plan ID, and
                           plan goal - this matches the existing PolicyEvaluator interface in internal/task/sentinel.go.")
  ⎿  ## ✅ Ready to Generate Plan

     **Goal**: Implement OPA-based policy enforcement during the /tw-done completion flow.
     … +34 lines (ctrl+o to expand)

⏺ The spec looks solid. Let me generate the plan with tasks.

⏺ taskwing-mcp - plan (MCP)(action: "generate", enriched_goal: "Technical Specification: Automated Policy Enforcement Hook\n\n1. **Objective**: Integrate Open Policy Agent (OPA) into the task
                           completion lifecycle to validate file modifications against project-defined policies before a task is transitioned to 'completed'.\n\n2. **Architecture
                           Integration**:\n    - **Trigger**: Modify `TaskApp.Complete` in `internal/app/task.go`. The enforcement must occur AFTER the agent provides the `files_modified` list
                           but BEFORE the SQLite transaction updates the task status.\n    - **Service**: Use existing `PolicyEnforcer` in `internal/task/sentinel.go` and
                           `PolicyEvaluatorAdapter` in `internal/policy/engine.go`. Create the policy engine and adapter in TaskApp.Complete.\n    - **Input Data**: The OPA input object must
                           include:\n        - `task`: {id, title, description, files_modified, files_created}\n        - `plan`: {id, goal}\n\n3. **Logic Flow**:\n    - Create PolicyEngine from
                            .taskwing/policies/ directory\n    - Create PolicyEvaluatorAdapter wrapping the engine\n    - Create PolicyEnforcer with the adapter\n    - Call
                           `PolicyEnforcer.Enforce(ctx, task, planGoal)`.\n    - If result.Allowed is false:\n        - Return error with violations list.\n        - Task status remains
                           `in_progress` in SQLite (don't call repo.CompleteTask).\n    - If allowed, proceed to existing completion flow.\n\n4. **Failure Handling**:\n    - MCP response should
                           include policy violations in the error message\n    - Format violations clearly so AI agent can understand what files caused the block\n\n5. **Dependencies**: Use
                           existing `github.com/open-policy-agent/opa/v1/rego` already in the project.", save: true)
  ⎿  Error: ## ❌ Error

     **Details**: goal is required for generate action

⏺ taskwing-mcp - plan (MCP)(action: "generate", goal: "Technical Specification: Automated Policy Enforcement Hook\n\n1. **Objective**: Integrate Open Policy Agent (OPA) into the task completion
                           lifecycle to validate file modifications against project-defined policies before a task is transitioned to 'completed'.\n\n2. **Architecture Integration**:\n    -
                           **Trigger**: Modify `TaskApp.Complete` in `internal/app/task.go`. The enforcement must occur AFTER the agent provides the `files_modified` list but BEFORE the SQLite
                           transaction updates the task status.\n    - **Service**: Use existing `PolicyEnforcer` in `internal/task/sentinel.go` and `PolicyEvaluatorAdapter` in
                           `internal/policy/engine.go`. Create the policy engine and adapter in TaskApp.Complete.\n    - **Input Data**: The OPA input object must include:\n        - `task`:
                           {id, title, description, files_modified, files_created}\n        - `plan`: {id, goal}\n\n3. **Logic Flow**:\n    - Create PolicyEngine from .taskwing/policies/
                           directory\n    - Create PolicyEvaluatorAdapter wrapping the engine\n    - Create PolicyEnforcer with the adapter\n    - Call `PolicyEnforcer.Enforce(ctx, task,
                           planGoal)`.\n    - If result.Allowed is false:\n        - Return error with violations list.\n        - Task status remains `in_progress` in SQLite (don't call
                           repo.CompleteTask).\n    - If allowed, proceed to existing completion flow.\n\n4. **Failure Handling**:\n    - MCP response should include policy violations in the
                           error message\n    - Format violations clearly so AI agent can understand what files caused the block\n\n5. **Dependencies**: Use existing
                           `github.com/open-policy-agent/opa/v1/rego` already in the project.", save: true)
  ⎿  Error: ## ❌ Error

     **Details**: enriched_goal is required for generate action

⏺ taskwing-mcp - plan (MCP)(action: "generate", goal: "Implement automatic policy enforcement during task completion", enriched_goal: "Technical Specification: Automated Policy Enforcement
                           Hook\n\n1. **Objective**: Integrate Open Policy Agent (OPA) into the task completion lifecycle to validate file modifications against project-defined policies before a
                            task is transitioned to 'completed'.\n\n2. **Architecture Integration**:\n    - **Trigger**: Modify `TaskApp.Complete` in `internal/app/task.go`. The enforcement must
                            occur AFTER the agent provides the `files_modified` list but BEFORE the SQLite transaction updates the task status.\n    - **Service**: Use existing `PolicyEnforcer`
                           in `internal/task/sentinel.go` and `PolicyEvaluatorAdapter` in `internal/policy/engine.go`. Create the policy engine and adapter in TaskApp.Complete.\n    - **Input
                           Data**: The OPA input object must include:\n        - `task`: {id, title, description, files_modified, files_created}\n        - `plan`: {id, goal}\n\n3. **Logic
                           Flow**:\n    - Create PolicyEngine from .taskwing/policies/ directory\n    - Create PolicyEvaluatorAdapter wrapping the engine\n    - Create PolicyEnforcer with the
                           adapter\n    - Call `PolicyEnforcer.Enforce(ctx, task, planGoal)`.\n    - If result.Allowed is false:\n        - Return error with violations list.\n        - Task
                           status remains `in_progress` in SQLite (don't call repo.CompleteTask).\n    - If allowed, proceed to existing completion flow.\n\n4. **Failure Handling**:\n    - MCP
                           response should include policy violations in the error message\n    - Format violations clearly so AI agent can understand what files caused the block\n\n5.
                           **Dependencies**: Use existing `github.com/open-policy-agent/opa/v1/rego` already in the project.", save: true)

✻ Transmuting… (ctrl+c to interrupt · 1m 33s · ↓ 2.2k tokens)
  ⎿  Tip: ┌──────────┐
           ) CC ✻ ┊ (  You have free guest passes
          └──────────┘ /passes to share


it keeps failing on these actions and wastes a lot of tokens bc of that


----  

❯ By the way, during this whole session, how was your experience with Taskwing CLI and MCP tools Could you give me some feedbacks So I can document them, and later I can use them for
  improvements

⏺ Honest Feedback on TaskWing Experience

  What Worked Well
  ┌──────────────────────────────┬───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
  │           Feature            │                                                                        Experience                                                                         │
  ├──────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Plan generation              │ The plan clarify → plan generate flow produced a well-structured 5-task plan with clear acceptance criteria. The priority ordering (P100→P60) made sense. │
  ├──────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Task context                 │ Each task came with relevant architecture context from the knowledge graph. The "Relevant Architecture Context" section was genuinely useful.             │
  ├──────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Auto-commit on task complete │ Loved that /tw-done automatically committed and pushed. Reduced friction significantly.                                                                   │
  ├──────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Sentinel report              │ The deviation detection ("1 unexpected file") caught real discrepancies between what I reported vs. what git showed.                                      │
  ├──────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Policy MCP tool              │ mcp__taskwing-mcp__policy check worked exactly as expected. Clear pass/fail output.                                                                       │
  └──────────────────────────────┴───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
  Pain Points & Friction
  ┌────────────────────────────────────────┬───────────────────────────────────────────────────────────────────────────────────────────┬─────────────────────────────────────────────────────────┐
  │                 Issue                  │                                          Impact                                           │                       Suggestion                        │
  ├────────────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────┤
  │ No way to add tasks to existing plan   │ I tried tw task add --plan but flag doesn't exist. Had to work around it.                 │ Add tw task add "title" --plan <id>                     │
  │ via CLI                                │                                                                                           │                                                         │
  ├────────────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────┤
  │ Plan with 0 tasks after --offline      │ Created plan but couldn't add tasks to test policy enforcement manually.                  │ Allow tw task create for manual task creation           │
  ├────────────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────┤
  │ Session ID management is invisible     │ I had to guess/track session IDs. Not clear when a session expires or how to resume.      │ Show session ID in prompt or status bar                 │
  ├────────────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────┤
  │ task_complete requires exact task_id   │ Had to copy-paste IDs. Error-prone.                                                       │ Support task_complete --current to complete the         │
  │                                        │                                                                                           │ in-progress task                                        │
  ├────────────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────┤
  │ No --dry-run for task complete         │ Couldn't preview what policy check would say before actually completing.                  │ Add tw task complete --dry-run                          │
  ├────────────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────┤
  │ Recall queries in task hints are       │ The suggested queries like "general patterns constraints decisions" weren't specific      │ Generate more targeted queries based on task content    │
  │ generic                                │ enough to be useful.                                                                      │                                                         │
  └────────────────────────────────────────┴───────────────────────────────────────────────────────────────────────────────────────────┴─────────────────────────────────────────────────────────┘
  MCP Tool Specific Issues
  ┌───────────────┬────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
  │     Tool      │                                                               Issue                                                                │
  ├───────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ plan generate │ Required both goal AND enriched_goal but error messages were confusing. First said "goal required", then "enriched_goal required". │
  ├───────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ task start    │ Worked fine, but the double output (task details printed twice) was redundant.                                                     │
  ├───────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ policy check  │ Excellent. No complaints.                                                                                                          │
  ├───────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ recall        │ Didn't use much this session, but in past sessions the results were sometimes too verbose.                                         │
  └───────────────┴────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
  Missing Features I Wanted

  1. tw policy test - Run policy against hypothetical files without needing a real task
  2. tw task current --json - Get current task ID programmatically for scripts
  3. tw plan status - Quick view of plan progress (X/Y tasks done) without full task list
  4. Policy dry-run in MCP - policy check works, but would be nice to have it integrated into task complete --preview

  Documentation Gaps

  - No docs on how policy enforcement integrates with task completion flow
  - The CLAUDE.md mentions PolicyEnforcer exists but not how to use it
  - Error messages like "Policy violations blocked task completion" are good but could link to docs

  Overall Rating

  7.5/10 - The core workflow (plan → tasks → complete → auto-commit) is solid and genuinely useful. The friction points are all solvable UX issues, not fundamental architectural problems. The
  policy enforcement feature we just built fills a real gap.

  Biggest win: The stop hook auto-continuation through tasks is genuinely productive. I completed 5 tasks without manually invoking next each time.

  Biggest gap: Task management is plan-centric but sometimes you need ad-hoc tasks. No escape hatch for that.