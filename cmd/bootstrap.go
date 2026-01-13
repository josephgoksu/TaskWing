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
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/josephgoksu/TaskWing/internal/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// bootstrapCmd represents the bootstrap command
var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Initialize project memory (fast, deterministic)",
	Long: `Initialize TaskWing for your repository.

By default, bootstrap runs in FAST MODE (no LLM required):
  • Creates .taskwing/ directory structure
  • Sets up AI assistant integration (Claude, Cursor, etc.)
  • Indexes code symbols (functions, types, etc.)
  • Extracts git statistics and documentation
  • Completes in ~5 seconds, always succeeds

Use --analyze for deep LLM-powered analysis (slower, requires API key):
  • Analyzes code patterns and architecture
  • Extracts decisions from git history
  • Understands WHY decisions were made`,
	RunE: func(cmd *cobra.Command, args []string) error {
		preview, _ := cmd.Flags().GetBool("preview")
		skipInit, _ := cmd.Flags().GetBool("skip-init")
		skipIndex, _ := cmd.Flags().GetBool("skip-index")
		forceIndex, _ := cmd.Flags().GetBool("force")
		analyze, _ := cmd.Flags().GetBool("analyze")
		trace, _ := cmd.Flags().GetBool("trace")
		traceFile, _ := cmd.Flags().GetString("trace-file")
		traceStdout, _ := cmd.Flags().GetBool("trace-stdout")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}

		// Load LLM config - only required for --analyze mode
		var llmCfg llm.Config
		if analyze {
			llmCfg, err = getLLMConfigForRole(cmd, llm.RoleBootstrap)
			if err != nil {
				return err
			}
		}

		// Initialize Service
		svc := bootstrap.NewService(cwd, llmCfg)

		// Determine project state - check three independent concerns:
		// 1. Project memory (.taskwing/) - stores knowledge base
		// 2. AI configs (.claude/, .codex/, .gemini/) - slash commands, hooks
		// 3. Global MCP registration (via AI CLIs) - tool availability
		taskwingDir := filepath.Join(cwd, ".taskwing")
		taskwingExists := true
		if _, err := os.Stat(taskwingDir); os.IsNotExist(err) {
			taskwingExists = false
		}

		// Check for existing global MCP configurations
		existingGlobalAIs := detectExistingMCPConfigs()
		hasGlobalMCP := len(existingGlobalAIs) > 0

		// Determine which AI configs exist locally (check against global AIs if any, else all known AIs)
		var checkAIs []string
		if hasGlobalMCP {
			checkAIs = existingGlobalAIs
		} else {
			checkAIs = []string{"claude", "codex", "gemini", "cursor", "copilot"}
		}
		missingLocalAIs := findMissingAIConfigs(cwd, checkAIs)
		existingLocalAIs := findExistingAIConfigs(cwd)
		hasAnyLocalAI := len(existingLocalAIs) > 0
		needsAISetup := len(missingLocalAIs) > 0 && hasGlobalMCP // Only auto-detect if global MCP exists

		// Determine if we need to run initialization
		needsInit := !taskwingExists || needsAISetup || (taskwingExists && !hasAnyLocalAI && !hasGlobalMCP)

		if !skipInit && needsInit {
			var selectedAIs []string
			skipMCPRegistration := false

			// Scenario 1: Nothing exists - true first time setup
			if !taskwingExists && !hasGlobalMCP {
				fmt.Println("🚀 First time setup - no existing configuration found")
				fmt.Println()
				fmt.Println("🤖 Which AI assistant(s) do you use?")
				fmt.Println()
				selectedAIs = promptAISelection()

				// Scenario 2: Global MCP exists but no local project
			} else if !taskwingExists && hasGlobalMCP {
				fmt.Println("📋 Setting up local project (global MCP config found)")
				fmt.Println()
				fmt.Printf("🔍 Found TaskWing registered globally for: %s\n", strings.Join(existingGlobalAIs, ", "))
				fmt.Println()
				fmt.Println("🤖 Which AI assistant(s) do you want to use?")
				fmt.Printf("   (Detected %s pre-selected)\n", strings.Join(existingGlobalAIs, ", "))
				fmt.Println()
				selectedAIs = promptAISelection(existingGlobalAIs...)
				// Note: MCP registration will be handled below - only for AIs not already registered

				// Scenario 3: .taskwing exists but some AI configs missing (recovery with global MCP)
			} else if taskwingExists && needsAISetup {
				fmt.Println("🔧 Restoring missing AI configurations")
				fmt.Println()
				fmt.Printf("   Missing local configs for: %s\n", strings.Join(missingLocalAIs, ", "))
				fmt.Printf("   Global MCP registered for: %s\n", strings.Join(existingGlobalAIs, ", "))
				fmt.Print("   Restore missing configs? [Y/n]: ")
				var input string
				fmt.Scanln(&input)
				input = strings.TrimSpace(strings.ToLower(input))
				if input == "" || input == "y" || input == "yes" {
					selectedAIs = missingLocalAIs // Only restore MISSING, not all
					skipMCPRegistration = true    // Already registered globally
					fmt.Println("   ✓ Will restore missing configs only")
					fmt.Println()
				} else {
					fmt.Println()
					fmt.Println("🤖 Which AI assistant(s) do you want to set up?")
					fmt.Println()
					selectedAIs = promptAISelection(existingGlobalAIs...)
				}

				// Scenario 4: .taskwing exists but NO AI configs and NO global MCP (reconfigure)
			} else if taskwingExists && !hasAnyLocalAI && !hasGlobalMCP {
				fmt.Println("🔧 No AI configurations found - let's set them up")
				fmt.Println()
				fmt.Println("🤖 Which AI assistant(s) do you use?")
				fmt.Println()
				selectedAIs = promptAISelection()
			}

			// Only proceed if user selected something
			if len(selectedAIs) > 0 {
				// Initialize/update project
				if err := svc.InitializeProject(viper.GetBool("verbose"), selectedAIs); err != nil {
					return fmt.Errorf("initialization failed: %w", err)
				}

				// Only run CLI registration if needed (and only for AIs not already registered)
				if !skipMCPRegistration {
					// Filter out AIs already registered globally
					var aisNeedingRegistration []string
					globalSet := make(map[string]bool)
					for _, ai := range existingGlobalAIs {
						globalSet[ai] = true
					}
					for _, ai := range selectedAIs {
						if !globalSet[ai] {
							aisNeedingRegistration = append(aisNeedingRegistration, ai)
						}
					}
					if len(aisNeedingRegistration) > 0 {
						installMCPServers(cwd, aisNeedingRegistration)
					}
				}

				fmt.Println("\n✓ TaskWing initialized!")
				fmt.Println()

				// Prompt for additional model configuration
				if err := promptAdditionalModels(); err != nil {
					fmt.Printf("⚠️  Additional model config skipped: %v\n", err)
				}
			} else {
				fmt.Println("\n⚠️  No AI assistants selected - skipping initialization")
				fmt.Println()
			}
		}

		// Detect workspace type
		ws, err := workspace.Detect(cwd)
		if err != nil {
			return fmt.Errorf("detect workspace: %w", err)
		}

		// Run code indexing FIRST (enables symbol-based context)
		// This is fast (~2-5s) and doesn't require LLM calls
		if !skipIndex && !preview {
			if err := runCodeIndexing(cmd.Context(), cwd, forceIndex, viper.GetBool("quiet")); err != nil {
				// Non-fatal: continue with fallback context gathering
				if !viper.GetBool("quiet") {
					fmt.Fprintf(os.Stderr, "⚠️  Code indexing failed: %v\n", err)
				}
			}
		}

		// Run deterministic metadata extraction (git stats, docs)
		// This is fast and always succeeds
		if !preview {
			if err := svc.RunDeterministicBootstrap(cmd.Context(), viper.GetBool("quiet")); err != nil {
				if !viper.GetBool("quiet") {
					fmt.Fprintf(os.Stderr, "⚠️  Metadata extraction failed: %v\n", err)
				}
			}
		}

		// Only run LLM analysis if --analyze flag is set
		if analyze {
			// Handle multi-repo workspaces
			if ws.IsMultiRepo() {
				if err := runMultiRepoBootstrap(cmd.Context(), svc, ws, preview); err != nil {
					return err
				}
			} else {
				// Run agent TUI flow with LLM analysis
				if err := runAgentTUI(cmd.Context(), svc, cwd, llmCfg, trace, traceFile, traceStdout, preview); err != nil {
					return err
				}
			}
		} else if !viper.GetBool("quiet") {
			fmt.Println()
			fmt.Println("✅ Bootstrap complete!")
			fmt.Println()
			fmt.Println("💡 Tip: Use 'tw bootstrap --analyze' for deep LLM-powered analysis (slower, requires API key)")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().Bool("skip-init", false, "Skip initialization prompt")
	bootstrapCmd.Flags().Bool("skip-index", false, "Skip code indexing (symbol extraction)")
	bootstrapCmd.Flags().Bool("force", false, "Force indexing even for large codebases (>5000 files)")
	bootstrapCmd.Flags().Bool("analyze", false, "Run LLM-powered deep analysis (slower, requires API key)")
	bootstrapCmd.Flags().Bool("trace", false, "Emit JSON event stream to stderr")
	bootstrapCmd.Flags().String("trace-file", "", "Write JSON event stream to file (default: .taskwing/logs/bootstrap.trace.jsonl)")
	bootstrapCmd.Flags().Bool("trace-stdout", false, "Emit JSON event stream to stderr (overrides trace file)")
}

// runAgentTUI handles the interactive UI part, delegating work to the service
func runAgentTUI(ctx context.Context, svc *bootstrap.Service, cwd string, llmCfg llm.Config, trace bool, traceFile string, traceStdout bool, preview bool) error {
	fmt.Println("")
	ui.RenderPageHeader("TaskWing Bootstrap", fmt.Sprintf("Using: %s (%s)", llmCfg.Model, llmCfg.Provider))

	projectName := filepath.Base(cwd)
	agentsList := bootstrap.NewDefaultAgents(llmCfg, cwd)
	defer core.CloseAgents(agentsList)

	input := core.Input{
		BasePath:    cwd,
		ProjectName: projectName,
		Mode:        core.ModeBootstrap,
		Verbose:     true,
	}

	stream := core.NewStreamingOutput(100)
	defer stream.Close()

	setupTrace(stream, trace, traceFile, traceStdout, cwd)

	// Run TUI
	tuiModel := ui.NewBootstrapModel(ctx, input, agentsList, stream)
	p := tea.NewProgram(tuiModel)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	bootstrapModel, ok := finalModel.(ui.BootstrapModel)
	if !ok || (bootstrapModel.Quitting && len(bootstrapModel.Results) < len(agentsList)) {
		fmt.Println("\n⚠️  Bootstrap cancelled.")
		return nil
	}

	// Check failures
	if err := checkAgentFailures(bootstrapModel.Agents); err != nil {
		return err
	}

	// Delegate processing/saving to service
	allFindings := core.AggregateFindings(bootstrapModel.Results)
	allRelationships := core.AggregateRelationships(bootstrapModel.Results)

	return svc.ProcessAndSaveResults(ctx, bootstrapModel.Results, allFindings, allRelationships, preview, viper.GetBool("quiet"))
}

// runMultiRepoBootstrap uses the service to analyze multiple repos
func runMultiRepoBootstrap(ctx context.Context, svc *bootstrap.Service, ws *workspace.Info, preview bool) error {
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
		}
	}
}

// setupTrace configures trace logging
func setupTrace(stream *core.StreamingOutput, trace bool, traceFile string, traceStdout bool, cwd string) {
	if !trace {
		return
	}
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
			fmt.Fprintf(os.Stderr, "failed to open trace file: %v\n", err)
			return
		}
		out = f
		// Note: file closing is loose here, relying on process exit
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

// promptAdditionalModels shows existing config or offers to configure query model, embedding, and reranking
func promptAdditionalModels() error {
	// Check what's already configured
	queryModel := viper.GetString("llm.models.query")
	embeddingProvider := viper.GetString("llm.embedding_provider")
	embeddingModel := viper.GetString("llm.embedding_model")
	rerankingEnabled := viper.GetBool("retrieval.reranking.enabled")
	rerankingURL := viper.GetString("retrieval.reranking.base_url")

	// Count configured items
	configured := 0
	if queryModel != "" {
		configured++
	}
	if embeddingModel != "" {
		configured++
	}
	if rerankingEnabled {
		configured++
	}

	// If all configured, just show status
	if configured == 3 {
		fmt.Println("📋 Model Configuration (from config file)")
		fmt.Printf("   ✓ Query model: %s\n", queryModel)
		fmt.Printf("   ✓ Embedding: %s (%s)\n", embeddingModel, embeddingProvider)
		fmt.Printf("   ✓ Reranking: enabled (%s)\n", rerankingURL)
		fmt.Println()
		return nil
	}

	// If some configured, show status and offer to configure missing
	if configured > 0 {
		fmt.Println("📋 Model Configuration")
		if queryModel != "" {
			fmt.Printf("   ✓ Query model: %s\n", queryModel)
		}
		if embeddingModel != "" {
			fmt.Printf("   ✓ Embedding: %s (%s)\n", embeddingModel, embeddingProvider)
		}
		if rerankingEnabled {
			fmt.Printf("   ✓ Reranking: enabled (%s)\n", rerankingURL)
		}
		fmt.Println()
	} else {
		fmt.Println("📋 Additional Model Configuration")
		fmt.Println("   You can configure these now or later via 'tw config'")
		fmt.Println()
	}

	var input string

	// Query model - only prompt if not configured
	if queryModel == "" {
		fmt.Print("   Configure fast query model? (for cheap/fast lookups) [y/N]: ")
		fmt.Scanln(&input)
		if input == "y" || input == "Y" || input == "yes" {
			if err := configureQueryModel(); err != nil {
				fmt.Printf("   ⚠️  Skipped: %v\n", err)
			}
			fmt.Println()
		}
	}

	// Embedding model - only prompt if not configured
	if embeddingModel == "" {
		fmt.Print("   Configure embedding model? (for semantic search) [y/N]: ")
		fmt.Scanln(&input)
		if input == "y" || input == "Y" || input == "yes" {
			if err := configureEmbedding(); err != nil {
				fmt.Printf("   ⚠️  Skipped: %v\n", err)
			}
			fmt.Println()
		}
	}

	// Reranking - only prompt if not configured
	if !rerankingEnabled {
		fmt.Print("   Configure reranking? (optional, improves search quality) [y/N]: ")
		fmt.Scanln(&input)
		if input == "y" || input == "Y" || input == "yes" {
			if err := configureReranking(); err != nil {
				fmt.Printf("   ⚠️  Skipped: %v\n", err)
			}
			fmt.Println()
		}
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
	stats, err := indexer.IndexDirectory(ctx, basePath)
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
		fmt.Printf("   ✅ Indexed %d files → %d symbols in %v\n",
			stats.FilesIndexed, stats.SymbolsFound, duration.Round(time.Millisecond))
		if stats.RelationsFound > 0 {
			fmt.Printf("   🔗 Discovered %d call relationships\n", stats.RelationsFound)
		}
		if len(stats.Errors) > 0 {
			fmt.Printf("   ⚠️  %d files skipped (parse errors)\n", len(stats.Errors))
		}
	}

	return nil
}
