# Linked Task List Feature

## Overview

Introduce an explicit doubly linked list across active tasks so TaskWing can express a canonical work sequence. Each task will track its immediate predecessor and successor, allowing both humans and AI agents to navigate work in focused order while still respecting the existing tree/dependency model.

## Goals

- Give AI tooling and CLI users first-class context about "what just happened" and "what comes next" within a lane.
- Support quick reordering without rewriting dependency graphs; linked list order should be lightweight and orthogonal to status/priority.
- Provide guardrails that keep the list coherent as tasks are added, completed, archived, or moved.

## Data Model Additions

- Extend `models.Task` with optional `PrevID` and `NextID` fields (UUID references, validated to avoid self loops).
- Maintain list integrity in the store: updates that set `PrevID`/`NextID` must atomically patch both adjacent tasks.
- Persist list metadata to archives so restored tasks can rejoin their previous position when possible.

## CLI & UX Touchpoints

- `add`/`plan` commands gain `--after` and `--before` flags to splice new tasks into the list.
- `taskwing next` and `taskwing prev` surface neighbor navigation.
- `list --chain` renders the linked order, highlighting breaks or orphaned tasks.

## MCP & Agent Enhancements

- New MCP tool `linked-list-navigate` returning current, prev, next, and optional upcoming window for an active task.
- Responses from existing tools (`get-current-task`, `task-summary`) embed linked list hints so agents can maintain conversational flow.
- Workflow health checks flag gaps (e.g., a task pointing to a completed neighbor) and propose repairs.

## Integrity & Edge Cases

- Recompute pointers when tasks are archived, deleted, or bulk-moved; fall back to nearest surviving neighbor.
- Prevent circular references through validation in both CLI and MCP surfaces.
- Provide migration utility to bootstrap the list from current priority ordering as an initial heuristic.

## Open Questions

- Should multiple parallel chains be supported (per status, per epic), or do we enforce a single canonical list?
- Do subtasks participate in the same chain as parents, or maintain isolated chains per level?
- How do we expose conflict resolution when concurrent agents attempt to reorder simultaneously?
