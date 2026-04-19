---
name: taskwing-context
description: Use when you need project knowledge for architectural context. Returns a compact summary by default.
---

# Project Context Dump

Load the project knowledge base into this conversation for architectural context.

## When to Use

- At the start of a session when you need to understand the project before making changes
- When the user says "what do you know about this project"
- When you need to check constraints before implementing something
- When planning work that touches multiple parts of the codebase

## Kill Table

| Impulse | Do Instead |
|---------|------------|
| Summarize or paraphrase the results | Show everything verbatim so the user can verify |
| Filter out "less important" knowledge | Present all nodes. You do not decide relevance for the user. |
| Modify the knowledge base | This is strictly read-only. Use `remember` MCP tool to persist changes. |
| Use this to bypass plan/verification gates | Context priming is not a substitute for workflow checkpoints |

## Operating Principles

1. **Constraints first.** Always present constraints before decisions and patterns. They are mandatory rules.
2. **Decisions second.** Technology and architecture choices frame the project.
3. **Patterns third.** Recurring practices inform how to write code in this project.

## Steps

1. Call MCP tool `ask` with `all=true` to get a compact knowledge summary (titles + one-liners, grouped by type):
```json
{"all": true}
```

2. Present the returned knowledge verbatim. The response is organized by type (constraints, decisions, patterns, features).

3. After presenting, confirm: "Project context loaded. I now have full visibility into your architecture. What would you like to work on?"

## Full Detail Mode

If you need the complete knowledge dump with snippets and evidence (for deep investigation), use paginated full detail:
```json
{"all": true, "detail": "full", "page": 1}
```
Continue with `page=2`, `page=3`, etc. until all pages are retrieved.

For scoped full detail on a specific topic:
```json
{"query": "auth", "detail": "full"}
```

## Important

- This is a READ-ONLY operation. It does not modify the knowledge base.
- If the knowledge base is empty, tell the user to run `taskwing bootstrap` first.
- Do NOT summarize or filter the results. Show everything so the user can verify.
