# Start Next TaskWing Task with Full Context

Execute these steps IN ORDER. Do not skip any step.

## Step 1: Get Next Task
Call MCP tool `task_next` to retrieve the highest priority pending task:
```json
{"session_id": "claude-session"}
```

Extract from the response:
- task_id, title, description
- scope (e.g., "auth", "vectorsearch", "api")
- keywords array
- acceptance_criteria
- suggested_recall_queries

If no task returned, inform user: "No pending tasks. Use 'tw plan list' to check plan status."

## Step 2: Fetch Scope-Relevant Context
Call MCP tool `recall` with query based on task scope:
```json
{"query": "[task.scope] patterns constraints decisions"}
```

Examples:
- scope "auth" → `{"query": "authentication cookies session patterns"}`
- scope "api" → `{"query": "api handlers middleware patterns"}`
- scope "vectorsearch" → `{"query": "lancedb embedding vector patterns"}`

Extract: patterns, constraints, related decisions.

## Step 3: Fetch Task-Specific Context
Call MCP tool `recall` with keywords from the task:

Use `suggested_recall_queries` if available, otherwise extract keywords from title.
```json
{"query": "[keywords from task title/description]"}
```

## Step 4: Claim the Task
Call MCP tool `task_start`:
```json
{"task_id": "[task_id from step 1]", "session_id": "claude-session"}
```

## Step 5: Present Unified Task Brief

Display this EXACT format:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 TASK: [task_id] (Priority: [priority])
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**[Title]**

## Description
[Full task description]

## Acceptance Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]
- [ ] [Criterion 3]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🏗️ ARCHITECTURE CONTEXT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Relevant Patterns
[Patterns from recall that apply to this task]

## Constraints
[Constraints that must be respected]

## Related Decisions
[Past decisions that inform this work]

## Key Files
[Files likely to be modified based on context]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Task claimed. Ready to begin.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Step 6: Begin Implementation
Proceed with the task, following the patterns and respecting the constraints shown above.

**CRITICAL**: You MUST call all MCP tools (task_next, recall x2, task_start) before showing the brief. Do not proceed without context.

## Fallback (No MCP)
```bash
tw task list                    # List all tasks
tw task show TASK_ID            # View task details
tw context -q "search term"     # Get context
```
