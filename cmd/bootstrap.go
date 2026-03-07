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
	"slices"
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
	"github.com/josephgoksu/TaskWing/internal/runner"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// bootstrapCmd represents the bootstrap command
var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Initialize project memory with deterministic code indexing",
	Long: `Initialize TaskWing for your repository.

Bootstrap indexes your codebase deterministically and, when an AI CLI is
detected on PATH, automatically runs architecture analysis:
  • Creates .taskwing/ directory structure
  • Sets up AI assistant integration (Claude, Cursor, etc.)
  • Auto-repairs managed local AI config drift
  • Indexes code symbols (functions, types, etc.)
  • Runs LLM-powered architecture analysis (auto-enabled when AI CLI detected)

Use --no-analyze to skip LLM analysis even when an AI CLI is available.

Bootstrap does NOT adopt unmanaged AI config automatically and does NOT mutate
global MCP in run mode. Use "taskwing doctor --fix" for explicit repair flows.`,
	RunE: runBootstrap,
}

// bootstrapContext accumulates stats across bootstrap phases for the final summary.
type bootstrapContext struct {
	startTime         time.Time
	totalActions      int
	currentAction     int
	filesScanned      int
	symbolsFound      int
	callRelationships int
	indexDuration     time.Duration
	metadataItems     int
	metadataDuration  time.Duration
	analysisFindings  int
	analysisRelations int
	analysisDuration  time.Duration
}

func (bc *bootstrapContext) nextPhase() int {
	bc.currentAction++
	return bc.currentAction
}

func (bc *bootstrapContext) toStats() ui.BootstrapStats {
	return ui.BootstrapStats{
		FilesScanned:      bc.filesScanned,
		SymbolsFound:      bc.symbolsFound,
		CallRelationships: bc.callRelationships,
		MetadataItems:     bc.metadataItems,
		AnalysisFindings:  bc.analysisFindings,
		AnalysisRelations: bc.analysisRelations,
		TotalDuration:     time.Since(bc.startTime),
	}
}

// runBootstrap is the main bootstrap command handler.
// It follows a three-phase architecture: Probe → Plan → Execute
func runBootstrap(cmd *cobra.Command, args []string) error {
	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 0: Parse and Validate Flags
	// ═══════════════════════════════════════════════════════════════════════
	onlyAgents, _ := cmd.Flags().GetStringSlice("only-agents")
	noAnalyze := getBoolFlag(cmd, "no-analyze") || getBoolFlag(cmd, "skip-analyze")
	analyzeExplicit := getBoolFlag(cmd, "analyze")
	autoAnalyze := !noAnalyze && (analyzeExplicit || len(runner.DetectCLIs()) > 0)
	timeout, _ := cmd.Flags().GetDuration("timeout")
	flags := bootstrap.Flags{
		Preview:     getBoolFlag(cmd, "preview"),
		SkipInit:    getBoolFlag(cmd, "skip-init"),
		SkipIndex:   getBoolFlag(cmd, "skip-index"),
		SkipAnalyze: !autoAnalyze, // Auto-enabled when AI CLI detected; opt out with --no-analyze
		Force:       getBoolFlag(cmd, "force"),
		Resume:      getBoolFlag(cmd, "resume"),
		OnlyAgents:  onlyAgents,
		Trace:       getBoolFlag(cmd, "trace"),
		TraceStdout: getBoolFlag(cmd, "trace-stdout"),
		TraceFile:   getStringFlag(cmd, "trace-file"),
		Verbose:     viper.GetBool("verbose"),
		Quiet:       viper.GetBool("quiet"),
		Debug:       getBoolFlag(cmd, "debug"),
		PreferCLI:   getStringFlag(cmd, "prefer-cli"),
		Timeout:     timeout,
		Model:       getStringFlag(cmd, "model"),
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
	logger.SetLastInput(fmt.Sprintf("bootstrap (no-analyze=%v, dir=%s)", flags.SkipAnalyze, cwd))

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

	// Show debug line on stderr if --debug
	if flags.Debug {
		fmt.Fprintln(os.Stderr, bootstrap.FormatPlanDebugLine(plan))
	}

	// Handle error mode
	if plan.Mode == bootstrap.ModeError {
		return plan.Error
	}

	// Show welcome panel and plan box (normal/verbose mode)
	if !flags.Quiet {
		fmt.Println(ui.RenderBootstrapWelcome())

		// Project info line
		fmt.Printf("  Project: %s", filepath.Base(cwd))
		if len(plan.SuggestedAIs) > 0 {
			fmt.Printf(" • AIs: %s", strings.Join(plan.SuggestedAIs, ", "))
		}
		fmt.Println()

		// Detected state
		fmt.Printf("\n  Detected: %s\n", plan.DetectedState)

		// Show plan box with numbered actions
		if len(plan.ActionSummary) > 0 {
			fmt.Print(ui.RenderPlanBox(plan.ActionSummary))
		}

		// Print drift warnings if any
		if len(plan.ManagedDriftAIs) > 0 || len(plan.UnmanagedDriftAIs) > 0 || len(plan.GlobalMCPDriftAIs) > 0 {
			fmt.Println()
			if len(plan.ManagedDriftAIs) > 0 {
				fmt.Printf("  • managed_drift_fixed: %s\n", strings.Join(plan.ManagedDriftAIs, ", "))
			}
			if len(plan.UnmanagedDriftAIs) > 0 {
				fmt.Printf("  • unmanaged_drift_detected: %s\n", strings.Join(plan.UnmanagedDriftAIs, ", "))
			}
			if len(plan.GlobalMCPDriftAIs) > 0 {
				fmt.Printf("  • global_mcp_drift_detected: %s\n", strings.Join(plan.GlobalMCPDriftAIs, ", "))
			}
		}

		// Show warnings
		if len(plan.Warnings) > 0 {
			for _, warning := range plan.Warnings {
				ui.PrintWarning(warning)
			}
		}
	}

	// Handle preview mode - show plan and exit
	if flags.Preview {
		fmt.Println()
		ui.PrintHint("Preview mode - no changes made.")
		return nil
	}

	// Handle NoOp mode
	if plan.Mode == bootstrap.ModeNoOp {
		if !flags.Quiet {
			fmt.Println()
			ui.PrintSuccess("Nothing to do - configuration is up to date.")
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
			return fmt.Errorf("LLM API key required for architecture analysis.\nConfigure via 'taskwing config set' or set a provider-specific env var (e.g. OPENAI_API_KEY, ANTHROPIC_API_KEY, GOOGLE_API_KEY).\nUse --no-analyze to skip: %w", err)
		}
	}

	// Initialize Service
	svc := bootstrap.NewService(cwd, llmCfg)

	// Prompt for repo selection in multi-repo workspaces.
	// This must happen before the action loop because ActionInitProject may not
	// be in the plan (e.g., ModeRun), but ActionLLMAnalyze still needs SelectedRepos.
	if plan.RequiresRepoSelection && slices.Contains(plan.Actions, bootstrap.ActionLLMAnalyze) {
		if ui.IsInteractive() {
			fmt.Println()
			fmt.Printf("%s Found %d repositories\n\n", ui.IconPackage, len(plan.DetectedRepos))
			plan.SelectedRepos = promptRepoSelection(plan.DetectedRepos)
		} else {
			plan.SelectedRepos = plan.DetectedRepos
			if !flags.Quiet {
				fmt.Printf("%s Non-interactive mode: bootstrapping all %d repositories\n", ui.IconPackage, len(plan.DetectedRepos))
			}
		}
	}

	// Execute actions in order
	bCtx := &bootstrapContext{startTime: time.Now(), totalActions: len(plan.Actions)}

	for _, action := range plan.Actions {
		if !flags.Quiet {
			ui.PrintPhaseSeparator()
		}
		if err := executeAction(cmd.Context(), action, svc, cwd, flags, plan, llmCfg, bCtx); err != nil {
			return err
		}
	}

	// Final output
	if !flags.Quiet {
		ui.PrintPhaseSeparator()

		// Drift warnings
		if len(plan.UnmanagedDriftAIs) > 0 {
			fmt.Println()
			ui.PrintWarning(fmt.Sprintf("unmanaged_drift_detected: %s", strings.Join(plan.UnmanagedDriftAIs, ", ")))
			fmt.Println("  ↳ Run `taskwing doctor --fix --adopt-unmanaged` to claim and repair unmanaged TaskWing-like configs.")
		}
		if len(plan.GlobalMCPDriftAIs) > 0 {
			fmt.Println()
			ui.PrintWarning(fmt.Sprintf("global_mcp_drift_detected: %s", strings.Join(plan.GlobalMCPDriftAIs, ", ")))
			fmt.Println("  ↳ Run `taskwing doctor --fix` to repair global MCP registration.")
		}

		// Summary panel
		fmt.Println()
		fmt.Println(ui.RenderBootstrapSummary(bCtx.toStats()))
	} else {
		fmt.Printf("✔ Bootstrap complete (%s)\n", ui.FormatDuration(bCtx.toStats().TotalDuration))
	}

	return nil
}

// executeAction executes a single bootstrap action.
func executeAction(ctx context.Context, action bootstrap.Action, svc *bootstrap.Service, cwd string, flags bootstrap.Flags, plan *bootstrap.Plan, llmCfg llm.Config, bCtx *bootstrapContext) error {
	switch action {
	case bootstrap.ActionInitProject:
		if err := executeInitProject(svc, flags, plan, bCtx); err != nil {
			return err
		}
		// Re-detect project context now that local .taskwing/ exists.
		// Without this, the cached context still points to ~/.taskwing/ (HOME)
		// and all subsequent DB operations write to the wrong database.
		if freshCtx, err := project.Detect(cwd); err == nil {
			_ = config.SetProjectContext(freshCtx)
		}
		return nil

	case bootstrap.ActionGenerateAIConfigs:
		return executeGenerateAIConfigs(svc, flags, plan, bCtx)

	case bootstrap.ActionInstallMCP:
		return executeInstallMCP(cwd, flags, plan, bCtx)

	case bootstrap.ActionIndexCode:
		return executeIndexCode(ctx, cwd, flags, bCtx)

	case bootstrap.ActionExtractMetadata:
		return executeExtractMetadata(ctx, svc, flags, bCtx)

	case bootstrap.ActionLLMAnalyze:
		return executeLLMAnalyze(ctx, svc, cwd, flags, llmCfg, plan, bCtx)

	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// executeInitProject handles project initialization with user prompts.
func executeInitProject(svc *bootstrap.Service, flags bootstrap.Flags, plan *bootstrap.Plan, bCtx *bootstrapContext) error {
	if !flags.Quiet {
		ui.PrintPhaseHeader(bCtx.nextPhase(), bCtx.totalActions, ui.IconRocket,
			"Initializing Project",
			"Creating .taskwing/ directory structure and AI configurations.")
	} else {
		step := bCtx.nextPhase()
		fmt.Printf("[%d/%d] Initializing project... ", step, bCtx.totalActions)
	}
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
					fmt.Printf("%s Non-interactive mode: configuring AI integrations for %s\n", ui.IconRobot, strings.Join(selectedAIs, ", "))
				} else {
					fmt.Printf("%s Non-interactive mode: no AI assistant selected; initializing project memory only\n", ui.IconRobot)
				}
			}
		} else {
			// Show appropriate prompt based on mode
			switch plan.Mode {
			case bootstrap.ModeFirstTime:
				if len(plan.SuggestedAIs) > 0 {
					fmt.Printf("%s Setting up local project\n", ui.IconTask)
					fmt.Printf("%s Detected global config for: %s\n", ui.IconSearch, strings.Join(plan.SuggestedAIs, ", "))
				} else {
					fmt.Printf("%s First time setup\n", ui.IconRocket)
				}
				fmt.Println()
				fmt.Printf("%s Which AI assistant(s) do you use?\n", ui.IconRobot)
				fmt.Println()
				selectedAIs = promptAISelection(plan.SuggestedAIs...)

			case bootstrap.ModeRepair:
				if len(plan.AIsNeedingRepair) > 0 {
					fmt.Printf("%s Restoring missing AI configurations\n", ui.IconWrench)
					fmt.Printf("   Missing: %s\n", strings.Join(plan.AIsNeedingRepair, ", "))
					fmt.Print("   Restore? [Y/n]: ")
					var input string
					_, _ = fmt.Scanln(&input)
					input = strings.TrimSpace(strings.ToLower(input))
					if input == "" || input == "y" || input == "yes" {
						selectedAIs = plan.AIsNeedingRepair
					} else {
						fmt.Println()
						fmt.Printf("%s Which AI assistant(s) do you want to set up?\n", ui.IconRobot)
						selectedAIs = promptAISelection(plan.SuggestedAIs...)
					}
				}

			case bootstrap.ModeReconfigure:
				fmt.Printf("%s No AI configurations found - let's set them up\n", ui.IconWrench)
				fmt.Println()
				fmt.Printf("%s Which AI assistant(s) do you use?\n", ui.IconRobot)
				fmt.Println()
				selectedAIs = promptAISelection()
			}
		}
		if len(selectedAIs) == 0 && !flags.Quiet {
			fmt.Println()
		ui.PrintWarning("No AI assistants selected - continuing with local project initialization only")
		}
	}

	// Store selected AIs in plan for subsequent actions
	plan.SelectedAIs = selectedAIs

	// Initialize project
	start := time.Now()
	if err := svc.InitializeProject(flags.Verbose, selectedAIs); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	if !flags.Quiet {
		ui.PrintPhaseResult("Project initialized", time.Since(start))
	} else {
		fmt.Printf("done (%s)\n", ui.FormatDuration(time.Since(start)))
	}
	return nil
}

// executeGenerateAIConfigs generates AI slash commands and hooks.
// This runs standalone when ActionInitProject isn't in the plan (e.g., ModeRepair with healthy project).
func executeGenerateAIConfigs(svc *bootstrap.Service, flags bootstrap.Flags, plan *bootstrap.Plan, bCtx *bootstrapContext) error {
	// Determine which AIs to configure
	var targetAIs []string
	if len(plan.SelectedAIs) > 0 {
		targetAIs = plan.SelectedAIs
	} else if len(plan.AIsNeedingRepair) > 0 {
		targetAIs = plan.AIsNeedingRepair
	}

	if len(targetAIs) == 0 {
		return nil
	}

	if !flags.Quiet {
		ui.PrintPhaseHeader(bCtx.nextPhase(), bCtx.totalActions, ui.IconWrench,
			"Generating AI Configurations",
			fmt.Sprintf("Setting up slash commands and hooks for %s.", strings.Join(targetAIs, ", ")))
	} else {
		step := bCtx.nextPhase()
		fmt.Printf("[%d/%d] Generating AI configs... ", step, bCtx.totalActions)
	}

	start := time.Now()
	if err := svc.RegenerateAIConfigs(flags.Verbose, targetAIs); err != nil {
		return fmt.Errorf("regenerate AI configs failed: %w", err)
	}

	if !flags.Quiet {
		ui.PrintPhaseResult("AI configurations regenerated", time.Since(start))
	} else {
		fmt.Printf("done (%s)\n", ui.FormatDuration(time.Since(start)))
	}
	return nil
}

// executeInstallMCP registers MCP servers with AI CLIs.
func executeInstallMCP(cwd string, flags bootstrap.Flags, plan *bootstrap.Plan, bCtx *bootstrapContext) error {
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

	if !flags.Quiet {
		ui.PrintPhaseHeader(bCtx.nextPhase(), bCtx.totalActions, ui.IconPlug,
			"Installing MCP Servers",
			fmt.Sprintf("Registering TaskWing with %s.", strings.Join(targetAIs, ", ")))
	} else {
		step := bCtx.nextPhase()
		fmt.Printf("[%d/%d] Installing MCP servers... ", step, bCtx.totalActions)
	}

	if len(aisNeedingRegistration) == 0 {
		if !flags.Quiet && len(existingGlobalAIs) > 0 {
			ui.PrintPhaseResult(fmt.Sprintf("MCP already configured for: %s", strings.Join(existingGlobalAIs, ", ")), 0)
		} else if flags.Quiet {
			fmt.Println("already configured")
		}
		return nil
	}

	start := time.Now()
	installMCPServers(cwd, aisNeedingRegistration)

	if !flags.Quiet {
		ui.PrintPhaseResult(fmt.Sprintf("MCP servers installed for: %s", strings.Join(aisNeedingRegistration, ", ")), time.Since(start))
	} else {
		fmt.Printf("done (%s)\n", ui.FormatDuration(time.Since(start)))
	}
	return nil
}

// executeIndexCode runs code symbol indexing.
func executeIndexCode(ctx context.Context, cwd string, flags bootstrap.Flags, bCtx *bootstrapContext) error {
	if !flags.Quiet {
		ui.PrintPhaseHeader(bCtx.nextPhase(), bCtx.totalActions, ui.IconSearch,
			"Indexing Code Symbols",
			"Scanning source files for functions, types, and call relationships.")
	} else {
		step := bCtx.nextPhase()
		fmt.Printf("[%d/%d] Indexing code symbols... ", step, bCtx.totalActions)
	}

	if err := runCodeIndexing(ctx, cwd, flags.Force, flags.Quiet, bCtx); err != nil {
		// Non-fatal: log and continue
		if !flags.Quiet {
			fmt.Fprintf(os.Stderr, "        %s  Code indexing failed: %v\n", ui.IconWarn, err)
		} else {
			fmt.Printf("failed (%v)\n", err)
		}
	}
	return nil
}

// executeExtractMetadata runs deterministic metadata extraction.
func executeExtractMetadata(ctx context.Context, svc *bootstrap.Service, flags bootstrap.Flags, bCtx *bootstrapContext) error {
	if !flags.Quiet {
		ui.PrintPhaseHeader(bCtx.nextPhase(), bCtx.totalActions, ui.IconStats,
			"Extracting Project Metadata",
			"Collecting git history and documentation files.")
	} else {
		step := bCtx.nextPhase()
		fmt.Printf("[%d/%d] Extracting metadata... ", step, bCtx.totalActions)
	}

	start := time.Now()
	result, err := svc.RunDeterministicBootstrap(ctx, flags.Quiet)
	if err != nil {
		if !flags.Quiet {
			fmt.Fprintf(os.Stderr, "        %s  Metadata extraction failed: %v\n", ui.IconWarn, err)
		} else {
			fmt.Printf("failed (%v)\n", err)
		}
	} else {
		if result != nil {
			bCtx.metadataItems = result.FindingsCount
			bCtx.metadataDuration = time.Since(start)
		}
		if result != nil && len(result.Warnings) > 0 && flags.Verbose {
			for _, w := range result.Warnings {
				fmt.Fprintf(os.Stderr, "        [warn] %s\n", w)
			}
		}
		if flags.Quiet {
			fmt.Printf("done (%s)\n", ui.FormatDuration(time.Since(start)))
		}
	}
	return nil
}

// executeLLMAnalyze runs architecture analysis via AI CLI runners or LLM agents.
// When AI CLIs (Claude Code, Gemini CLI, Codex CLI) are detected, uses them as
// headless subprocesses (no API keys needed). Falls back to internal LLM agents
// when --prefer-cli is not set and no CLIs are found.
func executeLLMAnalyze(ctx context.Context, svc *bootstrap.Service, cwd string, flags bootstrap.Flags, llmCfg llm.Config, plan *bootstrap.Plan, bCtx *bootstrapContext) error {
	// Detect workspace type
	ws, err := project.DetectWorkspace(cwd)
	if err != nil {
		return fmt.Errorf("detect workspace: %w", err)
	}

	// Handle multi-repo workspaces
	if ws.IsMultiRepo() {
		if len(plan.SelectedRepos) > 0 {
			ws.Services = plan.SelectedRepos
		}
		return runMultiRepoBootstrap(ctx, svc, ws, flags.Preview)
	}

	// Try runner-based analysis (spawns a single AI CLI subprocess, no API keys needed)
	preferCLI := runner.CLIType(flags.PreferCLI)
	selectedRunner, runnerErr := runner.PreferredRunner(preferCLI)

	if runnerErr == nil {
		return runRunnerAnalysis(ctx, svc, cwd, flags, selectedRunner, bCtx)
	}

	// Fallback: use internal LLM agents (requires API key)
	if llmCfg.APIKey != "" || llmCfg.Provider == "ollama" {
		return runAgentTUI(ctx, svc, cwd, llmCfg, flags)
	}

	return fmt.Errorf("no AI CLI detected and no LLM API key configured.\nInstall Claude Code, Gemini CLI, or Codex CLI, or configure an API key via 'taskwing config set'")
}

// runRunnerAnalysis executes bootstrap analysis by spawning parallel AI CLI instances.
func runRunnerAnalysis(ctx context.Context, svc *bootstrap.Service, cwd string, flags bootstrap.Flags, r runner.Runner, bCtx *bootstrapContext) error {
	model := flags.Model

	// Print phase header
	if !flags.Quiet {
		desc := fmt.Sprintf("Using %s", r.Type().String())
		if model != "" {
			desc += fmt.Sprintf(" (%s)", model)
		}
		desc += fmt.Sprintf(" — %d focus areas in parallel.", len(runner.FocusAreas))
		ui.PrintPhaseHeader(bCtx.nextPhase(), bCtx.totalActions, ui.IconRobot,
			"Architecture Analysis", desc)
	} else {
		step := bCtx.nextPhase()
		fmt.Printf("[%d/%d] Analyzing architecture... ", step, bCtx.totalActions)
	}

	analysisStart := time.Now()

	// Build existing context summary to avoid duplicate findings
	existingContext := buildExistingContext()

	// Run all focus areas in parallel through separate AI CLI instances
	type jobResult struct {
		id       string
		result   *runner.InvokeResult
		err      error
		duration time.Duration
	}
	results := make([]jobResult, len(runner.FocusAreas))

	// Build job IDs for TUI
	focusIDs := make([]string, len(runner.FocusAreas))
	for i, focus := range runner.FocusAreas {
		focusIDs[i] = focusAreaID(focus)
	}

	if !flags.Quiet {
		// Use Bubble Tea spinner TUI for non-quiet mode
		progressModel := ui.NewRunnerProgressModel(focusIDs)
		p := tea.NewProgram(progressModel, tea.WithOutput(os.Stderr))

		for i, focus := range runner.FocusAreas {
			go func(idx int, focusArea, jobID string) {
				jobStart := time.Now()

				// Build progress callback — suppress heartbeats, show thinking/tool in verbose only
				var progressCb runner.ProgressCallback
				if flags.Verbose || flags.Debug {
					progressCb = func(ev runner.ProgressEvent) {
						switch ev.Type {
						case runner.ProgressThinking:
							fmt.Fprintf(os.Stderr, "        [%s] thinking: %s\n", jobID, ev.Summary)
						case runner.ProgressToolUse:
							fmt.Fprintf(os.Stderr, "        [%s] using: %s\n", jobID, ev.Summary)
						case runner.ProgressHeartbeat:
							// Suppressed
						}
					}
				}

				res, err := r.Invoke(ctx, runner.InvokeRequest{
					Prompt:     runner.BootstrapAnalysisPrompt(cwd, existingContext, focusArea),
					WorkDir:    cwd,
					Timeout:    flags.Timeout,
					OnProgress: progressCb,
					Model:      model,
				})

				// Log runner stderr in debug mode (even on success)
				if flags.Debug && res != nil && res.Stderr != "" {
					fmt.Fprintf(os.Stderr, "[debug] [%s] stderr:\n%s\n", jobID, res.Stderr)
				}

				dur := time.Since(jobStart)
				results[idx] = jobResult{id: jobID, result: res, err: err, duration: dur}

				// Signal TUI
				doneMsg := ui.RunnerJobDoneMsg{ID: jobID, Duration: dur}
				if err != nil {
					doneMsg.ErrMsg = err.Error()
				} else if res != nil {
					var analysis runner.BootstrapAnalysis
					if decErr := res.Decode(&analysis); decErr != nil {
						doneMsg.ErrMsg = fmt.Sprintf("parse error: %v", decErr)
					} else {
						doneMsg.Findings = len(analysis.Findings)
						doneMsg.Rels = len(analysis.Relationships)
					}
				}
				p.Send(doneMsg)
			}(i, focus, focusIDs[i])
		}

		if _, err := p.Run(); err != nil {
			// TUI error is non-fatal, results are still in the slice
			fmt.Fprintf(os.Stderr, "        %s TUI error: %v\n", ui.IconWarn, err)
		}
	} else {
		// Quiet mode: use simple WaitGroup
		var wg sync.WaitGroup
		for i, focus := range runner.FocusAreas {
			wg.Add(1)
			go func(idx int, focusArea, jobID string) {
				defer wg.Done()
				jobStart := time.Now()

				res, err := r.Invoke(ctx, runner.InvokeRequest{
					Prompt:     runner.BootstrapAnalysisPrompt(cwd, existingContext, focusArea),
					WorkDir:    cwd,
					Timeout:    flags.Timeout,
					Model:      model,
				})

				results[idx] = jobResult{id: jobID, result: res, err: err, duration: time.Since(jobStart)}
			}(i, focus, focusIDs[i])
		}
		wg.Wait()
	}

	// Collect findings and relationships from all results
	var allFindings []core.Finding
	var allRelationships []core.Relationship
	var errors []string
	sourceAgent := string(r.Type())

	for _, jr := range results {
		if jr.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", jr.id, jr.err))
			continue
		}

		var analysis runner.BootstrapAnalysis
		if err := jr.result.Decode(&analysis); err != nil {
			errors = append(errors, fmt.Sprintf("%s: parse error: %v", jr.id, err))
			continue
		}

		findings := convertRunnerFindings(analysis.Findings, sourceAgent)
		allFindings = append(allFindings, findings...)

		rels := convertRunnerRelationships(analysis.Relationships)
		allRelationships = append(allRelationships, rels...)
	}

	// Update bCtx stats
	bCtx.analysisFindings = len(allFindings)
	bCtx.analysisRelations = len(allRelationships)
	bCtx.analysisDuration = time.Since(analysisStart)

	if len(allFindings) == 0 && len(errors) > 0 {
		return fmt.Errorf("all analysis jobs failed:\n  %s", strings.Join(errors, "\n  "))
	}

	// Report partial failures when some succeeded but others didn't
	if len(errors) > 0 && len(allFindings) > 0 && !flags.Quiet {
		fmt.Fprintf(os.Stderr, "\n        %s %d of %d analysis jobs had errors (partial results saved)\n",
			ui.IconWarn, len(errors), len(runner.FocusAreas))
	}

	if flags.Preview {
		fmt.Println()
		ui.PrintHint(fmt.Sprintf("Preview: %d findings, %d relationships from %d focus areas. Run without --preview to save.",
			len(allFindings), len(allRelationships), len(runner.FocusAreas)))
		return nil
	}

	if flags.Quiet {
		fmt.Printf("done (%s)\n", ui.FormatDuration(bCtx.analysisDuration))
	}

	// Ingest findings and relationships into knowledge system
	return svc.IngestDirectly(ctx, allFindings, allRelationships, flags.Quiet)
}

// convertRunnerFindings converts runner findings to core.Finding format,
// mapping all fields including metadata, debt classification, and evidence details.
func convertRunnerFindings(findings []runner.BootstrapFinding, sourceAgent string) []core.Finding {
	result := make([]core.Finding, 0, len(findings))
	for _, f := range findings {
		cf := core.Finding{
			Type:            core.FindingType(f.Type),
			Title:           f.Title,
			Description:     f.Description,
			Why:             f.Why,
			Tradeoffs:       f.Tradeoffs,
			ConfidenceScore: f.ConfidenceScore,
			SourceAgent:     sourceAgent,
			Metadata:        f.Metadata,
			DebtScore:       f.DebtScore,
			DebtReason:      f.DebtReason,
			RefactorHint:    f.RefactorHint,
		}
		for _, ev := range f.Evidence {
			cf.Evidence = append(cf.Evidence, core.Evidence{
				FilePath:     ev.FilePath,
				StartLine:    ev.StartLine,
				EndLine:      ev.EndLine,
				Snippet:      ev.Snippet,
				GrepPattern:  ev.GrepPattern,
				EvidenceType: ev.EvidenceType,
			})
		}
		result = append(result, cf)
	}
	return result
}

// convertRunnerRelationships converts runner relationships to core.Relationship format.
func convertRunnerRelationships(rels []runner.BootstrapRelationship) []core.Relationship {
	result := make([]core.Relationship, 0, len(rels))
	for _, r := range rels {
		result = append(result, core.Relationship{
			From:     r.From,
			To:       r.To,
			Relation: r.Relation,
			Reason:   r.Reason,
		})
	}
	return result
}

// buildExistingContext reads existing knowledge nodes to provide as context.
func buildExistingContext() string {
	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return ""
	}
	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return ""
	}
	defer func() { _ = repo.Close() }()

	nodes, err := repo.ListNodes("")
	if err != nil || len(nodes) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, n := range nodes {
		if i >= 20 { // Limit context size
			break
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", n.Type, n.Summary))
	}
	return sb.String()
}

// focusAreaID returns a short identifier for a focus area description.
func focusAreaID(focus string) string {
	if strings.Contains(focus, "decisions") {
		return "decisions"
	}
	if strings.Contains(focus, "patterns") {
		return "patterns"
	}
	if strings.Contains(focus, "constraints") {
		return "constraints"
	}
	if strings.Contains(focus, "git history") {
		return "git-history"
	}
	return "analysis"
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
	bootstrapCmd.Flags().Bool("no-analyze", false, "Skip LLM analysis even when an AI CLI is detected")
	bootstrapCmd.Flags().Bool("analyze", false, "Enable LLM-powered architecture analysis (kept for backward compat)")
	bootstrapCmd.Flags().Bool("skip-analyze", false, "Legacy alias for --no-analyze")
	bootstrapCmd.Flags().Bool("resume", false, "Resume from last checkpoint (skip completed agents)")
	bootstrapCmd.Flags().StringSlice("only-agents", nil, "Run only specified agents (e.g., --only-agents=code,doc)")
	bootstrapCmd.Flags().Bool("trace", false, "Emit JSON event stream to stderr")
	bootstrapCmd.Flags().String("trace-file", "", "Write JSON event stream to file (default: .taskwing/logs/bootstrap.trace.jsonl)")
	bootstrapCmd.Flags().Bool("trace-stdout", false, "Emit JSON event stream to stderr (overrides trace file)")
	bootstrapCmd.Flags().Bool("debug", false, "Enable debug logging (dumps project context, git paths, agent inputs)")
	bootstrapCmd.Flags().Duration("timeout", 0, "LLM request timeout (e.g., 5m, 10m). Overrides TASKWING_LLM_TIMEOUT env var. Default: 10m")
	bootstrapCmd.Flags().String("prefer-cli", "", "Preferred AI CLI for analysis (claude, gemini, codex)")
	bootstrapCmd.Flags().String("model", "haiku", "AI model to use for analysis (e.g., haiku, sonnet, opus)")

	// Hide legacy flags from main help
	_ = bootstrapCmd.Flags().MarkHidden("skip-analyze")
	_ = bootstrapCmd.Flags().MarkHidden("analyze")
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
		fmt.Fprintf(os.Stderr, "%s  Cannot resume: %v\n", ui.IconWarn, repoErr)
		flags.Resume = false
	}
	if repo != nil {
		defer func() { _ = repo.Close() }()
	}

	// Filter agents based on flags
	agentsList, skippedAgents := filterAgents(allAgents, flags, repo)

	// Show skipped agents
	if len(skippedAgents) > 0 && !flags.Quiet {
		fmt.Printf("%s  Skipping completed agents: %s\n", ui.IconSkip, strings.Join(skippedAgents, ", "))
	}

	// If all agents were skipped, nothing to do
	if len(agentsList) == 0 {
		if !flags.Quiet {
			ui.PrintSuccess("All agents already completed. Use 'bootstrap' without --resume to re-run.")
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
		fmt.Printf("\n%s  Bootstrap cancelled.\n", ui.IconWarn)
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

	fmt.Printf("%s Analyzing %d services. Running parallel analysis...\n", ui.IconPackage, ws.ServiceCount())

	findings, relationships, errs, err := svc.RunMultiRepoAnalysis(ctx, ws)
	if err != nil {
		return err
	}

	if len(errs) > 0 {
		fmt.Println()
		ui.PrintWarning("Some services had errors:")
		for _, e := range errs {
			fmt.Printf("   - %s\n", e)
		}
	}

	fmt.Printf("%s Aggregated: %d findings from %d services\n", ui.IconStats, len(findings), ws.ServiceCount()-len(errs))

	if preview {
		fmt.Println()
		ui.PrintHint("This was a preview. Run 'taskwing bootstrap' to save to memory.")
		return nil
	}

	return svc.IngestDirectly(ctx, findings, relationships, viper.GetBool("quiet"))
}

// promptRepoSelection prompts the user to select which repositories to bootstrap.
// Returns all repos on error or cancel to avoid silent no-op.
func promptRepoSelection(repos []string) []string {
	selected, err := ui.PromptRepoSelection(repos)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s  Repo selection failed: %v — analyzing all repositories\n", ui.IconWarn, err)
		return repos
	}
	if selected == nil {
		fmt.Printf("%s  Selection cancelled — analyzing all repositories\n", ui.IconWarn)
		return repos
	}
	return selected
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
				fmt.Fprintf(os.Stderr, "%s  OpenCode MCP installation failed: %v\n", ui.IconWarn, err)
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
	// Enable full payload capture so trace includes LLM messages and responses
	stream.SetIncludePayloads(true)
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
			fmt.Fprintf(os.Stderr, "%s Trace: %s\n", ui.IconInfo, traceFile)
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
		fmt.Fprintf(os.Stderr, "\n%s Bootstrap failed. Some agents errored:\n", ui.IconFail)
		for _, line := range failedAgents {
			fmt.Fprintf(os.Stderr, "  - %s\n", line)
		}
		return fmt.Errorf("bootstrap failed: %d agent(s) errored", len(failedAgents))
	}
	return nil
}

// runCodeIndexing runs the code intelligence indexer on the codebase.
// This extracts symbols (functions, types, etc.) for enhanced search and MCP ask.
func runCodeIndexing(ctx context.Context, basePath string, forceIndex, isQuiet bool, bCtx *bootstrapContext) error {
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
			fmt.Fprintf(os.Stderr, "        %s  Could not count files for indexing: %v\n", ui.IconWarn, err)
		}
		return nil // Non-fatal - skip indexing if we can't count
	}

	// Large codebase safety check
	const maxFilesWithoutForce = 5000
	if fileCount > maxFilesWithoutForce && !forceIndex {
		if !isQuiet {
			ui.PrintPhaseDetail(fmt.Sprintf("%s Large codebase detected: %d files to index", ui.IconWarn, fileCount))
			ui.PrintPhaseDetail("Run with --force to proceed, or use --skip-index to bypass.")
		}
		return nil // Not an error, just skip
	}

	bCtx.filesScanned = fileCount

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
			fmt.Fprintf(os.Stderr, "\r        %s Progress: %d%% (%d files, %d symbols)    ", ui.IconBolt, pct, stats.FilesIndexed, stats.SymbolsFound)
		}
	}

	// Re-create indexer with updated config (for progress callback)
	indexer = codeintel.NewIndexer(codeRepo, config)

	// Run indexing
	start := time.Now()

	// Prune stale files first
	prunedCount, err := indexer.PruneStaleFiles(ctx)
	if err != nil && !isQuiet {
		fmt.Fprintf(os.Stderr, "        %s Prune failed: %v\n", ui.IconWarn, err)
	}

	// Run incremental indexing
	stats, err := indexer.IncrementalIndex(ctx, basePath)
	if err != nil {
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "\r                                                              \n")
			fmt.Fprintf(os.Stderr, "        %s Indexing failed: %v\n", ui.IconWarn, err)
		}
		return nil // Non-fatal - bootstrap succeeded even if indexing fails
	}

	// Clear progress line and print summary
	if !isQuiet {
		fmt.Fprintf(os.Stderr, "\r                                                              \n")
		duration := time.Since(start)
		bCtx.symbolsFound = stats.SymbolsFound
		bCtx.callRelationships = stats.RelationsFound
		bCtx.indexDuration = duration

		ui.PrintPhaseResult(fmt.Sprintf("%d updates, pruned %d stale files", stats.FilesIndexed, prunedCount), duration)
		if stats.RelationsFound > 0 {
			ui.PrintPhaseResult(fmt.Sprintf("Discovered %d call relationships", stats.RelationsFound), 0)
		}
		if len(stats.Errors) > 0 {
			ui.PrintPhaseDetail(fmt.Sprintf("%s %d files skipped (parse errors)", ui.IconWarn, len(stats.Errors)))
		}
	} else {
		bCtx.symbolsFound = stats.SymbolsFound
		bCtx.callRelationships = stats.RelationsFound
		bCtx.indexDuration = time.Since(start)
		fmt.Printf("done (%s)\n", ui.FormatDuration(bCtx.indexDuration))
	}

	return nil
}
