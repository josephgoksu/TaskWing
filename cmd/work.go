/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/spf13/cobra"
)

var workCmd = &cobra.Command{
	Use:   "work",
	Short: "Prepare and start working on tasks",
	Long: `Unified command to prepare for autonomous task execution.

This command:
  1. Checks if TaskWing is initialized (runs bootstrap if needed)
  2. Ensures an active plan exists (prompts to create one)
  3. Initializes the session for hook tracking
  4. Provides instructions to start working

Examples:
  taskwing work                    # Prepare and get instructions
  taskwing work --launch           # Also launch Claude Code
  taskwing work --plan "my goal"   # Create a new plan and start`,
	RunE: func(cmd *cobra.Command, args []string) error {
		launch, _ := cmd.Flags().GetBool("launch")
		planGoal, _ := cmd.Flags().GetString("plan")
		return runWork(launch, planGoal)
	},
}

func init() {
	rootCmd.AddCommand(workCmd)
	workCmd.Flags().Bool("launch", false, "Launch Claude Code after setup")
	workCmd.Flags().String("plan", "", "Create a new plan with this goal")
}

func runWork(launch bool, planGoal string) error {
	if isJSON() {
		return runWorkJSON(launch, planGoal)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	fmt.Println("🚀 TaskWing Work Session")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Step 1: Check initialization
	taskwingDir := filepath.Join(cwd, ".taskwing")
	if _, err := os.Stat(taskwingDir); os.IsNotExist(err) {
		fmt.Println("📦 TaskWing not initialized. Running bootstrap...")
		fmt.Println()

		// Run bootstrap with timeout (5 minutes for LLM analysis)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		bootstrapCmd := exec.CommandContext(ctx, os.Args[0], "bootstrap")
		bootstrapCmd.Dir = cwd
		bootstrapCmd.Stdin = os.Stdin
		bootstrapCmd.Stdout = os.Stdout
		bootstrapCmd.Stderr = os.Stderr
		if err := bootstrapCmd.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("bootstrap timed out after 5 minutes")
			}
			return fmt.Errorf("bootstrap failed: %w", err)
		}
		fmt.Println()
	} else {
		fmt.Println("✅ TaskWing initialized")
	}

	// Step 2: Check/create hooks
	claudeSettingsPath := filepath.Join(cwd, ".claude", "settings.json")
	if _, err := os.Stat(claudeSettingsPath); os.IsNotExist(err) {
		fmt.Println("⚠️  Hooks not configured. Creating...")
		if err := bootstrap.NewInitializer(cwd).InstallHooksConfig("claude", false); err != nil {
			fmt.Printf("   Failed to create hooks: %v\n", err)
		} else {
			fmt.Println("✅ Hooks configured")
		}
	} else {
		fmt.Println("✅ Hooks configured")
	}

	// Step 3: Check MCP server
	mcpCheck := checkClaudeMCP()
	if mcpCheck.Status == "ok" {
		fmt.Println("✅ MCP server registered")
	} else {
		fmt.Printf("⚠️  %s\n", mcpCheck.Message)
		if mcpCheck.Hint != "" {
			fmt.Printf("   └─ %s\n", mcpCheck.Hint)
		}
	}

	// Step 4: Check/create plan
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	activePlan, _ := repo.GetActivePlan()

	if planGoal != "" {
		// Create new plan with provided goal (2 minute timeout for LLM)
		fmt.Printf("\n📋 Creating plan: %s\n", planGoal)
		planCtx, planCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer planCancel()

		planCmd := exec.CommandContext(planCtx, os.Args[0], "plan", "new", planGoal)
		planCmd.Dir = cwd
		planCmd.Stdin = os.Stdin
		planCmd.Stdout = os.Stdout
		planCmd.Stderr = os.Stderr
		if err := planCmd.Run(); err != nil {
			if planCtx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("plan creation timed out after 2 minutes")
			}
			return fmt.Errorf("plan creation failed: %w", err)
		}

		// Start the new plan (quick operation, 30 second timeout)
		startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer startCancel()

		startCmd := exec.CommandContext(startCtx, os.Args[0], "plan", "start", "latest")
		startCmd.Dir = cwd
		startCmd.Stdout = os.Stdout
		startCmd.Stderr = os.Stderr
		if err := startCmd.Run(); err != nil {
			if startCtx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("plan start timed out")
			}
			return fmt.Errorf("plan start failed: %w", err)
		}

		// Reload active plan
		activePlan, _ = repo.GetActivePlan()
	}

	if activePlan == nil {
		fmt.Println()
		fmt.Println("⚠️  No active plan")
		fmt.Println()

		// Prompt user
		fmt.Print("Would you like to create a plan? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "y" || response == "yes" {
			fmt.Print("Enter your development goal: ")
			goal, _ := reader.ReadString('\n')
			goal = strings.TrimSpace(goal)

			if goal != "" {
				// Create plan with timeout (2 minutes for LLM)
				planCtx, planCancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer planCancel()

				planCmd := exec.CommandContext(planCtx, os.Args[0], "plan", "new", goal)
				planCmd.Dir = cwd
				planCmd.Stdin = os.Stdin
				planCmd.Stdout = os.Stdout
				planCmd.Stderr = os.Stderr
				if err := planCmd.Run(); err != nil {
					if planCtx.Err() == context.DeadlineExceeded {
						return fmt.Errorf("plan creation timed out after 2 minutes")
					}
					return fmt.Errorf("plan creation failed: %w", err)
				}

				// Start plan with timeout (30 seconds)
				startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer startCancel()

				startCmd := exec.CommandContext(startCtx, os.Args[0], "plan", "start", "latest")
				startCmd.Dir = cwd
				startCmd.Stdout = os.Stdout
				startCmd.Stderr = os.Stderr
				if err := startCmd.Run(); err != nil {
					// Log warning but continue - plan was created
					fmt.Printf("⚠️  Could not auto-start plan: %v\n", err)
				}

				activePlan, _ = repo.GetActivePlan()
			}
		}
	}

	if activePlan != nil {
		completed := 0
		for _, t := range activePlan.Tasks {
			if t.Status == "completed" {
				completed++
			}
		}
		fmt.Printf("✅ Active plan: %s (%d/%d tasks)\n", activePlan.ID, completed, len(activePlan.Tasks))
	}

	// Step 5: Initialize session
	fmt.Println()
	fmt.Println("📡 Initializing session...")
	if err := runSessionInit(); err != nil {
		fmt.Printf("⚠️  Session init warning: %v\n", err)
	}

	// Step 6: Instructions or launch
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if launch {
		fmt.Println("🚀 Launching Claude Code...")
		fmt.Println()
		fmt.Println("Once Claude Code opens:")
		fmt.Println("  1. Run /tw-next to start the first task")
		fmt.Println("  2. Tasks will auto-continue until circuit breaker")
		fmt.Println()

		// Launch Claude Code
		cmd := exec.Command("claude")
		cmd.Dir = cwd
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	fmt.Println("✅ Ready to work!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Open Claude Code: claude")
	fmt.Println("  2. Run: /tw-next")
	fmt.Println("  3. Tasks will auto-continue until circuit breaker")
	fmt.Println()
	fmt.Println("Or run: taskwing work --launch")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return nil
}

type workCheck struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

type workPlanSummary struct {
	ID        string `json:"id"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

type workStatus struct {
	Initialized        bool             `json:"initialized"`
	Hooks              []workCheck      `json:"hooks,omitempty"`
	MCP                *workCheck       `json:"mcp,omitempty"`
	ActivePlan         *workPlanSummary `json:"active_plan,omitempty"`
	SessionInitialized bool             `json:"session_initialized"`
	Warnings           []string         `json:"warnings,omitempty"`
	Actions            []string         `json:"actions,omitempty"`
}

func runWorkJSON(launch bool, planGoal string) error {
	status := workStatus{
		SessionInitialized: false,
	}

	if launch {
		status.Warnings = append(status.Warnings, "launch_ignored_in_json")
	}
	if planGoal != "" {
		status.Warnings = append(status.Warnings, "plan_goal_ignored_in_json")
		status.Actions = append(status.Actions, fmt.Sprintf("taskwing plan new %q", planGoal))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	taskwingDir := filepath.Join(cwd, ".taskwing")
	if _, err := os.Stat(taskwingDir); os.IsNotExist(err) {
		status.Initialized = false
		status.Actions = append(status.Actions, "taskwing bootstrap")
	} else {
		status.Initialized = true
	}

	hookChecks := checkHooksConfig(cwd)
	for _, hc := range hookChecks {
		status.Hooks = append(status.Hooks, workCheck{
			Status:  hc.Status,
			Message: hc.Message,
			Hint:    hc.Hint,
		})
		if hc.Status == "warn" || hc.Status == "fail" {
			status.Warnings = append(status.Warnings, fmt.Sprintf("hooks_%s", hc.Status))
			if hc.Hint != "" {
				status.Actions = append(status.Actions, hc.Hint)
			}
		}
	}

	claudeMCP := checkClaudeMCP()
	if claudeMCP.Status != "" {
		status.MCP = &workCheck{
			Status:  claudeMCP.Status,
			Message: claudeMCP.Message,
			Hint:    claudeMCP.Hint,
		}
		if claudeMCP.Status == "warn" || claudeMCP.Status == "fail" {
			status.Warnings = append(status.Warnings, fmt.Sprintf("mcp_%s", claudeMCP.Status))
			if claudeMCP.Hint != "" {
				status.Actions = append(status.Actions, claudeMCP.Hint)
			}
		}
	}

	if status.Initialized {
		repo, repoErr := openRepo()
		if repoErr == nil {
			defer func() { _ = repo.Close() }()
			if activePlan, planErr := repo.GetActivePlan(); planErr == nil && activePlan != nil {
				completed := 0
				for _, t := range activePlan.Tasks {
					if t.Status == "completed" {
						completed++
					}
				}
				status.ActivePlan = &workPlanSummary{
					ID:        activePlan.ID,
					Completed: completed,
					Total:     len(activePlan.Tasks),
				}
			} else {
				status.Actions = append(status.Actions, "taskwing plan new \"your development goal\"")
			}
		}
	}

	if len(status.Actions) == 0 {
		status.Actions = append(status.Actions, "taskwing hook session-init")
		status.Actions = append(status.Actions, "claude")
	}

	return printJSON(status)
}
