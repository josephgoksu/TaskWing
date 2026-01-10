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
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/josephgoksu/TaskWing/internal/workspace"
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
  • LLM inference → Understands WHY decisions were made`,
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

		// Load LLM config
		llmCfg, err := getLLMConfigForRole(cmd, llm.RoleBootstrap)
		if err != nil {
			return err
		}

		// Initialize Service
		svc := bootstrap.NewService(cwd, llmCfg)

		// Check if .taskwing exists - if not, initialize first
		taskwingDir := filepath.Join(cwd, ".taskwing")
		if _, err := os.Stat(taskwingDir); os.IsNotExist(err) && !skipInit {
			fmt.Println("🚀 First time setup detected!")
			fmt.Println()

			var selectedAIs []string
			usingExisting := false

			// Check for existing global MCP configurations
			existingAIs := detectExistingMCPConfigs()
			if len(existingAIs) > 0 {
				fmt.Printf("🔍 Found TaskWing MCP in global CLI config for: %s\n", strings.Join(existingAIs, ", "))
				fmt.Println("   (Registered via CLI, e.g., 'codex mcp add' - not project-local)")
				fmt.Print("   Reuse this configuration? [Y/n]: ")
				var input string
				fmt.Scanln(&input)
				input = strings.TrimSpace(strings.ToLower(input))
				if input == "" || input == "y" || input == "yes" {
					selectedAIs = existingAIs
					usingExisting = true
					fmt.Println("   ✓ Reusing global MCP config, will create local project files")
					fmt.Println()
				} else {
					// User wants to reconfigure - pre-select existing ones
					fmt.Println()
					fmt.Println("🤖 Which AI assistant(s) do you use?")
					fmt.Printf("   (Detected %s pre-selected, modify as needed)\n", strings.Join(existingAIs, ", "))
					fmt.Println()
					selectedAIs = promptAISelection(existingAIs...)
				}
			} else {
				// No existing config, prompt normally
				fmt.Println("🤖 Which AI assistant(s) do you use?")
				fmt.Println()
				selectedAIs = promptAISelection()
			}

			if err := svc.InitializeProject(viper.GetBool("verbose"), selectedAIs); err != nil {
				return fmt.Errorf("initialization failed: %w", err)
			}

			// Only run CLI registration if not using existing global config
			// (installMCPServers runs 'claude mcp add', 'codex mcp add', etc.)
			// Note: Local project files (.claude/settings.json, slash commands) are
			// created above by InitializeProject regardless of usingExisting
			if !usingExisting {
				installMCPServers(cwd, selectedAIs)
			}

			fmt.Println("\n✓ TaskWing initialized!")
			fmt.Println()

			// Step 2: Prompt for additional model configuration
			if err := promptAdditionalModels(); err != nil {
				fmt.Printf("⚠️  Additional model config skipped: %v\n", err)
			}
		}

		// Detect workspace type
		ws, err := workspace.Detect(cwd)
		if err != nil {
			return fmt.Errorf("detect workspace: %w", err)
		}

		// Handle multi-repo workspaces
		if ws.IsMultiRepo() {
			return runMultiRepoBootstrap(cmd.Context(), svc, ws, preview)
		}

		// Default: run agent TUI flow
		return runAgentTUI(cmd.Context(), svc, cwd, llmCfg, trace, traceFile, traceStdout, preview)
	},
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().Bool("skip-init", false, "Skip initialization prompt")
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
