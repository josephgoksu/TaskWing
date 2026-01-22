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
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warn", "fail"
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// DoctorResult is the JSON output structure for doctor command
type DoctorResult struct {
	Status   string        `json:"status"` // "ok", "warn", "fail"
	Checks   []DoctorCheck `json:"checks"`
	Errors   int           `json:"errors"`
	Warnings int           `json:"warnings"`
}

func runDoctor() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	checks := []DoctorCheck{}
	hasErrors := false
	hasWarnings := false

	// Check 1: TaskWing initialized
	check := checkTaskWingInit(cwd)
	checks = append(checks, check)
	switch check.Status {
	case "fail":
		hasErrors = true
	case "warn":
		hasWarnings = true
	}

	// Check 2: MCP servers
	mcpChecks := checkMCPServers(cwd)
	checks = append(checks, mcpChecks...)

	// Check 3: Hooks configuration
	hookChecks := checkHooksConfig(cwd)
	checks = append(checks, hookChecks...)
	for _, c := range hookChecks {
		switch c.Status {
		case "fail":
			hasErrors = true
		case "warn":
			hasWarnings = true
		}
	}

	// Check 4: Active plan
	planCheck := checkActivePlan()
	checks = append(checks, planCheck)

	// Check 5: Session state
	sessionCheck := checkSession()
	checks = append(checks, sessionCheck)

	// Count errors and warnings
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

	// JSON output
	if isJSON() {
		status := "ok"
		if hasErrors {
			status = "fail"
		} else if hasWarnings {
			status = "warn"
		}
		return printJSON(DoctorResult{
			Status:   status,
			Checks:   checks,
			Errors:   errorCount,
			Warnings: warningCount,
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

	// Check OpenCode MCP (project-local opencode.json)
	openCodeChecks := checkOpenCodeMCP(cwd)
	checks = append(checks, openCodeChecks...)

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

// checkOpenCodeMCP validates OpenCode MCP configuration:
// 1. Checks if opencode.json exists at project root
// 2. Validates JSON structure and taskwing-mcp entry
// 3. Verifies command is JSON array and type is "local"
// 4. Validates .opencode/skills/*/SKILL.md files
func checkOpenCodeMCP(cwd string) []DoctorCheck {
	checks := []DoctorCheck{}

	// Check 1: opencode.json exists
	configPath := filepath.Join(cwd, "opencode.json")
	info, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		// No opencode.json - OpenCode is not configured (not an error, just skip)
		return checks
	}

	// Size limit for safety
	if info.Size() > 1024*1024 { // 1MB limit
		checks = append(checks, DoctorCheck{
			Name:    "MCP (OpenCode)",
			Status:  "warn",
			Message: "opencode.json too large to parse",
		})
		return checks
	}

	// Check 2: Parse and validate JSON structure
	content, err := os.ReadFile(configPath)
	if err != nil {
		checks = append(checks, DoctorCheck{
			Name:    "MCP (OpenCode)",
			Status:  "warn",
			Message: "Could not read opencode.json",
			Hint:    "Check file permissions",
		})
		return checks
	}

	var config OpenCodeConfig
	if err := json.Unmarshal(content, &config); err != nil {
		checks = append(checks, DoctorCheck{
			Name:    "MCP (OpenCode)",
			Status:  "fail",
			Message: "Invalid JSON in opencode.json",
			Hint:    "Run: jq . opencode.json to validate syntax",
		})
		return checks
	}

	// Check 3: Validate MCP section exists
	if len(config.MCP) == 0 {
		checks = append(checks, DoctorCheck{
			Name:    "MCP (OpenCode)",
			Status:  "fail",
			Message: "No MCP servers in opencode.json",
			Hint:    "Run: taskwing mcp install opencode",
		})
		return checks
	}

	// Check 4: Find taskwing-mcp entry (may have project suffix)
	var serverCfg *OpenCodeMCPServerConfig
	var serverName string
	for name, cfg := range config.MCP {
		if strings.HasPrefix(name, "taskwing-mcp") {
			serverCfg = &cfg
			serverName = name
			break
		}
	}

	if serverCfg == nil {
		checks = append(checks, DoctorCheck{
			Name:    "MCP (OpenCode)",
			Status:  "fail",
			Message: "taskwing-mcp not found in opencode.json",
			Hint:    "Run: taskwing mcp install opencode",
		})
		return checks
	}

	// Check 5: Validate type is "local"
	if serverCfg.Type != "local" {
		checks = append(checks, DoctorCheck{
			Name:    "MCP (OpenCode)",
			Status:  "fail",
			Message: fmt.Sprintf("Invalid type %q (expected \"local\")", serverCfg.Type),
			Hint:    "Run: taskwing mcp install opencode to regenerate",
		})
		return checks
	}

	// Check 6: Validate command is array with at least 2 elements
	if len(serverCfg.Command) < 2 {
		checks = append(checks, DoctorCheck{
			Name:    "MCP (OpenCode)",
			Status:  "fail",
			Message: "Invalid command format (expected array with binary and 'mcp')",
			Hint:    "Run: taskwing mcp install opencode to regenerate",
		})
		return checks
	}

	// MCP config is valid
	checks = append(checks, DoctorCheck{
		Name:    "MCP (OpenCode)",
		Status:  "ok",
		Message: fmt.Sprintf("%s registered in opencode.json", serverName),
	})

	// Check 7: Validate skills (optional - warn if issues)
	skillsChecks := checkOpenCodeSkills(cwd)
	checks = append(checks, skillsChecks...)

	return checks
}

// checkOpenCodeSkills validates .opencode/skills/*/SKILL.md files
func checkOpenCodeSkills(cwd string) []DoctorCheck {
	checks := []DoctorCheck{}

	skillsDir := filepath.Join(cwd, ".opencode", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		// No skills directory - not an error, skills are optional
		return checks
	}

	// Find all SKILL.md files
	pattern := filepath.Join(skillsDir, "*", "SKILL.md")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		// No skills found - not an error
		return checks
	}

	validSkills := 0
	invalidSkills := []string{}

	for _, skillPath := range matches {
		// Get the skill directory name
		skillDirName := filepath.Base(filepath.Dir(skillPath))

		// Read and validate SKILL.md
		content, err := os.ReadFile(skillPath)
		if err != nil {
			invalidSkills = append(invalidSkills, skillDirName+": unreadable")
			continue
		}

		// Check for frontmatter markers
		contentStr := string(content)
		if !strings.HasPrefix(contentStr, "---") {
			invalidSkills = append(invalidSkills, skillDirName+": missing YAML frontmatter")
			continue
		}

		// Extract frontmatter
		parts := strings.SplitN(contentStr, "---", 3)
		if len(parts) < 3 {
			invalidSkills = append(invalidSkills, skillDirName+": incomplete frontmatter")
			continue
		}

		frontmatter := parts[1]

		// Check for required fields (simple validation - name and description)
		hasName := strings.Contains(frontmatter, "name:")
		hasDescription := strings.Contains(frontmatter, "description:")

		if !hasName || !hasDescription {
			missing := []string{}
			if !hasName {
				missing = append(missing, "name")
			}
			if !hasDescription {
				missing = append(missing, "description")
			}
			invalidSkills = append(invalidSkills, skillDirName+": missing "+strings.Join(missing, ", "))
			continue
		}

		// Extract name from frontmatter and verify it matches directory
		// Simple extraction - look for "name: value" pattern
		for _, line := range strings.Split(frontmatter, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "name:") {
				nameValue := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				// Remove quotes if present
				nameValue = strings.Trim(nameValue, "\"'")
				if nameValue != skillDirName {
					invalidSkills = append(invalidSkills, fmt.Sprintf("%s: name mismatch (name: %q != dir: %q)", skillDirName, nameValue, skillDirName))
					continue
				}
				break
			}
		}

		validSkills++
	}

	if len(invalidSkills) > 0 {
		checks = append(checks, DoctorCheck{
			Name:    "Skills (OpenCode)",
			Status:  "warn",
			Message: fmt.Sprintf("%d valid, %d invalid skills", validSkills, len(invalidSkills)),
			Hint:    "Invalid: " + strings.Join(invalidSkills, "; ") + ". For development, use taskwing-local-dev-mcp",
		})
	} else if validSkills > 0 {
		checks = append(checks, DoctorCheck{
			Name:    "Skills (OpenCode)",
			Status:  "ok",
			Message: fmt.Sprintf("%d skills configured", validSkills),
		})
	}

	return checks
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
