/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// slashNextContent is the prompt content for /tw-next
const slashNextContent = `# Start Next TaskWing Task with Full Context

Execute these steps IN ORDER. Do not skip any step.

## Step 1: Get Next Task
Call MCP tool ` + "`task_next`" + ` to retrieve the highest priority pending task:
` + "```json" + `
{"session_id": "claude-session"}
` + "```" + `

Extract from the response:
- task_id, title, description
- scope (e.g., "auth", "vectorsearch", "api")
- keywords array
- acceptance_criteria
- suggested_recall_queries

If no task returned, inform user: "No pending tasks. Use 'taskwing plan list' to check plan status."

## Step 2: Fetch Scope-Relevant Context
Call MCP tool ` + "`recall`" + ` with query based on task scope:
` + "```json" + `
{"query": "[task.scope] patterns constraints decisions"}
` + "```" + `

Examples:
- scope "auth" → ` + "`{\"query\": \"authentication cookies session patterns\"}`" + `
- scope "api" → ` + "`{\"query\": \"api handlers middleware patterns\"}`" + `
- scope "vectorsearch" → ` + "`{\"query\": \"lancedb embedding vector patterns\"}`" + `

Extract: patterns, constraints, related decisions.

## Step 3: Fetch Task-Specific Context
Call MCP tool ` + "`recall`" + ` with keywords from the task.
Use ` + "`suggested_recall_queries`" + ` if available, otherwise extract keywords from title.
` + "```json" + `
{"query": "[keywords from task title/description]"}
` + "```" + `

## Step 4: Claim the Task
Call MCP tool ` + "`task_start`" + `:
` + "```json" + `
{"task_id": "[task_id from step 1]", "session_id": "claude-session"}
` + "```" + `

## Step 5: Present Unified Task Brief

Display in this format:
` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 TASK: [task_id] (Priority: [priority])
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**[Title]**

## Description
[Full task description]

## Acceptance Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]
- [ ] [Criterion 3]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🏗️ ARCHITECTURE CONTEXT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Relevant Patterns
[Patterns from recall that apply to this task]

## Constraints
[Constraints that must be respected]

## Related Decisions
[Past decisions that inform this work]

## Key Files
[Files likely to be modified based on context]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Task claimed. Ready to begin.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Step 6: Begin Implementation
Proceed with the task, following the patterns and respecting the constraints shown above.

**CRITICAL**: You MUST call all MCP tools (task_next, recall x2, task_start) before showing the brief.

## Fallback (No MCP)
` + "```bash" + `
tw task list                    # List all tasks
tw task show TASK_ID            # View task details
tw context -q "search term"     # Get context
` + "```" + `
`

// slashDoneContent is the prompt content for /tw-done
const slashDoneContent = `# Complete Task with Architecture-Aware Summary

Execute these steps IN ORDER.

## Step 1: Get Current Task
Call MCP tool ` + "`task_current`" + `:
` + "```json" + `
{"session_id": "claude-session"}
` + "```" + `

If no active task, inform user and stop.

## Step 2: Generate Completion Report

Create a structured summary covering:

### Files Modified
List all files changed with purpose of change.

### Acceptance Criteria Verification
For each criterion:
- ✅ **Met**: [How it was satisfied]
- ❌ **Not Met**: [Why, and what's needed]
- ⚠️ **Partial**: [What was done, what remains]

### Pattern Compliance
Confirm alignment with codebase patterns.

### Technical Debt / Follow-ups
- TODOs introduced
- Tests not written
- Edge cases not handled

## Step 3: Mark Complete
Call MCP tool ` + "`task_complete`" + `:
` + "```json" + `
{
  "task_id": "[task_id]",
  "summary": "[The structured summary from Step 2]",
  "files_modified": ["path/to/file1.go", "path/to/file2.go"]
}
` + "```" + `

## Step 4: Confirm to User

Display:
` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ TASK COMPLETE: [task_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Summary report]

Recorded in TaskWing memory.
Use /tw-next to continue with next priority task.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Fallback (No MCP)
` + "```bash" + `
tw task complete TASK_ID
` + "```" + `
`

// slashStatusContent is the prompt content for /tw-status
const slashStatusContent = `# Show Current Task Status

## Step 1: Get Current Task
Call MCP tool ` + "`task_current`" + `:
` + "```json" + `
{"session_id": "claude-session"}
` + "```" + `

If no active task:
` + "```" + `
No active task. Use /tw-next to start the next priority task.
` + "```" + `

## Step 2: Display Status

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 CURRENT TASK STATUS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Task: [task_id] - [title]
Priority: [priority]
Status: [status]
Started: [claimed_at timestamp]
Scope: [scope]

## Acceptance Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Commands:
  /tw-done    - Complete this task
  /tw-brief   - Fetch more context
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Fallback (No MCP)
` + "```bash" + `
tw task list --status in_progress
tw plan list
` + "```" + `
`

// slashPlanContent is the prompt content for /tw-plan
const slashPlanContent = `# Create Development Plan with Goal

**Usage:** ` + "`/tw-plan <your goal>`" + `

**Example:** ` + "`/tw-plan Add Stripe billing integration`" + `

## Step 0: Check for Goal

**If $ARGUMENTS is empty or not provided:**
Ask the user: "What do you want to build? Please describe your goal."
Wait for user response, then use that as the goal.

**If $ARGUMENTS is provided:**
Use $ARGUMENTS as the goal and proceed to Step 1.

## Step 1: Initial Clarification

Call MCP tool ` + "`plan_clarify`" + ` with the user's goal:
` + "```json" + `
{"goal": "[goal from Step 0]"}
` + "```" + `

Extract: questions, goal_summary, enriched_goal, is_ready_to_plan, context_used.

## Step 2: Ask Clarifying Questions (Loop)

**If is_ready_to_plan is false:**
Present the questions to the user. Wait for user response.

**If user says "auto":**
Call plan_clarify again with auto_answer: true.

**If user provides answers:**
Format answers as JSON and call plan_clarify again.

Repeat until is_ready_to_plan is true.

## Step 3: Generate Plan

When is_ready_to_plan is true, call MCP tool ` + "`plan_generate`" + `:
` + "```json" + `
{
  "goal": "$ARGUMENTS",
  "enriched_goal": "[enriched_goal from step 2]",
  "save": true
}
` + "```" + `

## Step 4: Present Plan Summary

Display the generated plan:
` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ PLAN CREATED: [plan_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Goal:** [goal]

## Generated Tasks

| # | Title | Priority |
|---|-------|----------|
| 1 | [Task 1 title] | [priority] |
| 2 | [Task 2 title] | [priority] |
...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 Plan saved and set as active.

**Next steps:**
- Run /tw-next to start working on the first task
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Fallback (No MCP)
` + "```bash" + `
tw plan new "Your goal description"
` + "```" + `
`

// slashSimplifyContent is the prompt content for /tw-simplify
const slashSimplifyContent = `# Simplify Code

**Usage:** ` + "`/tw-simplify [file_path or paste code]`" + `

Reduce code complexity while preserving behavior.

## Step 1: Get the Code

**If $ARGUMENTS is a file path:**
Call MCP tool ` + "`code`" + ` with action=simplify:
` + "```json" + `
{"action": "simplify", "file_path": "[file path from arguments]"}
` + "```" + `

**If $ARGUMENTS is code or empty:**
Ask the user to paste the code, then call:
` + "```json" + `
{"action": "simplify", "code": "[pasted code]"}
` + "```" + `

## Step 2: Review Results

The tool returns:
- Simplified code
- Line count reduction (before/after)
- List of changes made with reasoning
- Risk assessment

## Step 3: Present to User

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🧹 CODE SIMPLIFICATION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Simplified Code
[The simplified version]

## Summary
Lines: [before] → [after] (-[reduction]%)
Risk: [risk level]

## Changes Made
- [What was changed and why]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Step 4: Offer to Apply

Ask if the user wants to apply the changes to the file.

## Fallback (No MCP)
` + "```bash" + `
# Manual review recommended
` + "```" + `
`

// slashDebugContent is the prompt content for /tw-debug
const slashDebugContent = `# Debug Issue

**Usage:** ` + "`/tw-debug <problem description>`" + `

**Example:** ` + "`/tw-debug API returns 500 on /users endpoint`" + `

Get systematic debugging help using AI-powered analysis.

## Step 1: Gather Information

**If $ARGUMENTS is empty:**
Ask the user: "What issue are you experiencing? Please describe the problem, and optionally include any error messages or stack traces."
Wait for user response.

**If $ARGUMENTS is provided:**
Use $ARGUMENTS as the problem description.

## Step 2: Call Debug Tool

Call MCP tool ` + "`debug`" + `:
` + "```json" + `
{
  "problem": "[problem description]",
  "error": "[error message if available]",
  "stack_trace": "[stack trace if available]"
}
` + "```" + `

## Step 3: Present Analysis

Display the debug analysis:
` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔍 DEBUG ANALYSIS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Most Likely Cause
[Primary hypothesis]

## Hypotheses (Ranked)
🔴 1. [High likelihood cause]
   [Reasoning]
   📍 Check: [file locations]

🟡 2. [Medium likelihood cause]
   [Reasoning]

🔵 3. [Lower likelihood cause]
   [Reasoning]

## Investigation Steps
1. [First step to try]
   ` + "```" + `
   [command to run]
   ` + "```" + `

2. [Second step]
   ...

## Quick Fixes
- [Quick fix if applicable]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Step 4: Offer to Help

Ask if the user wants help implementing any of the investigation steps or fixes.

## Fallback (No MCP)
` + "```bash" + `
tw context -q "error handling [component]"
` + "```" + `
`

// slashExplainContent is the prompt content for /tw-explain
const slashExplainContent = `# Explain Code Symbol

**Usage:** ` + "`/tw-explain <symbol_name>`" + `

**Example:** ` + "`/tw-explain NewRecallApp`" + `

Get a deep-dive explanation of a code symbol including its purpose, usage patterns, and call graph.

## Step 1: Get the Symbol

**If $ARGUMENTS is empty:**
Ask the user: "What symbol would you like me to explain? (function, type, method, or variable name)"
Wait for user response.

**If $ARGUMENTS is provided:**
Use $ARGUMENTS as the symbol query.

## Step 2: Call Explain Tool

Call MCP tool ` + "`code`" + ` with action=explain:
` + "```json" + `
{"action": "explain", "query": "[symbol name from arguments]"}
` + "```" + `

Optional: Add depth parameter (1-5) for call graph depth:
` + "```json" + `
{"action": "explain", "query": "[symbol]", "depth": 3}
` + "```" + `

## Step 3: Present Explanation

Display the analysis:
` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📖 SYMBOL EXPLANATION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## [Symbol Name]
📍 [file_path:line_number]

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
` + "```" + `

## Step 4: Offer Follow-ups

Suggest related actions:
- Explain a related symbol
- View call graph with /tw-callers
- See impact analysis

## Fallback (No MCP)
` + "```bash" + `
tw explain <symbol_name>
tw context -q "<symbol_name> usage"
` + "```" + `
`

// slashBriefContent is the prompt content for /tw-brief
const slashBriefContent = `# Project Knowledge Brief

Run ` + "`taskwing list`" + ` to get the compact project knowledge inventory.

This outputs all knowledge nodes grouped by type:
- **Decisions**: Architectural choices and rationale
- **Features**: Product capabilities and components
- **Constraints**: Rules and limitations to follow
- **Patterns**: Recurring architectural solutions
- **Documentation**: README, CLAUDE.md, etc.

The output is compact (bullet summaries only, no IDs or file paths) and token-efficient for AI context.

## Usage

` + "```bash" + `
taskwing list
` + "```" + `

Present the output to prime the conversation with project knowledge.
`
