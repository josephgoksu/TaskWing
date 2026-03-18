---
name: taskwing-next
description: Use when you are ready to start the next approved TaskWing task with full context.
---

# Start Next TaskWing Task with Full Context

## TaskWing Workflow Contract v1 (Always On)
1. No implementation before a clarified and approved plan/task checkpoint.
2. No completion claim without fresh verification evidence.
3. No debug fix proposal without root-cause evidence.

If any gate fails, stop and request the missing approval or evidence.

Execute these steps IN ORDER. Do not skip any step.

## Step 1: Get Next Task
Call MCP tool `task` with action `next` to retrieve the highest-priority pending task:
```json
{"action": "next"}
```

`session_id` is optional when called through MCP transport; include it only for explicit cross-session orchestration.

Extract from the response:
- task_id, title, description
- scope (e.g., "auth", "vectorsearch", "api")
- keywords array
- acceptance_criteria
- suggested_ask_queries

If no task returned, inform user: "No pending tasks. Use /taskwing:status to check plan status."

## Step 2: Fetch Scope-Relevant Context
Call MCP tool `ask` with query based on task scope:
```json
{"query": "[task.scope] patterns constraints decisions"}
```

Examples:
- scope "auth" -> `{"query": "authentication cookies session patterns"}`
- scope "api" -> `{"query": "api handlers middleware patterns"}`
- scope "vectorsearch" -> `{"query": "lancedb embedding vector patterns"}`

Extract: patterns, constraints, related decisions.

## Step 3: Fetch Task-Specific Context
Call MCP tool `ask` with keywords from the task.
Use `suggested_ask_queries` if available, otherwise extract keywords from title.
```json
{"query": "[keywords from task title/description]"}
```

## Step 4: Claim the Task
Call MCP tool `task` with action `start`:
```json
{"action": "start", "task_id": "[task_id from step 1]"}
```

## Step 5: Present Unified Task Brief

Display in this format:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TASK: [task_id] (Priority: [priority])
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**[Title]**

## Description
[Full task description]

## Acceptance Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]
- [ ] [Criterion 3]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ARCHITECTURE CONTEXT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Relevant Patterns
[Patterns from ask that apply to this task]

## Constraints
[Constraints that must be respected]

## Related Decisions
[Past decisions that inform this work]

## Key Files
[Files likely to be modified based on context]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Task claimed. Ready to begin.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Step 6: Implementation Start Gate (Hard Gate)
Before writing or editing code, ask for an explicit checkpoint:
"Implementation checkpoint: proceed with task [task_id] now?"

If approval is missing or unclear, STOP and respond with:
"REFUSAL: I can't start implementation yet. Plan/task checkpoint is incomplete. Please approve this task checkpoint first."

## Step 7: Begin Implementation (Only After Approval)
Proceed with the task, following the patterns and respecting the constraints shown above.

**CRITICAL**: You MUST call all MCP tools (`task(next)`, `ask` x2, `task(start)`) before showing the brief and before requesting implementation approval.

## Fallback (No MCP)
```bash
taskwing task list                    # List all tasks
taskwing task list --status pending   # Identify next pending task
```
Use /taskwing:status to check active plan progress.
