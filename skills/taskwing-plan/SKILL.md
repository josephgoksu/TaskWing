---
name: taskwing-plan
description: Use when you need to clarify a goal and build an approved execution plan.
argument-hint: "[goal description] or [--batch goal description]"
---

# Create Development Plan with Goal

**Usage:** `/taskwing:plan <your goal>` or `/taskwing:plan --batch <your goal>`

**Example:** `/taskwing:plan Add Stripe billing integration`

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

Call MCP tool `plan` with action `clarify` and the user's goal:
```json
{"action": "clarify", "goal": "[goal from Step 0]"}
```

Extract: clarify_session_id, questions, goal_summary, enriched_goal, is_ready_to_plan, context_used.

## Step 2: Ask Clarifying Questions (Loop)

**If is_ready_to_plan is false:**
Present the questions to the user. Wait for user response.

**If user says "auto":**
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

**Next steps:**
- Run /taskwing:next to start working on the first task
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

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

Ask clarifying questions until is_ready_to_plan is true.
Save the clarify_session_id and enriched_goal for subsequent steps.

**CHECKPOINT 1**: User approves the enriched goal before proceeding.
If approval is not explicit, STOP and use the refusal text above.

## Step 3: Decompose into Phases

Call MCP tool `plan` with action=decompose:
```json
{
  "action": "decompose",
  "plan_id": "[plan_id from Step 2]",
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

**Next steps:**
- Run /taskwing:next to start working on the first task
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

## Fallback (No MCP)
```bash
taskwing goal "Your goal description"  # Preferred
taskwing plan new "Your goal description"  # Advanced mode
taskwing plan new --non-interactive "Your goal description"  # Headless mode
```
