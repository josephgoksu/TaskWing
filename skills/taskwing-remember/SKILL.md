---
name: taskwing-remember
description: Use when you want to persist a decision, pattern, or insight to project memory.
---

# Store Knowledge in Project Memory

This is a persistence command and must not be used to bypass planning, verification, or debug gates.

Call MCP tool `remember` to persist a decision, pattern, or insight to project memory.

Use:
```json
{"content": "[the knowledge to store]"}
```

Optionally specify a type (decision, pattern, constraint, note):
```json
{"content": "[the knowledge to store]", "type": "decision"}
```

The content will be classified automatically using AI if no type is provided.
