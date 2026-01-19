# Tests for TaskWing Default Policy

package taskwing.policy

import rego.v1

# ═══════════════════════════════════════════════════════════════════════════════
# Tests for environment file protection
# ═══════════════════════════════════════════════════════════════════════════════

test_deny_env_file_modification if {
    # Should deny modification of .env file
    result := deny with input as {"task": {"files_modified": [".env"]}}
    count(result) > 0
}

test_deny_env_local_modification if {
    # Should deny modification of .env.local file
    result := deny with input as {"task": {"files_modified": [".env.local"]}}
    count(result) > 0
}

test_deny_env_production_modification if {
    # Should deny modification of .env.production file
    result := deny with input as {"task": {"files_modified": [".env.production"]}}
    count(result) > 0
}

test_deny_nested_env_file if {
    # Should deny modification of nested .env files
    result := deny with input as {"task": {"files_modified": ["config/.env"]}}
    count(result) > 0
}

test_deny_env_file_creation if {
    # Should deny creation of new .env files
    result := deny with input as {"task": {"files_created": [".env.staging"]}}
    count(result) > 0
}

test_allow_regular_file if {
    # Should allow modification of regular files
    result := deny with input as {"task": {"files_modified": ["main.go"]}}
    count(result) == 0
}

# ═══════════════════════════════════════════════════════════════════════════════
# Tests for secrets directory protection
# ═══════════════════════════════════════════════════════════════════════════════

test_deny_secrets_modification if {
    # Should deny modification of files in secrets directory
    result := deny with input as {"task": {"files_modified": ["secrets/api_key.txt"]}}
    count(result) > 0
}

test_deny_nested_secrets_modification if {
    # Should deny modification of files in nested secrets directory
    result := deny with input as {"task": {"files_modified": ["config/secrets/api_key.txt"]}}
    count(result) > 0
}

test_deny_secrets_creation if {
    # Should deny creation of files in secrets directory
    result := deny with input as {"task": {"files_created": ["secrets/new_secret.txt"]}}
    count(result) > 0
}

test_allow_non_secrets_file if {
    # Should allow modification of files not in secrets
    result := deny with input as {"task": {"files_modified": ["config/settings.json"]}}
    count(result) == 0
}

# ═══════════════════════════════════════════════════════════════════════════════
# Tests for helper functions
# ═══════════════════════════════════════════════════════════════════════════════

test_is_env_file_root if {
    is_env_file(".env")
}

test_is_env_file_local if {
    is_env_file(".env.local")
}

test_is_env_file_nested if {
    is_env_file("config/.env")
}

test_is_not_env_file if {
    not is_env_file("main.go")
}

test_is_secrets_file if {
    is_secrets_file("secrets/key.txt")
}

test_is_secrets_file_nested if {
    is_secrets_file("config/secrets/key.txt")
}

test_is_not_secrets_file if {
    not is_secrets_file("config/settings.json")
}
