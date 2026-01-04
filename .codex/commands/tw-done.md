# Complete Task with Architecture-Aware Summary

Execute these steps IN ORDER.

## Step 1: Get Current Task
Call MCP tool `task_current`:
```json
{"session_id": "claude-session"}
```

If no active task, inform user and stop.

## Step 2: Fetch Original Context
Call MCP tool `recall` with the task scope to retrieve patterns/constraints that were meant to be followed:
```json
{"query": "[task.scope] patterns constraints"}
```

## Step 3: Generate Completion Report

Create a structured summary covering:

### Files Modified
List all files changed:
- File path
- Lines added/removed (approximate)
- Purpose of change

### Acceptance Criteria Verification
For each criterion from the task:
- ✅ **Met**: [How it was satisfied]
- ❌ **Not Met**: [Why, and what's needed]
- ⚠️ **Partial**: [What was done, what remains]

### Pattern Compliance
Confirm alignment with codebase patterns from recall:
- [Pattern name]: ✅ Followed / ⚠️ Deviated because [reason]

### Constraint Adherence
Confirm constraints were respected:
- [Constraint]: ✅ Respected / ❌ Violated (requires review)

### Technical Debt / Follow-ups
- TODOs introduced
- Tests not written
- Edge cases not handled

## Step 4: Mark Complete
Call MCP tool `task_complete`:
```json
{
  "task_id": "[task_id]",
  "summary": "[The structured summary from Step 3]",
  "files_modified": ["path/to/file1.go", "path/to/file2.go"]
}
```

## Step 5: Confirm to User

Display:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ TASK COMPLETE: [task_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Summary report from Step 3]

Recorded in TaskWing memory.
Use /tw-next to continue with next priority task.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**CRITICAL**: Do not tell user task is complete until task_complete MCP returns success.

## Fallback (No MCP)
```bash
tw task complete TASK_ID
```
