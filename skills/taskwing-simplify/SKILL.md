---
name: taskwing-simplify
description: Use when you want to simplify code while preserving behavior.
argument-hint: "[file_path or paste code]"
---

# Simplify Code

**Usage:** `/taskwing:simplify [file_path or paste code]`

Reduce code complexity while preserving behavior.
This command is optimization-only and must not bypass planning, verification, or debugging gates.

## Step 1: Get the Code

**If $ARGUMENTS is a file path:**
Call MCP tool `code` with action=simplify:
```json
{"action": "simplify", "file_path": "[file path from arguments]"}
```

**If $ARGUMENTS is code or empty:**
Ask the user to paste the code, then call:
```json
{"action": "simplify", "code": "[pasted code]"}
```

## Step 2: Review Results

The tool returns:
- Simplified code
- Line count reduction (before/after)
- List of changes made with reasoning
- Risk assessment

## Step 3: Present to User

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
CODE SIMPLIFICATION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Simplified Code
[The simplified version]

## Summary
Lines: [before] -> [after] (-[reduction]%)
Risk: [risk level]

## Changes Made
- [What was changed and why]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Step 4: Offer to Apply

Ask if the user wants to apply the changes to the file.

## Fallback (No MCP)
```bash
# Manual review recommended
```
