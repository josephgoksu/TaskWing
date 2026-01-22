# TaskWing CLI Tutorial

> Give your AI coding assistant permanent memory and autonomous task execution.

TaskWing extracts architectural knowledge from your codebase and exposes it to AI tools (Claude Code, Codex, Gemini, OpenCode) via MCP. It also enables autonomous task execution through plans and hooks.

---

## Quick Start (2 minutes)

```bash
# 1. Install
brew install josephgoksu/tap/taskwing

# 2. Bootstrap your project
cd your-project
taskwing bootstrap

# 3. Follow the prompts to:
#    - Select your AI tool (Claude, Codex, Gemini, OpenCode)
#    - Configure MCP integration
```

That's it. TaskWing will analyze your codebase and configure your AI tool.

---

## Understanding TaskWing

### Core Concepts

| Concept | What It Does |
|---------|--------------|
| **Bootstrap** | Scans your codebase and extracts patterns, decisions, constraints |
| **Memory** | SQLite database storing architectural knowledge |
| **MCP Server** | Exposes `recall` tool so AI can query your architecture |
| **Plans** | High-level goals broken into prioritized tasks |
| **Hooks** | Auto-continue to next task when one completes |

### The Workflow

```
┌─────────────────────────────────────────────────────────────┐
│  1. BOOTSTRAP                                               │
│     taskwing bootstrap                                      │
│     → Scans codebase, extracts knowledge                    │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  2. CREATE PLAN                                             │
│     taskwing plan new "Add user authentication"             │
│     → AI generates tasks with priorities                    │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  3. START PLAN                                              │
│     taskwing plan start latest                              │
│     → Activates the plan for execution                      │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  4. WORK ON TASKS                                           │
│     Open your AI tool → Run /tw-next                        │
│     → AI gets task + architecture context                   │
│     → Implements task                                       │
│     → Marks complete → Auto-continues to next               │
└─────────────────────────────────────────────────────────────┘
```

---

## Step-by-Step Guide

### Step 1: Initialize Your Project

```bash
cd your-project
taskwing bootstrap
```

You'll be prompted to select your AI tools:
```
🤖 Which AI assistant(s) do you use?

  [✓] claude     - Claude Code
  [ ] cursor     - Cursor
  [ ] copilot    - GitHub Copilot
  [✓] gemini     - Gemini CLI
  [✓] codex      - OpenAI Codex
  [ ] opencode   - OpenCode
```

This creates:
- `.taskwing/` - Memory database and plans
- `.claude/commands/` - Slash commands (if Claude selected)
- `.codex/commands/` - Slash commands (if Codex selected)
- `.gemini/commands/` - Slash commands (if Gemini selected)
- `.opencode/skills/` - Skills (if OpenCode selected)
- `opencode.json` - MCP config at project root (if OpenCode selected)
- MCP server configuration for each tool
- Hooks for autonomous execution (Claude, Codex, OpenCode)

### Step 2: Verify Setup

```bash
taskwing doctor
```

Output:
```
🩺 TaskWing Doctor
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ Initialization: .taskwing/ directory exists
✅ MCP (Claude): taskwing-mcp registered
✅ Hooks (Claude): Configured (SessionStart, Stop, SessionEnd)
⚠️  Active Plan: No active plan
   └─ Run: taskwing plan new "your goal"
```

### Step 3: Create a Plan

```bash
taskwing plan new "Add user authentication with JWT"
```

The AI analyzes your codebase and generates tasks:
```
Plan: plan-1767481570 | 6 tasks

## Task: Set up JWT middleware
Priority: 100 | Agent: coder
...

## Task: Create login endpoint
Priority: 90 | Agent: coder
...
```

### Step 4: Start the Plan

```bash
taskwing plan start latest
```

### Step 5: Open Your AI Tool

**For Claude Code:**
```bash
claude
```

**For OpenAI Codex:**
```bash
codex
```

**For Gemini CLI:**
```bash
gemini
```

### Step 6: Start Working

In your AI tool, run:
```
/tw-next
```

The AI will:
1. Fetch the next task
2. Query architecture context via MCP
3. Claim the task
4. Show you the task brief
5. Begin implementation

### Step 7: Tasks Auto-Continue

When the AI completes a task and calls `task_complete`, the **Stop hook** fires and automatically injects the next task. This continues until:

- All tasks complete
- Circuit breaker triggers (default: 5 tasks or 30 minutes)
- A task is blocked

---

## AI Tool Configuration

### Claude Code

**Hooks**: ✅ Supported (auto-continue works)

**Setup:**
```bash
taskwing mcp install claude
```

**Slash Commands:**
- `/tw-next` - Start next task
- `/tw-done` - Complete current task
- `/tw-block` - Mark task as blocked
- `/tw-status` - Show current task
- `/tw-context` - Fetch architecture context
- `/tw-brief` - Get project knowledge brief

**Configuration:**
```bash
taskwing config set hooks.max-tasks 10      # More tasks before pause
taskwing config set hooks.max-minutes 60    # Longer session duration
```

---

### OpenAI Codex

**Hooks**: ✅ Supported (auto-continue works)

**Setup:**
```bash
taskwing mcp install codex
```

**Slash Commands:** Same as Claude Code (`/tw-next`, `/tw-done`, etc.)

**Configuration:** Same as Claude Code

---

### Gemini CLI

**Hooks**: ❌ Not currently supported

Gemini works with TaskWing but requires manual task continuation.

**Setup:**
```bash
taskwing mcp install gemini
```

**Workflow (Manual):**
```
/tw-next          # Get and start task
# ... work on task ...
/tw-done          # Complete task
/tw-next          # Manually start next task
```

---

### OpenCode

**Hooks**: ✅ Supported via plugins (auto-continue works)
**Skills**: ✅ Custom slash commands via `.opencode/skills/`
**MCP**: ✅ Supported via `opencode.json`

**Setup:**
```bash
taskwing bootstrap  # Select 'opencode' when prompted
# Or install MCP separately:
taskwing mcp install opencode
```

This creates:
- `opencode.json` - MCP server configuration **at project root** (required location)
- `.opencode/skills/` - TaskWing slash commands (tw-next, tw-done, etc.)
- `.opencode/plugins/taskwing-hooks.js` - Hooks for auto-continue

**opencode.json Example:**

The `opencode.json` file **must live at the repository root**. It configures the MCP server:
```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "taskwing-mcp": {
      "type": "local",
      "command": ["taskwing", "mcp"],
      "timeout": 5000
    }
  }
}
```

**Skills (Slash Commands):**
```
/tw-next     - Start next task
/tw-done     - Complete current task
/tw-brief    - Get project knowledge brief
/tw-status   - Show current task status
/tw-context  - Fetch architecture context
/tw-block    - Mark current task as blocked
```

**Skill Structure:**

Skills live in `.opencode/skills/<skill-name>/SKILL.md`. The **directory name must match** the `name` field in the YAML frontmatter:
```yaml
---
name: tw-brief
description: Get compact project knowledge brief (decisions, patterns, constraints)
---

!taskwing slash brief
```

Valid skill names follow the pattern: `^[a-z0-9]+(-[a-z0-9]+)*$` (lowercase, hyphens allowed)

**Plugin Structure:**

Plugins live in `.opencode/plugins/` and use JavaScript with Bun's shell API (`ctx.$`):
```javascript
// .opencode/plugins/taskwing-hooks.js
export default async (ctx) => ({
  // session.created: Called when a new session starts
  "session.created": async (event) => {
    await ctx.$`taskwing hook session-init`;
  },

  // session.idle: Called when task completes (auto-continue)
  "session.idle": async (event) => {
    await ctx.$`taskwing hook continue-check --max-tasks=5 --max-minutes=30`;
  }
});
```

**Doctor Checks:**

Run `taskwing doctor` to verify your OpenCode installation:
```bash
taskwing doctor
# ✅ MCP (OpenCode): taskwing-mcp registered in opencode.json
# ✅ Skills (OpenCode): 6 skills validated
```

**Integration Testing:**
```bash
# Run OpenCode integration tests
make test-opencode

# Or directly:
go test -v ./tests/integration/... -run "TestOpenCode"
```

**Development Notes:**

> ⚠️ **CRITICAL**: When developing or testing TaskWing code changes, you **MUST use `taskwing-local-dev-mcp`** instead of the production MCP. The production `taskwing-mcp` uses the Homebrew-installed binary, which won't reflect your code changes.

```bash
# Development workflow:
# 1. Make code changes
# 2. Build: make build
# 3. Test via local dev MCP (uses ./bin/taskwing)
# 4. Run tests: make test-opencode
```

For OpenCode development specifically:
- During development, configure `taskwing-local-dev-mcp` in your opencode.json
- The production MCP (`taskwing-mcp`) uses the Homebrew-installed binary
- Changes to code require rebuild: `make build`

---

### Cursor / GitHub Copilot

**Hooks**: ❌ Not supported
**MCP**: ✅ Supported

These tools can use TaskWing's `recall` MCP tool to query architecture, but don't support autonomous task execution.

**Setup:**
```bash
taskwing mcp install cursor
taskwing mcp install copilot
```

---

## Command Reference

### Core Commands

| Command | Description |
|---------|-------------|
| `taskwing bootstrap` | Initialize project, scan codebase |
| `taskwing doctor` | Diagnose setup issues |
| `taskwing work` | Unified entry point (bootstrap + plan + session) |

### Plan Commands

| Command | Description |
|---------|-------------|
| `taskwing plan new "goal"` | Create a new plan |
| `taskwing plan list` | List all plans |
| `taskwing plan start <id>` | Activate a plan |
| `taskwing plan status` | Show current plan progress |

### Task Commands

| Command | Description |
|---------|-------------|
| `taskwing task list` | List tasks in active plan |
| `taskwing task show <id>` | Show task details |

### Context Commands

| Command | Description |
|---------|-------------|
| `taskwing context` | Show architecture overview |
| `taskwing context -q "auth"` | Search for specific context |

### Config Commands

| Command | Description |
|---------|-------------|
| `taskwing config show` | Show current configuration |
| `taskwing config set hooks.max-tasks 10` | Set max tasks per session |
| `taskwing config set hooks.max-minutes 60` | Set max session duration |
| `taskwing config set hooks.enabled false` | Disable auto-continue |

### Hook Commands (Advanced)

| Command | Description |
|---------|-------------|
| `taskwing hook session-init` | Initialize session (called by SessionStart hook) |
| `taskwing hook continue-check` | Check if should continue (called by Stop hook) |
| `taskwing hook session-end` | Cleanup session (called by SessionEnd hook) |
| `taskwing hook status` | Show current session state |

---

## Troubleshooting

### "No active session"

The session initializes when you open your AI tool. If using manual mode:
```bash
taskwing hook session-init
```

### "Hooks not firing"

1. Check hooks are configured: `taskwing doctor`
2. Restart your AI tool after bootstrap
3. Verify with `/hooks` command in Claude/Codex

### "MCP server not found"

```bash
taskwing mcp install claude  # or codex, gemini, cursor
```

Then restart your AI tool.

### "Tasks not auto-continuing"

Only Claude Code and Codex support hooks. For Gemini/Cursor/Copilot, manually run `/tw-next` after each task.

---

## Examples

### Example 1: Quick Feature Development

```bash
# One-liner to start working
taskwing work --plan "Add dark mode toggle"

# Opens Claude Code, run:
/tw-next
```

### Example 2: Extended Session

```bash
# Increase limits for longer work
taskwing config set hooks.max-tasks 20
taskwing config set hooks.max-minutes 120

# Start working
taskwing work --launch
```

### Example 3: Using with Gemini (Manual Mode)

```bash
taskwing bootstrap          # Select gemini
taskwing plan new "Refactor API handlers"
taskwing plan start latest

gemini                      # Open Gemini CLI
/tw-next                    # Start first task
# ... complete task ...
/tw-done                    # Mark complete
/tw-next                    # Start next task (manual)
```

---

## Architecture

```
.taskwing/
├── memory/
│   ├── memory.db           # SQLite database (source of truth)
│   ├── hook_session.json   # Session state for hooks
│   └── index.json          # Search index cache
├── plans/
│   └── *.md                # Plan markdown files
└── logs/
    └── *.jsonl             # Trace logs

.claude/                    # Claude Code config
├── settings.json           # Hooks configuration
└── commands/
    ├── tw-next.md
    ├── tw-done.md
    └── ...

.codex/                     # OpenAI Codex config (same structure)
.gemini/                    # Gemini CLI config
```

---

## Getting Help

```bash
taskwing --help             # All commands
taskwing <command> --help   # Command-specific help
taskwing doctor             # Diagnose issues
```

Report issues: https://github.com/josephgoksu/TaskWing/issues
