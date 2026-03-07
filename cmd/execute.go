/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/runner"
	"github.com/josephgoksu/TaskWing/internal/task"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute tasks from the active plan using your AI CLI",
	Long: `Execute tasks by spawning your installed AI CLI (Claude Code, Gemini CLI, Codex CLI)
as a headless subprocess. No API keys needed — uses whatever AI CLI you already have.

The AI CLI gets full tool access to modify files, run commands, and implement changes.

Examples:
  taskwing execute              # Execute next pending task
  taskwing execute --all        # Execute all remaining tasks sequentially
  taskwing execute --dry-run    # Show what would be executed
  taskwing execute --max-tasks 3  # Execute up to 3 tasks then stop`,
	RunE: runExecute,
}

func init() {
	rootCmd.AddCommand(executeCmd)
	executeCmd.Flags().Bool("all", false, "Execute all remaining tasks sequentially")
	executeCmd.Flags().Bool("dry-run", false, "Show what would be executed without running")
	executeCmd.Flags().Int("max-tasks", 0, "Maximum number of tasks to execute (0 = unlimited)")
	executeCmd.Flags().String("prefer-cli", "", "Preferred AI CLI (claude, gemini, codex)")
	executeCmd.Flags().Duration("timeout", 10*time.Minute, "Timeout per task execution")
}

func runExecute(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	all, _ := cmd.Flags().GetBool("all")
	maxTasks, _ := cmd.Flags().GetInt("max-tasks")
	preferCLI, _ := cmd.Flags().GetString("prefer-cli")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// Open repo
	repo, err := openRepoOrHandleMissingMemory()
	if err != nil {
		return err
	}
	if repo == nil {
		return nil
	}
	defer func() { _ = repo.Close() }()

	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return fmt.Errorf("get memory path: %w", err)
	}
	svc := task.NewService(repo, memoryPath)

	// Get active plan
	planID, err := svc.GetActivePlanID()
	if err != nil || planID == "" {
		return fmt.Errorf("no active plan. Create one with: taskwing plan \"<description>\"")
	}

	plan, err := svc.GetPlanWithTasks(planID)
	if err != nil {
		return fmt.Errorf("load active plan: %w", err)
	}

	// Collect pending tasks
	allPending := filterPendingTasks(plan.Tasks)
	if len(allPending) == 0 {
		if !isQuiet() {
			ui.PrintSuccess("All tasks completed. Nothing to execute.")
		}
		return nil
	}

	if !isQuiet() {
		fmt.Printf("\n%s Active plan: %s (%d pending tasks)\n", ui.IconTask, plan.Goal, len(allPending))
	}

	// Apply max-tasks limit
	pendingTasks := allPending
	if !all && maxTasks == 0 {
		maxTasks = 1 // Default: execute one task
	}
	if maxTasks > 0 && len(pendingTasks) > maxTasks {
		pendingTasks = pendingTasks[:maxTasks]
	}

	// Dry run: just show what would execute
	if dryRun {
		return showDryRun(pendingTasks)
	}

	// Detect AI CLI runner
	cliRunner, err := runner.PreferredRunner(runner.CLIType(preferCLI))
	if err != nil {
		return fmt.Errorf("no AI CLI found: %w\nInstall Claude Code, Gemini CLI, or Codex CLI", err)
	}

	if !isQuiet() {
		fmt.Printf("%s Using %s for execution\n\n", ui.IconRobot, cliRunner.Type().String())
	}

	cwd, _ := os.Getwd()

	// Execute tasks sequentially
	for i, t := range pendingTasks {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if !isQuiet() {
			fmt.Printf("[%d/%d] %s\n", i+1, len(pendingTasks), t.Title)
			fmt.Printf("  [%s working...]\n", cliRunner.Type().String())
		}

		// Mark as in_progress
		if err := repo.UpdateTaskStatus(t.ID, task.StatusInProgress); err != nil {
			return fmt.Errorf("update task status: %w", err)
		}

		// Build execution prompt
		prompt := runner.TaskExecutionPrompt(
			t.Title,
			t.Description,
			t.AcceptanceCriteria,
			t.ContextSummary,
			t.ValidationSteps,
		)

		// Execute via AI CLI with file access
		result, err := cliRunner.InvokeWithFiles(ctx, runner.InvokeRequest{
			Prompt:  prompt,
			WorkDir: cwd,
			Timeout: timeout,
		})

		if err != nil {
			// Mark as failed
			_ = repo.UpdateTaskStatus(t.ID, task.StatusFailed)
			if !isQuiet() {
				fmt.Printf("  %s Failed: %v\n\n", ui.IconFail, err)
			}
			return fmt.Errorf("task %q failed: %w", t.Title, err)
		}

		// Try to parse execution output
		var execOutput runner.ExecuteOutput
		if decErr := result.Decode(&execOutput); decErr != nil {
			// Even if we can't parse the output, the task may have succeeded
			// (the AI CLI may not have output valid JSON)
			if !isQuiet() {
				fmt.Printf("  %s Complete (output not parseable)\n\n", ui.IconOK)
			}
			_ = repo.UpdateTaskStatus(t.ID, task.StatusCompleted)
			continue
		}

		switch execOutput.Status {
		case "completed":
			_ = repo.UpdateTaskStatus(t.ID, task.StatusCompleted)
			if !isQuiet() {
				fmt.Printf("  %s Complete: %s\n\n", ui.IconOK, execOutput.Summary)
			}
		case "partial":
			_ = repo.UpdateTaskStatus(t.ID, task.StatusInProgress)
			if !isQuiet() {
				fmt.Printf("  %s Partial: %s\n\n", ui.IconPartial, execOutput.Summary)
			}
			return fmt.Errorf("task %q partially completed: %s", t.Title, execOutput.Summary)
		default: // "failed" or unknown
			_ = repo.UpdateTaskStatus(t.ID, task.StatusFailed)
			if !isQuiet() {
				fmt.Printf("  %s Failed: %s\n\n", ui.IconFail, execOutput.Error)
			}
			return fmt.Errorf("task %q failed: %s", t.Title, execOutput.Error)
		}
	}

	if !isQuiet() {
		ui.PrintSuccess(fmt.Sprintf("%d task(s) completed.", len(pendingTasks)))
	}

	return nil
}

// filterPendingTasks returns tasks that are ready to execute (pending or ready status).
func filterPendingTasks(tasks []task.Task) []task.Task {
	var pending []task.Task
	for _, t := range tasks {
		if t.Status == task.StatusPending || t.Status == task.StatusReady {
			pending = append(pending, t)
		}
	}
	return pending
}

// showDryRun displays what would be executed without running.
func showDryRun(tasks []task.Task) error {
	// Show detected CLIs
	detected := runner.DetectCLIs()
	if len(detected) > 0 {
		fmt.Println("\nDetected AI CLIs:")
		for _, d := range detected {
			fmt.Printf("  • %s (%s)\n", d.Type.String(), d.BinaryPath)
		}
	} else {
		fmt.Println()
		ui.PrintWarning("No AI CLIs detected")
	}

	fmt.Printf("\nTasks to execute (%d):\n", len(tasks))
	for i, t := range tasks {
		fmt.Printf("  %d. %s\n", i+1, t.Title)
		fmt.Printf("     Complexity: %s | Agent: %s\n", t.Complexity, t.AssignedAgent)
		if len(t.AcceptanceCriteria) > 0 {
			fmt.Printf("     Criteria: %d items\n", len(t.AcceptanceCriteria))
		}
	}
	fmt.Printf("\n%s Run without --dry-run to execute.\n", ui.IconHint)
	return nil
}
