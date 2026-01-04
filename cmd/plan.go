package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/planning"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/task"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage development plans",
	Long: `Create, view, and export development plans using AI agents.

Examples:
  # Create a new plan interactively
  tw plan new "Add OAuth2 authentication"

  # List all existing plans
  tw plan list

  # Export the most recent plan to a file
  tw plan export latest

  # Export a specific plan to a custom path
  tw plan export plan-123456 -o auth-plan.md

  # Export plan to stdout for piping
  tw plan export latest --stdout | pbcopy`,
}

var planNewCmd = &cobra.Command{
	Use:   "new \"Goal Description\"",
	Short: "Create a new plan from a goal",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		goal := args[0]

		cfg, err := getLLMConfig(cmd)
		if err != nil {
			return fmt.Errorf("llm config: %w", err)
		}

		repo, err := openRepo()
		if err != nil {
			return fmt.Errorf("open repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		// Initialize Agents
		clarifyingAgent := planning.NewClarifyingAgent(cfg)
		defer func() { _ = clarifyingAgent.Close() }()
		planningAgent := planning.NewPlanningAgent(cfg)
		defer func() { _ = planningAgent.Close() }()
		ks := knowledge.NewService(repo, cfg)

		// Create Streaming Output for "The Pulse"
		stream := core.NewStreamingOutput(100)
		defer stream.Close()

		// Initialize TUI Model
		model := ui.NewPlanModel(
			ctx,
			goal,
			clarifyingAgent,
			planningAgent,
			ks,
			repo,
			stream,
		)

		// Run TUI
		p := tea.NewProgram(model)
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("tui error: %w", err)
		}

		// Check result
		m, ok := finalModel.(ui.PlanModel)
		if !ok {
			return fmt.Errorf("internal error: invalid model type")
		}

		if m.State == ui.StateError {
			return m.Err
		}

		if m.State == ui.StateSuccess && m.PlanID != "" {
			// Fetch and print final plan
			createdPlan, err := repo.GetPlan(m.PlanID)
			if err != nil {
				return fmt.Errorf("fetch created plan: %w", err)
			}

			fmt.Println()
			printPlanView(createdPlan)

			noExport, _ := cmd.Flags().GetBool("no-export")
			exportPath, _ := cmd.Flags().GetString("export-path")
			if !noExport && !viper.GetBool("preview") {
				content := formatPlanMarkdown(createdPlan)
				output, err := exportPlanToFile(createdPlan, content, exportPath)
				if err != nil {
					return fmt.Errorf("export plan: %w", err)
				}
				if !isQuiet() && !isJSON() {
					fmt.Printf("\nSaved: %s (latest.md updated)\n", output)
				}
			}
		}

		return nil
	},
}

var planListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all plans",
	Long:  `List all development plans. Use --json for machine-readable output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		plans, err := repo.ListPlans()
		if err != nil {
			return err
		}

		// Handle JSON output
		if isJSON() {
			type planSummary struct {
				ID        string `json:"id"`
				Goal      string `json:"goal"`
				CreatedAt string `json:"created_at"`
				TaskCount int    `json:"task_count"`
			}
			var summaries []planSummary
			for _, p := range plans {
				tasks, _ := repo.ListTasks(p.ID)
				summaries = append(summaries, planSummary{
					ID:        p.ID,
					Goal:      p.Goal,
					CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
					TaskCount: len(tasks),
				})
			}
			return printJSON(summaries)
		}

		// Handle empty state
		if len(plans) == 0 {
			fmt.Println("No plans found.")
			fmt.Println("\nCreate one with: tw plan new \"Your goal\"")
			return nil
		}

		ui.RenderPageHeader("TaskWing Plan List", "")

		// Table header with proper formatting
		headerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Bold(true)
		idStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("75"))
		dateStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
		goalStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

		fmt.Printf("%-18s %-12s %-6s %s\n",
			headerStyle.Render("ID"),
			headerStyle.Render("CREATED"),
			headerStyle.Render("TASKS"),
			headerStyle.Render("GOAL"))

		for _, p := range plans {
			// Get task count
			tasks, _ := repo.ListTasks(p.ID)
			taskCount := len(tasks)

			// Truncate goal for display
			goal := p.Goal
			if len(goal) > 60 {
				goal = goal[:57] + "..."
			}

			fmt.Printf("%-18s %-12s %-6s %s\n",
				idStyle.Render(p.ID),
				dateStyle.Render(p.CreatedAt.Format("2006-01-02")),
				dateStyle.Render(fmt.Sprintf("%d", taskCount)),
				goalStyle.Render(goal))
		}

		fmt.Printf("\n%s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			fmt.Sprintf("Total: %d plan(s)", len(plans))))

		return nil
	},
}

// printPlanDetails prints a plan and its tasks in Markdown format.
// Used by both `plan new` (inline output) and `plan export`.
func printPlanDetails(plan *task.Plan) {
	fmt.Printf("# Plan: %s\n\n", plan.Goal)
	fmt.Printf("**Refined Goal**: %s\n\n", plan.EnrichedGoal)

	for _, t := range plan.Tasks {
		fmt.Printf("## Task: %s\n", t.Title)
		fmt.Printf("**Priority**: %d | **Agent**: %s\n\n", t.Priority, t.AssignedAgent)
		fmt.Printf("%s\n\n", t.Description)

		if len(t.AcceptanceCriteria) > 0 {
			fmt.Println("### Acceptance Criteria")
			for _, ac := range t.AcceptanceCriteria {
				fmt.Printf("- [ ] %s\n", ac)
			}
			fmt.Println()
		}

		if len(t.ValidationSteps) > 0 {
			fmt.Println("### Validation")
			fmt.Println("```bash")
			for _, vs := range t.ValidationSteps {
				fmt.Println(vs)
			}
			fmt.Println("```")
			fmt.Println()
		}
	}
}

// printPlanView renders a plan-first view, then next steps.
func printPlanView(plan *task.Plan) {
	taskCount := len(plan.Tasks)
	fmt.Printf("Plan: %s | %d tasks\n\n", plan.ID, taskCount)

	printPlanDetails(plan)

	fmt.Println("Next steps:")
	fmt.Println("  • tw task list --plan " + plan.ID)
	fmt.Println("  • tw context <query>")
}

var planExportCmd = &cobra.Command{
	Use:   "export [plan-id]",
	Short: "Export plan to Markdown",
	Long: `Export a plan to Markdown format in .taskwing/plans/.

By default, creates a semantically named file from the plan goal.
Use --stdout to print to stdout for piping to other tools.
Use --output to specify a custom file path.

Special values for plan-id:
  latest    Export the most recently created plan

Examples:
  tw plan export latest                    # Export to .taskwing/plans/
  tw plan export latest --stdout | pbcopy  # Copy to clipboard
  tw plan export plan-123 -o custom.md     # Custom path`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		// Resolve plan ID (support "latest" alias)
		planID := args[0]
		if planID == "latest" {
			planID, err = resolveLatestPlanID(repo)
			if err != nil {
				return err
			}
		}

		plan, err := repo.GetPlan(planID)
		if err != nil {
			return fmt.Errorf("plan not found: %s", planID)
		}

		content := formatPlanMarkdown(plan)
		taskCount := len(plan.Tasks)

		// Check flags
		toStdout, _ := cmd.Flags().GetBool("stdout")
		customOutput, _ := cmd.Flags().GetString("output")

		if toStdout {
			fmt.Print(content)
			return nil
		}

		output, err := exportPlanToFile(plan, content, customOutput)
		if err != nil {
			return err
		}

		// Success message
		fileSize := len(content)
		sizeStr := formatFileSize(fileSize)

		fmt.Println()
		fmt.Printf("✓ Plan exported to %s\n", output)
		fmt.Printf("  %d tasks | %s\n\n", taskCount, sizeStr)

		fmt.Println("Next steps:")
		fmt.Println("  • Open in your AI assistant and run /taskwing")
		fmt.Println("  • Or: tw task list --plan " + planID)
		return nil
	},
}

var planShowCmd = &cobra.Command{
	Use:   "show [plan-id]",
	Short: "Show a plan in the terminal",
	Long: `Print a plan to stdout for quick inspection.

Special values for plan-id:
  latest    Show the most recently created plan

Examples:
  tw plan show latest
  tw plan show plan-123456`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		planID := args[0]
		if planID == "latest" {
			plans, err := repo.ListPlans()
			if err != nil {
				return fmt.Errorf("list plans: %w", err)
			}
			if len(plans) == 0 {
				return fmt.Errorf("no plans found. Create one with: tw plan new \"Your goal\"")
			}
			planID = plans[0].ID
		}

		plan, err := repo.GetPlan(planID)
		if err != nil {
			return fmt.Errorf("plan not found: %s", planID)
		}

		printPlanView(plan)
		return nil
	},
}

var planDeleteCmd = &cobra.Command{
	Use:   "delete [plan-id]",
	Short: "Delete a plan and its tasks",
	Long: `Delete a plan by ID. Associated tasks are removed automatically.

Special values for plan-id:
  latest    Delete the most recently created plan

Examples:
  tw plan delete plan-123456
  tw plan delete latest --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		planID := args[0]
		if planID == "latest" {
			plans, err := repo.ListPlans()
			if err != nil {
				return fmt.Errorf("list plans: %w", err)
			}
			if len(plans) == 0 {
				return fmt.Errorf("no plans found. Create one with: tw plan new \"Your goal\"")
			}
			planID = plans[0].ID
		}

		plan, err := repo.GetPlan(planID)
		if err != nil {
			return fmt.Errorf("plan not found: %s", planID)
		}

		force, _ := cmd.Flags().GetBool("force")
		if !force && !isJSON() {
			fmt.Printf("\n  Plan:  %s\n", plan.ID)
			fmt.Printf("  Goal:  %s\n", plan.Goal)
			fmt.Printf("  Tasks: %d\n\n", len(plan.Tasks))
			if !confirmOrAbort("⚠️  Delete this plan and all its tasks? [y/N]: ") {
				return nil
			}
		}

		if err := repo.DeletePlan(planID); err != nil {
			return fmt.Errorf("delete plan: %w", err)
		}

		if isJSON() {
			return printJSON(deletedResponse{
				Status: "deleted",
				ID:     planID,
				Goal:   plan.Goal,
				Tasks:  len(plan.Tasks),
			})
		} else if !isQuiet() {
			fmt.Printf("✓ Deleted plan %s (%d tasks)\n", planID, len(plan.Tasks))
		}

		return nil
	},
}

var planUpdateCmd = &cobra.Command{
	Use:   "update [plan-id]",
	Short: "Update a plan",
	Long: `Update a plan's goal, enriched goal, or status.

Special values for plan-id:
  latest    Update the most recently created plan

Examples:
  tw plan update plan-123456 --goal "New goal"
  tw plan update latest --enriched-goal "Refined goal"
  tw plan update plan-123456 --status active`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		planID := args[0]
		goal, _ := cmd.Flags().GetString("goal")
		enrichedGoal, _ := cmd.Flags().GetString("enriched-goal")
		statusStr, _ := cmd.Flags().GetString("status")

		if goal == "" && enrichedGoal == "" && statusStr == "" {
			return fmt.Errorf("no fields to update")
		}

		var status task.PlanStatus
		if statusStr != "" {
			status = task.PlanStatus(statusStr)
			if !isValidPlanStatus(status) {
				return fmt.Errorf("invalid status: %s", statusStr)
			}
		}

		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		if planID == "latest" {
			planID, err = resolveLatestPlanID(repo)
			if err != nil {
				return err
			}
		}

		if err := repo.UpdatePlan(planID, goal, enrichedGoal, status); err != nil {
			return err
		}

		updated, err := repo.GetPlan(planID)
		if err != nil {
			return fmt.Errorf("fetch updated plan: %w", err)
		}

		if isJSON() {
			return printJSON(updated)
		}

		if !isQuiet() {
			fmt.Printf("✓ Updated plan %s\n", planID)
		}
		return nil
	},
}

var planRenameCmd = &cobra.Command{
	Use:   "rename [plan-id] \"new goal\"",
	Short: "Rename a plan goal",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		planID := args[0]
		newGoal := strings.TrimSpace(args[1])
		if newGoal == "" {
			return fmt.Errorf("new goal is required")
		}

		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		if planID == "latest" {
			planID, err = resolveLatestPlanID(repo)
			if err != nil {
				return err
			}
		}

		if err := repo.UpdatePlan(planID, newGoal, "", ""); err != nil {
			return err
		}

		if isJSON() {
			updated, _ := repo.GetPlan(planID)
			return printJSON(updated)
		}

		if !isQuiet() {
			fmt.Printf("✓ Renamed plan %s\n", planID)
		}
		return nil
	},
}

var planArchiveCmd = &cobra.Command{
	Use:   "archive [plan-id]",
	Short: "Archive a plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		planID := args[0]
		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		if planID == "latest" {
			planID, err = resolveLatestPlanID(repo)
			if err != nil {
				return err
			}
		}

		if err := repo.UpdatePlan(planID, "", "", task.PlanStatusArchived); err != nil {
			return err
		}

		if isJSON() {
			updated, _ := repo.GetPlan(planID)
			return printJSON(updated)
		}

		if !isQuiet() {
			fmt.Printf("✓ Archived plan %s\n", planID)
		}
		return nil
	},
}

var planUnarchiveCmd = &cobra.Command{
	Use:   "unarchive [plan-id]",
	Short: "Unarchive a plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		planID := args[0]
		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		if planID == "latest" {
			planID, err = resolveLatestPlanID(repo)
			if err != nil {
				return err
			}
		}

		if err := repo.UpdatePlan(planID, "", "", task.PlanStatusActive); err != nil {
			return err
		}

		if isJSON() {
			updated, _ := repo.GetPlan(planID)
			return printJSON(updated)
		}

		if !isQuiet() {
			fmt.Printf("✓ Unarchived plan %s\n", planID)
		}
		return nil
	},
}

var planStartCmd = &cobra.Command{
	Use:   "start [plan-id]",
	Short: "Set a plan as the active working plan",
	Long: `Set a plan as the currently active plan. This updates MCP context
so AI tools know what you're working on.

Special values for plan-id:
  latest    Use the most recently created plan

Examples:
  tw plan start plan-123456
  tw plan start latest`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		planID := args[0]
		if planID == "latest" {
			planID, err = resolveLatestPlanID(repo)
			if err != nil {
				return err
			}
		}

		plan, err := repo.GetPlan(planID)
		if err != nil {
			return fmt.Errorf("plan not found: %s", planID)
		}

		// Save active plan to state file
		if err := setActivePlan(planID); err != nil {
			return fmt.Errorf("save active plan: %w", err)
		}

		if isJSON() {
			return printJSON(map[string]any{
				"status":  "active",
				"plan_id": planID,
				"goal":    plan.Goal,
				"tasks":   len(plan.Tasks),
			})
		}

		if !isQuiet() {
			fmt.Printf("\n✓ Active plan: %s\n", planID)
			fmt.Printf("  Goal: %s\n", plan.Goal)
			fmt.Printf("  Tasks: %d\n\n", len(plan.Tasks))
			fmt.Println("Next steps:")
			fmt.Println("  • tw plan status          — Check progress")
			fmt.Println("  • tw task done <task-id>  — Mark tasks complete")
			fmt.Println("  • tw mcp                  — Start MCP server with plan context")
		}
		return nil
	},
}

var planStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current active plan and progress",
	Long:  `Display the currently active plan, task completion status, and progress.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planID, err := getActivePlan()
		if err != nil || planID == "" {
			if isJSON() {
				return printJSON(map[string]any{"status": "no_active_plan"})
			}
			fmt.Println("No active plan. Set one with: tw plan start <plan-id>")
			return nil
		}

		repo, err := openRepo()
		if err != nil {
			return err
		}
		defer func() { _ = repo.Close() }()

		plan, err := repo.GetPlan(planID)
		if err != nil {
			// Plan was deleted but state still references it
			_ = clearActivePlan()
			if isJSON() {
				return printJSON(map[string]any{"status": "plan_not_found", "plan_id": planID})
			}
			fmt.Printf("Active plan %s no longer exists. Cleared.\n", planID)
			return nil
		}

		tasks, _ := repo.ListTasks(planID)
		var done, total int
		total = len(tasks)
		for _, t := range tasks {
			if t.Status == task.StatusCompleted {
				done++
			}
		}

		if isJSON() {
			return printJSON(map[string]any{
				"plan_id":      planID,
				"goal":         plan.Goal,
				"tasks_total":  total,
				"tasks_done":   done,
				"progress_pct": float64(done) / float64(total) * 100,
			})
		}

		// Header
		progressPct := 0
		if total > 0 {
			progressPct = done * 100 / total
		}

		passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		pendingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

		fmt.Printf("\n📋 Active Plan: %s\n", planID)
		fmt.Printf("   %s\n\n", plan.Goal)

		// Progress bar
		barWidth := 30
		filled := barWidth * done / max(total, 1)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		fmt.Printf("   Progress: [%s] %d%% (%d/%d)\n\n", bar, progressPct, done, total)

		// Task list
		fmt.Println("   Tasks:")
		for _, t := range tasks {
			var status string
			title := t.Title
			if t.Status == task.StatusCompleted {
				status = passStyle.Render("[✓]")
				title = dimStyle.Render(title)
			} else {
				status = pendingStyle.Render("[ ]")
			}
			// Truncate task ID for display
			tid := t.ID
			if len(tid) > 12 {
				tid = tid[:12]
			}
			fmt.Printf("   %s %s %s\n", status, dimStyle.Render(tid), title)
		}

		fmt.Println()
		return nil
	},
}

// setActivePlan saves the active plan ID to state file
func setActivePlan(planID string) error {
	statePath := filepath.Join(config.GetMemoryBasePath(), "state.json")
	data := fmt.Sprintf(`{"active_plan": "%s"}`, planID)
	return os.WriteFile(statePath, []byte(data), 0644)
}

// getActivePlan reads the active plan ID from state file
func getActivePlan() (string, error) {
	statePath := filepath.Join(config.GetMemoryBasePath(), "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return "", err
	}
	// Simple parsing - extract plan ID
	var planID string
	_, _ = fmt.Sscanf(string(data), `{"active_plan": "%s"}`, &planID)
	planID = strings.Trim(planID, `"`)
	return planID, nil
}

// clearActivePlan removes the active plan state
func clearActivePlan() error {
	statePath := filepath.Join(config.GetMemoryBasePath(), "state.json")
	return os.Remove(statePath)
}

// formatPlanMarkdown returns the plan as a markdown string
func formatPlanMarkdown(plan *task.Plan) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("# Plan: %s\n\n", plan.Goal))
	buf.WriteString(fmt.Sprintf("**Refined Goal**: %s\n\n", plan.EnrichedGoal))

	for _, t := range plan.Tasks {
		buf.WriteString(fmt.Sprintf("## Task: %s\n", t.Title))
		buf.WriteString(fmt.Sprintf("**Priority**: %d | **Agent**: %s\n\n", t.Priority, t.AssignedAgent))
		buf.WriteString(fmt.Sprintf("%s\n\n", t.Description))

		if len(t.AcceptanceCriteria) > 0 {
			buf.WriteString("### Acceptance Criteria\n")
			for _, ac := range t.AcceptanceCriteria {
				buf.WriteString(fmt.Sprintf("- [ ] %s\n", ac))
			}
			buf.WriteString("\n")
		}

		if len(t.ValidationSteps) > 0 {
			buf.WriteString("### Validation\n```bash\n")
			for _, vs := range t.ValidationSteps {
				buf.WriteString(vs + "\n")
			}
			buf.WriteString("```\n\n")
		}
	}
	return buf.String()
}

func exportPlanToFile(plan *task.Plan, content, customOutput string) (string, error) {
	var output string
	var plansDir string

	if customOutput != "" {
		output = customOutput
	} else {
		plansDir = filepath.Join(config.GetMemoryBasePath(), "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			return "", fmt.Errorf("create plans directory: %w", err)
		}
		filename := generatePlanFilename(plan.Goal, plan.CreatedAt)
		output = filepath.Join(plansDir, filename)
	}

	if err := writeFile(output, content); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	if plansDir != "" {
		latestPath := filepath.Join(plansDir, "latest.md")
		_ = os.Remove(latestPath)
		if err := os.Symlink(filepath.Base(output), latestPath); err != nil {
			_ = writeFile(latestPath, content)
		}
	}

	return output, nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// formatFileSize returns a human-readable file size
func formatFileSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// generatePlanFilename creates a semantic filename from the plan goal
// Format: YYYY-MM-DD-slugified-goal.md
func generatePlanFilename(goal string, createdAt time.Time) string {
	// Date prefix
	dateStr := createdAt.Format("2006-01-02")

	// Slugify the goal
	slug := strings.ToLower(goal)

	// Replace common words and clean up
	slug = strings.ReplaceAll(slug, " and ", "-")
	slug = strings.ReplaceAll(slug, " or ", "-")
	slug = strings.ReplaceAll(slug, " the ", "-")
	slug = strings.ReplaceAll(slug, " a ", "-")
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove non-alphanumeric (keep hyphens)
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	slug = reg.ReplaceAllString(slug, "")

	// Clean up multiple hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	// Truncate if too long
	if len(slug) > 50 {
		slug = slug[:50]
		// Don't end with hyphen
		slug = strings.TrimRight(slug, "-")
	}

	return fmt.Sprintf("%s-%s.md", dateStr, slug)
}

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

	// Export flags
	planExportCmd.Flags().StringP("output", "o", "", "Output file path (e.g., plan.md)")
	planExportCmd.Flags().Bool("stdout", false, "Print to stdout instead of file (for piping)")
	planDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	planNewCmd.Flags().Bool("no-export", false, "Skip writing the plan to .taskwing/plans")
	planNewCmd.Flags().String("export-path", "", "Custom output path for plan export")
	planUpdateCmd.Flags().String("goal", "", "Update the plan goal")
	planUpdateCmd.Flags().String("enriched-goal", "", "Update the enriched goal")
	planUpdateCmd.Flags().String("status", "", "Update the plan status (draft, active, completed, archived)")
}
