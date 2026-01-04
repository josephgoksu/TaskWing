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
			if err := createSingleSlashCommand(basePath, aiCfg, verbose); err != nil {
				return err
			}
			// Also create task lifecycle commands
			if err := createTaskSlashCommands(basePath, aiCfg, verbose); err != nil {
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

// createTaskSlashCommands creates task lifecycle slash commands for AI assistants
func createTaskSlashCommands(basePath string, aiCfg aiConfig, verbose bool) error {
	if aiCfg.fileExt == ".toml" {
		// Create TOML format commands for Gemini
		return createGeminiTaskCommands(basePath, aiCfg, verbose)
	}

	commandsDir := filepath.Join(basePath, aiCfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	// Define task lifecycle commands with full context-aware workflows
	commands := []struct {
		name    string
		content string
	}{
		{
			name: "tw-next.md",
			content: `# Start Next TaskWing Task with Full Context

Execute these steps IN ORDER. Do not skip any step.

## Step 1: Get Next Task
Call MCP tool ` + "`task_next`" + ` to retrieve the highest priority pending task:
` + "```json" + `
{"session_id": "claude-session"}
` + "```" + `

Extract from the response:
- task_id, title, description
- scope (e.g., "auth", "vectorsearch", "api")
- keywords array
- acceptance_criteria
- suggested_recall_queries

If no task returned, inform user: "No pending tasks. Use 'taskwing plan list' to check plan status."

## Step 2: Fetch Scope-Relevant Context
Call MCP tool ` + "`recall`" + ` with query based on task scope:
` + "```json" + `
{"query": "[task.scope] patterns constraints decisions"}
` + "```" + `

Examples:
- scope "auth" → ` + "`{\"query\": \"authentication cookies session patterns\"}`" + `
- scope "api" → ` + "`{\"query\": \"api handlers middleware patterns\"}`" + `
- scope "vectorsearch" → ` + "`{\"query\": \"lancedb embedding vector patterns\"}`" + `

Extract: patterns, constraints, related decisions.

## Step 3: Fetch Task-Specific Context
Call MCP tool ` + "`recall`" + ` with keywords from the task:

Use ` + "`suggested_recall_queries`" + ` if available, otherwise extract keywords from title.
` + "```json" + `
{"query": "[keywords from task title/description]"}
` + "```" + `

## Step 4: Claim the Task
Call MCP tool ` + "`task_start`" + `:
` + "```json" + `
{"task_id": "[task_id from step 1]", "session_id": "claude-session"}
` + "```" + `

## Step 5: Present Unified Task Brief

Display this EXACT format:

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 TASK: [task_id] (Priority: [priority])
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**[Title]**

## Description
[Full task description]

## Acceptance Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]
- [ ] [Criterion 3]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🏗️ ARCHITECTURE CONTEXT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Relevant Patterns
[Patterns from recall that apply to this task]

## Constraints
[Constraints that must be respected]

## Related Decisions
[Past decisions that inform this work]

## Key Files
[Files likely to be modified based on context]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Task claimed. Ready to begin.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Step 6: Begin Implementation
Proceed with the task, following the patterns and respecting the constraints shown above.

**CRITICAL**: You MUST call all MCP tools (task_next, recall x2, task_start) before showing the brief. Do not proceed without context.

## Fallback (No MCP)
` + "```bash" + `
tw task list                    # List all tasks
tw task show TASK_ID            # View task details
tw context -q "search term"     # Get context
` + "```" + `
`,
		},
		{
			name: "tw-done.md",
			content: `# Complete Task with Architecture-Aware Summary

Execute these steps IN ORDER.

## Step 1: Get Current Task
Call MCP tool ` + "`task_current`" + `:
` + "```json" + `
{"session_id": "claude-session"}
` + "```" + `

If no active task, inform user and stop.

## Step 2: Fetch Original Context
Call MCP tool ` + "`recall`" + ` with the task scope to retrieve patterns/constraints that were meant to be followed:
` + "```json" + `
{"query": "[task.scope] patterns constraints"}
` + "```" + `

## Step 3: Generate Completion Report

Create a structured summary covering:

### Files Modified
List all files changed:
- File path
- Lines added/removed (approximate)
- Purpose of change

### Acceptance Criteria Verification
For each criterion from the task:
- ✅ **Met**: [How it was satisfied]
- ❌ **Not Met**: [Why, and what's needed]
- ⚠️ **Partial**: [What was done, what remains]

### Pattern Compliance
Confirm alignment with codebase patterns from recall:
- [Pattern name]: ✅ Followed / ⚠️ Deviated because [reason]

### Constraint Adherence
Confirm constraints were respected:
- [Constraint]: ✅ Respected / ❌ Violated (requires review)

### Technical Debt / Follow-ups
- TODOs introduced
- Tests not written
- Edge cases not handled

## Step 4: Mark Complete
Call MCP tool ` + "`task_complete`" + `:
` + "```json" + `
{
  "task_id": "[task_id]",
  "summary": "[The structured summary from Step 3]",
  "files_modified": ["path/to/file1.go", "path/to/file2.go"]
}
` + "```" + `

## Step 5: Confirm to User

Display:
` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ TASK COMPLETE: [task_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Summary report from Step 3]

Recorded in TaskWing memory.
Use /tw-next to continue with next priority task.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

**CRITICAL**: Do not tell user task is complete until task_complete MCP returns success.

## Fallback (No MCP)
` + "```bash" + `
tw task complete TASK_ID
` + "```" + `
`,
		},
		{
			name: "tw-context.md",
			content: `# Fetch Additional Context for Current Task

Use this when you need more architectural context mid-task.

## Step 1: Get Current Task
Call MCP tool ` + "`task_current`" + `:
` + "```json" + `
{"session_id": "claude-session"}
` + "```" + `

Extract the task scope and keywords.

## Step 2: Fetch Requested Context
Call MCP tool ` + "`recall`" + `:

If user provided a query argument:
` + "```json" + `
{"query": "[user's query]"}
` + "```" + `

If no query provided, use task scope:
` + "```json" + `
{"query": "[task.scope] patterns constraints decisions"}
` + "```" + `

## Step 3: Display Context

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔍 CONTEXT: [query]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Patterns
[Pattern results from recall]

## Constraints
[Constraint results from recall]

## Decisions
[Decision results from recall]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Usage Examples
- ` + "`/tw-context`" + ` - Get context for current task scope
- ` + "`/tw-context authentication`" + ` - Search for auth-related context
- ` + "`/tw-context error handling patterns`" + ` - Specific search

## Fallback (No MCP)
` + "```bash" + `
tw context -q "search term"
tw context --answer "question about codebase"
` + "```" + `
`,
		},
		{
			name: "tw-status.md",
			content: `# Show Current Task Status with Context

## Step 1: Get Current Task
Call MCP tool ` + "`task_current`" + `:
` + "```json" + `
{"session_id": "claude-session"}
` + "```" + `

If no active task:
` + "```" + `
No active task. Use /tw-next to start the next priority task.
` + "```" + `

## Step 2: Get Task Context
Call MCP tool ` + "`recall`" + ` with task scope:
` + "```json" + `
{"query": "[task.scope] constraints patterns"}
` + "```" + `

## Step 3: Display Status

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 CURRENT TASK STATUS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Task: [task_id] - [title]
Priority: [priority]
Status: [status]
Started: [claimed_at timestamp]
Scope: [scope]

## Acceptance Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]

## Active Constraints
[Constraints from recall that apply to this task]

## Patterns to Follow
[Patterns from recall for this scope]

## Dependencies
[List of dependent tasks and their status]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Commands:
  /tw-done    - Complete this task
  /tw-block   - Mark as blocked
  /tw-context - Fetch more context
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

## Fallback (No MCP)
` + "```bash" + `
tw task list --status in_progress
tw plan list
` + "```" + `
`,
		},
		{
			name: "tw-block.md",
			content: `# Mark Current Task as Blocked

Use this when you cannot proceed with the current task.

## Step 1: Get Current Task
Call MCP tool ` + "`task_current`" + `:
` + "```json" + `
{"session_id": "claude-session"}
` + "```" + `

Confirm task_id and current status.

## Step 2: Document the Blocker
Identify the specific reason:
- Missing API documentation or credentials
- Dependent task not completed
- Need clarification from user
- External service unavailable
- Technical limitation discovered

## Step 3: Block the Task
Call MCP tool ` + "`task_block`" + `:
` + "```json" + `
{
  "task_id": "[task_id]",
  "reason": "[Detailed description of why blocked and what's needed to unblock]"
}
` + "```" + `

## Step 4: Confirm and Offer Next Steps

` + "```" + `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚫 TASK BLOCKED: [task_id]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Reason: [block reason]

To unblock: [specific action needed]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
` + "```" + `

Ask user: "Would you like to work on the next available task? Use /tw-next to continue."

## Step 5: Unblocking (when ready)
When the blocker is resolved, call MCP tool ` + "`task_unblock`" + `:
` + "```json" + `
{"task_id": "[task_id]"}
` + "```" + `

## Fallback (No MCP)
` + "```bash" + `
tw task update TASK_ID --status blocked
` + "```" + `
`,
		},
	}

	for _, cmd := range commands {
		filePath := filepath.Join(commandsDir, cmd.name)
		if err := os.WriteFile(filePath, []byte(cmd.content), 0644); err != nil {
			return fmt.Errorf("create %s: %w", cmd.name, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s/%s\n", aiCfg.commandsDir, cmd.name)
		}
	}

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
		// Use the correct installer command for each AI assistant
		fileName = "taskwing.md"
		content = `# Fetch project architecture context

Retrieve codebase knowledge (patterns, decisions, constraints) via the TaskWing MCP server.

## Prerequisites
TaskWing MCP server must be configured. If not set up, run:
` + "```bash" + `
tw mcp install ` + aiCfg.name + `
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

// HooksConfig represents the hooks configuration for AI assistants
type HooksConfig struct {
	Hooks map[string][]HookMatcher `json:"hooks"`
}

// HookMatcher represents a hook trigger
type HookMatcher struct {
	Hooks []HookCommand `json:"hooks"`
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

	// Create hooks configuration
	config := HooksConfig{
		Hooks: map[string][]HookMatcher{
			"SessionStart": {
				{
					Hooks: []HookCommand{
						{
							Type:    "command",
							Command: "taskwing hook session-init",
							Timeout: 10,
						},
					},
				},
			},
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
			"SessionEnd": {
				{
					Hooks: []HookCommand{
						{
							Type:    "command",
							Command: "taskwing hook session-end",
							Timeout: 5,
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

// createGeminiTaskCommands creates TOML format task commands for Gemini CLI
func createGeminiTaskCommands(basePath string, aiCfg aiConfig, verbose bool) error {
	commandsDir := filepath.Join(basePath, aiCfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	// Define task lifecycle commands in TOML format
	commands := []struct {
		name    string
		content string
	}{
		{
			name: "tw-next.toml",
			content: `description = "Start next TaskWing task with full context"

prompt = """Execute these steps IN ORDER:

1. Call MCP tool task_next with: {"session_id": "gemini-session"}
   Extract: task_id, title, description, scope, keywords, acceptance_criteria

2. Call MCP tool recall with: {"query": "[scope] patterns constraints"}
   Extract architecture context for this task.

3. Call MCP tool task_start with: {"task_id": "[task_id]", "session_id": "gemini-session"}

4. Display the task brief and begin implementation.

IMPORTANT: Complete all MCP calls before starting work."""
`,
		},
		{
			name: "tw-done.toml",
			content: `description = "Complete current TaskWing task"

prompt = """Execute these steps:

1. Call MCP tool task_current with: {"session_id": "gemini-session"}
   If no active task, inform user.

2. Call MCP tool recall with: {"query": "[task.scope] patterns constraints"}
   Verify work followed architecture patterns.

3. Create a summary of:
   - Files modified
   - Acceptance criteria status
   - Pattern compliance

4. Call MCP tool task_complete with:
   {"task_id": "[task_id]", "summary": "[summary]", "files_modified": ["file1", "file2"]}

5. Inform user task is complete. Suggest running /tw-next for next task."""
`,
		},
		{
			name: "tw-status.toml",
			content: `description = "Show current TaskWing task status"

prompt = """Execute these steps:

1. Call MCP tool task_current with: {"session_id": "gemini-session"}
   If no active task, tell user to run /tw-next.

2. Call MCP tool recall with: {"query": "[task.scope] constraints patterns"}

3. Display task status including:
   - Task ID, title, priority
   - Acceptance criteria
   - Relevant constraints and patterns"""
`,
		},
		{
			name: "tw-block.toml",
			content: `description = "Mark current TaskWing task as blocked"

prompt = """Execute these steps:

1. Call MCP tool task_current with: {"session_id": "gemini-session"}
   If no active task, inform user.

2. Ask user for block reason if not provided.

3. Call MCP tool task_block with:
   {"task_id": "[task_id]", "reason": "[detailed reason]"}

4. Confirm task is blocked. Suggest /tw-next for next available task."""
`,
		},
		{
			name: "tw-context.toml",
			content: `description = "Fetch TaskWing architecture context"

prompt = """Call the TaskWing MCP recall tool to get architecture context.

If user provided a query, use: {"query": "[user's query]"}
Otherwise, get task scope and use: {"query": "[scope] patterns constraints decisions"}

Display the results organized by:
- Patterns
- Constraints
- Decisions"""
`,
		},
	}

	for _, cmd := range commands {
		filePath := filepath.Join(commandsDir, cmd.name)
		if err := os.WriteFile(filePath, []byte(cmd.content), 0644); err != nil {
			return fmt.Errorf("create %s: %w", cmd.name, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s/%s\n", aiCfg.commandsDir, cmd.name)
		}
	}

	return nil
}
