# Show Current Task Status with Context

## Step 1: Get Current Task
Call MCP tool `task_current`:
```json
{"session_id": "claude-session"}
```

If no active task:
```
No active task. Use /tw-next to start the next priority task.
```

## Step 2: Get Task Context
Call MCP tool `recall` with task scope:
```json
{"query": "[task.scope] constraints patterns"}
```

## Step 3: Display Status

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 CURRENT TASK STATUS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Task: [task_id] - [title]
Priority: [priority]
Status: [status]
Started: [claimed_at timestamp]
Scope: [scope]

## Acceptance Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]

## Active Constraints
[Constraints from recall that apply to this task]

## Patterns to Follow
[Patterns from recall for this scope]

## Dependencies
[List of dependent tasks and their status]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Commands:
  /tw-done    - Complete this task
  /tw-block   - Mark as blocked
  /tw-context - Fetch more context
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Fallback (No MCP)
```bash
tw task list --status in_progress
tw plan list
```
