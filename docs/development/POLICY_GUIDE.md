# Policy-as-Code with Open Policy Agent (OPA)

> Define enterprise guardrails for AI-assisted code changes using Rego policies

TaskWing embeds Open Policy Agent (OPA) to enforce code policies. Policies are written in Rego and stored in `.taskwing/policies/*.rego`. They define what AI agents can and cannot modify in your codebase.

**Why OPA?** "We use OPA" is an enterprise compliance checkbox. Netflix, Goldman Sachs, and Atlassian use it.

---

## Quick Setup

```bash
# 1. Initialize default policies
taskwing policy init

# 2. Check files against policies
taskwing policy check main.go utils.go

# 3. Check git staged files
taskwing policy check --staged

# 4. List loaded policies
taskwing policy list

# 5. Validate policy syntax
taskwing policy test
```

The default policy protects:
- Environment files (`.env`, `.env.local`, `.env.production`, etc.)
- Secrets directory (`secrets/**`)
- Warns on large file changes (> 500 lines)

---

## CLI Commands

### `taskwing policy init`

Creates the default policy file at `.taskwing/policies/default.rego`.

```bash
$ taskwing policy init

✓ Created default policy: .taskwing/policies/default.rego

The default policy protects:
  • Environment files (.env, .env.local, etc.)
  • Secrets directory (secrets/**)
  • Warns on large file changes (> 500 lines)

Customize this file or add more .rego files to .taskwing/policies/
```

### `taskwing policy list`

Lists all loaded Rego policies.

```bash
$ taskwing policy list

Policies directory: /path/to/project/.taskwing/policies
Loaded 2 policy file(s):

  • default (.taskwing/policies/default.rego)
  • security (.taskwing/policies/security.rego)
```

### `taskwing policy check [files...]`

Evaluates files against loaded policies.

```bash
# Check specific files
$ taskwing policy check internal/auth/login.go core/router.go

Checking 2 file(s) against 1 policy file(s)...

  • internal/auth/login.go
  • core/router.go

✓ All files passed policy checks

# Check staged git files
$ taskwing policy check --staged

# Check with violations
$ taskwing policy check .env

Checking 1 file(s) against 1 policy file(s)...

  • .env

✗ Policy violations detected:
  BLOCKED: Environment file '.env' is protected and cannot be modified by AI agents

Error: policy check failed with 1 violation(s)
```

**Exit codes:**
- `0` - All files pass
- `1` - Policy violations detected (useful for CI/CD)

### `taskwing policy test`

Validates policy syntax and runs OPA tests.

```bash
$ taskwing policy test

Found 1 test file(s):
  • .taskwing/policies/default_test.rego

Validating policy syntax...
  ✓ default: valid

✓ All policies are syntactically valid
```

---

## Writing Rego Policies

Policies use the Rego language. They live in `.taskwing/policies/*.rego`.

### Basic Structure

```rego
package taskwing.policy

import rego.v1

# Deny rules block modifications
deny contains msg if {
    # condition
    msg := "BLOCKED: Reason message"
}

# Warn rules emit advisories (don't block)
warn contains msg if {
    # condition
    msg := "WARNING: Advisory message"
}
```

### OPA Input Structure

Policies receive this input structure:

```json
{
  "task": {
    "id": "task-123",
    "title": "Add login endpoint",
    "files_modified": ["internal/auth/login.go", "core/router.go"],
    "files_created": ["internal/auth/login_test.go"]
  },
  "plan": {
    "id": "plan-456",
    "goal": "Implement user authentication"
  },
  "context": {
    "protected_zones": ["core/**", "vendor/**"],
    "project_type": "drupal"
  }
}
```

### Example: Protect Environment Files

```rego
package taskwing.policy

import rego.v1

# Helper function for env file detection
is_env_file(file) if startswith(file, ".env")
is_env_file(file) if contains(file, "/.env")

# Block modifications to .env files
deny contains msg if {
    some file in input.task.files_modified
    is_env_file(file)
    msg := sprintf("BLOCKED: Environment file '%s' is protected", [file])
}

# Block creation of .env files
deny contains msg if {
    some file in input.task.files_created
    is_env_file(file)
    msg := sprintf("BLOCKED: Cannot create environment file '%s'", [file])
}
```

### Example: Protect Drupal Core

```rego
package taskwing.policy

import rego.v1

# Deny modifications to Drupal core
deny contains msg if {
    some file in input.task.files_modified
    startswith(file, "core/")
    msg := sprintf("BLOCKED: Drupal core file '%s' requires maintainer review", [file])
}
```

### Example: Forbidden Patterns (Hardcoded Secrets)

```rego
package taskwing.policy

import rego.v1

# Block hardcoded secrets in Go files
deny contains msg if {
    some file in input.task.files_modified
    endswith(file, ".go")
    taskwing.has_pattern(file, `(password|secret|api_key)\s*[:=]\s*"[^"]{8,}"`)
    msg := sprintf("BLOCKED: Potential hardcoded secret in '%s'", [file])
}
```

### Example: Layer Boundaries

```rego
package taskwing.policy

import rego.v1

# Controllers cannot import database package directly
deny contains msg if {
    some file in input.task.files_modified
    contains(file, "/controller/")
    imports := taskwing.file_imports(file)
    some imp in imports
    contains(imp, "database")
    msg := sprintf("BLOCKED: Controller '%s' imports database. Use service layer.", [file])
}
```

### Example: Large File Warning

```rego
package taskwing.policy

import rego.v1

# Advisory for large files (doesn't block)
warn contains msg if {
    some file in input.task.files_modified
    lines := taskwing.file_line_count(file)
    lines > 500
    msg := sprintf("WARNING: Large file '%s' (%d lines) - review carefully", [file, lines])
}
```

---

## Custom Built-in Functions

TaskWing registers custom OPA built-ins for code intelligence:

| Function | Description |
|----------|-------------|
| `taskwing.file_line_count(file)` | Returns line count of a file |
| `taskwing.has_pattern(file, regex)` | Checks if file contains regex pattern |
| `taskwing.file_imports(file)` | Returns import statements from file |
| `taskwing.symbol_exists(file, symbol)` | Checks if symbol exists in file AST |

### Example Usage

```rego
# Check line count
lines := taskwing.file_line_count("main.go")
lines > 500

# Check for pattern
taskwing.has_pattern("config.go", `password\s*=`)

# Check imports (Go files)
imports := taskwing.file_imports("handler.go")
some imp in imports
contains(imp, "database/sql")
```

---

## MCP Integration

AI assistants can interact with policies via the `policy` MCP tool.

### Actions

| Action | Description |
|--------|-------------|
| `check` | Evaluate files against policies |
| `list` | List loaded policies |
| `explain` | Get details about a specific policy |

### MCP Tool Parameters

```json
{
  "action": "check",
  "files": ["main.go", "config.go"],
  "task_id": "task-123",
  "task_title": "Add feature",
  "plan_id": "plan-456",
  "plan_goal": "Implement auth"
}
```

### Example: Claude Code Checking Files

```
Claude: Let me check if these files pass policy checks...

[Uses MCP tool: policy with action=check, files=["internal/auth/login.go"]]

Result: ✓ All 1 file(s) passed policy checks
```

---

## Integration Points

### Task Completion Validation

When a task is marked complete, the policy engine evaluates all modified files:

```
Task Complete → Policy Evaluation → Allow/Deny
```

If policies fail, the task completion is blocked with violation messages.

### Planning Context

Policies are injected into AI planning context as **MANDATORY CONSTRAINTS**:

```
MANDATORY CONSTRAINTS (from policies):
• Cannot modify files in core/
• Cannot modify .env files
• Must use service layer for database access
```

### Hook Circuit Breaker

Policy violations trigger the autonomous execution circuit breaker, stopping the agent for human review.

---

## Audit Trail

Policy decisions are logged to SQLite for compliance:

```sql
SELECT * FROM policy_decisions ORDER BY evaluated_at DESC LIMIT 5;
```

| Column | Description |
|--------|-------------|
| `decision_id` | Unique OPA decision ID |
| `policy_path` | e.g., "taskwing.policy" |
| `result` | "allow" or "deny" |
| `violations` | JSON array of deny messages |
| `input_json` | Full input for replay |
| `task_id` | Related task |
| `session_id` | AI session |
| `evaluated_at` | Timestamp |

---

## JSON Output

All commands support `--json` for machine-readable output:

```bash
$ taskwing policy check .env --json
{
  "status": "deny",
  "decision_id": "dec-abc123",
  "files": [".env"],
  "violations": [
    "BLOCKED: Environment file '.env' is protected and cannot be modified by AI agents"
  ]
}
```

```bash
$ taskwing policy list --json
{
  "policies_dir": ".taskwing/policies",
  "count": 1,
  "policies": [
    {
      "name": "default",
      "path": ".taskwing/policies/default.rego"
    }
  ]
}
```

---

## Testing Policies

Create `*_test.rego` files alongside your policies:

```rego
# .taskwing/policies/default_test.rego
package taskwing.policy

import rego.v1

test_deny_env_file if {
    deny with input as {"task": {"files_modified": [".env"]}}
}

test_allow_regular_file if {
    not deny with input as {"task": {"files_modified": ["main.go"]}}
}

test_deny_nested_env_file if {
    deny with input as {"task": {"files_modified": ["config/.env.local"]}}
}
```

Run tests:
```bash
taskwing policy test
```

---

## File Structure

```
.taskwing/
├── policies/
│   ├── default.rego          # Default protections
│   ├── default_test.rego     # Tests for default policy
│   ├── security.rego         # Custom security rules
│   └── architecture.rego     # Layer boundary rules
└── memory/
    └── memory.db             # Contains policy_decisions table
```

---

## Best Practices

### 1. Start with Defaults

The default policy covers common cases. Add custom rules incrementally:

```bash
taskwing policy init  # Creates sensible defaults
```

### 2. Use Helper Functions

Group related checks into helper functions for readability:

```rego
is_env_file(file) if startswith(file, ".env")
is_env_file(file) if contains(file, "/.env")

is_core_file(file) if startswith(file, "core/")
is_core_file(file) if startswith(file, "vendor/")
```

### 3. Prefer Deny Over Warn

Use `deny` for hard rules, `warn` for advisories:

```rego
# Hard rule - blocks execution
deny contains msg if { ... }

# Advisory - logged but doesn't block
warn contains msg if { ... }
```

### 4. Test Your Policies

Write tests for edge cases:

```rego
# Test root-level .env
test_deny_root_env if {
    deny with input as {"task": {"files_modified": [".env"]}}
}

# Test nested .env
test_deny_nested_env if {
    deny with input as {"task": {"files_modified": ["config/.env.local"]}}
}

# Test allowed files
test_allow_go_file if {
    not deny with input as {"task": {"files_modified": ["main.go"]}}
}
```

### 5. Use sprintf for Clear Messages

Include context in violation messages:

```rego
msg := sprintf("BLOCKED: File '%s' matches protected zone '%s'", [file, zone])
```

---

## Troubleshooting

### Policies Not Loading

```bash
# Check policies directory exists
ls -la .taskwing/policies/

# List loaded policies
taskwing policy list

# Validate syntax
taskwing policy test
```

### Glob Patterns Not Matching

OPA's `glob.match` may not match root-level files. Use helper functions:

```rego
# BAD: Won't match .env at root
glob.match("**/.env*", ["/"], file)

# GOOD: Use helper function
is_env_file(file) if startswith(file, ".env")
is_env_file(file) if contains(file, "/.env")
```

### Policy Violations in CI/CD

The `policy check` command returns exit code 1 on violations:

```yaml
# GitHub Actions example
- name: Check policies
  run: taskwing policy check --staged
  # Fails the job if violations found
```

### Debugging Policy Input

Use `--verbose` to see the full OPA input:

```bash
taskwing policy check main.go --verbose
```

Or check with JSON output:
```bash
taskwing policy check main.go --json 2>&1 | jq .
```

---

## Future Enhancements

1. **Full OPA Test Runner** - Execute `opa test` directly
2. **Policy Bundles** - Download policies from remote sources
3. **Policy Versioning** - Track policy changes over time
4. **Custom Built-ins** - More code intelligence functions
5. **IDE Integration** - Real-time policy feedback in editors
