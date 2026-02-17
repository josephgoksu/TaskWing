# TaskWing Workflow Pack (Codex, Claude, OpenCode)

Workflow Pack defines a consistent activation path for assistants without platform rewrites.

## Goal

Get users to one visible success loop in under 15 minutes:
1. Clarify and approve a plan.
2. Start one scoped task with context.
3. Complete with verification evidence.

## Distribution Defaults

- Install slash commands via bootstrap for all supported assistants.
- Install session hooks/plugins where supported (`taskwing hook session-init`, `continue-check`, `session-end`).
- Keep MCP setup canonical (`taskwing mcp install <assistant>`).

## First-Run Activation Path

1. `taskwing bootstrap`
2. `taskwing goal "<goal>"`
3. `/tw-next`
4. Implement scoped change
5. `/tw-done`

Expected first-run success signal:
- One completed task with explicit verification evidence.

## Platform Notes

- Codex/Claude: command files + hooks from TaskWing bootstrap.
- OpenCode: command files + `.opencode/plugins/taskwing-hooks.js`.
- Copilot/Cursor/Gemini: command generation + local MCP config path.

## Reliability Requirements

- Prompt contracts in core commands are hard gates.
- Trigger-focused command descriptions (`Use when ...`) are required.
- Cross-assistant command description parity must remain true.

