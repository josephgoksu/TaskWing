/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/task"
	"github.com/spf13/cobra"
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

Use this to troubleshoot issues or verify setup after bootstrap.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor()
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

// DoctorCheck represents a single diagnostic check
type DoctorCheck struct {
	Name    string
	Status  string // "ok", "warn", "fail"
	Message string
	Hint    string
}

func runDoctor() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	fmt.Println("🩺 TaskWing Doctor")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	checks := []DoctorCheck{}
	hasErrors := false

	// Check 1: TaskWing initialized
	check := checkTaskWingInit(cwd)
	checks = append(checks, check)
	if check.Status == "fail" {
		hasErrors = true
	}

	// Check 2: MCP servers
	mcpChecks := checkMCPServers(cwd)
	checks = append(checks, mcpChecks...)

	// Check 3: Hooks configuration
	hookChecks := checkHooksConfig(cwd)
	checks = append(checks, hookChecks...)
	for _, c := range hookChecks {
		if c.Status == "fail" {
			hasErrors = true
		}
	}

	// Check 4: Active plan
	planCheck := checkActivePlan()
	checks = append(checks, planCheck)

	// Check 5: Session state
	sessionCheck := checkSession()
	checks = append(checks, sessionCheck)

	// Print all checks
	for _, c := range checks {
		printCheck(c)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Summary and next steps
	if hasErrors {
		fmt.Println("❌ Issues found. Fix the errors above before continuing.")
	} else {
		fmt.Println("✅ Everything looks good!")
		printNextSteps(checks)
	}

	return nil
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

func checkMCPServers(cwd string) []DoctorCheck {
	checks := []DoctorCheck{}

	// Check Claude Code MCP
	claudeCheck := checkClaudeMCP()
	if claudeCheck.Status != "" {
		checks = append(checks, claudeCheck)
	}

	// Check Gemini MCP (with size limit)
	geminiPath := filepath.Join(cwd, ".gemini", "settings.json")
	if info, err := os.Stat(geminiPath); err == nil && info.Size() < 1024*1024 { // 1MB limit
		if content, err := os.ReadFile(geminiPath); err == nil {
			var config map[string]any
			if err := json.Unmarshal(content, &config); err == nil {
				if servers, ok := config["mcpServers"].(map[string]any); ok {
					if _, hasTaskwing := servers["taskwing-mcp"]; hasTaskwing {
						checks = append(checks, DoctorCheck{
							Name:    "MCP (Gemini)",
							Status:  "ok",
							Message: "taskwing-mcp registered",
						})
					}
				}
			}
		}
	}

	// Check Codex MCP
	codexCheck := checkCodexMCP()
	if codexCheck.Status != "" {
		checks = append(checks, codexCheck)
	}

	// Check Cursor MCP (with size limit)
	cursorPath := filepath.Join(cwd, ".cursor", "mcp.json")
	if info, err := os.Stat(cursorPath); err == nil && info.Size() < 1024*1024 { // 1MB limit
		if content, err := os.ReadFile(cursorPath); err == nil {
			var config map[string]any
			if err := json.Unmarshal(content, &config); err == nil {
				if servers, ok := config["mcpServers"].(map[string]any); ok {
					if _, hasTaskwing := servers["taskwing-mcp"]; hasTaskwing {
						checks = append(checks, DoctorCheck{
							Name:    "MCP (Cursor)",
							Status:  "ok",
							Message: "taskwing-mcp registered",
						})
					}
				}
			}
		}
	}

	if len(checks) == 0 {
		checks = append(checks, DoctorCheck{
			Name:    "MCP Servers",
			Status:  "warn",
			Message: "No MCP servers configured",
			Hint:    "Run: taskwing mcp install claude (or gemini, codex, cursor)",
		})
	}

	return checks
}

func checkClaudeMCP() DoctorCheck {
	// Check if claude CLI exists
	if _, err := exec.LookPath("claude"); err != nil {
		// Return empty check - Claude not installed is not an error, just skip
		return DoctorCheck{}
	}

	// Run claude mcp list with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "mcp", "list")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return DoctorCheck{
				Name:    "MCP (Claude)",
				Status:  "warn",
				Message: "Timeout checking MCP servers",
			}
		}
		return DoctorCheck{
			Name:    "MCP (Claude)",
			Status:  "warn",
			Message: "Could not check MCP servers",
		}
	}

	if strings.Contains(string(output), "taskwing-mcp") {
		return DoctorCheck{
			Name:    "MCP (Claude)",
			Status:  "ok",
			Message: "taskwing-mcp registered",
		}
	}

	return DoctorCheck{
		Name:    "MCP (Claude)",
		Status:  "warn",
		Message: "taskwing-mcp not registered",
		Hint:    "Run: taskwing mcp install claude",
	}
}

func checkCodexMCP() DoctorCheck {
	// Check if codex CLI exists
	if _, err := exec.LookPath("codex"); err != nil {
		return DoctorCheck{} // Not installed, skip
	}

	// Run codex mcp list with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codex", "mcp", "list")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return DoctorCheck{
				Name:    "MCP (Codex)",
				Status:  "warn",
				Message: "Timeout checking MCP servers",
			}
		}
		return DoctorCheck{
			Name:    "MCP (Codex)",
			Status:  "warn",
			Message: "Could not check MCP servers",
		}
	}

	if strings.Contains(string(output), "taskwing-mcp") {
		return DoctorCheck{
			Name:    "MCP (Codex)",
			Status:  "ok",
			Message: "taskwing-mcp registered",
		}
	}

	return DoctorCheck{
		Name:    "MCP (Codex)",
		Status:  "warn",
		Message: "taskwing-mcp not registered",
		Hint:    "Run: taskwing mcp install codex",
	}
}

func checkHooksConfig(cwd string) []DoctorCheck {
	checks := []DoctorCheck{}

	// Check Claude hooks
	claudeSettingsPath := filepath.Join(cwd, ".claude", "settings.json")
	claudeCheck := checkHooksFile(claudeSettingsPath, "Claude")
	if claudeCheck.Status != "" {
		checks = append(checks, claudeCheck)
	}

	// Check Codex hooks
	codexSettingsPath := filepath.Join(cwd, ".codex", "settings.json")
	codexCheck := checkHooksFile(codexSettingsPath, "Codex")
	if codexCheck.Status != "" {
		checks = append(checks, codexCheck)
	}

	if len(checks) == 0 {
		checks = append(checks, DoctorCheck{
			Name:    "Hooks",
			Status:  "warn",
			Message: "No hooks configured",
			Hint:    "Run: taskwing bootstrap (select claude or codex)",
		})
	}

	return checks
}

func checkHooksFile(settingsPath, aiName string) DoctorCheck {
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		return DoctorCheck{} // File doesn't exist, skip
	}

	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return DoctorCheck{
			Name:    fmt.Sprintf("Hooks (%s)", aiName),
			Status:  "warn",
			Message: "Invalid settings.json",
			Hint:    "Check JSON syntax in " + settingsPath,
		}
	}

	hooks, hasHooks := config["hooks"]
	if !hasHooks {
		return DoctorCheck{
			Name:    fmt.Sprintf("Hooks (%s)", aiName),
			Status:  "warn",
			Message: "No hooks in settings.json",
			Hint:    "Run: taskwing bootstrap",
		}
	}

	// Check for Stop hook specifically
	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		return DoctorCheck{
			Name:    fmt.Sprintf("Hooks (%s)", aiName),
			Status:  "warn",
			Message: "Invalid hooks format",
		}
	}

	if _, hasStop := hooksMap["Stop"]; !hasStop {
		return DoctorCheck{
			Name:    fmt.Sprintf("Hooks (%s)", aiName),
			Status:  "warn",
			Message: "Missing Stop hook (required for auto-continue)",
			Hint:    "Run: taskwing bootstrap",
		}
	}

	return DoctorCheck{
		Name:    fmt.Sprintf("Hooks (%s)", aiName),
		Status:  "ok",
		Message: "Configured (SessionStart, Stop, SessionEnd)",
	}
}

func checkActivePlan() DoctorCheck {
	repo, err := openRepo()
	if err != nil {
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
			Hint:    "Run: taskwing plan new \"your goal\" && taskwing plan start latest",
		}
	}

	// Count task statuses
	pending, inProgress, completed, blocked := 0, 0, 0, 0
	for _, t := range plan.Tasks {
		switch t.Status {
		case task.StatusPending:
			pending++
		case task.StatusInProgress:
			inProgress++
		case task.StatusCompleted:
			completed++
		case task.StatusBlocked:
			blocked++
		}
	}

	total := len(plan.Tasks)
	progress := 0
	if total > 0 {
		progress = completed * 100 / total
	}

	msg := fmt.Sprintf("%s (%d%% complete: %d/%d tasks)", plan.ID, progress, completed, total)
	if blocked > 0 {
		msg += fmt.Sprintf(" [%d blocked]", blocked)
	}

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
		fmt.Println("  1. Create a plan: taskwing plan new \"your development goal\"")
		fmt.Println("  2. Start the plan: taskwing plan start latest")
		fmt.Println("  3. Open Claude Code and run: /tw-next")
	} else if !hasSession {
		fmt.Println("  1. Open Claude Code (session will auto-initialize)")
		fmt.Println("  2. Run: /tw-next")
	} else {
		fmt.Println("  • In Claude Code, run: /tw-next")
		fmt.Println("  • Tasks will auto-continue until circuit breaker triggers")
	}
}
