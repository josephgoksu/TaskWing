# Policy Enforcement

> How TaskWing enforces architectural constraints during AI-assisted development

This document covers the policy enforcement lifecycle: from defining constraints to validating them in dry-run mode and enforcing them during task completion.

---

## Enforcement Flow

```
Define Policy → Test (Dry-Run) → Task Completion → Enforcement
     ↓                ↓                 ↓
  .rego file    `policy test`    Auto-validated
```

1. **Define**: Write Rego policies in `.taskwing/policies/`
2. **Test**: Validate with `policy test` (dry-run mode)
3. **Enforce**: Automatic validation during `task complete`

---

## Dry-Run vs Enforcement Modes

TaskWing supports two modes for policy validation:

| Mode | Command | Use Case | Database Writes |
|------|---------|----------|-----------------|
| **Dry-Run** | `policy test [files]` | Testing hypothetical changes | None |
| **Enforcement** | `task complete` | Production validation | Policy decision logged |

### Dry-Run Mode

Test what would happen if you modified certain files, before actually modifying them:

```bash
# Test hypothetical file modifications
taskwing policy test .env secrets/api-key.json

# Output:
# Policy Dry-Run Validation
# =========================
# Checking 2 hypothetical file(s) against 1 policy file(s)...
#
# Files to validate:
#   • .env
#   • secrets/api-key.json
#
# ✗ Policy violations detected:
#   BLOCKED: Environment file '.env' is protected
#   BLOCKED: Secrets file 'secrets/api-key.json' is protected
```

**Key features:**
- Files don't need to exist (test hypothetical paths)
- No database writes occur
- Returns non-zero exit code on violations
- Supports `--json` for programmatic use

### Enforcement Mode

During task completion, policies are automatically evaluated:

```bash
# When completing a task, files are validated
taskwing task complete --summary "Added login endpoint" \
    --files-modified internal/auth/login.go,.env

# If .env is in files_modified, completion is blocked:
# ❌ Task Completion Blocked
#
# Policy violations detected:
#   BLOCKED: Environment file '.env' is protected
#
# Task remains in_progress status.
```

---

## Defining Architectural Constraints

### Step 1: Initialize Policies

```bash
taskwing policy init
```

Creates `.taskwing/policies/default.rego` with sensible defaults.

### Step 2: Write Custom Constraints

Create `.taskwing/policies/architecture.rego`:

```rego
package taskwing.policy

import rego.v1

# === LAYER BOUNDARIES ===
# Controllers cannot directly import database package

deny contains msg if {
    some file in input.task.files_modified
    contains(file, "/controller/")
    imports := taskwing.file_imports(file)
    some imp in imports
    contains(imp, "database")
    msg := sprintf("ARCHITECTURAL VIOLATION: Controller '%s' imports database directly. Use service layer.", [file])
}

# === PROTECTED DIRECTORIES ===
# Vendor and generated code are read-only

deny contains msg if {
    some file in input.task.files_modified
    startswith(file, "vendor/")
    msg := sprintf("BLOCKED: Vendor directory '%s' is read-only", [file])
}

deny contains msg if {
    some file in input.task.files_modified
    startswith(file, "generated/")
    msg := sprintf("BLOCKED: Generated code '%s' should not be manually modified", [file])
}

# === FILE SIZE LIMITS ===
# Warn on large files (advisory, doesn't block)

warn contains msg if {
    some file in input.task.files_modified
    lines := taskwing.file_line_count(file)
    lines > 500
    msg := sprintf("WARNING: Large file '%s' (%d lines) - consider refactoring", [file, lines])
}
```

### Step 3: Test with Dry-Run

Validate your constraints before using them:

```bash
# Test valid file - should pass
taskwing policy test internal/service/user.go

# Test invalid file - should fail
taskwing policy test vendor/github.com/pkg/errors/errors.go

# Test multiple files at once
taskwing policy test .env secrets/key.pem vendor/lib.go
```

### Step 4: Add Unit Tests

Create `.taskwing/policies/architecture_test.rego`:

```rego
package taskwing.policy

import rego.v1

# Test: vendor files are blocked
test_deny_vendor_modification if {
    deny with input as {"task": {"files_modified": ["vendor/github.com/pkg/errors/errors.go"]}}
}

# Test: service files are allowed
test_allow_service_file if {
    not deny with input as {"task": {"files_modified": ["internal/service/user.go"]}}
}

# Test: controller importing database is blocked
test_deny_controller_database_import if {
    deny with input as {
        "task": {"files_modified": ["internal/controller/user.go"]}
    }
}
```

Run unit tests:

```bash
taskwing policy test  # Runs *_test.rego files
```

---

## Valid vs Invalid Policy Examples

### Valid Policy Definition

```rego
package taskwing.policy

import rego.v1

# Good: Clear, specific rule with helpful message
deny contains msg if {
    some file in input.task.files_modified
    startswith(file, "core/")
    msg := sprintf("BLOCKED: Core module '%s' requires senior review. Contact #platform-team.", [file])
}
```

### Invalid Policy Definition

```rego
# BAD: Missing package declaration
deny if {
    input.task.files_modified[_] == ".env"
}

# BAD: No message returned - won't show why it blocked
deny if {
    some file in input.task.files_modified
    startswith(file, ".env")
    # Missing: msg := "..."
}

# BAD: Using deprecated Rego syntax
deny[msg] {  # Old syntax - use "deny contains msg if" instead
    msg := "something"
}
```

### Common Pitfalls

| Issue | Problem | Solution |
|-------|---------|----------|
| No `import rego.v1` | Uses deprecated Rego syntax | Add `import rego.v1` after package |
| Missing package | Policy won't load | Add `package taskwing.policy` |
| No message | Silent blocks confuse users | Always set `msg := sprintf(...)` |
| Glob patterns | May not match root files | Use helper functions like `startswith()` |

---

## Integration with Task Lifecycle

### Task Start

No policy enforcement - allows exploration.

### Task In Progress

AI can call `policy check` via MCP to pre-validate:

```json
{
  "action": "check",
  "files": ["internal/auth/login.go", ".env"],
  "task_id": "task-123"
}
```

### Task Complete

**Automatic enforcement occurs:**

1. `TaskApp.Complete()` is called
2. `PolicyEnforcer.Enforce()` evaluates files_modified
3. If violations detected:
   - Task remains `in_progress`
   - Error returned with violation messages
4. If allowed:
   - Task marked `completed`
   - Policy decision logged to SQLite

```go
// internal/app/task.go - enforcement flow
func (t *TaskApp) Complete(ctx context.Context, req CompleteRequest) (*CompleteResult, error) {
    // ... validate request ...

    // Policy enforcement
    if t.policyEnforcer != nil && len(req.FilesModified) > 0 {
        result := t.policyEnforcer.Enforce(ctx, task, planGoal)
        if !result.Allowed {
            return nil, fmt.Errorf("policy violations: %v", result.Violations)
        }
    }

    // ... complete task ...
}
```

---

## CLI Commands Reference

### `policy init`

Create default policy file:

```bash
taskwing policy init
```

### `policy list`

List loaded policies:

```bash
taskwing policy list
taskwing policy list --json
```

### `policy check`

Check existing files against policies:

```bash
# Check specific files
taskwing policy check main.go internal/auth/login.go

# Check staged git files
taskwing policy check --staged

# JSON output
taskwing policy check main.go --json
```

### `policy test`

Two modes of operation:

```bash
# Mode 1: Run OPA unit tests (*_test.rego files)
taskwing policy test

# Mode 2: Dry-run validation of hypothetical files
taskwing policy test .env secrets/key.pem

# With context
taskwing policy test --task-id task-123 --plan-id plan-456 internal/auth.go

# JSON output
taskwing policy test --json .env
```

**Exit Codes:**

| Code | Meaning |
|------|---------|
| 0 | All tests/checks passed |
| 1 | Policy violations or test failures |

---

## MCP Tool Integration

AI assistants interact via the `policy` MCP tool:

### Check Action

```json
{
  "action": "check",
  "files": ["internal/auth/login.go"],
  "task_id": "task-123",
  "task_title": "Add login endpoint",
  "plan_id": "plan-456",
  "plan_goal": "Implement auth"
}
```

Response on violation:

```
## Policy Check Results

**Status**: deny
**Files Checked**: 1

### Violations

- BLOCKED: Environment file '.env' is protected and cannot be modified by AI agents

### Recommendation

Remove protected files from your changes or request policy exception.
```

### List Action

```json
{"action": "list"}
```

### Explain Action

```json
{"action": "explain", "policy_name": "default"}
```

---

## Audit Trail

All policy decisions are logged to SQLite:

```sql
SELECT decision_id, result, violations, task_id, evaluated_at
FROM policy_decisions
ORDER BY evaluated_at DESC
LIMIT 10;
```

Example output:

| decision_id | result | violations | task_id | evaluated_at |
|-------------|--------|------------|---------|--------------|
| dec-abc123 | deny | ["BLOCKED: .env protected"] | task-456 | 2024-01-15 10:30:00 |
| dec-def456 | allow | [] | task-789 | 2024-01-15 10:25:00 |

---

## Best Practices

### 1. Test Before Enforcing

Always validate new policies with dry-run:

```bash
# Add new policy
vim .taskwing/policies/security.rego

# Test with hypothetical files
taskwing policy test .env secrets/api.key internal/handler.go

# If satisfied, policies are now active for task completion
```

### 2. Start Permissive, Add Restrictions

```rego
# Start with warnings, promote to denials after validation
warn contains msg if {
    some file in input.task.files_modified
    startswith(file, "legacy/")
    msg := "WARNING: Modifying legacy code - ensure tests pass"
}
```

### 3. Use Clear Messages

```rego
# Good: Tells user what and why
msg := sprintf("BLOCKED: File '%s' in protected zone. Contact #security-team for exceptions.", [file])

# Bad: Unhelpful
msg := "blocked"
```

### 4. Group Related Rules

Organize policies by concern:

```
.taskwing/policies/
├── default.rego          # Basic protections
├── security.rego         # Secrets, auth, crypto
├── architecture.rego     # Layer boundaries, patterns
└── compliance.rego       # Regulatory requirements
```

### 5. Version Control Policies

Track policy changes alongside code:

```bash
git add .taskwing/policies/
git commit -m "policy: add security rules for secrets detection"
```

---

## Troubleshooting

### Policy Not Enforcing

```bash
# Check policies are loaded
taskwing policy list

# Verify syntax
taskwing policy test

# Check verbose output
taskwing policy check main.go --verbose
```

### Dry-Run Passes But Enforcement Fails

Ensure files in `--files-modified` match what the policy checks:

```bash
# What you're testing
taskwing policy test internal/auth.go

# What task completion sends
files_modified: ["internal/auth.go", ".env"]  # .env causes failure!
```

### False Positives

Refine your rules with more specific conditions:

```rego
# Too broad - blocks all vendor files
deny contains msg if {
    contains(file, "vendor")
    ...
}

# Better - only blocks vendor directory
deny contains msg if {
    startswith(file, "vendor/")
    ...
}
```

---

## Related Documentation

- [POLICY_GUIDE.md](./POLICY_GUIDE.md) - Full Rego policy reference
- [AUTONOMOUS_HOOKS.md](./AUTONOMOUS_HOOKS.md) - Hook circuit breakers
- [DATA_MODEL.md](./DATA_MODEL.md) - Policy decisions schema
