---
name: taskwing-explain
description: Use when you need a deep explanation of a code symbol and its call graph.
argument-hint: "[symbol_name]"
---

# Explain Code Symbol

**Usage:** `/taskwing:explain <symbol_name>`

**Example:** `/taskwing:explain NewAskApp`

Get a deep-dive explanation of a code symbol including its purpose, usage patterns, and call graph.
This is an analysis command and must not be used to bypass planning, verification, or debug gates.

## Step 1: Get the Symbol

**If $ARGUMENTS is empty:**
Ask the user: "What symbol would you like me to explain? (function, type, method, or variable name)"
Wait for user response.

**If $ARGUMENTS is provided:**
Use $ARGUMENTS as the symbol query.

## Step 2: Call Explain Tool

Call MCP tool `code` with action=explain:
```json
{"action": "explain", "query": "[symbol name from arguments]"}
```

Optional: Add depth parameter (1-5) for call graph depth:
```json
{"action": "explain", "query": "[symbol]", "depth": 3}
```

## Step 3: Present Explanation

Display the analysis:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
SYMBOL EXPLANATION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## [Symbol Name]
[file_path:line_number]

## Summary
[One-line description of what this symbol does]

## Detailed Explanation
[Multi-paragraph explanation of purpose, behavior, and implementation details]

## Connections
[Related symbols, dependencies, and how this fits in the codebase]

## Common Pitfalls
[Mistakes to avoid when using or modifying this symbol]

## Usage Examples
[Code examples showing how to use this symbol]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Step 4: Offer Follow-ups

Suggest related actions:
- Explain a related symbol
- View call graph with MCP tool `code` action `callers`
- See impact analysis

## Fallback (No MCP)
```bash
taskwing mcp
```
