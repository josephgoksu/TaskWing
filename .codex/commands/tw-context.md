# Fetch Additional Context for Current Task

Use this when you need more architectural context mid-task.

## Step 1: Get Current Task
Call MCP tool `task_current`:
```json
{"session_id": "claude-session"}
```

Extract the task scope and keywords.

## Step 2: Fetch Requested Context
Call MCP tool `recall`:

If user provided a query argument:
```json
{"query": "[user's query]"}
```

If no query provided, use task scope:
```json
{"query": "[task.scope] patterns constraints decisions"}
```

## Step 3: Display Context

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔍 CONTEXT: [query]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Patterns
[Pattern results from recall]

## Constraints
[Constraint results from recall]

## Decisions
[Decision results from recall]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Usage Examples
- `/tw-context` - Get context for current task scope
- `/tw-context authentication` - Search for auth-related context
- `/tw-context error handling patterns` - Specific search

## Fallback (No MCP)
```bash
tw context -q "search term"
tw context --answer "question about codebase"
```
