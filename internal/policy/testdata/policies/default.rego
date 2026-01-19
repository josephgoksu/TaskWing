# TaskWing Default Policy
# This file defines enterprise guardrails for AI-assisted code changes.

package taskwing.policy

import rego.v1

# ═══════════════════════════════════════════════════════════════════════════════
# HELPER FUNCTIONS
# ═══════════════════════════════════════════════════════════════════════════════

# Check if file is an environment file (.env, .env.local, .env.production, etc.)
is_env_file(file) if startswith(file, ".env")

is_env_file(file) if contains(file, "/.env")

# Check if file is in secrets directory
is_secrets_file(file) if startswith(file, "secrets/")

is_secrets_file(file) if contains(file, "/secrets/")

# ═══════════════════════════════════════════════════════════════════════════════
# PROTECTED FILES - AI agents MUST NOT modify these files
# ═══════════════════════════════════════════════════════════════════════════════

# Deny modifications to environment files (.env, .env.local, .env.production, etc.)
deny contains msg if {
    some file in input.task.files_modified
    is_env_file(file)
    msg := sprintf("BLOCKED: Environment file '%s' is protected", [file])
}

deny contains msg if {
    some file in input.task.files_created
    is_env_file(file)
    msg := sprintf("BLOCKED: Cannot create environment file '%s'", [file])
}

# Deny modifications to secrets directory
deny contains msg if {
    some file in input.task.files_modified
    is_secrets_file(file)
    msg := sprintf("BLOCKED: Secrets file '%s' is protected", [file])
}

deny contains msg if {
    some file in input.task.files_created
    is_secrets_file(file)
    msg := sprintf("BLOCKED: Cannot create file '%s' in secrets directory", [file])
}

# ═══════════════════════════════════════════════════════════════════════════════
# WARNINGS - Advisory messages that don't block execution
# ═══════════════════════════════════════════════════════════════════════════════

# Warn on large file changes (> 500 lines)
warn contains msg if {
    some file in input.task.files_modified
    taskwing.file_line_count(file)  > 500
    msg := sprintf("WARNING: Large file '%s' is being modified", [file])
}
