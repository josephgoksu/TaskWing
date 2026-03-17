---
name: taskwing-status
description: Use when you need current task progress and acceptance criteria status.
---

# Show Current Task Status

This is a read-only status command. Do not use it to bypass plan, verification, or debug gates.

## Step 1: Get Current Task
Call MCP tool `task` with action `current`:
```json
{"action": "current"}
```

If no active task:
```
No active task. Use /taskwing:next to start the next priority task.
```

## Step 2: Display Status

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
CURRENT TASK STATUS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Task: [task_id] - [title]
Priority: [priority]
Status: [status]
Started: [claimed_at timestamp]
Scope: [scope]

## Acceptance Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Commands:
  /taskwing:done    - Complete this task
  /taskwing:ask     - Fetch more context
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Fallback (No MCP)
```bash
taskwing task list --status in_progress
taskwing plan list
```
