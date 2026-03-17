---
name: taskwing-debug
description: Use when an issue requires root-cause-first debugging before proposing fixes.
argument-hint: "[problem description]"
---

# Debug Issue

**Usage:** `/taskwing:debug <problem description>`

**Example:** `/taskwing:debug API returns 500 on /users endpoint`

## TaskWing Workflow Contract v1 (Always On)
1. No implementation before a clarified and approved plan/task checkpoint.
2. No completion claim without fresh verification evidence.
3. No debug fix proposal without root-cause evidence.

## Phase 1: Capture Problem Statement

**If $ARGUMENTS is empty:**
Ask the user: "What issue are you experiencing? Please describe the problem, and optionally include any error messages or stack traces."
Wait for user response.

**If $ARGUMENTS is provided:**
Use $ARGUMENTS as the problem description.

## Phase 2: Root-Cause Evidence Collection (Hard Gate)
Call MCP tool `debug` with the best available evidence:

```json
{
  "problem": "[problem description]",
  "error": "[error message if available]",
  "stack_trace": "[stack trace if available]"
}
```

Do NOT propose fixes yet. First collect and present:
- likely failing component
- top hypotheses
- concrete investigation commands

If the response lacks root-cause evidence (only symptoms), STOP and respond with:
"REFUSAL: I can't propose a fix yet. Root-cause evidence is missing. Run the investigation steps first and share results."

## Phase 3: Present Investigation Plan
Display the debug analysis:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
DEBUG ANALYSIS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Most Likely Cause
[Primary hypothesis]

## Hypotheses (Ranked)
1. [High likelihood cause]
   [Reasoning]
   Check: [file locations]

2. [Medium likelihood cause]
   [Reasoning]

3. [Lower likelihood cause]
   [Reasoning]

## Investigation Steps
1. [First step to try]
   ```
   [command to run]
   ```

2. [Second step]
   ...

## Quick Fixes
- [Quick fix if applicable]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Phase 4: Fix Proposal (Only After Evidence Gate Passes)
After Phase 2 evidence is present, propose the smallest safe fix and ask whether to implement it now.

## Step 5: Offer to Help
Ask if the user wants help running investigation steps or implementing the proposed fix.

## Fallback (No MCP)
```bash
taskwing plan status
```
