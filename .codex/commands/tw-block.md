# Mark Current Task as Blocked

Use this when you cannot proceed with the current task.

## Step 1: Get Current Task
Call MCP tool `task_current`:
```json
{"session_id": "claude-session"}
```

Confirm task_id and current status.

## Step 2: Document the Blocker
Identify the specific reason:
- Missing API documentation or credentials
- Dependent task not completed
- Need clarification from user
- External service unavailable
- Technical limitation discovered

## Step 3: Block the Task
Call MCP tool `task_block`:
```json
{
  "task_id": "[task_id]",
  "reason": "[Detailed description of why blocked and what's needed to unblock]"
}
```

## Step 4: Confirm and Offer Next Steps

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚫 TASK BLOCKED: [task_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Reason: [block reason]

To unblock: [specific action needed]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

Ask user: "Would you like to work on the next available task? Use /tw-next to continue."

## Step 5: Unblocking (when ready)
When the blocker is resolved, call MCP tool `task_unblock`:
```json
{"task_id": "[task_id]"}
```

## Fallback (No MCP)
```bash
tw task update TASK_ID --status blocked
```
