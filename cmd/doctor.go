/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/task"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check TaskWing setup and diagnose issues",
	Long: `Validate your TaskWing installation and configuration.

Checks:
  • TaskWing initialization (.taskwing/ directory)
  • MCP server registration for AI tools
  • Hooks configuration for autonomous execution
  • Active plan and task status
  • Session state

Use this to troubleshoot issues or verify setup after bootstrap.

Repair mode:
  • --fix applies an explicit repair plan
  • --adopt-unmanaged allows claiming unmanaged TaskWing-like AI configs (with backup)
  • --yes is required for non-interactive global/adoption mutations`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor(cmd)
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().Bool("fix", false, "Automatically apply safe repairs for detected integration drift")
	doctorCmd.Flags().Bool("yes", false, "Auto-confirm prompts (required for non-interactive fix flows)")
	doctorCmd.Flags().Bool("adopt-unmanaged", false, "Allow adopting unmanaged TaskWing-like configs before repair")
	doctorCmd.Flags().String("ai", "", "Comma-separated AI list to target during repair (e.g., claude,codex)")
	doctorCmd.Flags().Bool("dry-run", false, "Show planned repairs without mutating files/config")
}

// DoctorCheck represents a single diagnostic check
type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warn", "fail"
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// DoctorResult is the JSON output structure for doctor command
type DoctorResult struct {
	Status         string                   `json:"status"` // "ok", "warn", "fail"
	Checks         []DoctorCheck            `json:"checks"`
	Errors         int                      `json:"errors"`
	Warnings       int                      `json:"warnings"`
	RepairPlan     []bootstrap.RepairAction `json:"repair_plan,omitempty"`
	AppliedRepairs []bootstrap.RepairAction `json:"applied_repairs,omitempty"`
	SkippedRepairs []bootstrap.RepairAction `json:"skipped_repairs,omitempty"`
	BlockedRepairs []bootstrap.RepairAction `json:"blocked_repairs,omitempty"`
}

type doctorFixOptions struct {
	Fix            bool
	Yes            bool
	AdoptUnmanaged bool
	DryRun         bool
	TargetAIs      []string
}

func evaluateDoctorState(cwd string) ([]DoctorCheck, map[string]bootstrap.IntegrationReport, bool, bool, int, int) {
	checks := []DoctorCheck{}

	// Check 1: TaskWing initialized
	checks = append(checks, checkTaskWingInit(cwd))

	// Check 2: Active plan
	checks = append(checks, checkActivePlan())

	// Check 3: Session state
	checks = append(checks, checkSession())

	// Check 4: Shared integration evaluator (source of truth for bootstrap + doctor repair)
	globalMap := makeGlobalMCPMap(detectExistingMCPConfigs())
	reports := bootstrap.EvaluateIntegrations(cwd, globalMap)
	checks = append(checks, checksFromIntegrationReports(reports)...)

	hasErrors := false
	hasWarnings := false
	errorCount := 0
	warningCount := 0
	for _, c := range checks {
		switch c.Status {
		case "warn":
			hasWarnings = true
			warningCount++
		case "fail":
			hasErrors = true
			errorCount++
		}
	}

	return checks, reports, hasErrors, hasWarnings, errorCount, warningCount
}

func runDoctor(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}
	opts := doctorFixOptions{
		Fix:            getBoolFlag(cmd, "fix"),
		Yes:            getBoolFlag(cmd, "yes"),
		AdoptUnmanaged: getBoolFlag(cmd, "adopt-unmanaged"),
		DryRun:         getBoolFlag(cmd, "dry-run"),
		TargetAIs:      parseCSVFlag(getStringFlag(cmd, "ai")),
	}

	checks, reports, hasErrors, hasWarnings, errorCount, warningCount := evaluateDoctorState(cwd)

	var repairPlan []bootstrap.RepairAction
	var appliedRepairs []bootstrap.RepairAction
	var skippedRepairs []bootstrap.RepairAction
	var blockedRepairs []bootstrap.RepairAction

	if opts.Fix {
		built := bootstrap.BuildRepairPlan(reports, bootstrap.RepairPlanOptions{
			TargetAIs:                opts.TargetAIs,
			IncludeGlobalMutations:   true,
			IncludeUnmanagedAdoption: opts.AdoptUnmanaged,
		})
		repairPlan = built.Actions
		appliedRepairs, skippedRepairs, blockedRepairs, err = applyRepairPlan(cwd, built, opts)
		if err != nil {
			return err
		}

		// Re-evaluate after apply (unless dry-run) to reflect final health.
		if !opts.DryRun {
			checks, _, hasErrors, hasWarnings, errorCount, warningCount = evaluateDoctorState(cwd)
		}
	}

	// JSON output
	if isJSON() {
		status := "ok"
		if hasErrors {
			status = "fail"
		} else if hasWarnings {
			status = "warn"
		}
		return printJSON(DoctorResult{
			Status:         status,
			Checks:         checks,
			Errors:         errorCount,
			Warnings:       warningCount,
			RepairPlan:     repairPlan,
			AppliedRepairs: appliedRepairs,
			SkippedRepairs: skippedRepairs,
			BlockedRepairs: blockedRepairs,
		})
	}

	// Human-readable output
	fmt.Println("🩺 TaskWing Doctor")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Print all checks
	for _, c := range checks {
		printCheck(c)
	}

	if opts.Fix {
		fmt.Println()
		fmt.Println("🔧 Repair Summary")
		fmt.Printf("   Planned: %d\n", len(repairPlan))
		fmt.Printf("   Applied: %d\n", len(appliedRepairs))
		fmt.Printf("   Skipped: %d\n", len(skippedRepairs))
		fmt.Printf("   Blocked: %d\n", len(blockedRepairs))
		for _, action := range blockedRepairs {
			fmt.Printf("   ⊘ %s/%s: %s\n", action.AI, action.Component, action.Reason)
		}
		for _, action := range skippedRepairs {
			fmt.Printf("   ⊘ %s/%s: %s\n", action.AI, action.Component, action.Reason)
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Summary and next steps
	if hasErrors {
		fmt.Println("❌ Issues found. Fix the errors above before continuing.")
	} else if hasWarnings {
		fmt.Println("⚠️  Warnings found. Review the warnings above.")
		printNextSteps(checks)
	} else {
		fmt.Println("✅ Everything looks good!")
		printNextSteps(checks)
	}

	return nil
}

func parseCSVFlag(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func makeGlobalMCPMap(ais []string) map[string]bool {
	out := make(map[string]bool, len(ais))
	for _, ai := range ais {
		out[ai] = true
	}
	return out
}

func printCheck(c DoctorCheck) {
	var icon string
	switch c.Status {
	case "ok":
		icon = "✅"
	case "warn":
		icon = "⚠️ "
	case "fail":
		icon = "❌"
	}

	fmt.Printf("%s %s: %s\n", icon, c.Name, c.Message)
	if c.Hint != "" && c.Status != "ok" {
		fmt.Printf("   └─ %s\n", c.Hint)
	}
}

func checksFromIntegrationReports(reports map[string]bootstrap.IntegrationReport) []DoctorCheck {
	ais := make([]string, 0, len(reports))
	for ai := range reports {
		ais = append(ais, ai)
	}
	sort.Strings(ais)

	checks := make([]DoctorCheck, 0)
	for _, ai := range ais {
		report := reports[ai]
		if len(report.Issues) == 0 {
			checks = append(checks, DoctorCheck{
				Name:    fmt.Sprintf("Integration (%s)", ai),
				Status:  "ok",
				Message: "Healthy",
			})
			continue
		}
		for _, issue := range report.Issues {
			status := "warn"
			if issue.Status == bootstrap.ComponentStatusInvalid {
				status = "fail"
			}
			hint := fmt.Sprintf("Run: taskwing doctor --fix --ai %s", ai)
			if issue.AdoptRequired {
				hint = fmt.Sprintf("Run: taskwing doctor --fix --adopt-unmanaged --ai %s", ai)
			}
			if issue.MutatesGlobal {
				hint = fmt.Sprintf("Run: taskwing doctor --fix --yes --ai %s", ai)
			}
			checks = append(checks, DoctorCheck{
				Name:    fmt.Sprintf("Integration (%s/%s)", ai, issue.Component),
				Status:  status,
				Message: issue.Reason,
				Hint:    hint,
			})
		}
	}
	return checks
}

func applyRepairPlan(cwd string, plan bootstrap.RepairPlan, opts doctorFixOptions) ([]bootstrap.RepairAction, []bootstrap.RepairAction, []bootstrap.RepairAction, error) {
	applied := make([]bootstrap.RepairAction, 0)
	skipped := make([]bootstrap.RepairAction, 0)
	blocked := make([]bootstrap.RepairAction, 0)

	if len(plan.Actions) == 0 {
		return applied, skipped, blocked, nil
	}

	needsConfirmation := false
	for _, action := range plan.Actions {
		if !action.Apply {
			blocked = append(blocked, action)
			continue
		}
		if action.MutatesGlobal || action.RequiresAdoption {
			needsConfirmation = true
		}
	}

	if needsConfirmation && !opts.Yes {
		if !ui.IsInteractive() {
			return nil, nil, nil, fmt.Errorf("doctor --fix requires --yes in non-interactive mode when global/adoption changes are needed")
		}
		fmt.Print("Apply repair actions that may mutate global/adopt unmanaged configs? [y/N]: ")
		var input string
		_, _ = fmt.Scanln(&input)
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			return applied, skipped, blocked, nil
		}
	}

	binPath, err := os.Executable()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve executable path: %w", err)
	}
	if absPath, err := filepath.Abs(binPath); err == nil {
		binPath = filepath.Clean(absPath)
	}

	init := bootstrap.NewInitializer(cwd)
	for _, action := range plan.Actions {
		if !action.Apply {
			continue
		}
		if opts.DryRun {
			action.Apply = false
			action.Reason = "dry-run"
			skipped = append(skipped, action)
			continue
		}
		if strings.HasPrefix(action.Primitive, "adopt_and_") {
			if _, adoptErr := init.AdoptAIConfig(action.AI, viper.GetBool("verbose")); adoptErr != nil {
				action.Apply = false
				action.Reason = "adoption failed: " + adoptErr.Error()
				skipped = append(skipped, action)
				continue
			}
		}
		primitive := strings.TrimPrefix(action.Primitive, "adopt_and_")
		if err := applyRepairPrimitive(primitive, action.AI, cwd, binPath, init); err != nil {
			action.Apply = false
			action.Reason = err.Error()
			skipped = append(skipped, action)
			continue
		}
		applied = append(applied, action)
	}

	return applied, skipped, blocked, nil
}

func applyRepairPrimitive(primitive, aiName, cwd, binPath string, init *bootstrap.Initializer) error {
	switch primitive {
	case "repairCommands":
		return init.CreateSlashCommands(aiName, viper.GetBool("verbose"))
	case "repairHooks":
		return init.InstallHooksConfig(aiName, viper.GetBool("verbose"))
	case "repairPlugin":
		if aiName != "opencode" {
			return nil
		}
		return init.InstallHooksConfig("opencode", viper.GetBool("verbose"))
	case "repairLocalMCP":
		return installMCPForTarget(aiName, binPath, cwd)
	case "repairGlobalMCP":
		return installMCPForTarget(aiName, binPath, cwd)
	default:
		return fmt.Errorf("unknown repair primitive: %s", primitive)
	}
}

func checkTaskWingInit(cwd string) DoctorCheck {
	taskwingDir := filepath.Join(cwd, ".taskwing")
	memoryDir := filepath.Join(taskwingDir, "memory")

	if _, err := os.Stat(taskwingDir); os.IsNotExist(err) {
		return DoctorCheck{
			Name:    "Initialization",
			Status:  "fail",
			Message: "Not initialized",
			Hint:    "Run: taskwing bootstrap",
		}
	}

	if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
		return DoctorCheck{
			Name:    "Initialization",
			Status:  "warn",
			Message: "Partially initialized (missing memory/)",
			Hint:    "Run: taskwing bootstrap",
		}
	}

	return DoctorCheck{
		Name:    "Initialization",
		Status:  "ok",
		Message: ".taskwing/ directory exists",
	}
}

func checkActivePlan() DoctorCheck {
	repo, err := openRepo()
	if err != nil {
		if isMissingProjectMemoryError(err) {
			return DoctorCheck{
				Name:    "Active Plan",
				Status:  "warn",
				Message: "No project memory found",
				Hint:    "Run: taskwing bootstrap",
			}
		}
		return DoctorCheck{
			Name:    "Active Plan",
			Status:  "warn",
			Message: "Could not open repository",
		}
	}
	defer func() { _ = repo.Close() }()

	plan, err := repo.GetActivePlan()
	if err != nil || plan == nil {
		return DoctorCheck{
			Name:    "Active Plan",
			Status:  "warn",
			Message: "No active plan",
			Hint:    "Run: taskwing goal \"your goal\"",
		}
	}

	// Count task statuses
	pending, inProgress, completed := 0, 0, 0
	for _, t := range plan.Tasks {
		switch t.Status {
		case task.StatusPending:
			pending++
		case task.StatusInProgress:
			inProgress++
		case task.StatusCompleted:
			completed++
		}
	}

	total := len(plan.Tasks)
	progress := 0
	if total > 0 {
		progress = completed * 100 / total
	}

	msg := fmt.Sprintf("%s (%d%% complete: %d/%d tasks)", plan.ID, progress, completed, total)

	return DoctorCheck{
		Name:    "Active Plan",
		Status:  "ok",
		Message: msg,
	}
}

func checkSession() DoctorCheck {
	session, err := loadHookSession()
	if err != nil {
		return DoctorCheck{
			Name:    "Session",
			Status:  "warn",
			Message: "No active session",
			Hint:    "Session auto-starts when you open Claude Code (SessionStart hook)",
		}
	}

	msg := fmt.Sprintf("%s (tasks: %d started, %d completed)",
		session.SessionID, session.TasksStarted, session.TasksCompleted)

	return DoctorCheck{
		Name:    "Session",
		Status:  "ok",
		Message: msg,
	}
}

func printNextSteps(checks []DoctorCheck) {
	// Determine what user should do next based on checks
	hasActivePlan := false
	hasSession := false

	for _, c := range checks {
		if c.Name == "Active Plan" && c.Status == "ok" {
			hasActivePlan = true
		}
		if c.Name == "Session" && c.Status == "ok" {
			hasSession = true
		}
	}

	fmt.Println()
	fmt.Println("Next steps:")
	if !hasActivePlan {
		fmt.Println("  1. Create and activate plan: taskwing goal \"your development goal\"")
		fmt.Println("  2. Open Claude Code and run: /tw-next")
	} else if !hasSession {
		fmt.Println("  1. Open Claude Code (session will auto-initialize)")
		fmt.Println("  2. Run: /tw-next")
	} else {
		fmt.Println("  • In Claude Code, run: /tw-next")
		fmt.Println("  • Tasks will auto-continue until circuit breaker triggers")
	}
}
