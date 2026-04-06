# ADR: Command Risk Classification for MCP Tools

## Status
Proposed (documentation only - no runtime implementation yet)

## Context
TaskWing exposes MCP tools (ask, task, plan, code, debug, remember) to AI assistants via stdio transport. Currently all tools are available immediately with no risk gating. As TaskWing adds more capable tools (file writes, git operations, node deletion), a classification scheme is needed to prevent destructive actions without explicit user approval.

## Risk Tiers

| Tier | Label | Behavior | Examples |
|---|---|---|---|
| **T0** | Safe | Auto-execute, no confirmation | `ask`, `code` (read-only queries) |
| **T1** | Write | Execute with audit trail | `remember`, `task complete` (writes to local SQLite) |
| **T2** | Risky | Require explicit user confirmation | Future: `delete-node`, `rewrite-file`, `git commit` |
| **T3** | Destructive | Block unless plan-approved + user confirmed | Future: `clear-knowledge`, `git push --force`, `rm -rf` |

## Decision
When destructive tools (T2/T3) are added to the MCP surface:

1. Each MCP tool handler must declare its risk tier
2. The MCP handler chain checks the tier before execution
3. T2 tools prompt for confirmation via the MCP response (tool returns a confirmation request instead of executing)
4. T3 tools require both an active approved plan AND explicit user confirmation
5. The existing OPA policy engine (`internal/policy/`) can evaluate T2/T3 tool calls against project policies

## Gating Rules

T2/T3 tools must satisfy these gates (consistent with the Workflow Contract v1):

- **Plan gate**: A clarified and approved plan must be active
- **Task gate**: The tool call must be relevant to the current in-progress task
- **Evidence gate**: For T3, prior root-cause evidence must exist before destructive action
- **Confirmation gate**: User must explicitly approve (not just "auto" or "skip")

## Current State
- All current MCP tools are T0 (read-only) or T1 (local SQLite writes)
- No T2/T3 tools exist yet
- The OPA policy engine is built but only runs during task completion, not per-tool-call
- When T2/T3 tools are introduced, wire `policy.NewPolicyEvaluatorAdapter()` into the MCP handler chain

## Implementation Notes (for future reference)
- Add a `RiskTier` field to the MCP tool registration in `internal/mcp/handlers.go`
- Check tier in the handler dispatch before calling the tool implementation
- For T2: return a structured confirmation request in the MCP response
- For T3: check `policy.Engine.Evaluate()` with the tool call context
- Log all T1+ tool executions to the session audit trail
