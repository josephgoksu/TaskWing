# TaskWing Workflow Contract v1

This contract defines non-negotiable behavior gates for TaskWing-guided execution.

## Scope

Applies to slash flows and hook-driven sessions:
- `/tw-plan`
- `/tw-next`
- `/tw-done`
- `/tw-debug`

## Gate 1: Plan/Task Checkpoint Before Implementation

Rule:
- Do not start implementation before a clarified and approved plan/task checkpoint.

Refusal language:
- `REFUSAL: I can't start implementation yet. Plan/task checkpoint is incomplete. Please approve this task checkpoint first.`
- `REFUSAL: I can't move past planning yet. Clarification checkpoint is incomplete. Please approve the clarified goal first.`

KPI:
- `% of implementation starts with explicit checkpoint approval in the same session`

## Gate 2: Verification Evidence Before Completion

Rule:
- Do not mark a task complete without fresh verification evidence from the current completion attempt.

Refusal language:
- `REFUSAL: I can't mark this task done yet. Verification evidence is missing. Run fresh checks and include the output.`

KPI:
- `% of completed tasks with at least one fresh verification command + output snippet`

## Gate 3: Root-Cause Evidence Before Debug Fix Proposals

Rule:
- Do not propose fixes in debug flows before root-cause evidence is collected.

Refusal language:
- `REFUSAL: I can't propose a fix yet. Root-cause evidence is missing. Run the investigation steps first and share results.`

KPI:
- `% of debug sessions where a fix proposal is preceded by evidence-backed root-cause analysis`

## Operating Policy

- These gates are hard blockers for core workflow commands.
- Commands that are primarily read-only (`/tw-ask`, `/tw-status`, `/tw-explain`, `/tw-simplify`) remain lightweight but must not bypass these gates.
- Prompt regressions against this contract are release blockers.
