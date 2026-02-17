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
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/logger"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/project"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// bootstrapCmd represents the bootstrap command
var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Initialize project memory with LLM-powered analysis",
	Long: `Initialize TaskWing for your repository.

Bootstrap analyzes your codebase to extract architectural knowledge:
  • Creates .taskwing/ directory structure
  • Sets up AI assistant integration (Claude, Cursor, etc.)
  • Auto-repairs managed local AI config drift
  • Indexes code symbols (functions, types, etc.)
  • Analyzes code patterns and architecture (requires API key)
  • Extracts decisions and understands WHY choices were made

Bootstrap does NOT adopt unmanaged AI config automatically and does NOT mutate
global MCP in run mode. Use "taskwing doctor --fix" for explicit repair flows.

Requires an LLM API key (set via 'taskwing config set' or provider-specific env var).

Use --skip-analyze for CI/testing (deterministic, no LLM).`,
	RunE: runBootstrap,
}

// runBootstrap is the main bootstrap command handler.
// It follows a three-phase architecture: Probe → Plan → Execute
func runBootstrap(cmd *cobra.Command, args []string) error {
	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 0: Parse and Validate Flags
	// ═══════════════════════════════════════════════════════════════════════
	onlyAgents, _ := cmd.Flags().GetStringSlice("only-agents")
	flags := bootstrap.Flags{
		Preview:     getBoolFlag(cmd, "preview"),
		SkipInit:    getBoolFlag(cmd, "skip-init"),
		SkipIndex:   getBoolFlag(cmd, "skip-index"),
		SkipAnalyze: getBoolFlag(cmd, "skip-analyze"),
		Force:       getBoolFlag(cmd, "force"),
		Resume:      getBoolFlag(cmd, "resume"),
		OnlyAgents:  onlyAgents,
		Trace:       getBoolFlag(cmd, "trace"),
		TraceStdout: getBoolFlag(cmd, "trace-stdout"),
		TraceFile:   getStringFlag(cmd, "trace-file"),
		Verbose:     viper.GetBool("verbose"),
		Quiet:       viper.GetBool("quiet"),
		Debug:       getBoolFlag(cmd, "debug"),
	}

	// Validate flags early - fail fast on contradictions
	if err := bootstrap.ValidateFlags(flags); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Handle --timeout flag: set TASKWING_LLM_TIMEOUT env var to override default
	// This must be done before LLM client creation to ensure the timeout is picked up
	if timeout, _ := cmd.Flags().GetDuration("timeout"); timeout > 0 {
		if err := os.Setenv("TASKWING_LLM_TIMEOUT", timeout.String()); err != nil {
			return fmt.Errorf("set timeout env var: %w", err)
		}
		if flags.Debug {
			fmt.Fprintf(os.Stderr, "[debug] LLM timeout set to %v via --timeout flag\n", timeout)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	// Track user input for crash logging
	logger.SetLastInput(fmt.Sprintf("bootstrap (skip-analyze=%v, dir=%s)", flags.SkipAnalyze, cwd))

	// Debug mode: dump diagnostic info early
	if flags.Debug {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "╭─────────────────────────────────────────────────────────────╮")
		fmt.Fprintln(os.Stderr, "│                    DEBUG MODE ENABLED                       │")
		fmt.Fprintln(os.Stderr, "╰─────────────────────────────────────────────────────────────╯")
		fmt.Fprintf(os.Stderr, "[debug] cwd: %s\n", cwd)

		// Dump fresh project detection (what SHOULD be used)
		fmt.Fprintln(os.Stderr, "[debug] --- Fresh project.Detect(cwd) ---")
		freshCtx, _ := project.Detect(cwd)
		if freshCtx != nil {
			fmt.Fprintf(os.Stderr, "[debug] fresh.RootPath: %s\n", freshCtx.RootPath)
			fmt.Fprintf(os.Stderr, "[debug] fresh.GitRoot: %s\n", freshCtx.GitRoot)
			fmt.Fprintf(os.Stderr, "[debug] fresh.MarkerType: %s\n", freshCtx.MarkerType)
			fmt.Fprintf(os.Stderr, "[debug] fresh.IsMonorepo: %v\n", freshCtx.IsMonorepo)
			fmt.Fprintf(os.Stderr, "[debug] fresh.RelativeGitPath(): %s\n", freshCtx.RelativeGitPath())
		} else {
			fmt.Fprintln(os.Stderr, "[debug] fresh.Detect() returned nil")
		}

		// Dump cached config.GetProjectContext() (what agents ACTUALLY use)
		fmt.Fprintln(os.Stderr, "[debug] --- Cached config.GetProjectContext() ---")
		cachedCtx := config.GetProjectContext()
		if cachedCtx != nil {
			fmt.Fprintf(os.Stderr, "[debug] cached.RootPath: %s\n", cachedCtx.RootPath)
			fmt.Fprintf(os.Stderr, "[debug] cached.GitRoot: %s\n", cachedCtx.GitRoot)
			fmt.Fprintf(os.Stderr, "[debug] cached.MarkerType: %s\n", cachedCtx.MarkerType)
			fmt.Fprintf(os.Stderr, "[debug] cached.IsMonorepo: %v\n", cachedCtx.IsMonorepo)
			fmt.Fprintf(os.Stderr, "[debug] cached.RelativeGitPath(): %s\n", cachedCtx.RelativeGitPath())
		} else {
			fmt.Fprintln(os.Stderr, "[debug] cached.GetProjectContext() returned nil")
		}
		fmt.Fprintln(os.Stderr, "")
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 1: Probe Environment (no side effects)
	// ═══════════════════════════════════════════════════════════════════════
	existingGlobalAIs := detectExistingMCPConfigs()
	globalMCPSet := make(map[string]bool, len(existingGlobalAIs))
	for _, ai := range existingGlobalAIs {
		globalMCPSet[ai] = true
	}
	previousDetector := bootstrap.GlobalMCPDetector
	bootstrap.GlobalMCPDetector = func(aiName string) bool {
		return globalMCPSet[aiName]
	}
	defer func() { bootstrap.GlobalMCPDetector = previousDetector }()

	snapshot, err := bootstrap.ProbeEnvironment(cwd)
	if err != nil {
		return fmt.Errorf("probe environment: %w", err)
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 2: Decide Plan (pure function, deterministic)
	// ═══════════════════════════════════════════════════════════════════════
	plan := bootstrap.DecidePlan(snapshot, flags)

	// Always show plan summary (even in quiet mode, single line)
	fmt.Print(bootstrap.FormatPlanSummary(plan, flags.Quiet))

	// Handle error mode
	if plan.Mode == bootstrap.ModeError {
		return plan.Error
	}

	// Handle preview mode - show plan and exit
	if flags.Preview {
		fmt.Println("\n💡 Preview mode - no changes made.")
		return nil
	}

	// Handle NoOp mode
	if plan.Mode == bootstrap.ModeNoOp {
		if !flags.Quiet {
			fmt.Println("\n✅ Nothing to do - configuration is up to date.")
		}
		return nil
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 3: Execute Plan
	// ═══════════════════════════════════════════════════════════════════════

	// Load LLM config only if plan requires it
	var llmCfg llm.Config
	if plan.RequiresLLMConfig {
		llmCfg, err = getLLMConfigForRole(cmd, llm.RoleBootstrap)
		if err != nil {
			return fmt.Errorf("TaskWing requires an LLM API key to analyze your architecture.\nConfigure via 'taskwing config set' or set a provider-specific env var (e.g. TASKWING_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, GOOGLE_API_KEY, BEDROCK_API_KEY).\nUse --skip-analyze for CI/testing without LLM: %w", err)
		}
	}

	// Initialize Service
	svc := bootstrap.NewService(cwd, llmCfg)

	// Execute actions in order
	for _, action := range plan.Actions {
		if err := executeAction(cmd.Context(), action, svc, cwd, flags, plan, llmCfg); err != nil {
			return err
		}
	}

	// Final success message
	if !flags.Quiet {
		if len(plan.ManagedDriftAIs) > 0 {
			fmt.Printf("managed_drift_fixed: %s\n", strings.Join(plan.ManagedDriftAIs, ", "))
		}
		if len(plan.UnmanagedDriftAIs) > 0 {
			fmt.Printf("unmanaged_drift_detected: %s\n", strings.Join(plan.UnmanagedDriftAIs, ", "))
			fmt.Println("  ↳ Run `taskwing doctor --fix --adopt-unmanaged` to claim and repair unmanaged TaskWing-like configs.")
		}
		if len(plan.GlobalMCPDriftAIs) > 0 {
			fmt.Printf("global_mcp_drift_detected: %s\n", strings.Join(plan.GlobalMCPDriftAIs, ", "))
			fmt.Println("  ↳ Run `taskwing doctor --fix` to repair global MCP registration.")
		}
		fmt.Println()
		fmt.Println("✅ Bootstrap complete!")
	}

	return nil
}

// executeAction executes a single bootstrap action.
func executeAction(ctx context.Context, action bootstrap.Action, svc *bootstrap.Service, cwd string, flags bootstrap.Flags, plan *bootstrap.Plan, llmCfg llm.Config) error {
	switch action {
	case bootstrap.ActionInitProject:
		return executeInitProject(svc, flags, plan)

	case bootstrap.ActionGenerateAIConfigs:
		return executeGenerateAIConfigs(svc, flags, plan)

	case bootstrap.ActionInstallMCP:
		return executeInstallMCP(cwd, flags, plan)

	case bootstrap.ActionIndexCode:
		return executeIndexCode(ctx, cwd, flags)

	case bootstrap.ActionExtractMetadata:
		return executeExtractMetadata(ctx, svc, flags)

	case bootstrap.ActionLLMAnalyze:
		return executeLLMAnalyze(ctx, svc, cwd, flags, llmCfg)

	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// executeInitProject handles project initialization with user prompts.
func executeInitProject(svc *bootstrap.Service, flags bootstrap.Flags, plan *bootstrap.Plan) error {
	var selectedAIs []string

	if plan.RequiresUserInput {
		// In non-interactive environments, avoid TTY prompts and use deterministic defaults.
		if !ui.IsInteractive() {
			switch {
			case len(plan.AIsNeedingRepair) > 0:
				selectedAIs = append(selectedAIs, plan.AIsNeedingRepair...)
			case len(plan.SuggestedAIs) > 0:
				selectedAIs = append(selectedAIs, plan.SuggestedAIs...)
			}
			if !flags.Quiet {
				if len(selectedAIs) > 0 {
					fmt.Printf("🤖 Non-interactive mode: configuring AI integrations for %s\n", strings.Join(selectedAIs, ", "))
				} else {
					fmt.Println("🤖 Non-interactive mode: no AI assistant selected; initializing project memory only")
				}
			}
		} else {
			// Show appropriate prompt based on mode
			switch plan.Mode {
			case bootstrap.ModeFirstTime:
				if len(plan.SuggestedAIs) > 0 {
					fmt.Println("📋 Setting up local project")
					fmt.Printf("🔍 Detected global config for: %s\n", strings.Join(plan.SuggestedAIs, ", "))
				} else {
					fmt.Println("🚀 First time setup")
				}
				fmt.Println()
				fmt.Println("🤖 Which AI assistant(s) do you use?")
				fmt.Println()
				selectedAIs = promptAISelection(plan.SuggestedAIs...)

			case bootstrap.ModeRepair:
				if len(plan.AIsNeedingRepair) > 0 {
					fmt.Println("🔧 Restoring missing AI configurations")
					fmt.Printf("   Missing: %s\n", strings.Join(plan.AIsNeedingRepair, ", "))
					fmt.Print("   Restore? [Y/n]: ")
					var input string
					_, _ = fmt.Scanln(&input)
					input = strings.TrimSpace(strings.ToLower(input))
					if input == "" || input == "y" || input == "yes" {
						selectedAIs = plan.AIsNeedingRepair
					} else {
						fmt.Println()
						fmt.Println("🤖 Which AI assistant(s) do you want to set up?")
						selectedAIs = promptAISelection(plan.SuggestedAIs...)
					}
				}

			case bootstrap.ModeReconfigure:
				fmt.Println("🔧 No AI configurations found - let's set them up")
				fmt.Println()
				fmt.Println("🤖 Which AI assistant(s) do you use?")
				fmt.Println()
				selectedAIs = promptAISelection()
			}
		}
		if len(selectedAIs) == 0 && !flags.Quiet {
			fmt.Println("\n⚠️  No AI assistants selected - continuing with local project initialization only")
		}
	}

	// Store selected AIs in plan for subsequent actions
	plan.SelectedAIs = selectedAIs

	// Initialize project
	if err := svc.InitializeProject(flags.Verbose, selectedAIs); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	fmt.Println("✓ Project initialized")
	return nil
}

// executeGenerateAIConfigs generates AI slash commands and hooks.
// This runs standalone when ActionInitProject isn't in the plan (e.g., ModeRepair with healthy project).
func executeGenerateAIConfigs(svc *bootstrap.Service, flags bootstrap.Flags, plan *bootstrap.Plan) error {
	// Determine which AIs to configure
	var targetAIs []string
	if len(plan.SelectedAIs) > 0 {
		// User already selected AIs (from executeInitProject or previous step)
		targetAIs = plan.SelectedAIs
	} else if len(plan.AIsNeedingRepair) > 0 {
		// In repair mode, use the AIs that need repair
		targetAIs = plan.AIsNeedingRepair
	}

	if len(targetAIs) == 0 {
		// No AIs to configure - this is a no-op
		return nil
	}

	// Generate configs for each target AI
	if !flags.Quiet {
		fmt.Printf("🔧 Regenerating AI configurations for: %s\n", strings.Join(targetAIs, ", "))
	}

	if err := svc.RegenerateAIConfigs(flags.Verbose, targetAIs); err != nil {
		return fmt.Errorf("regenerate AI configs failed: %w", err)
	}

	if !flags.Quiet {
		fmt.Println("✓ AI configurations regenerated")
	}
	return nil
}

// executeInstallMCP registers MCP servers with AI CLIs.
func executeInstallMCP(cwd string, flags bootstrap.Flags, plan *bootstrap.Plan) error {
	// Determine which AIs need MCP registration
	var targetAIs []string
	if len(plan.SelectedAIs) > 0 {
		targetAIs = plan.SelectedAIs
	} else if len(plan.SuggestedAIs) > 0 {
		targetAIs = plan.SuggestedAIs
	}

	if len(targetAIs) == 0 {
		return nil
	}

	// Build set of AIs that already have global MCP configured
	globalSet := make(map[string]bool)
	existingGlobalAIs := detectExistingMCPConfigs()
	for _, ai := range existingGlobalAIs {
		globalSet[ai] = true
	}

	// Filter to only AIs that need registration
	var aisNeedingRegistration []string
	for _, ai := range targetAIs {
		if !globalSet[ai] {
			aisNeedingRegistration = append(aisNeedingRegistration, ai)
		}
	}

	if len(aisNeedingRegistration) == 0 {
		if !flags.Quiet && len(existingGlobalAIs) > 0 {
			fmt.Printf("✓ MCP already configured globally for: %s\n", strings.Join(existingGlobalAIs, ", "))
		}
		return nil
	}

	if !flags.Quiet {
		fmt.Printf("🔌 Installing MCP servers for: %s\n", strings.Join(aisNeedingRegistration, ", "))
	}

	installMCPServers(cwd, aisNeedingRegistration)

	if !flags.Quiet {
		fmt.Println("✓ MCP servers installed")
	}
	return nil
}

// executeIndexCode runs code symbol indexing.
func executeIndexCode(ctx context.Context, cwd string, flags bootstrap.Flags) error {
	if err := runCodeIndexing(ctx, cwd, flags.Force, flags.Quiet); err != nil {
		// Non-fatal: log and continue
		if !flags.Quiet {
			fmt.Fprintf(os.Stderr, "⚠️  Code indexing failed: %v\n", err)
		}
	}
	return nil
}

// executeExtractMetadata runs deterministic metadata extraction.
func executeExtractMetadata(ctx context.Context, svc *bootstrap.Service, flags bootstrap.Flags) error {
	result, err := svc.RunDeterministicBootstrap(ctx, flags.Quiet)
	if err != nil {
		if !flags.Quiet {
			fmt.Fprintf(os.Stderr, "⚠️  Metadata extraction failed: %v\n", err)
		}
	} else if result != nil && len(result.Warnings) > 0 && flags.Verbose {
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "   [warn] %s\n", w)
		}
	}
	return nil
}

// executeLLMAnalyze runs LLM-powered deep analysis.
func executeLLMAnalyze(ctx context.Context, svc *bootstrap.Service, cwd string, flags bootstrap.Flags, llmCfg llm.Config) error {
	// Detect workspace type
	ws, err := project.DetectWorkspace(cwd)
	if err != nil {
		return fmt.Errorf("detect workspace: %w", err)
	}

	// Handle multi-repo workspaces
	if ws.IsMultiRepo() {
		return runMultiRepoBootstrap(ctx, svc, ws, flags.Preview)
	}

	// Run agent TUI flow with LLM analysis
	return runAgentTUI(ctx, svc, cwd, llmCfg, flags)
}

// Helper functions for flag parsing
func getBoolFlag(cmd *cobra.Command, name string) bool {
	val, _ := cmd.Flags().GetBool(name)
	return val
}

func getStringFlag(cmd *cobra.Command, name string) string {
	val, _ := cmd.Flags().GetString(name)
	return val
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().Bool("skip-init", false, "Skip initialization prompt")
	bootstrapCmd.Flags().Bool("skip-index", false, "Skip code indexing (symbol extraction)")
	bootstrapCmd.Flags().Bool("force", false, "Force indexing even for large codebases (>5000 files)")
	bootstrapCmd.Flags().Bool("skip-analyze", false, "Skip LLM analysis (for CI/testing)")
	bootstrapCmd.Flags().Bool("resume", false, "Resume from last checkpoint (skip completed agents)")
	bootstrapCmd.Flags().StringSlice("only-agents", nil, "Run only specified agents (e.g., --only-agents=code,doc)")
	bootstrapCmd.Flags().Bool("trace", false, "Emit JSON event stream to stderr")
	bootstrapCmd.Flags().String("trace-file", "", "Write JSON event stream to file (default: .taskwing/logs/bootstrap.trace.jsonl)")
	bootstrapCmd.Flags().Bool("trace-stdout", false, "Emit JSON event stream to stderr (overrides trace file)")
	bootstrapCmd.Flags().Bool("debug", false, "Enable debug logging (dumps project context, git paths, agent inputs)")
	bootstrapCmd.Flags().Duration("timeout", 0, "LLM request timeout (e.g., 5m, 10m). Overrides TASKWING_LLM_TIMEOUT env var. Default: 5m")

	// Hide --skip-analyze from main help (documented in CLAUDE.md)
	_ = bootstrapCmd.Flags().MarkHidden("skip-analyze")
}

// runAgentTUI handles the interactive UI part, delegating work to the service
func runAgentTUI(ctx context.Context, svc *bootstrap.Service, cwd string, llmCfg llm.Config, flags bootstrap.Flags) error {
	fmt.Println("")
	ui.RenderPageHeader("TaskWing Bootstrap", fmt.Sprintf("Using: %s (%s)", llmCfg.Model, llmCfg.Provider))

	projectName := filepath.Base(cwd)
	allAgents := bootstrap.NewDefaultAgents(llmCfg, cwd)
	defer core.CloseAgents(allAgents)

	// Open repository for checkpoint tracking
	repo, repoErr := openRepo()
	if repoErr != nil && flags.Resume {
		fmt.Fprintf(os.Stderr, "⚠️  Cannot resume: %v\n", repoErr)
		flags.Resume = false
	}
	if repo != nil {
		defer func() { _ = repo.Close() }()
	}

	// Filter agents based on flags
	agentsList, skippedAgents := filterAgents(allAgents, flags, repo)

	// Show skipped agents
	if len(skippedAgents) > 0 && !flags.Quiet {
		fmt.Printf("⏭️  Skipping completed agents: %s\n", strings.Join(skippedAgents, ", "))
	}

	// If all agents were skipped, nothing to do
	if len(agentsList) == 0 {
		if !flags.Quiet {
			fmt.Println("✅ All agents already completed. Use 'bootstrap' without --resume to re-run.")
		}
		return nil
	}

	input := core.Input{
		BasePath:    cwd,
		ProjectName: projectName,
		Mode:        core.ModeBootstrap,
		Verbose:     flags.Verbose || flags.Debug,
	}

	stream := core.NewStreamingOutput(100)
	defer stream.Close()

	traceCleanup := setupTrace(stream, flags.Trace, flags.TraceFile, flags.TraceStdout, cwd)
	defer traceCleanup()

	// Run TUI
	tuiModel := ui.NewBootstrapModel(ctx, input, agentsList, stream)
	programOptions := []tea.ProgramOption{}
	if !ui.IsInteractive() {
		// Headless fallback for CI/non-TTY environments.
		programOptions = append(programOptions, tea.WithInput(nil), tea.WithoutRenderer())
	}
	p := tea.NewProgram(tuiModel, programOptions...)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	bootstrapModel, ok := finalModel.(ui.BootstrapModel)
	if !ok || (bootstrapModel.Quitting && len(bootstrapModel.Results) < len(agentsList)) {
		fmt.Println("\n⚠️  Bootstrap cancelled.")
		return nil
	}

	// Update checkpoint state for completed agents
	if repo != nil {
		store := repo.GetDB()
		if store != nil {
			updateAgentCheckpoints(bootstrapModel.Agents, store)
		}
	}

	// Check failures
	if err := checkAgentFailures(bootstrapModel.Agents); err != nil {
		return err
	}

	// Delegate processing/saving to service
	allFindings := core.AggregateFindings(bootstrapModel.Results)
	allRelationships := core.AggregateRelationships(bootstrapModel.Results)

	return svc.ProcessAndSaveResults(ctx, bootstrapModel.Results, allFindings, allRelationships, flags.Preview, viper.GetBool("quiet"))
}

// filterAgents filters agents based on resume state and --only-agents flag.
// Returns the filtered list and names of skipped agents.
func filterAgents(agents []core.Agent, flags bootstrap.Flags, repo *memory.Repository) ([]core.Agent, []string) {
	var filtered []core.Agent
	var skipped []string

	// Build set of agents to run (if --only-agents specified)
	onlySet := make(map[string]bool)
	for _, name := range flags.OnlyAgents {
		onlySet[strings.ToLower(name)] = true
	}

	// Get store for checkpoint queries
	var store *memory.SQLiteStore
	if repo != nil {
		store = repo.GetDB()
	}

	for _, agent := range agents {
		name := agent.Name()

		// Check --only-agents filter
		if len(onlySet) > 0 && !onlySet[strings.ToLower(name)] {
			skipped = append(skipped, name+" (filtered)")
			continue
		}

		// Check resume state
		if flags.Resume && store != nil {
			completed, err := store.HasCompletedBootstrap(name)
			if err == nil && completed {
				skipped = append(skipped, name+" (cached)")
				continue
			}
		}

		filtered = append(filtered, agent)
	}

	return filtered, skipped
}

// updateAgentCheckpoints updates the bootstrap_state table based on agent results.
func updateAgentCheckpoints(agents []*ui.AgentState, store *memory.SQLiteStore) {
	for _, agent := range agents {
		state := &memory.BootstrapState{
			Component: agent.Name,
		}

		switch agent.Status {
		case ui.StatusDone:
			state.Status = memory.BootstrapStatusCompleted
			if agent.Result != nil {
				state.Metadata = map[string]any{
					"findings_count": len(agent.Result.Findings),
					"duration_ms":    agent.Result.Duration.Milliseconds(),
				}
			}
		case ui.StatusError:
			state.Status = memory.BootstrapStatusFailed
			if agent.Err != nil {
				state.ErrorMessage = agent.Err.Error()
			}
		default:
			state.Status = memory.BootstrapStatusPending
		}

		_ = store.SetBootstrapState(state)
	}
}

// runMultiRepoBootstrap uses the service to analyze multiple repos
func runMultiRepoBootstrap(ctx context.Context, svc *bootstrap.Service, ws *project.WorkspaceInfo, preview bool) error {
	fmt.Println("")
	ui.RenderPageHeader("TaskWing Multi-Repo Bootstrap", fmt.Sprintf("Workspace: %s | Services: %d", ws.Name, ws.ServiceCount()))

	fmt.Printf("📦 Detected %d services. Running parallel analysis...\n", ws.ServiceCount())

	findings, relationships, errs, err := svc.RunMultiRepoAnalysis(ctx, ws)
	if err != nil {
		return err
	}

	if len(errs) > 0 {
		fmt.Println("\n⚠️  Some services had errors:")
		for _, e := range errs {
			fmt.Printf("   - %s\n", e)
		}
	}

	fmt.Printf("📊 Aggregated: %d findings from %d services\n", len(findings), ws.ServiceCount()-len(errs))

	if preview {
		fmt.Println("\n💡 This was a preview. Run 'taskwing bootstrap' to save to memory.")
		return nil
	}

	return svc.IngestDirectly(ctx, findings, relationships, viper.GetBool("quiet"))
}

// installMCPServers handles the binary installation calls (kept in CLI layer)
func installMCPServers(basePath string, selectedAIs []string) {
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
		case "opencode":
			// OpenCode: creates opencode.json at project root
			// During development, use taskwing-local-dev-mcp for testing changes
			if err := installOpenCode(binPath, basePath); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  OpenCode MCP installation failed: %v\n", err)
			}
		}
	}
}

// setupTrace configures trace logging and returns a cleanup function.
// The cleanup function should be deferred to close the trace file handle.
func setupTrace(stream *core.StreamingOutput, trace bool, traceFile string, traceStdout bool, cwd string) func() {
	if !trace {
		return func() {} // No-op cleanup
	}
	if traceFile == "" {
		traceFile = filepath.Join(cwd, ".taskwing", "logs", "bootstrap.trace.jsonl")
	}
	var out *os.File
	var cleanup func()
	if traceStdout {
		out = os.Stderr
		cleanup = func() {} // Don't close stderr
	} else {
		_ = os.MkdirAll(filepath.Dir(traceFile), 0755)
		f, err := os.OpenFile(traceFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open trace file: %v\n", err)
			return func() {}
		}
		out = f
		cleanup = func() { _ = f.Close() }
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

	return cleanup
}

func checkAgentFailures(agents []*ui.AgentState) error {
	var failedAgents []string
	for _, state := range agents {
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
	return nil
}

// runCodeIndexing runs the code intelligence indexer on the codebase.
// This extracts symbols (functions, types, etc.) for enhanced search and MCP recall.
func runCodeIndexing(ctx context.Context, basePath string, forceIndex, isQuiet bool) error {
	// Open repository to get database handle
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Get database handle
	store := repo.GetDB()
	if store == nil {
		return fmt.Errorf("database store not available")
	}
	db := store.DB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	// Create code intelligence repository and indexer
	codeRepo := codeintel.NewRepository(db)
	config := codeintel.DefaultIndexerConfig()
	indexer := codeintel.NewIndexer(codeRepo, config)

	// Count files first for safety check
	fileCount, err := indexer.CountSupportedFiles(basePath)
	if err != nil {
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "⚠️  Could not count files for indexing: %v\n", err)
		}
		return nil // Non-fatal - skip indexing if we can't count
	}

	// Large codebase safety check
	const maxFilesWithoutForce = 5000
	if fileCount > maxFilesWithoutForce && !forceIndex {
		fmt.Println()
		fmt.Printf("⚠️  Large codebase detected: %d files to index\n", fileCount)
		fmt.Printf("   This may take a while and consume resources.\n")
		fmt.Printf("   Run with --force to proceed, or use --skip-index to bypass.\n")
		return nil // Not an error, just skip
	}

	// Print header
	if !isQuiet {
		fmt.Println()
		fmt.Println("📇 Code Intelligence Indexing")
		fmt.Println("────────────────────────────")
		fmt.Printf("   🔍 Scanning %d source files...\n", fileCount)
	}

	// Configure progress callback with more detail
	var lastUpdate time.Time
	if !isQuiet {
		config.OnProgress = func(stats codeintel.IndexStats) {
			// Throttle updates to avoid flickering
			if time.Since(lastUpdate) < 100*time.Millisecond {
				return
			}
			lastUpdate = time.Now()
			pct := 0
			if stats.FilesScanned > 0 {
				pct = (stats.FilesIndexed * 100) / stats.FilesScanned
			}
			fmt.Fprintf(os.Stderr, "\r   ⚡ Progress: %d%% (%d files, %d symbols)    ", pct, stats.FilesIndexed, stats.SymbolsFound)
		}
	}

	// Re-create indexer with updated config (for progress callback)
	indexer = codeintel.NewIndexer(codeRepo, config)

	// Run indexing
	start := time.Now()

	// Prune stale files first
	prunedCount, err := indexer.PruneStaleFiles(ctx)
	if err != nil && !isQuiet {
		fmt.Fprintf(os.Stderr, "   ⚠️  Prune failed: %v\n", err)
	}

	// Run incremental indexing
	stats, err := indexer.IncrementalIndex(ctx, basePath)
	if err != nil {
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "\r                                                        \n")
			fmt.Fprintf(os.Stderr, "   ⚠️  Indexing failed: %v\n", err)
		}
		return nil // Non-fatal - bootstrap succeeded even if indexing fails
	}

	// Clear progress line and print summary
	if !isQuiet {
		fmt.Fprintf(os.Stderr, "\r                                                        \n")
		duration := time.Since(start)
		fmt.Printf("   ✅ Indexed %d updates, pruned %d files in %v\n",
			stats.FilesIndexed, prunedCount, duration.Round(time.Millisecond))
		if stats.RelationsFound > 0 {
			fmt.Printf("   🔗 Discovered %d call relationships\n", stats.RelationsFound)
		}
		if len(stats.Errors) > 0 {
			fmt.Printf("   ⚠️  %d files skipped (parse errors)\n", len(stats.Errors))
		}
	}

	return nil
}
