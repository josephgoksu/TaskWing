/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// slashNextContent is the prompt content for /tw-next
const slashNextContent = `# Start Next TaskWing Task with Full Context

## TaskWing Workflow Contract v1 (Always On)
1. No implementation before a clarified and approved plan/task checkpoint.
2. No completion claim without fresh verification evidence.
3. No debug fix proposal without root-cause evidence.

If any gate fails, stop and request the missing approval or evidence.

Execute these steps IN ORDER. Do not skip any step.

## Step 1: Get Next Task
Call MCP tool ` + "`task`" + ` with action ` + "`next`" + ` to retrieve the highest-priority pending task:
` + "```json" + `
{"action": "next"}
` + "```" + `

` + "`session_id`" + ` is optional when called through MCP transport; include it only for explicit cross-session orchestration.

Extract from the response:
- task_id, title, description
- scope (e.g., "auth", "vectorsearch", "api")
- keywords array
- acceptance_criteria
- suggested_ask_queries

If no task returned, inform user: "No pending tasks. Use 'taskwing plan list' to check plan status."

## Step 2: Fetch Scope-Relevant Context
Call MCP tool ` + "`ask`" + ` with query based on task scope:
` + "```json" + `
{"query": "[task.scope] patterns constraints decisions"}
` + "```" + `

Examples:
- scope "auth" → ` + "`{\"query\": \"authentication cookies session patterns\"}`" + `
- scope "api" → ` + "`{\"query\": \"api handlers middleware patterns\"}`" + `
- scope "vectorsearch" → ` + "`{\"query\": \"lancedb embedding vector patterns\"}`" + `

Extract: patterns, constraints, related decisions.

## Step 3: Fetch Task-Specific Context
Call MCP tool ` + "`ask`" + ` with keywords from the task.
Use ` + "`suggested_ask_queries`" + ` if available, otherwise extract keywords from title.
` + "```json" + `
{"query": "[keywords from task title/description]"}
` + "```" + `

## Step 4: Claim the Task
Call MCP tool ` + "`task`" + ` with action ` + "`start`" + `:
` + "```json" + `
{"action": "start", "task_id": "[task_id from step 1]"}
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
[Patterns from ask that apply to this task]

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

## Step 6: Implementation Start Gate (Hard Gate)
Before writing or editing code, ask for an explicit checkpoint:
"Implementation checkpoint: proceed with task [task_id] now?"

If approval is missing or unclear, STOP and respond with:
"REFUSAL: I can't start implementation yet. Plan/task checkpoint is incomplete. Please approve this task checkpoint first."

## Step 7: Begin Implementation (Only After Approval)
Proceed with the task, following the patterns and respecting the constraints shown above.

**CRITICAL**: You MUST call all MCP tools (` + "`task(next)`" + `, ` + "`ask`" + ` x2, ` + "`task(start)`" + `) before showing the brief and before requesting implementation approval.

## Fallback (No MCP)
` + "```bash" + `
taskwing task list                    # List all tasks
taskwing task list --status pending   # Identify next pending task
taskwing plan status                  # Check active plan progress
` + "```" + `
`

// slashDoneContent is the prompt content for /tw-done
const slashDoneContent = `# Complete Task with Architecture-Aware Summary

## TaskWing Workflow Contract v1 (Always On)
1. No implementation before a clarified and approved plan/task checkpoint.
2. No completion claim without fresh verification evidence.
3. No debug fix proposal without root-cause evidence.

Execute these steps IN ORDER.

## Step 1: Get Current Task
Call MCP tool ` + "`task`" + ` with action ` + "`current`" + `:
` + "```json" + `
{"action": "current"}
` + "```" + `

If no active task, inform user and stop.

## Step 2: Collect Fresh Verification Evidence
Run the most relevant verification commands for the task (tests, lint, build, or targeted checks).

Document:
- command run
- exit status
- short output snippet proving pass/fail

If verification was not run in this completion attempt, STOP and respond with:
"REFUSAL: I can't mark this task done yet. Verification evidence is missing. Run fresh checks and include the output."

## Step 3: Generate Completion Report

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

## Step 4: Completion Gate (Hard Gate)
Before calling ` + "`task complete`" + `, confirm:
- evidence is fresh (from Step 2)
- acceptance criteria status is explicit
- unresolved failures are called out

If any item is missing, STOP and use the refusal text above.

## Step 5: Mark Complete
Call MCP tool ` + "`task`" + ` with action ` + "`complete`" + `:
` + "```json" + `
{
  "action": "complete",
  "task_id": "[task_id]",
  "summary": "[The structured summary from Step 2]",
  "files_modified": ["path/to/file1.go", "path/to/file2.go"]
}
` + "```" + `

## Step 6: Confirm to User

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
taskwing task complete TASK_ID
` + "```" + `
`

// slashStatusContent is the prompt content for /tw-status
const slashStatusContent = `# Show Current Task Status

This is a read-only status command. Do not use it to bypass plan, verification, or debug gates.

## Step 1: Get Current Task
Call MCP tool ` + "`task`" + ` with action ` + "`current`" + `:
` + "```json" + `
{"action": "current"}
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
  /tw-ask     - Fetch more context
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Fallback (No MCP)
` + "```bash" + `
taskwing task list --status in_progress
taskwing plan list
` + "```" + `
`

// slashPlanContent is the prompt content for /tw-plan
const slashPlanContent = `# Create Development Plan with Goal

**Usage:** ` + "`/tw-plan <your goal>`" + ` or ` + "`/tw-plan --batch <your goal>`" + `

**Example:** ` + "`/tw-plan Add Stripe billing integration`" + `

## TaskWing Workflow Contract v1 (Always On)
1. No implementation before a clarified and approved plan/task checkpoint.
2. No completion claim without fresh verification evidence.
3. No debug fix proposal without root-cause evidence.

Hard gate for this command:
- Do NOT generate, decompose, expand, or finalize a plan until the clarified goal checkpoint is explicitly approved.
- If approval is missing, STOP and respond with:
  "REFUSAL: I can't move past planning yet. Clarification checkpoint is incomplete. Please approve the clarified goal first."

## Mode Selection

The plan tool supports two modes:
- **Interactive (default)**: Staged workflow with checkpoints at phases and tasks
- **Batch (--batch flag)**: Original all-at-once generation

Check if $ARGUMENTS contains "--batch" flag:
- If yes: Use batch mode (Steps 1-4)
- If no: Use interactive mode (Steps 1-8)

---

# BATCH MODE (when --batch is used)

## Step 0: Check for Goal

**If $ARGUMENTS is empty or not provided:**
Ask the user: "What do you want to build? Please describe your goal."
Wait for user response, then use that as the goal.

**If $ARGUMENTS is provided:**
Use $ARGUMENTS as the goal and proceed to Step 1.

## Step 1: Initial Clarification

Call MCP tool ` + "`plan`" + ` with action ` + "`clarify`" + ` and the user's goal:
` + "```json" + `
{"action": "clarify", "goal": "[goal from Step 0]"}
` + "```" + `

Extract: clarify_session_id, questions, goal_summary, enriched_goal, is_ready_to_plan, context_used.

## Step 2: Ask Clarifying Questions (Loop)

**If is_ready_to_plan is false:**
Present the questions to the user. Wait for user response.

**If user says "auto":**
Call ` + "`plan`" + ` again with action ` + "`clarify`" + `, clarify_session_id, and auto_answer: true.

**If user provides answers:**
Format answers as JSON and call ` + "`plan`" + ` again with action ` + "`clarify`" + ` and clarify_session_id:
` + "```json" + `
{
  "action": "clarify",
  "clarify_session_id": "[clarify_session_id from previous clarify step]",
  "answers": [{"question":"...","answer":"..."}]
}
` + "```" + `

Repeat until is_ready_to_plan is true.

## Step 3: Clarification Checkpoint Approval (Hard Gate)
Before generating:
- present enriched_goal and assumptions
- ask for explicit approval ("approve", "yes", or equivalent)

If approval is not explicit, STOP and use the refusal text above.

## Step 4: Generate Plan

When is_ready_to_plan is true, call MCP tool ` + "`plan`" + ` with action ` + "`generate`" + `:
` + "```json" + `
{
  "action": "generate",
  "goal": "$ARGUMENTS",
  "clarify_session_id": "[clarify_session_id from clarify loop]",
  "enriched_goal": "[enriched_goal from step 2]",
  "save": true
}
` + "```" + `

## Step 5: Present Plan Summary

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

---

# INTERACTIVE MODE (default when no --batch flag)

## Step 1: Check for Goal (Same as Batch)

**If $ARGUMENTS is empty or not provided:**
Ask the user: "What do you want to build? Please describe your goal."
Wait for user response, then use that as the goal.

## Step 2: Clarify Goal

Call MCP tool ` + "`plan`" + ` with action=clarify:
` + "```json" + `
{"action": "clarify", "goal": "[goal from Step 1]", "mode": "interactive"}
` + "```" + `

Ask clarifying questions until is_ready_to_plan is true.
Save the clarify_session_id and enriched_goal for subsequent steps.

**CHECKPOINT 1**: User approves the enriched goal before proceeding.
If approval is not explicit, STOP and use the refusal text above.

## Step 3: Decompose into Phases

Call MCP tool ` + "`plan`" + ` with action=decompose:
` + "```json" + `
{
  "action": "decompose",
  "plan_id": "[plan_id from Step 2]",
  "enriched_goal": "[enriched_goal from Step 2]"
}
` + "```" + `

This returns 3-5 high-level phases. Present them to the user:

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 PROPOSED PHASES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Phase 1: [Title]
[Description]
Rationale: [Why this phase is needed]
Expected tasks: [N]

## Phase 2: [Title]
...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

**CHECKPOINT 2**: Ask user to:
- Approve phases as-is
- Request regeneration with feedback
- Skip specific phases

## Step 4: Expand Each Phase (Loop)

For each approved phase, call MCP tool ` + "`plan`" + ` with action=expand:
` + "```json" + `
{
  "action": "expand",
  "plan_id": "[plan_id]",
  "phase_id": "[phase_id]"
}
` + "```" + `

This returns 2-4 detailed tasks for the phase. Present them:

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 TASKS FOR PHASE: [Phase Title]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Task 1: [Title]
Priority: [priority]
Description: [description]
Acceptance Criteria:
- [criterion 1]
- [criterion 2]

## Task 2: [Title]
...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Remaining phases: [N]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

**CHECKPOINT 3** (per phase): Ask user to:
- Approve tasks and continue to next phase
- Request regeneration with feedback
- Skip this phase

Repeat for each phase until all are expanded.

## Step 5: Finalize Plan

After all phases are expanded, call MCP tool ` + "`plan`" + ` with action=finalize:
` + "```json" + `
{
  "action": "finalize",
  "plan_id": "[plan_id]"
}
` + "```" + `

## Step 6: Present Final Summary

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ PLAN FINALIZED: [plan_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Goal:** [goal]

## Phases & Tasks

### Phase 1: [Title]
  1. [Task 1 title] (Priority: [P])
  2. [Task 2 title] (Priority: [P])

### Phase 2: [Title]
  3. [Task 3 title] (Priority: [P])
  4. [Task 4 title] (Priority: [P])

...

**Total:** [N] phases, [M] tasks
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 Plan saved and set as active.

**Next steps:**
- Run /tw-next to start working on the first task
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

---

## Fallback (No MCP)
` + "```bash" + `
taskwing goal "Your goal description"  # Preferred
taskwing plan new "Your goal description"  # Advanced mode
taskwing plan new --non-interactive "Your goal description"  # Headless mode
` + "```" + `
`

// slashSimplifyContent is the prompt content for /tw-simplify
const slashSimplifyContent = `# Simplify Code

**Usage:** ` + "`/tw-simplify [file_path or paste code]`" + `

Reduce code complexity while preserving behavior.
This command is optimization-only and must not bypass planning, verification, or debugging gates.

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
Call MCP tool ` + "`debug`" + ` with the best available evidence:

` + "```json" + `
{
  "problem": "[problem description]",
  "error": "[error message if available]",
  "stack_trace": "[stack trace if available]"
}
` + "```" + `

Do NOT propose fixes yet. First collect and present:
- likely failing component
- top hypotheses
- concrete investigation commands

If the response lacks root-cause evidence (only symptoms), STOP and respond with:
"REFUSAL: I can't propose a fix yet. Root-cause evidence is missing. Run the investigation steps first and share results."

## Phase 3: Present Investigation Plan
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

## Phase 4: Fix Proposal (Only After Evidence Gate Passes)
After Phase 2 evidence is present, propose the smallest safe fix and ask whether to implement it now.

## Step 5: Offer to Help
Ask if the user wants help running investigation steps or implementing the proposed fix.

## Fallback (No MCP)
` + "```bash" + `
taskwing plan status
` + "```" + `
`

// slashExplainContent is the prompt content for /tw-explain
const slashExplainContent = `# Explain Code Symbol

**Usage:** ` + "`/tw-explain <symbol_name>`" + `

**Example:** ` + "`/tw-explain NewAskApp`" + `

Get a deep-dive explanation of a code symbol including its purpose, usage patterns, and call graph.
This is an analysis command and must not be used to bypass planning, verification, or debug gates.

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
- View call graph with MCP tool ` + "`code`" + ` action ` + "`callers`" + `
- See impact analysis

## Fallback (No MCP)
` + "```bash" + `
taskwing mcp
` + "```" + `
`

// slashAskContent is the prompt content for /tw-ask
const slashAskContent = `# Project Knowledge Brief

This is a context-priming command and must not be used to bypass planning, verification, or debug gates.

Call MCP tool ` + "`ask`" + ` to get a compact project knowledge brief.

Use:
` + "```json" + `
{"query":"project decisions patterns constraints", "answer": true}
` + "```" + `

If you need broader coverage, run:
` + "```json" + `
{"all": true}
` + "```" + `

Present the returned summary and top results to prime the conversation with project knowledge.
`

// slashRememberContent is the prompt content for /tw-remember
const slashRememberContent = `# Store Knowledge in Project Memory

This is a persistence command and must not be used to bypass planning, verification, or debug gates.

Call MCP tool ` + "`remember`" + ` to persist a decision, pattern, or insight to project memory.

Use:
` + "```json" + `
{"content": "[the knowledge to store]"}
` + "```" + `

Optionally specify a type (decision, pattern, constraint, note):
` + "```json" + `
{"content": "[the knowledge to store]", "type": "decision"}
` + "```" + `

The content will be classified automatically using AI if no type is provided.
`
