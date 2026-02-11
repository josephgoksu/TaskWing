package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/logger"
	"github.com/josephgoksu/TaskWing/internal/task"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/josephgoksu/TaskWing/internal/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.AddCommand(planNewCmd)
	planCmd.AddCommand(planListCmd)
	planCmd.AddCommand(planExportCmd)
	planCmd.AddCommand(planShowCmd)
	planCmd.AddCommand(planDeleteCmd)
	planCmd.AddCommand(planUpdateCmd)
	planCmd.AddCommand(planRenameCmd)
	planCmd.AddCommand(planArchiveCmd)
	planCmd.AddCommand(planUnarchiveCmd)
	planCmd.AddCommand(planStartCmd)
	planCmd.AddCommand(planStatusCmd)

	// Flags
	planNewCmd.Flags().Bool("no-export", false, "Skip automatic export")
	planNewCmd.Flags().String("export-path", "", "Custom path to export plan")
	planNewCmd.Flags().Bool("non-interactive", false, "Run without user interaction (headless)")
	planNewCmd.Flags().Bool("offline", false, "Disable LLM usage (create a draft plan without tasks)")
	planNewCmd.Flags().Bool("no-llm", false, "Alias for --offline")

	planExportCmd.Flags().Bool("stdout", false, "Print to stdout")
	planExportCmd.Flags().StringP("output", "o", "", "Custom output path")

	planDeleteCmd.Flags().Bool("force", false, "Skip confirmation")

	planUpdateCmd.Flags().String("goal", "", "Update goal")
	planUpdateCmd.Flags().String("enriched-goal", "", "Update enriched goal")
	planUpdateCmd.Flags().String("status", "", "Update status")

	// List flags
	planListCmd.Flags().StringP("query", "q", "", "Filter by goal/enriched goal")
	planListCmd.Flags().StringP("status", "s", "", "Filter by status (draft, active, completed, verified, needs_revision, archived)")
}

// Wrapper to handle repo lifecycle automatically
func runWithService(runFunc func(svc *task.Service, cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
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
		return runFunc(svc, cmd, args)
	}
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage development plans",
	Long: `Create, view, and export development plans using AI agents.

Examples:
  taskwing goal "Add OAuth2 authentication"
  taskwing plan list
  taskwing plan export latest
  taskwing plan start latest`,
}

var planNewCmd = &cobra.Command{
	Use:   "new \"Goal Description\"",
	Short: "Create a new plan from a goal",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		goal := args[0]

		// Track user input for crash logging
		logger.SetLastInput(fmt.Sprintf("plan new %q", goal))

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

		offline, _ := cmd.Flags().GetBool("offline")
		noLLM, _ := cmd.Flags().GetBool("no-llm")
		if noLLM {
			offline = true
		}
		if offline {
			if !isQuiet() && !isJSON() {
				fmt.Fprintln(os.Stderr, "⚠️  Offline mode: LLM disabled. Creating a draft plan without tasks.")
			}
			draft := &task.Plan{
				Goal:         goal,
				EnrichedGoal: goal,
				Status:       task.PlanStatusDraft,
				Tasks:        []task.Task{},
			}
			if err := repo.CreatePlan(draft); err != nil {
				return fmt.Errorf("create draft plan: %w", err)
			}
			createdPlan, err := svc.GetPlanWithTasks(draft.ID)
			if err != nil {
				return fmt.Errorf("fetch created plan: %w", err)
			}
			fmt.Println()
			printPlanView(createdPlan)

			noExport, _ := cmd.Flags().GetBool("no-export")
			exportPath, _ := cmd.Flags().GetString("export-path")
			if !noExport && !viper.GetBool("preview") {
				outputPath, err := svc.ExportPlanToFile(createdPlan, exportPath)
				if err != nil {
					return fmt.Errorf("export plan: %w", err)
				}
				if !isQuiet() && !isJSON() {
					fmt.Printf("\nSaved: %s\n", outputPath)
				}
			}
			return nil
		}

		cfg, err := getLLMConfigForRole(cmd, llm.RoleBootstrap)
		if err != nil {
			return fmt.Errorf("llm config: %w", err)
		}

		// Initialize App Layer
		// Agents are now managed internally by PlanApp methods
		appCtx := app.NewContextWithConfig(repo, cfg)
		planApp := app.NewPlanApp(appCtx)

		nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
		if !nonInteractive && !hasTTY() {
			nonInteractive = true
			if !isQuiet() && !isJSON() {
				fmt.Fprintln(os.Stderr, "⚠️  No TTY detected; falling back to --non-interactive")
			}
		}
		if nonInteractive {
			// Headless Flow
			fmt.Printf("Analyzing goal: %q...\n", goal)

			clarifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			clarifyRes, err := planApp.Clarify(clarifyCtx, app.ClarifyOptions{
				Goal:       goal,
				AutoAnswer: true,
			})
			if err != nil {
				if errors.Is(clarifyCtx.Err(), context.DeadlineExceeded) {
					return fmt.Errorf("clarification timed out after 2 minutes")
				}
				return fmt.Errorf("clarification error: %w", err)
			}
			if !clarifyRes.Success {
				return fmt.Errorf("clarification failed: %s", clarifyRes.Message)
			}
			fmt.Printf("Goal refined: %s\nGenerating plan...\n", clarifyRes.GoalSummary)

			genCtx, genCancel := context.WithTimeout(ctx, 2*time.Minute)
			defer genCancel()
			genRes, err := planApp.Generate(genCtx, app.GenerateOptions{
				Goal:         goal,
				EnrichedGoal: clarifyRes.EnrichedGoal,
				Save:         true,
			})
			if err != nil {
				if errors.Is(genCtx.Err(), context.DeadlineExceeded) {
					return fmt.Errorf("plan generation timed out after 2 minutes")
				}
				return fmt.Errorf("generation error: %w", err)
			}
			if !genRes.Success {
				return fmt.Errorf("generation failed: %s", genRes.Message)
			}

			// Reuse success logic
			createdPlan, err := svc.GetPlanWithTasks(genRes.PlanID)
			if err != nil {
				return fmt.Errorf("fetch created plan: %w", err)
			}
			fmt.Println()
			printPlanView(createdPlan)

			if !isQuiet() && !isJSON() {
				if len(genRes.SemanticWarnings) > 0 || len(genRes.SemanticErrors) > 0 {
					fmt.Println()
					fmt.Printf("Semantic validation (non-blocking): %d warning(s), %d error(s)\n",
						len(genRes.SemanticWarnings), len(genRes.SemanticErrors))
					for _, w := range genRes.SemanticWarnings {
						fmt.Printf("  ⚠ %s\n", w)
					}
					for _, e := range genRes.SemanticErrors {
						fmt.Printf("  ⚠ %s\n", e)
					}
				}
			}

			// Export logic
			noExport, _ := cmd.Flags().GetBool("no-export")
			exportPath, _ := cmd.Flags().GetString("export-path")
			if !noExport && !viper.GetBool("preview") {
				outputPath, err := svc.ExportPlanToFile(createdPlan, exportPath)
				if err != nil {
					return fmt.Errorf("export plan: %w", err)
				}
				if !isQuiet() && !isJSON() {
					fmt.Printf("\nSaved: %s\n", outputPath)
				}
			}
			return nil
		}

		ks := knowledge.NewService(repo, cfg)

		stream := core.NewStreamingOutput(100)
		defer stream.Close()

		model := ui.NewPlanModel(
			ctx,
			goal,
			planApp,
			ks,
			repo,
			stream,
			memoryPath,
		)

		p := tea.NewProgram(model)
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("tui error: %w", err)
		}

		m, ok := finalModel.(ui.PlanModel)
		if !ok || m.State == ui.StateError {
			if m.State == ui.StateError {
				return m.Err
			}
			return fmt.Errorf("internal error: invalid model type")
		}

		if m.State == ui.StateSuccess && m.PlanID != "" {
			createdPlan, err := svc.GetPlanWithTasks(m.PlanID)
			if err != nil {
				return fmt.Errorf("fetch created plan: %w", err)
			}

			fmt.Println()
			printPlanView(createdPlan)

			noExport, _ := cmd.Flags().GetBool("no-export")
			exportPath, _ := cmd.Flags().GetString("export-path")
			if !noExport && !viper.GetBool("preview") {
				outputPath, err := svc.ExportPlanToFile(createdPlan, exportPath)
				if err != nil {
					return fmt.Errorf("export plan: %w", err)
				}
				if !isQuiet() && !isJSON() {
					fmt.Printf("\nSaved: %s\n", outputPath)
				}
			}
		}

		return nil
	},
}

func hasTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

var planListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all plans",
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")
		statusStr, _ := cmd.Flags().GetString("status")

		// If using positional args for query
		if len(args) > 0 {
			query = strings.Join(args, " ")
		}

		var plans []task.Plan
		var err error

		if query != "" || statusStr != "" {
			plans, err = svc.SearchPlans(query, task.PlanStatus(statusStr))
		} else {
			plans, err = svc.ListPlans()
		}

		if err != nil {
			return err
		}

		if isJSON() {
			return printJSON(plans)
		}

		if len(plans) == 0 {
			fmt.Println("No plans found matching criteria.")
			return nil
		}

		ui.RenderPageHeader("TaskWing Plan List", "")
		printPlanTable(plans)
		return nil
	}),
}

func printPlanTable(plans []task.Plan) {
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Bold(true)
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	goalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

	fmt.Printf("%-18s %-12s %-6s %s\n",
		headerStyle.Render("ID"), headerStyle.Render("CREATED"), headerStyle.Render("TASKS"), headerStyle.Render("GOAL"))

	for _, p := range plans {
		goal := p.Goal
		if len(goal) > 60 {
			goal = goal[:57] + "..."
		}
		// Tasks count - service ListPlans probably returns plans without tasks or with?
		// ListPlans sets TaskCount but leaves Tasks nil for efficiency.
		// Use GetTaskCount() to get the count regardless of how the plan was loaded.
		fmt.Printf("%-18s %-12s %-6d %s\n",
			idStyle.Render(p.ID),
			dateStyle.Render(p.CreatedAt.Format("2006-01-02")),
			p.GetTaskCount(),
			goalStyle.Render(goal))
	}
	fmt.Printf("\n%s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(fmt.Sprintf("Total: %d plan(s)", len(plans))))
}

var planExportCmd = &cobra.Command{
	Use:   "export [plan-id]",
	Short: "Export plan to Markdown",
	Args:  cobra.ExactArgs(1),
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		plan, err := svc.GetPlanWithTasks(args[0])
		if err != nil {
			return err
		}

		toStdout, _ := cmd.Flags().GetBool("stdout")
		if toStdout {
			fmt.Print(svc.FormatPlanMarkdown(plan))
			return nil
		}

		customOutput, _ := cmd.Flags().GetString("output")
		outputPath, err := svc.ExportPlanToFile(plan, customOutput)
		if err != nil {
			return err
		}

		fmt.Printf("\n✓ Plan exported to %s\n", outputPath)
		return nil
	}),
}

var planShowCmd = &cobra.Command{
	Use:   "show [plan-id]",
	Short: "Show a plan in the terminal",
	Args:  cobra.ExactArgs(1),
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		plan, err := svc.GetPlanWithTasks(args[0])
		if err != nil {
			return err
		}
		printPlanView(plan)
		return nil
	}),
}

var planDeleteCmd = &cobra.Command{
	Use:   "delete [plan-id]",
	Short: "Delete a plan and its tasks",
	Args:  cobra.ExactArgs(1),
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		planID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force && !isJSON() {
			plan, err := svc.GetPlan(planID)
			if err == nil {
				fmt.Printf("Delete plan %s: \"%s\"? [y/N]: ", plan.ID, plan.Goal)
				if !confirmOrAbort("") {
					return nil
				}
			}
		}

		if err := svc.DeletePlan(planID); err != nil {
			return err
		}

		if !isQuiet() && !isJSON() {
			fmt.Printf("✓ Deleted plan %s\n", planID)
		}
		return nil
	}),
}

var planUpdateCmd = &cobra.Command{
	Use:   "update [plan-id]",
	Short: "Update a plan",
	Args:  cobra.ExactArgs(1),
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		goal, _ := cmd.Flags().GetString("goal")
		enrichedGoal, _ := cmd.Flags().GetString("enriched-goal")
		statusStr, _ := cmd.Flags().GetString("status")

		var status task.PlanStatus
		if statusStr != "" {
			status = task.PlanStatus(statusStr)
		}

		if err := svc.UpdatePlan(args[0], goal, enrichedGoal, status); err != nil {
			return err
		}

		if !isQuiet() && !isJSON() {
			fmt.Printf("✓ Updated plan %s\n", args[0])
		}
		return nil
	}),
}

var planRenameCmd = &cobra.Command{
	Use:   "rename [plan-id] \"new goal\"",
	Short: "Rename a plan goal",
	Args:  cobra.ExactArgs(2),
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		if err := svc.RenamePlan(args[0], args[1]); err != nil {
			return err
		}
		if !isQuiet() {
			fmt.Printf("✓ Renamed plan %s\n", args[0])
		}
		return nil
	}),
}

var planArchiveCmd = &cobra.Command{
	Use:   "archive [plan-id]",
	Short: "Archive a plan",
	Args:  cobra.ExactArgs(1),
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		if err := svc.ArchivePlan(args[0]); err != nil {
			return err
		}
		if !isQuiet() {
			fmt.Printf("✓ Archived plan %s\n", args[0])
		}
		return nil
	}),
}

var planUnarchiveCmd = &cobra.Command{
	Use:   "unarchive [plan-id]",
	Short: "Unarchive a plan",
	Args:  cobra.ExactArgs(1),
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		if err := svc.UnarchivePlan(args[0]); err != nil {
			return err
		}
		if !isQuiet() {
			fmt.Printf("✓ Unarchived plan %s\n", args[0])
		}
		return nil
	}),
}

var planStartCmd = &cobra.Command{
	Use:   "start [plan-id]",
	Short: "Set a plan as the active working plan",
	Args:  cobra.ExactArgs(1),
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		if err := svc.SetActivePlan(args[0]); err != nil {
			return err
		}

		plan, _ := svc.GetPlanWithTasks(args[0]) // Get resolved plan details
		if !isQuiet() {
			fmt.Printf("\n✓ Active plan: %s\n", plan.ID)
			fmt.Printf("  Goal: %s\n", plan.Goal)
			fmt.Printf("  Tasks: %d\n\n", len(plan.Tasks))
		}
		return nil
	}),
}

var planStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current active plan and progress",
	Long: `Show the status of the current active plan including progress.

Displays the plan goal, task counts, and completion percentage.

Examples:
  taskwing plan status
  taskwing plan status --json`,
	RunE: runWithService(func(svc *task.Service, cmd *cobra.Command, args []string) error {
		planID, err := svc.GetActivePlanID()
		if err != nil || planID == "" {
			if isJSON() {
				return printJSON(map[string]any{
					"success": false,
					"message": "No active plan",
				})
			}
			fmt.Println("No active plan. Set one with: taskwing goal \"<goal>\"")
			return nil
		}

		plan, err := svc.GetPlanWithTasks(planID)
		if err != nil {
			_ = svc.ClearActivePlan()
			if isJSON() {
				return printJSON(map[string]any{
					"success": false,
					"message": fmt.Sprintf("Active plan %s no longer exists", planID),
				})
			}
			fmt.Printf("Active plan %s no longer exists. Cleared.\n", planID)
			return nil
		}

		if isJSON() {
			return printPlanStatusJSON(plan)
		}

		printStatus(plan)
		return nil
	}),
}

// PlanStatusResponse is the JSON response for plan status
type PlanStatusResponse struct {
	Success     bool   `json:"success"`
	PlanID      string `json:"plan_id"`
	Goal        string `json:"goal"`
	Status      string `json:"status"`
	Total       int    `json:"total_tasks"`
	Completed   int    `json:"completed_tasks"`
	Pending     int    `json:"pending_tasks"`
	InProgress  int    `json:"in_progress_tasks"`
	ProgressPct int    `json:"progress_percent"`
}

func printPlanStatusJSON(plan *task.Plan) error {
	completed := 0
	inProgress := 0
	pending := 0
	for _, t := range plan.Tasks {
		switch t.Status {
		case task.StatusCompleted:
			completed++
		case task.StatusInProgress:
			inProgress++
		default:
			pending++
		}
	}

	total := len(plan.Tasks)
	progressPct := 0
	if total > 0 {
		progressPct = completed * 100 / total
	}

	return printJSON(PlanStatusResponse{
		Success:     true,
		PlanID:      plan.ID,
		Goal:        plan.Goal,
		Status:      string(plan.Status),
		Total:       total,
		Completed:   completed,
		Pending:     pending,
		InProgress:  inProgress,
		ProgressPct: progressPct,
	})
}

func printStatus(plan *task.Plan) {
	done := 0
	total := len(plan.Tasks)
	for _, t := range plan.Tasks {
		if t.Status == task.StatusCompleted {
			done++
		}
	}

	progressPct := 0
	if total > 0 {
		progressPct = done * 100 / total
	}

	fmt.Printf("\n📋 Active Plan: %s\n", plan.ID)
	fmt.Printf("   %s\n\n", plan.Goal)

	barWidth := 30
	filled := barWidth * done / max(total, 1)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Printf("   Progress: [%s] %d%% (%d/%d)\n\n", bar, progressPct, done, total)

	passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	pendingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	fmt.Println("   Tasks:")
	for _, t := range plan.Tasks {
		statusMarker := pendingStyle.Render("[ ]")
		title := t.Title

		if t.Status == task.StatusCompleted {
			statusMarker = passStyle.Render("[✓]")
			title = dimStyle.Render(title)
		}

		// Use ShortID for consistent task ID display
		tid := util.ShortID(t.ID, util.TaskIDLength)
		fmt.Printf("   %s %s %s\n", statusMarker, dimStyle.Render(tid), title)
	}
	fmt.Println()
}

func printPlanView(plan *task.Plan) {
	taskCount := len(plan.Tasks)
	fmt.Printf("Plan: %s | %d tasks\n\n", plan.ID, taskCount)

	fmt.Printf("# Plan: %s\n\n", plan.Goal)
	fmt.Printf("**Refined Goal**: %s\n\n", plan.EnrichedGoal)

	for _, t := range plan.Tasks {
		fmt.Printf("## Task: %s\n", t.Title)
		fmt.Printf("**Priority**: %d | **Agent**: %s\n\n", t.Priority, t.AssignedAgent)
		fmt.Printf("%s\n\n", t.Description)
	}

	fmt.Println("Next steps:")
	fmt.Println("  • taskwing task list --plan " + plan.ID)
	fmt.Println("  • /tw-next")
}
