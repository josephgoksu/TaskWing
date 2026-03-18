---
name: taskwing-context
description: Use when you need the full project knowledge dump for complete architectural context.
---

# Project Context Dump

Inject the complete project knowledge base into this conversation so you have full architectural context.

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
4. **Completeness over brevity.** Do not omit nodes. The user needs the full picture.

## Steps

1. Call MCP tool `ask` with a broad query to retrieve all knowledge:
```json
{"query": "project decisions patterns constraints features", "all": true}
```

2. Present the returned knowledge organized by type:
   - **Constraints** first (mandatory rules)
   - **Decisions** (technology and architecture choices)
   - **Patterns** (recurring practices)
   - **Features** (product capabilities)

3. After presenting, confirm: "Project context loaded. I now have full visibility into your architecture. What would you like to work on?"

## Important

- This is a READ-ONLY operation. It does not modify the knowledge base.
- If the knowledge base is empty, tell the user to run `tw bootstrap` first.
- Do NOT summarize or filter the results. Show everything so the user can verify.
