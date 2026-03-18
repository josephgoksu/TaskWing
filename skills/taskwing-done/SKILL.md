---
name: taskwing-done
description: Use when implementation is verified and you are ready to complete the current task.
---

# Complete Task with Architecture-Aware Summary

The Workflow Contract lives in CLAUDE.md (single source of truth). Obey it always.

## Kill Table

| Impulse | Do Instead |
|---------|------------|
| Mark complete without running verification | STOP with refusal text. Evidence is non-negotiable. |
| Reuse verification output from earlier in the conversation | Run fresh checks in this completion attempt |
| Skip acceptance criteria check | Every criterion must be explicitly addressed (met/not met/partial) |
| Silently drop unmet criteria | Call them out. Partial completion is honest; silent omission is not. |

## Operating Principles

1. **Verification is non-negotiable.** Every completion requires fresh evidence from this attempt.
2. **Evidence must be fresh.** "I ran tests earlier" does not count. Run them now.
3. **Acceptance criteria are explicit.** Each one gets a verdict: met, not met, or partial with explanation.

Execute these steps IN ORDER.

## Step 1: Get Current Task
Call MCP tool `task` with action `current`:
```json
{"action": "current"}
```

If no active task, inform user and stop.

## Step 2: Collect Fresh Verification Evidence
Run the most relevant verification commands for the task (tests, lint, build, or targeted checks).

Document:
- command run
- exit status
- short output snippet proving pass/fail

If verification was not run in this completion attempt, STOP and respond with:
"REFUSAL: I can't mark this task done yet. Verification evidence is missing. Run fresh checks and include the output."

## Step 3: Generate Completion Report

Create a structured summary covering:

### Files Modified
List all files changed with purpose of change.

### Acceptance Criteria Verification
For each criterion:
- **Met**: [How it was satisfied]
- **Not Met**: [Why, and what's needed]
- **Partial**: [What was done, what remains]

### Pattern Compliance
Confirm alignment with codebase patterns.

### Technical Debt / Follow-ups
- TODOs introduced
- Tests not written
- Edge cases not handled

## Step 4: Completion Gate (Hard Gate)
Before calling `task complete`, confirm:
- evidence is fresh (from Step 2)
- acceptance criteria status is explicit
- unresolved failures are called out

If any item is missing, STOP and use the refusal text above.

## Step 5: Mark Complete
Call MCP tool `task` with action `complete`:
```json
{
  "action": "complete",
  "task_id": "[task_id]",
  "summary": "[The structured summary from Step 2]",
  "files_modified": ["path/to/file1.go", "path/to/file2.go"]
}
```

## Step 6: Confirm to User

Display:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TASK COMPLETE: [task_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Summary report]

Recorded in TaskWing memory.
Use /taskwing:next to continue with next priority task.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Fallback (No MCP)
```bash
taskwing task complete TASK_ID
```
