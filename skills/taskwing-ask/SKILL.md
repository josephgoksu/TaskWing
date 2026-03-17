---
name: taskwing-ask
description: Use when you need to search project knowledge (decisions, patterns, constraints).
---

# Project Knowledge Brief

This is a context-priming command and must not be used to bypass planning, verification, or debug gates.

Call MCP tool `ask` to get a compact project knowledge brief.

Use:
```json
{"query":"project decisions patterns constraints", "answer": true}
```

If you need broader coverage, run:
```json
{"all": true}
```

Present the returned summary and top results to prime the conversation with project knowledge.
