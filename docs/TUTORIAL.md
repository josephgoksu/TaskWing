# TaskWing Tutorial

TaskWing helps you move from one goal to executed tasks while keeping architecture context persistent across AI sessions.

## 1. Bootstrap

```bash
cd your-project
taskwing bootstrap
```

This creates `.taskwing/` and installs AI assistant integration files.

## 2. Create and Activate a Plan

```bash
taskwing goal "Add user authentication"
```

`taskwing goal` runs clarify -> generate -> activate in one step.

## 3. Execute with Slash Commands

In your AI tool:

```text
/tw-next
```

When done:

```text
/tw-done
```

Check current status:

```text
/tw-status
```

## 4. Inspect Progress from CLI

```bash
taskwing plan status
taskwing task list
```

## 5. MCP Server

Run MCP server when your AI tool needs stdio MCP integration:

```bash
taskwing mcp
```

## 6. Local Runtime (Optional)

Run TaskWing API/dashboard tooling locally:

```bash
taskwing start
```

Default bind is `127.0.0.1`.

## 7. Troubleshooting

```bash
taskwing doctor
taskwing config show
```

## Command Surface (Focused)

Top-level commands for daily use:

- `taskwing bootstrap`
- `taskwing goal`
- `taskwing plan`
- `taskwing task`
- `taskwing slash`
- `taskwing mcp`
- `taskwing start`
- `taskwing doctor`
- `taskwing config`
- `taskwing version`
