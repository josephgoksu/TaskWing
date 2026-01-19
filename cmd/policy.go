/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/policy"
	"github.com/spf13/cobra"
)

// DefaultRegoPolicy is the default policy file content.
const DefaultRegoPolicy = `# TaskWing Default Policy
# This file defines enterprise guardrails for AI-assisted code changes.
# Learn more: https://www.openpolicyagent.org/docs/latest/policy-language/

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
    msg := sprintf("BLOCKED: Environment file '%s' is protected and cannot be modified by AI agents", [file])
}

deny contains msg if {
    some file in input.task.files_created
    is_env_file(file)
    msg := sprintf("BLOCKED: Cannot create environment file '%s' - environment files are protected", [file])
}

# Deny modifications to secrets directory
deny contains msg if {
    some file in input.task.files_modified
    is_secrets_file(file)
    msg := sprintf("BLOCKED: Secrets file '%s' is protected and cannot be modified by AI agents", [file])
}

deny contains msg if {
    some file in input.task.files_created
    is_secrets_file(file)
    msg := sprintf("BLOCKED: Cannot create file '%s' in protected secrets directory", [file])
}

# ═══════════════════════════════════════════════════════════════════════════════
# WARNINGS - Advisory messages that don't block execution
# ═══════════════════════════════════════════════════════════════════════════════

# Warn on large file changes (> 500 lines)
warn contains msg if {
    some file in input.task.files_modified
    lines := taskwing.file_line_count(file)
    lines > 500
    msg := sprintf("WARNING: Large file '%s' (%d lines) is being modified - review carefully", [file, lines])
}
`

// policyCmd represents the policy parent command
var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage OPA policies for code guardrails",
	Long: `Manage Open Policy Agent (OPA) policies that define enterprise guardrails.

Policies are written in Rego and stored in .taskwing/policies/*.rego.
They define what AI agents can and cannot do in your codebase.

Examples:
  taskwing policy init          # Create default policy file
  taskwing policy list          # List loaded policies
  taskwing policy check main.go # Check file against policies
  taskwing policy test          # Run policy tests`,
}

// policyInitCmd creates the default policy file
var policyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize default policy file",
	Long: `Create the default policy file in .taskwing/policies/default.rego.

The default policy protects:
  • Environment files (.env, .env.local, etc.)
  • Secrets directory (secrets/**)
  • Warns on large file changes (> 500 lines)

You can customize this file or add additional .rego files.`,
	RunE: runPolicyInit,
}

// policyListCmd lists loaded policies
var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List loaded policies",
	Long: `List all Rego policy files loaded from .taskwing/policies/.

Shows the name and path of each policy file.`,
	RunE: runPolicyList,
}

// policyCheckCmd checks files against policies
var policyCheckCmd = &cobra.Command{
	Use:   "check [files...]",
	Short: "Check files against policies",
	Long: `Evaluate files against loaded policies.

Without arguments, checks all staged git files.
With file arguments, checks the specified files.

Examples:
  taskwing policy check                    # Check staged files
  taskwing policy check main.go utils.go   # Check specific files
  taskwing policy check --staged           # Explicitly check staged files`,
	RunE: runPolicyCheck,
}

// policyTestCmd runs OPA policy tests
var policyTestCmd = &cobra.Command{
	Use:   "test [files...]",
	Short: "Run policy tests or dry-run validation",
	Long: `Run OPA policy tests or validate hypothetical files against policies.

Without arguments, runs *_test.rego unit tests in .taskwing/policies/.

With file arguments, performs a dry-run policy check against the specified
file paths. Files don't need to exist - this is useful for testing what
would happen if you modified certain files.

Examples:
  taskwing policy test                      # Run OPA unit tests
  taskwing policy test .env secrets/key.pem # Dry-run check (files need not exist)
  taskwing policy test --task-id T1 src/api.go  # Include task context in check

Test file example (.taskwing/policies/default_test.rego):
  package taskwing.policy

  test_deny_env_file {
      deny with input as {"task": {"files_modified": [".env"]}}
  }`,
	RunE: runPolicyTest,
}

var policyCheckStaged bool
var policyTestTaskID string
var policyTestPlanID string

func init() {
	rootCmd.AddCommand(policyCmd)

	policyCmd.AddCommand(policyInitCmd)
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyCheckCmd)
	policyCmd.AddCommand(policyTestCmd)

	policyCheckCmd.Flags().BoolVar(&policyCheckStaged, "staged", false, "Check git staged files")

	// Flags for policy test dry-run mode
	policyTestCmd.Flags().StringVar(&policyTestTaskID, "task-id", "", "Task ID to include in policy context")
	policyTestCmd.Flags().StringVar(&policyTestPlanID, "plan-id", "", "Plan ID to include in policy context")
}

func runPolicyInit(cmd *cobra.Command, args []string) error {
	projectRoot, err := config.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("get project root: %w", err)
	}

	policiesDir := policy.GetPoliciesPath(projectRoot)
	defaultPolicyPath := filepath.Join(policiesDir, "default.rego")

	// Check if file already exists
	if _, err := os.Stat(defaultPolicyPath); err == nil {
		if !isQuiet() {
			cmd.Printf("Policy file already exists: %s\n", defaultPolicyPath)
			cmd.Println("Use --force to overwrite (not implemented yet).")
		}
		return nil
	}

	// Create policies directory if it doesn't exist
	if err := os.MkdirAll(policiesDir, 0755); err != nil {
		return fmt.Errorf("create policies directory: %w", err)
	}

	// Write default policy file
	if err := os.WriteFile(defaultPolicyPath, []byte(DefaultRegoPolicy), 0644); err != nil {
		return fmt.Errorf("write default policy: %w", err)
	}

	if isJSON() {
		return printJSON(map[string]any{
			"created": defaultPolicyPath,
			"status":  "success",
		})
	}

	cmd.Printf("✓ Created default policy: %s\n", defaultPolicyPath)
	cmd.Println("\nThe default policy protects:")
	cmd.Println("  • Environment files (.env, .env.local, etc.)")
	cmd.Println("  • Secrets directory (secrets/**)")
	cmd.Println("  • Warns on large file changes (> 500 lines)")
	cmd.Println("\nCustomize this file or add more .rego files to .taskwing/policies/")

	return nil
}

func runPolicyList(cmd *cobra.Command, args []string) error {
	projectRoot, err := config.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("get project root: %w", err)
	}

	policiesDir := policy.GetPoliciesPath(projectRoot)
	loader := policy.NewOsLoader(policiesDir)

	policies, err := loader.LoadAll()
	if err != nil {
		return fmt.Errorf("load policies: %w", err)
	}

	if isJSON() {
		return printJSON(map[string]any{
			"policies_dir": policiesDir,
			"count":        len(policies),
			"policies":     policies,
		})
	}

	if len(policies) == 0 {
		cmd.Println("No policies loaded.")
		cmd.Println("Run 'taskwing policy init' to create the default policy.")
		return nil
	}

	cmd.Printf("Policies directory: %s\n", policiesDir)
	cmd.Printf("Loaded %d policy file(s):\n\n", len(policies))

	for _, p := range policies {
		// Show relative path if possible
		relPath, err := filepath.Rel(projectRoot, p.Path)
		if err != nil {
			relPath = p.Path
		}
		cmd.Printf("  • %s (%s)\n", p.Name, relPath)
	}

	return nil
}

func runPolicyCheck(cmd *cobra.Command, args []string) error {
	projectRoot, err := config.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("get project root: %w", err)
	}

	// Determine files to check
	var filesToCheck []string
	if len(args) > 0 {
		filesToCheck = args
	} else if policyCheckStaged || len(args) == 0 {
		// Get staged files from git
		staged, err := getStagedFiles(projectRoot)
		if err != nil {
			return fmt.Errorf("get staged files: %w", err)
		}
		filesToCheck = staged
	}

	if len(filesToCheck) == 0 {
		if isJSON() {
			return printJSON(map[string]any{
				"status":  "success",
				"message": "No files to check",
				"files":   []string{},
			})
		}
		cmd.Println("No files to check.")
		cmd.Println("Specify files as arguments or stage files with git.")
		return nil
	}

	// Create policy engine
	engine, err := policy.NewEngine(policy.EngineConfig{
		WorkDir: projectRoot,
	})
	if err != nil {
		return fmt.Errorf("create policy engine: %w", err)
	}

	if engine.PolicyCount() == 0 {
		if isJSON() {
			return printJSON(map[string]any{
				"status":  "success",
				"message": "No policies loaded - all files allowed",
				"files":   filesToCheck,
			})
		}
		cmd.Println("No policies loaded - all files allowed by default.")
		cmd.Println("Run 'taskwing policy init' to create the default policy.")
		return nil
	}

	// Evaluate files against policies
	ctx := context.Background()
	decision, err := engine.EvaluateFiles(ctx, filesToCheck, nil)
	if err != nil {
		return fmt.Errorf("evaluate policies: %w", err)
	}

	if isJSON() {
		return printJSON(map[string]any{
			"status":      decision.Result,
			"decision_id": decision.DecisionID,
			"files":       filesToCheck,
			"violations":  decision.Violations,
		})
	}

	// Display results
	cmd.Printf("Checking %d file(s) against %d policy file(s)...\n\n", len(filesToCheck), engine.PolicyCount())

	for _, f := range filesToCheck {
		cmd.Printf("  • %s\n", f)
	}
	cmd.Println()

	if decision.IsAllowed() {
		cmd.Println("✓ All files passed policy checks")
		return nil
	}

	cmd.Println("✗ Policy violations detected:")
	for _, v := range decision.Violations {
		cmd.Printf("  %s\n", v)
	}

	// Return error to signal failure for CI/CD
	return fmt.Errorf("policy check failed with %d violation(s)", len(decision.Violations))
}

func runPolicyTest(cmd *cobra.Command, args []string) error {
	projectRoot, err := config.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("get project root: %w", err)
	}

	// If file arguments are provided, run dry-run policy validation
	if len(args) > 0 {
		return runPolicyTestDryRun(cmd, projectRoot, args)
	}

	// Otherwise, run OPA unit tests
	return runPolicyTestOPA(cmd, projectRoot)
}

// runPolicyTestDryRun validates hypothetical files against policies without database writes.
// This is a dry-run mode - files don't need to exist.
func runPolicyTestDryRun(cmd *cobra.Command, projectRoot string, files []string) error {
	// Create policy engine
	engine, err := policy.NewEngine(policy.EngineConfig{
		WorkDir: projectRoot,
	})
	if err != nil {
		return fmt.Errorf("create policy engine: %w", err)
	}

	if engine.PolicyCount() == 0 {
		if isJSON() {
			return printJSON(map[string]any{
				"status":  "success",
				"message": "No policies loaded - all files allowed",
				"mode":    "dry-run",
				"files":   files,
			})
		}
		cmd.Println("No policies loaded - all files allowed by default.")
		cmd.Println("Run 'taskwing policy init' to create the default policy.")
		return nil
	}

	// Build policy input with optional task/plan context
	// This is a dry-run, so we use hypothetical file paths
	inputBuilder := policy.NewContextBuilder(projectRoot).
		WithTask(policyTestTaskID, "").
		WithTaskFiles(files, nil).
		WithPlan(policyTestPlanID, "")

	// Evaluate against policies
	ctx := context.Background()
	decision, err := engine.Evaluate(ctx, inputBuilder.Build())
	if err != nil {
		return fmt.Errorf("evaluate policies: %w", err)
	}

	if isJSON() {
		return printJSON(map[string]any{
			"status":      decision.Result,
			"mode":        "dry-run",
			"decision_id": decision.DecisionID,
			"files":       files,
			"violations":  decision.Violations,
			"task_id":     policyTestTaskID,
			"plan_id":     policyTestPlanID,
		})
	}

	// Display results
	cmd.Println("Policy Dry-Run Validation")
	cmd.Println("=========================")
	cmd.Printf("Checking %d hypothetical file(s) against %d policy file(s)...\n\n", len(files), engine.PolicyCount())

	cmd.Println("Files to validate:")
	for _, f := range files {
		cmd.Printf("  • %s\n", f)
	}
	cmd.Println()

	if policyTestTaskID != "" {
		cmd.Printf("Task context: %s\n", policyTestTaskID)
	}
	if policyTestPlanID != "" {
		cmd.Printf("Plan context: %s\n", policyTestPlanID)
	}
	if policyTestTaskID != "" || policyTestPlanID != "" {
		cmd.Println()
	}

	if decision.IsAllowed() {
		cmd.Println("✓ All files would pass policy checks")
		return nil
	}

	cmd.Println("✗ Policy violations detected:")
	for _, v := range decision.Violations {
		cmd.Printf("  %s\n", v)
	}

	// Return error to signal failure
	return fmt.Errorf("dry-run failed: %d policy violation(s)", len(decision.Violations))
}

// runPolicyTestOPA runs OPA unit tests from *_test.rego files.
func runPolicyTestOPA(cmd *cobra.Command, projectRoot string) error {
	policiesDir := policy.GetPoliciesPath(projectRoot)

	// Check if policies directory exists
	if _, err := os.Stat(policiesDir); os.IsNotExist(err) {
		if isJSON() {
			return printJSON(map[string]any{
				"status":  "error",
				"message": "No policies directory found",
			})
		}
		cmd.Println("No policies directory found.")
		cmd.Println("Run 'taskwing policy init' to create the default policy.")
		return nil
	}

	// Create test runner
	runner := policy.NewTestRunner(nil, policiesDir, projectRoot)

	// Check if there are any test files
	hasTests, err := runner.HasTests()
	if err != nil {
		return fmt.Errorf("check for test files: %w", err)
	}

	if !hasTests {
		if isJSON() {
			return printJSON(map[string]any{
				"status":  "success",
				"message": "No test files found",
				"tests":   0,
			})
		}
		cmd.Println("No test files found in", policiesDir)
		cmd.Println("\nCreate *_test.rego files to add policy tests.")
		cmd.Println("Example: .taskwing/policies/default_test.rego")
		return nil
	}

	// Run tests
	ctx := context.Background()
	summary, err := runner.Run(ctx)
	if err != nil {
		if isJSON() {
			return printJSON(map[string]any{
				"status":  "error",
				"message": err.Error(),
			})
		}
		return fmt.Errorf("run tests: %w", err)
	}

	if isJSON() {
		return printJSON(map[string]any{
			"status":   "success",
			"passed":   summary.Passed,
			"failed":   summary.Failed,
			"errored":  summary.Errored,
			"skipped":  summary.Skipped,
			"total":    summary.Total,
			"duration": summary.Duration.String(),
			"results":  summary.Results,
		})
	}

	// Display results
	cmd.Printf("Running OPA tests in %s...\n\n", policiesDir)

	for _, result := range summary.Results {
		// Extract just the test name from the full path
		name := result.Name
		if idx := strings.LastIndex(name, "."); idx > 0 {
			name = name[idx+1:]
		}

		if result.Passed {
			cmd.Printf("  ✓ %s (%s)\n", name, result.Duration.Round(time.Millisecond))
		} else if result.Failed {
			cmd.Printf("  ✗ %s: FAIL\n", name)
		} else if result.Error != "" {
			cmd.Printf("  ✗ %s: %s\n", name, result.Error)
		} else if result.Skipped {
			cmd.Printf("  - %s: skipped\n", name)
		}

		// Show any output
		for _, out := range result.Output {
			cmd.Printf("      %s\n", out)
		}
	}

	// Print summary
	cmd.Print(summary.FormatSummary())

	if !summary.AllPassed() {
		return fmt.Errorf("policy tests failed: %d failures, %d errors", summary.Failed, summary.Errored)
	}

	return nil
}

// getStagedFiles returns the list of staged files from git.
func getStagedFiles(projectRoot string) ([]string, error) {
	// Use git diff --cached to get staged files
	gitCmd := exec.Command("git", "diff", "--cached", "--name-only")
	gitCmd.Dir = projectRoot
	output, err := gitCmd.Output()
	if err != nil {
		// If git command fails, return empty list (not a git repo or no staged files)
		return []string{}, nil
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
