# Autonomous Task Execution with Hooks

> Enable Claude Code to automatically continue through TaskWing plans

TaskWing integrates with Claude Code's hook system to create an autonomous task execution loop. When one task completes, the system automatically injects the next task context, allowing Claude to work through an entire plan with minimal human intervention.

---

## Quick Setup

```bash
# 1. Ensure TaskWing is bootstrapped
taskwing bootstrap

# 2. The hook configuration is already in .claude/settings.json
# Just start Claude Code in your project directory

# 3. Create and activate a plan
taskwing plan new "Your development goal"
taskwing plan start latest

# 4. Start working - use /tw-next to begin first task
# Subsequent tasks will auto-continue
```

> **Note**: Slash commands like `/tw-next` are dynamic wrappers that call `taskwing slash next` at runtime. This ensures command content always matches your installed CLI version—no manual update needed after `brew upgrade taskwing`.

---

## How It Works

### The Autonomous Loop

```
┌─────────────────────────────────────────────────────────────┐
│                    SESSION LIFECYCLE                         │
└─────────────────────────────────────────────────────────────┘

SessionStart Hook
       │
       ▼
┌──────────────────┐
│ taskwing hook    │  Initializes session tracking:
│ session-init     │  • Session ID
│                  │  • Start timestamp
│                  │  • Active plan detection
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ User runs        │  First task requires manual start
│ /tw-next         │  (or direct task_start MCP call)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Claude works     │  Implements the task following
│ on task          │  architecture context
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Claude calls     │  Marks task complete with summary
│ task_complete    │  and files_modified
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Claude attempts  │  Natural end of response
│ to stop          │
└────────┬─────────┘
         │
         ▼
Stop Hook
       │
       ▼
┌──────────────────┐     ┌─────────────────────────────────┐
│ taskwing hook    │────▶│ Circuit Breaker Checks:         │
│ continue-check   │     │ • Max tasks reached?            │
│                  │     │ • Max duration exceeded?        │
│                  │     │ • Current task blocked?         │
│                  │     │ • No more pending tasks?        │
└────────┬─────────┘     └─────────────────────────────────┘
         │
         ├─── Circuit breaker triggered ───▶ {"decision": "approve"}
         │                                          │
         │                                          ▼
         │                                   Claude stops
         │                                   (human review)
         │
         └─── More tasks available ───▶ {"decision": "block",
                                          "context": "next task..."}
                                                │
                                                ▼
                                         Claude continues
                                         with next task
                                                │
                                                └───────┐
                                                        │
         ┌──────────────────────────────────────────────┘
         │
         ▼
    (Loop repeats until circuit breaker or plan complete)

         │
         ▼
SessionEnd Hook
       │
       ▼
┌──────────────────┐
│ taskwing hook    │  Logs session summary:
│ session-end      │  • Duration
│                  │  • Tasks completed
└──────────────────┘
```

---

## Hook Commands

### `taskwing hook session-init`

Called by `SessionStart` hook. Initializes session tracking.

```bash
$ taskwing hook session-init

TaskWing Session Initialized
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Session ID: session-1767478355
Started: 22:12:35
Active Plan: plan-abc123

The Stop hook is configured to automatically continue to the next task.
Circuit breakers: Max 5 tasks, Max 30 minutes.

Use /tw-next to start the first task, or it will auto-continue after each task.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### `taskwing hook continue-check`

Called by `Stop` hook. Determines if Claude should continue or stop.

**Flags:**
- `--max-tasks=N` - Maximum tasks per session (default: 5)
- `--max-minutes=N` - Maximum session duration (default: 30)

**Output (allow stop):**
```json
{"decision": "approve", "reason": "Circuit breaker: Completed 5/5 tasks this session."}
```

**Output (continue to next task):**
```json
{
  "decision": "block",
  "reason": "Continue to task 2/5: Implement authentication middleware",
  "context": "... next task details and architecture context ..."
}
```

### `taskwing hook session-end`

Called by `SessionEnd` hook. Logs summary and cleans up.

```bash
$ taskwing hook session-end

TaskWing Session Complete
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Session: session-1767478355
Duration: 25 minutes
Tasks Completed: 4
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### `taskwing hook status`

Check current session state (for debugging).

```bash
$ taskwing hook status
{
  "active": true,
  "session_id": "session-1767478355",
  "started_at": "2026-01-03T22:12:35Z",
  "elapsed_minutes": 15,
  "tasks_completed": 3,
  "tasks_started": 4,
  "current_task_id": "task-xyz789",
  "plan_id": "plan-abc123"
}
```

---

## Configuration

### `.claude/settings.json`

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "taskwing hook session-init",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "taskwing hook continue-check --max-tasks=5 --max-minutes=30",
            "timeout": 15
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "taskwing hook session-end",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

### Customizing Circuit Breakers

Adjust limits based on your workflow:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "taskwing hook continue-check --max-tasks=10 --max-minutes=60",
            "timeout": 15
          }
        ]
      }
    ]
  }
}
```

---

## Circuit Breakers

The system includes multiple safety mechanisms to prevent runaway execution:

| Breaker | Default | Trigger |
|---------|---------|---------|
| Max Tasks | 5 | Stop after N tasks for human review |
| Max Duration | 30 min | Stop after N minutes |
| Blocked Task | - | Stop if current task is blocked |
| No Active Plan | - | Stop if no plan is set |
| Plan Complete | - | Stop when all tasks are done |

When a circuit breaker triggers, Claude stops and you can:
- Review completed work
- Adjust the plan if needed
- Resume with `/tw-next` or start a new session

---

## Session State

Session data is stored in `.taskwing/memory/hook_session.json`:

```json
{
  "session_id": "session-1767478355",
  "started_at": "2026-01-03T22:12:35Z",
  "tasks_completed": 3,
  "tasks_started": 4,
  "current_task_id": "task-xyz789",
  "plan_id": "plan-abc123"
}
```

This file is automatically created by `session-init` and removed by `session-end`.

---

## Best Practices

### 1. Start with Conservative Limits

Begin with `--max-tasks=3` until you trust the loop:

```json
"command": "taskwing hook continue-check --max-tasks=3 --max-minutes=15"
```

### 2. Review After Each Session

The circuit breaker forces review points. Use them to:
- Verify completed tasks meet acceptance criteria
- Check for quality degradation
- Adjust the plan if needed

### 3. Use Task Blocking for Issues

If Claude encounters a problem mid-task, it should use the `/tw-block` command:
```
/tw-block
```

Then provide the block reason when prompted. Alternatively, Claude can call the `task_block` MCP tool directly with the task_id and reason.

This triggers the circuit breaker and stops execution.

### 4. Monitor Token Usage

Long autonomous sessions consume significant tokens. The duration limit helps control costs.

---

## Troubleshooting

### Hooks Not Firing

1. Check hooks are registered: `/hooks` in Claude Code
2. Verify settings.json is valid JSON
3. Ensure `taskwing` is in PATH (install via Homebrew: `brew install josephgoksu/tap/taskwing`)

### Session Not Initialized

The `session-init` output appears in Claude Code's context. If missing:
```bash
taskwing hook session-init  # Run manually to debug
```

### Continue-Check Returns "approve" Unexpectedly

Check the reason in the JSON output:
```bash
taskwing hook continue-check  # Run manually
```

Common causes:
- No active plan (`taskwing plan start <id>`)
- Circuit breaker reached (check `taskwing hook status`)
- All tasks complete

### Context Not Injected

The `context` field in the block response should contain next task details. If empty:
- Check task has description and acceptance criteria
- Verify recall queries return results (`taskwing context -q "query"`)

---

## Architecture

### Files

```
cmd/hook.go                           # Hook command implementation
.claude/settings.json                 # Claude Code hook configuration
.codex/settings.json                  # Codex hook configuration
.taskwing/memory/hook_session.json    # Session state (runtime)
```

### Hook Response Schema

```go
type HookResponse struct {
    Decision string `json:"decision"`  // "approve" or "block"
    Reason   string `json:"reason"`    // Human-readable explanation
    Context  string `json:"context"`   // Injected into conversation (block only)
}
```

### Session State Schema

```go
type HookSession struct {
    SessionID      string    `json:"session_id"`
    StartedAt      time.Time `json:"started_at"`
    TasksCompleted int       `json:"tasks_completed"`
    TasksStarted   int       `json:"tasks_started"`
    CurrentTaskID  string    `json:"current_task_id,omitempty"`
    PlanID         string    `json:"plan_id,omitempty"`
}
```

---

## Future Enhancements

Planned improvements for the autonomous execution system:

1. **Quality Gates** - Run tests between tasks, block on failure
2. **Parallel Execution** - Spawn multiple sessions for independent tasks
3. **Cost Tracking** - Monitor token usage per session
4. **Compact Between Tasks** - Reduce context pollution on long sessions
5. **LLM-Based Continuation** - Use prompt hooks for intelligent decisions
