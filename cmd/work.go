/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

		// Run bootstrap
		bootstrapCmd := exec.Command(os.Args[0], "bootstrap")
		bootstrapCmd.Dir = cwd
		bootstrapCmd.Stdin = os.Stdin
		bootstrapCmd.Stdout = os.Stdout
		bootstrapCmd.Stderr = os.Stderr
		if err := bootstrapCmd.Run(); err != nil {
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
		if err := installHooksConfig(cwd, "claude", false); err != nil {
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
		// Create new plan with provided goal
		fmt.Printf("\n📋 Creating plan: %s\n", planGoal)
		planCmd := exec.Command(os.Args[0], "plan", "new", planGoal)
		planCmd.Dir = cwd
		planCmd.Stdin = os.Stdin
		planCmd.Stdout = os.Stdout
		planCmd.Stderr = os.Stderr
		if err := planCmd.Run(); err != nil {
			return fmt.Errorf("plan creation failed: %w", err)
		}

		// Start the new plan
		startCmd := exec.Command(os.Args[0], "plan", "start", "latest")
		startCmd.Dir = cwd
		startCmd.Stdout = os.Stdout
		startCmd.Stderr = os.Stderr
		if err := startCmd.Run(); err != nil {
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
				planCmd := exec.Command(os.Args[0], "plan", "new", goal)
				planCmd.Dir = cwd
				planCmd.Stdin = os.Stdin
				planCmd.Stdout = os.Stdout
				planCmd.Stderr = os.Stderr
				if err := planCmd.Run(); err != nil {
					return fmt.Errorf("plan creation failed: %w", err)
				}

				startCmd := exec.Command(os.Args[0], "plan", "start", "latest")
				startCmd.Dir = cwd
				startCmd.Stdout = os.Stdout
				startCmd.Stderr = os.Stderr
				_ = startCmd.Run()

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
