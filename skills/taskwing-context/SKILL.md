---
name: taskwing-context
description: Dump full project knowledge into the conversation for complete architectural context.
---

# Project Context Dump

Inject the complete project knowledge base into this conversation so you have full architectural context.

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

## When to Use

- At the start of a session when you need to understand the project before making changes
- When the user says "what do you know about this project"
- When you need to check constraints before implementing something
- When planning work that touches multiple parts of the codebase

## Important

- This is a READ-ONLY operation. It does not modify the knowledge base.
- If the knowledge base is empty, tell the user to run `tw bootstrap` first.
- Do NOT summarize or filter the results. Show everything so the user can verify.
