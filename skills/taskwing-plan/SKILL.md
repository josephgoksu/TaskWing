---
name: taskwing-plan
description: Use when you need to clarify a goal and build an approved execution plan.
argument-hint: "[goal description] or [--batch goal description]"
---

# Create Development Plan with Goal

**Usage:** `/taskwing:plan <your goal>` or `/taskwing:plan --batch <your goal>`

**Example:** `/taskwing:plan Add Stripe billing integration`

The Workflow Contract lives in CLAUDE.md (single source of truth). Obey it always.

## Kill Table

| Impulse | Do Instead |
|---------|------------|
| Auto-answer clarifying questions yourself | Present them to the user and WAIT |
| Skip the clarification checkpoint | STOP with refusal text |
| Plan without clarifying first | Always run clarify before generate |
| Assume the user approved | Require explicit "approve", "yes", or equivalent |

## Operating Principles

1. **Goal clarity first.** Never generate a plan from a vague goal. Run clarify until is_ready_to_plan is true.
2. **User approves at every gate.** Checkpoints are hard gates, not suggestions. Missing approval = STOP.
3. **Auto-start after finalize.** Once the plan is approved, immediately call `task(next)` with auto_start=true. Do not wait for the user to say "/taskwing:next".

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

Call MCP tool `plan` with action `clarify` and the user's goal:
```json
{"action": "clarify", "goal": "[goal from Step 0]"}
```

Extract: clarify_session_id, questions, goal_summary, enriched_goal, is_ready_to_plan, context_used.

## Step 2: Ask Clarifying Questions (Loop)

**CRITICAL: Do NOT answer questions yourself. Present them to the user and WAIT.**

**If is_ready_to_plan is false:**
Present each question to the user exactly as returned. The questions include options (e.g., "Option A vs Option B"). Wait for the user to pick or modify.

**If user says "auto" or "skip":**
Call `plan` again with action `clarify`, clarify_session_id, and auto_answer: true.

**If user provides answers:**
Format answers as JSON and call `plan` again with action `clarify` and clarify_session_id:
```json
{
  "action": "clarify",
  "clarify_session_id": "[clarify_session_id from previous clarify step]",
  "answers": [{"question":"...","answer":"..."}]
}
```

Repeat until is_ready_to_plan is true.

## Step 3: Clarification Checkpoint Approval (Hard Gate)
Before generating:
- present enriched_goal and assumptions
- ask for explicit approval ("approve", "yes", or equivalent)

If approval is not explicit, STOP and use the refusal text above.

## Step 4: Generate Plan

When is_ready_to_plan is true, call MCP tool `plan` with action `generate`:
```json
{
  "action": "generate",
  "goal": "$ARGUMENTS",
  "clarify_session_id": "[clarify_session_id from clarify loop]",
  "enriched_goal": "[enriched_goal from step 2]",
  "save": true
}
```

## Step 5: Present Plan Summary

Display the generated plan:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
PLAN CREATED: [plan_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Goal:** [goal]

## Generated Tasks

| # | Title | Priority |
|---|-------|----------|
| 1 | [Task 1 title] | [priority] |
| 2 | [Task 2 title] | [priority] |
...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Plan saved and set as active.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**After presenting the plan summary, immediately start working on the first task.**
Call MCP tool `task` with action=`next` and auto_start=true. Do NOT wait for the user to say "/taskwing:next" -- the plan was just approved, start executing.

---

# INTERACTIVE MODE (default when no --batch flag)

## Step 1: Check for Goal (Same as Batch)

**If $ARGUMENTS is empty or not provided:**
Ask the user: "What do you want to build? Please describe your goal."
Wait for user response, then use that as the goal.

## Step 2: Clarify Goal

Call MCP tool `plan` with action=clarify:
```json
{"action": "clarify", "goal": "[goal from Step 1]", "mode": "interactive"}
```

**CRITICAL: Present all clarifying questions to the user. Do NOT answer them yourself.**
The questions include options the user can pick from. Wait for the user to respond before proceeding.
If user says "auto" or "skip", call clarify with auto_answer: true.

Loop until is_ready_to_plan is true.
Save the clarify_session_id and enriched_goal for subsequent steps.

**CHECKPOINT 1**: User approves the enriched goal before proceeding.
If approval is not explicit, STOP and use the refusal text above.

## Step 3: Decompose into Phases

Call MCP tool `plan` with action=decompose:
```json
{
  "action": "decompose",
  "clarify_session_id": "[clarify_session_id from Step 2]",
  "enriched_goal": "[enriched_goal from Step 2]"
}
```

This returns 3-5 high-level phases. Present them to the user:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
PROPOSED PHASES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Phase 1: [Title]
[Description]
Rationale: [Why this phase is needed]
Expected tasks: [N]

## Phase 2: [Title]
...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**CHECKPOINT 2**: Ask user to:
- Approve phases as-is
- Request regeneration with feedback
- Skip specific phases

## Step 4: Expand Each Phase (Loop)

For each approved phase, call MCP tool `plan` with action=expand:
```json
{
  "action": "expand",
  "plan_id": "[plan_id]",
  "phase_id": "[phase_id]"
}
```

This returns 2-4 detailed tasks for the phase. Present them:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TASKS FOR PHASE: [Phase Title]
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
```

**CHECKPOINT 3** (per phase): Ask user to:
- Approve tasks and continue to next phase
- Request regeneration with feedback
- Skip this phase

Repeat for each phase until all are expanded.

## Step 5: Finalize Plan

After all phases are expanded, call MCP tool `plan` with action=finalize:
```json
{
  "action": "finalize",
  "plan_id": "[plan_id]"
}
```

## Step 6: Present Final Summary

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
PLAN FINALIZED: [plan_id]
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
Plan saved and set as active.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**After presenting the plan summary, immediately start working on the first task.**
Call MCP tool `task` with action=`next` and auto_start=true. Do NOT wait for the user to say "/taskwing:next" -- the plan was just approved, start executing.

---

## Fallback (No MCP)
Use /taskwing:plan in your AI tool to create and manage plans.
