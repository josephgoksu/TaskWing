/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var askCmd = &cobra.Command{
	Use:          "ask <question>",
	Short:        "Search project knowledge and code symbols",
	SilenceUsage: true,
	Long: `Query the project knowledge base from the CLI.

Searches architectural knowledge (decisions, patterns, constraints) and
code symbols (functions, types, interfaces) using the same pipeline as
the MCP ask tool.

By default, uses hybrid search (FTS + vector). Use --fts-only to skip
embedding API calls for faster, offline results.

Examples:
  taskwing ask "how does authentication work"
  taskwing ask "SQLite schema design" --limit 10
  taskwing ask "how does the MCP server work" --answer
  taskwing ask "task state machine" --json
  taskwing ask "API endpoints" --fts-only
  taskwing ask "auth" --workspace=osprey`,
	Args: cobra.ExactArgs(1),
	RunE: runAsk,
}

func init() {
	rootCmd.AddCommand(askCmd)
	askCmd.Flags().BoolP("answer", "a", false, "Generate a RAG answer (uses LLM, slower)")
	askCmd.Flags().StringP("workspace", "w", "", "Filter by workspace (monorepo)")
	askCmd.Flags().IntP("limit", "l", 5, "Max knowledge results")
	askCmd.Flags().Bool("no-symbols", false, "Skip code symbol search")
	askCmd.Flags().Bool("fts-only", false, "Disable vector search (faster, no embedding API call)")
}

func runAsk(cmd *cobra.Command, args []string) error {
	query := args[0]

	repo, err := openRepoOrHandleMissingMemory()
	if err != nil {
		return err
	}
	if repo == nil {
		return nil
	}
	defer func() { _ = repo.Close() }()

	cfg, err := getLLMConfigForRole(cmd, llm.RoleQuery)
	if err != nil {
		return fmt.Errorf("llm config: %w", err)
	}

	askApp := app.NewAskApp(app.NewContextWithConfig(repo, cfg))

	// Build options from flags
	limit, _ := cmd.Flags().GetInt("limit")
	noSymbols, _ := cmd.Flags().GetBool("no-symbols")
	ftsOnly, _ := cmd.Flags().GetBool("fts-only")
	generateAnswer, _ := cmd.Flags().GetBool("answer")
	workspace, _ := cmd.Flags().GetString("workspace")

	if workspace != "" {
		if err := app.ValidateWorkspace(workspace); err != nil {
			return err
		}
	}

	opts := app.DefaultAskOptions()
	opts.Limit = limit
	opts.IncludeSymbols = !noSymbols
	opts.DisableVector = ftsOnly
	opts.GenerateAnswer = generateAnswer
	opts.Workspace = workspace

	if generateAnswer {
		opts.StreamWriter = os.Stdout
	}

	result, err := askApp.Query(cmd.Context(), query, opts)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if isJSON() {
		return printJSON(result)
	}

	ui.RenderAskResult(result, viper.GetBool("verbose"))
	return nil
}
