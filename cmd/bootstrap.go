/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/josephgoksu/TaskWing/internal/agents"
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

The bootstrap command analyzes:
  • Directory structure → Detects features
  • Git history → Extracts decisions from conventional commits
  • LLM inference → Understands WHY decisions were made

Examples:
  taskwing bootstrap --preview              # Preview with LLM analysis
  taskwing bootstrap                        # Generate with parallel agent analysis`,
	RunE: func(cmd *cobra.Command, args []string) error {
		preview, _ := cmd.Flags().GetBool("preview")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}

		// Use centralized config loader
		llmCfg, err := getLLMConfig(cmd)
		if err != nil {
			return err
		}

		// Default: use parallel agent architecture
		return runAgentBootstrap(cmd.Context(), cwd, preview, llmCfg)
	},
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
}

// runAgentBootstrap uses the parallel agent architecture for analysis
func runAgentBootstrap(ctx context.Context, cwd string, preview bool, llmCfg llm.Config) error {
	fmt.Println("")
	fmt.Println("┌──────────────────────────────────────────────────────────────┐")
	fmt.Println("│  🤖 TaskWing Agent Bootstrap                        	        │")
	fmt.Println("└──────────────────────────────────────────────────────────────┘")
	fmt.Println("")
	fmt.Printf("  ⚡ Using: %s (%s) with parallel agents\n", llmCfg.Model, llmCfg.Provider)

	projectName := filepath.Base(cwd)

	// Create agents
	docAgent := agents.NewDocAgent(llmCfg)
	codeAgent := agents.NewReactCodeAgent(llmCfg, cwd)
	gitAgent := agents.NewGitAgent(llmCfg)
	depsAgent := agents.NewDepsAgent(llmCfg)

	agentsList := []agents.Agent{docAgent, codeAgent, gitAgent, depsAgent}

	// Prepare input
	input := agents.Input{
		BasePath:    cwd,
		ProjectName: projectName,
		Mode:        agents.ModeBootstrap,
		Verbose:     true, // Will be suppressed in TUI
	}

	// Run TUI
	tuiModel := ui.NewBootstrapModel(ctx, input, agentsList)
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

	// Aggregate findings
	allFindings := agents.AggregateFindings(bootstrapModel.Results)

	// Render the dashboard summary using new UI component
	ui.RenderBootstrapDashboard(allFindings)

	if preview || viper.GetBool("preview") {
		fmt.Println("\n💡 This was a preview. Run 'taskwing bootstrap' to save to memory.")
		return nil
	}

	// Save to memory using KnowledgeService
	memoryPath := config.GetMemoryBasePath()
	store, err := memory.NewSQLiteStore(memoryPath)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	files := memory.NewMarkdownStore(memoryPath)
	repo := memory.NewRepository(store, files)
	defer func() { _ = repo.Close() }()

	// Create Service
	ks := knowledge.NewService(repo, llmCfg)

	// Ingest
	return ks.IngestFindings(ctx, allFindings, !viper.GetBool("quiet"))
}
