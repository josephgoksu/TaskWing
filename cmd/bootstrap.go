/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// bootstrapCmd represents the bootstrap command
var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Auto-generate project memory from existing repo",
	Long: `Scan your repository and automatically generate features and decisions.

If this is the first run, TaskWing will initialize the project:
  • Create .taskwing/ directory structure
  • Set up AI assistant integration (Claude, Cursor, etc.)
  • Configure LLM settings

The bootstrap command analyzes:
  • Directory structure → Detects features
  • Git history → Extracts decisions from conventional commits
  • LLM inference → Understands WHY decisions were made

Examples:
  tw bootstrap                        # Initialize (if needed) + analyze
  tw bootstrap --preview              # Preview without saving
  tw bootstrap --skip-init            # Skip initialization prompt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		preview, _ := cmd.Flags().GetBool("preview")
		skipInit, _ := cmd.Flags().GetBool("skip-init")
		trace, _ := cmd.Flags().GetBool("trace")
		traceFile, _ := cmd.Flags().GetString("trace-file")
		traceStdout, _ := cmd.Flags().GetBool("trace-stdout")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}

		// Check if .taskwing exists - if not, initialize first
		taskwingDir := filepath.Join(cwd, ".taskwing")
		if _, err := os.Stat(taskwingDir); os.IsNotExist(err) && !skipInit {
			fmt.Println("🚀 First time setup detected!")
			fmt.Println()
			if err := runAutoInit(cwd, cmd); err != nil {
				return fmt.Errorf("initialization failed: %w", err)
			}
			fmt.Println()
		}

		// Use centralized config loader
		llmCfg, err := getLLMConfigForRole(cmd, llm.RoleBootstrap)
		if err != nil {
			return err
		}

		// Default: use parallel agent architecture
		return runAgentBootstrap(cmd.Context(), cwd, preview, llmCfg, trace, traceFile, traceStdout)
	},
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().Bool("skip-init", false, "Skip initialization prompt")
	bootstrapCmd.Flags().Bool("trace", false, "Emit JSON event stream to stderr")
	bootstrapCmd.Flags().String("trace-file", "", "Write JSON event stream to file (default: .taskwing/logs/bootstrap.trace.jsonl)")
	bootstrapCmd.Flags().Bool("trace-stdout", false, "Emit JSON event stream to stderr (overrides trace file)")
}

// runAgentBootstrap uses the parallel agent architecture for analysis
func runAgentBootstrap(ctx context.Context, cwd string, preview bool, llmCfg llm.Config, trace bool, traceFile string, traceStdout bool) error {
	fmt.Println("")
	ui.RenderPageHeader("TaskWing Bootstrap", fmt.Sprintf("Using: %s (%s)", llmCfg.Model, llmCfg.Provider))

	projectName := filepath.Base(cwd)

	// Create agents (all deterministic - no ReAct loops)
	agentsList := bootstrap.NewDefaultAgents(llmCfg, cwd)
	defer core.CloseAgents(agentsList)

	// Prepare input
	input := core.Input{
		BasePath:    cwd,
		ProjectName: projectName,
		Mode:        core.ModeBootstrap,
		Verbose:     true, // Will be suppressed in TUI
	}

	// Initialize streaming output for "The Pulse"
	stream := core.NewStreamingOutput(100)
	defer stream.Close()
	if trace {
		if traceFile == "" {
			traceFile = filepath.Join(cwd, ".taskwing", "logs", "bootstrap.trace.jsonl")
		}
		var out *os.File
		if traceStdout {
			out = os.Stderr
		} else {
			_ = os.MkdirAll(filepath.Dir(traceFile), 0755)
			f, err := os.OpenFile(traceFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("open trace file: %w", err)
			}
			out = f
			defer func() { _ = f.Close() }()
			if !viper.GetBool("quiet") {
				fmt.Fprintf(os.Stderr, "🧾 Trace: %s\n", traceFile)
			}
		}

		var mu sync.Mutex
		stream.AddObserver(func(e core.StreamEvent) {
			payload := map[string]any{
				"type":      e.Type,
				"timestamp": e.Timestamp.Format(time.RFC3339Nano),
				"agent":     e.Agent,
				"content":   e.Content,
				"metadata":  e.Metadata,
			}
			if b, err := json.Marshal(payload); err == nil {
				mu.Lock()
				_, _ = fmt.Fprintln(out, string(b))
				mu.Unlock()
			}
		})
	}

	// Run TUI
	tuiModel := ui.NewBootstrapModel(ctx, input, agentsList, stream)
	p := tea.NewProgram(tuiModel)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	bootstrapModel, ok := finalModel.(ui.BootstrapModel)
	if !ok {
		return fmt.Errorf("internal error: invalid model type")
	}

	if bootstrapModel.Quitting && len(bootstrapModel.Results) < len(agentsList) {
		fmt.Println("\n⚠️  Bootstrap cancelled.")
		return nil
	}

	var failedAgents []string
	for _, state := range bootstrapModel.Agents {
		if state.Status == ui.StatusError || state.Err != nil {
			errMsg := "unknown error"
			if state.Err != nil {
				errMsg = state.Err.Error()
			}
			failedAgents = append(failedAgents, fmt.Sprintf("%s: %s", state.Name, errMsg))
		}
	}
	if len(failedAgents) > 0 {
		fmt.Fprintln(os.Stderr, "\n✗ Bootstrap failed. Some agents errored:")
		for _, line := range failedAgents {
			fmt.Fprintf(os.Stderr, "  - %s\n", line)
		}
		return fmt.Errorf("bootstrap failed: %d agent(s) errored", len(failedAgents))
	}

	// Aggregate findings and relationships
	allFindings := core.AggregateFindings(bootstrapModel.Results)
	allRelationships := core.AggregateRelationships(bootstrapModel.Results)

	// Generate bootstrap report
	report := generateBootstrapReport(cwd, bootstrapModel.Results, allFindings)

	// Save report to disk (always, even in preview mode)
	reportPath := filepath.Join(cwd, ".taskwing", "last-bootstrap-report.json")
	if err := saveBootstrapReport(reportPath, report); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Failed to save bootstrap report: %v\n", err)
	}

	// Print coverage summary
	printCoverageSummary(report)

	if preview || viper.GetBool("preview") {
		fmt.Println("\n💡 This was a preview. Run 'taskwing bootstrap' to save to memory.")
		return nil
	}

	// Save to memory using KnowledgeService
	memoryPath := config.GetMemoryBasePath()
	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Create Service
	ks := knowledge.NewService(repo, llmCfg)
	ks.SetBasePath(cwd) // Enable evidence verification

	// Ingest (with verification and LLM-extracted relationships)
	return ks.IngestFindingsWithRelationships(ctx, allFindings, allRelationships, nil, !viper.GetBool("quiet"))
}

// generateBootstrapReport creates a report from agent results
func generateBootstrapReport(projectPath string, results []core.Output, findings []core.Finding) *core.BootstrapReport {
	report := core.NewBootstrapReport(projectPath)

	// Add per-agent reports
	for _, result := range results {
		agentReport := core.AgentReport{
			Name:         result.AgentName,
			Duration:     result.Duration,
			TokensUsed:   result.TokensUsed,
			FindingCount: len(result.Findings),
			Coverage:     result.Coverage,
		}
		if result.Error != nil {
			agentReport.Error = result.Error.Error()
		}
		report.AddAgentReport(result.AgentName, agentReport)
	}

	// Calculate totals
	var totalDuration time.Duration
	for _, r := range results {
		totalDuration += r.Duration
	}
	report.Finalize(findings, totalDuration)

	return report
}

// saveBootstrapReport writes the report to a JSON file
func saveBootstrapReport(path string, report *core.BootstrapReport) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

// printCoverageSummary outputs a human-readable coverage summary
func printCoverageSummary(report *core.BootstrapReport) {
	fmt.Println()
	fmt.Println("📊 Bootstrap Coverage Report")
	fmt.Println("────────────────────────────")
	fmt.Printf("   Files analyzed: %d\n", report.Coverage.FilesAnalyzed)
	fmt.Printf("   Files skipped:  %d\n", report.Coverage.FilesSkipped)
	fmt.Printf("   Coverage:       %.1f%%\n", report.Coverage.CoveragePercent)
	fmt.Printf("   Total findings: %d\n", report.TotalFindings)

	// Breakdown by type
	if len(report.FindingCounts) > 0 {
		fmt.Println()
		fmt.Println("   Findings by type:")
		for fType, count := range report.FindingCounts {
			fmt.Printf("     • %s: %d\n", fType, count)
		}
	}

	// Per-agent summary
	fmt.Println()
	fmt.Println("   Per-agent coverage:")
	for name, ar := range report.AgentReports {
		status := "✓"
		if ar.Error != "" {
			status = "✗"
		}
		fileWord := "files"
		if ar.Coverage.FilesAnalyzed == 1 {
			fileWord = "file"
		}
		findingWord := "findings"
		if ar.FindingCount == 1 {
			findingWord = "finding"
		}
		fmt.Printf("     %s %s: %d %s, %d %s\n", status, name, ar.Coverage.FilesAnalyzed, fileWord, ar.FindingCount, findingWord)
	}

	fmt.Println()
	fmt.Printf("📄 Full report: .taskwing/last-bootstrap-report.json\n")
}

// runAutoInit initializes .taskwing/ structure when first running bootstrap
func runAutoInit(basePath string, cmd *cobra.Command) error {
	verbose := viper.GetBool("verbose")

	// Create .taskwing structure
	fmt.Println("📁 Creating .taskwing/ structure...")
	dirs := []string{
		".taskwing",
		".taskwing/memory",
		".taskwing/plans",
	}
	for _, dir := range dirs {
		fullPath := filepath.Join(basePath, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s\n", dir)
		}
	}

	// Prompt for AI IDE integrations (slash commands)
	fmt.Println()
	fmt.Println("🤖 Which AI assistant(s) do you use?")
	fmt.Println()
	selectedAIs := promptAISelection()
	if len(selectedAIs) == 0 {
		fmt.Println("  Skipping AI setup (rerun 'taskwing bootstrap' to add assistants)")
	} else {
		for _, ai := range selectedAIs {
			aiCfg := aiConfigs[ai]
			fmt.Printf("📝 Creating %s commands...\n", aiCfg.displayName)
			if err := createSlashCommands(basePath, aiCfg, verbose); err != nil {
				return err
			}
		}

		// Install MCP servers for selected AIs
		binPath, _ := os.Executable()
		if absPath, err := filepath.Abs(binPath); err == nil {
			binPath = filepath.Clean(absPath)
		}
		for _, ai := range selectedAIs {
			switch ai {
			case "claude":
				installClaude(binPath, basePath)
			case "gemini":
				installGeminiCLI(binPath, basePath)
			case "codex":
				installCodexGlobal(binPath, basePath)
			case "cursor":
				installLocalMCP(basePath, ".cursor", "mcp.json", binPath)
			case "copilot":
				installCopilot(binPath, basePath)
			}
		}

		// Install hooks configuration for supported AIs (Claude, Codex)
		for _, ai := range selectedAIs {
			if err := installHooksConfig(basePath, ai, verbose); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Failed to install hooks for %s: %v\n", ai, err)
			}
		}

		// Update agent documentation with hooks info
		for _, ai := range selectedAIs {
			if err := updateAgentDocs(basePath, ai, verbose); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Failed to update docs for %s: %v\n", ai, err)
			}
		}
	}

	fmt.Println("\n✓ TaskWing initialized!")

	// NOTE: config.yaml is NOT created here anymore.
	// It will be created by config.SaveGlobalLLMConfig() after LLM provider selection
	// in getLLMConfig() which runs after this function.
	return nil
}

// createSlashCommands creates all slash commands (main + task lifecycle) for AI assistants.
// These are THIN WRAPPERS that call `taskwing slash <name>` at runtime.
// This ensures command content always matches the installed CLI version.
func createSlashCommands(basePath string, aiCfg aiConfig, verbose bool) error {
	commandsDir := filepath.Join(basePath, aiCfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	// Define all commands in one place
	commands := []struct {
		baseName    string
		slashCmd    string
		description string
	}{
		// Main command
		{"taskwing", "taskwing", "Fetch project architecture context from TaskWing"},
		// Task lifecycle commands
		{"tw-next", "next", "Start next TaskWing task with full context"},
		{"tw-done", "done", "Complete current task with architecture-aware summary"},
		{"tw-context", "context", "Fetch additional context for current task"},
		{"tw-status", "status", "Show current task status"},
		{"tw-block", "block", "Mark current task as blocked"},
		{"tw-plan", "plan", "Create development plan with goal"},
	}

	isTOML := aiCfg.fileExt == ".toml"

	for _, cmd := range commands {
		var content, fileName string

		if isTOML {
			// Gemini TOML format: !{taskwing slash ...}
			fileName = cmd.baseName + ".toml"
			content = fmt.Sprintf(`description = "%s"

prompt = """!{taskwing slash %s}"""
`, cmd.description, cmd.slashCmd)
		} else {
			// Markdown format (Claude/Cursor): !taskwing slash ...
			fileName = cmd.baseName + ".md"
			content = fmt.Sprintf(`---
description: %s
---
!taskwing slash %s
`, cmd.description, cmd.slashCmd)
		}

		filePath := filepath.Join(commandsDir, fileName)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("create %s: %w", fileName, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s/%s (dynamic)\n", aiCfg.commandsDir, fileName)
		}
	}

	return nil
}

// HooksConfig represents the hooks configuration for AI assistants
// Uses nested object format: {"hooks": {"EventName": [{"hooks": [...]}]}}
type HooksConfig struct {
	Hooks map[string][]HookMatcher `json:"hooks"`
}

// HookMatcher represents a hook trigger with optional matcher
type HookMatcher struct {
	Matcher *HookMatcherConfig `json:"matcher,omitempty"`
	Hooks   []HookCommand      `json:"hooks"`
}

// HookMatcherConfig represents matcher conditions (optional)
type HookMatcherConfig struct {
	Tools []string `json:"tools,omitempty"`
}

// HookCommand represents a single hook command
type HookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// installHooksConfig creates the hooks configuration file for AI assistants
func installHooksConfig(basePath string, aiName string, verbose bool) error {
	// Only Claude and Codex support hooks currently
	var settingsPath string
	switch aiName {
	case "claude":
		settingsPath = filepath.Join(basePath, ".claude", "settings.json")
	case "codex":
		settingsPath = filepath.Join(basePath, ".codex", "settings.json")
	default:
		// Gemini, Cursor, Copilot don't have hooks support yet
		return nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	// Check if settings file already exists and has hooks
	if content, err := os.ReadFile(settingsPath); err == nil {
		var existing map[string]any
		if err := json.Unmarshal(content, &existing); err == nil {
			if _, hasHooks := existing["hooks"]; hasHooks {
				if verbose {
					fmt.Printf("  ℹ️  Hooks already configured in %s\n", settingsPath)
				}
				return nil
			}
		}
	}

	// Create hooks configuration using nested object format
	// IMPORTANT: Claude Code only supports these events: PreToolUse, PostToolUse, Notification, Stop
	// SessionStart and SessionEnd are NOT valid events - they are silently ignored!
	// The continue-check command auto-initializes the session on first call, so no SessionStart needed.
	config := HooksConfig{
		Hooks: map[string][]HookMatcher{
			"Stop": {
				{
					Hooks: []HookCommand{
						{
							Type:    "command",
							Command: "taskwing hook continue-check --max-tasks=5 --max-minutes=30",
							Timeout: 15,
						},
					},
				},
			},
		},
	}

	// Write the config
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hooks config: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("write hooks config: %w", err)
	}

	if verbose {
		fmt.Printf("  ✓ Created hooks config: %s\n", settingsPath)
	}

	return nil
}

// hooksDocSection is the documentation to add to agent markdown files
const hooksDocSection = `

### Autonomous Task Execution (Hooks)

TaskWing integrates with Claude Code's hook system for autonomous plan execution:

` + "```bash" + `
taskwing hook session-init      # Initialize session tracking (SessionStart hook)
taskwing hook continue-check    # Check if should continue to next task (Stop hook)
taskwing hook session-end       # Cleanup session (SessionEnd hook)
taskwing hook status            # View current session state
` + "```" + `

**Circuit breakers** prevent runaway execution:
- ` + "`--max-tasks=5`" + ` - Stop after N tasks for human review
- ` + "`--max-minutes=30`" + ` - Stop after N minutes

Configuration in ` + "`.claude/settings.json`" + ` enables auto-continuation through plans.
`

// updateAgentDocs updates agent markdown files (CLAUDE.md, AGENTS.md, etc.) with hooks documentation
func updateAgentDocs(basePath string, aiName string, verbose bool) error {
	// Determine which files to check based on AI type
	var filesToCheck []string
	switch aiName {
	case "claude":
		filesToCheck = []string{"CLAUDE.md", "AGENTS.md"}
	case "codex":
		filesToCheck = []string{"AGENTS.md", "CODEX.md"}
	case "gemini":
		filesToCheck = []string{"GEMINI.md", "AGENTS.md"}
	default:
		filesToCheck = []string{"AGENTS.md"}
	}

	for _, fileName := range filesToCheck {
		filePath := filepath.Join(basePath, fileName)

		// Check if file exists
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue // File doesn't exist, skip
		}

		// Check if hooks section already exists
		if strings.Contains(string(content), "Autonomous Task Execution") ||
			strings.Contains(string(content), "tw hook session-init") ||
			strings.Contains(string(content), "taskwing hook session-init") {
			if verbose {
				fmt.Printf("  ℹ️  Hooks docs already in %s\n", fileName)
			}
			continue
		}

		// Append hooks documentation
		newContent := string(content) + hooksDocSection

		if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("update %s: %w", fileName, err)
		}

		if verbose {
			fmt.Printf("  ✓ Added hooks docs to %s\n", fileName)
		}

		// Only update one file per AI
		break
	}

	return nil
}
