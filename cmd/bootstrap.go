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
		llmCfg, err := getLLMConfig(cmd)
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
	return ks.IngestFindingsWithRelationships(ctx, allFindings, allRelationships, !viper.GetBool("quiet"))
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
		fmt.Printf("     %s %s: %d files, %d findings\n", status, name, ar.Coverage.FilesAnalyzed, ar.FindingCount)
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
		fmt.Println("  Skipping AI setup (rerun 'tw bootstrap' to add assistants)")
	} else {
		for _, ai := range selectedAIs {
			aiCfg := aiConfigs[ai]
			fmt.Printf("📝 Creating %s commands...\n", aiCfg.displayName)
			if err := createSingleSlashCommand(basePath, aiCfg, verbose); err != nil {
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
	}

	fmt.Println("\n✓ TaskWing initialized!")

	// NOTE: config.yaml is NOT created here anymore.
	// It will be created by config.SaveGlobalLLMConfig() after LLM provider selection
	// in getLLMConfig() which runs after this function.
	return nil
}

// createSingleSlashCommand creates the unified /taskwing command
func createSingleSlashCommand(basePath string, aiCfg aiConfig, verbose bool) error {
	commandsDir := filepath.Join(basePath, aiCfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	var content string
	var fileName string

	if aiCfg.fileExt == ".toml" {
		// TOML format for Gemini CLI
		fileName = "taskwing.toml"
		content = `description = "Fetch project context from TaskWing"

prompt = """Call the TaskWing MCP recall tool NOW to understand this codebase.

Use it with:
- No query: Get overview (features, decisions, patterns, constraints)
- With query: {"query": "search term"} for semantic search

This is the first thing you should do when working on this project."""
`
	} else {
		// Markdown format for Claude, Cursor, Copilot, Codex
		fileName = "taskwing.md"
		content = `# Fetch project architecture context

Retrieve codebase knowledge (patterns, decisions, constraints) via the TaskWing MCP server.

## Prerequisites
TaskWing MCP server must be configured. If not set up, run:
` + "```bash" + `
tw mcp install claude
` + "```" + `

## MCP Tool: ` + "`recall`" + `
- **Overview mode** (no params): Returns summary of features, decisions, patterns, constraints
- **Search mode**: ` + "`{\"query\": \"authentication\"}`" + ` for semantic search across project memory

## When to Use
- Starting work on an unfamiliar codebase
- Before implementing features (check existing patterns)
- When unsure about architecture decisions
- Finding constraints before making changes

## Fallback (No MCP)
If MCP is unavailable, use the CLI directly:
` + "```bash" + `
tw context              # Overview
tw context -q "search"  # Semantic search
tw context --answer     # AI-generated response
` + "```" + `
`
	}

	filePath := filepath.Join(commandsDir, fileName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("create %s: %w", fileName, err)
	}
	if verbose {
		fmt.Printf("  ✓ Created %s/%s\n", aiCfg.commandsDir, fileName)
	}

	return nil
}
