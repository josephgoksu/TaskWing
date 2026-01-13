/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

// explainCmd represents the explain command
var explainCmd = &cobra.Command{
	Use:   "explain <symbol>",
	Short: "Deep dive into a code symbol",
	Long: `Explain how a code symbol fits into the system.

Shows call graph context (who calls it, what it calls), impact analysis
(how many dependents), related architectural decisions, and generates
an AI explanation of the symbol's purpose.

Requires bootstrapped code intelligence (run 'tw bootstrap' first).

Examples:
  tw explain NewRecallApp
  tw explain "CreateFeature" --depth 3
  tw explain --id 42 --verbose`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExplain,
}

var (
	explainDepth    int
	explainSymbolID uint32
)

func init() {
	rootCmd.AddCommand(explainCmd)
	explainCmd.Flags().IntVar(&explainDepth, "depth", 2, "Call graph traversal depth (1-5)")
	explainCmd.Flags().Uint32Var(&explainSymbolID, "id", 0, "Lookup by symbol ID directly")
}

func runExplain(cmd *cobra.Command, args []string) error {
	// Validate input
	query := ""
	if len(args) > 0 {
		query = args[0]
	}
	if query == "" && explainSymbolID == 0 {
		return fmt.Errorf("specify a symbol name or use --id flag")
	}

	// 1. Initialize repository
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// 2. Create app context with LLM config
	llmCfg, err := getLLMConfigForRole(cmd, llm.RoleQuery)
	if err != nil {
		return err
	}

	// Get base path for source code fetching
	basePath, _ := config.GetProjectRoot()

	appCtx := app.NewContextWithConfig(repo, llmCfg)
	appCtx.BasePath = basePath
	explainApp := app.NewExplainApp(appCtx)

	// 3. Show progress
	ctx := context.Background()
	var streamWriter io.Writer

	displayName := query
	if displayName == "" {
		displayName = fmt.Sprintf("symbol #%d", explainSymbolID)
	}

	if !isQuiet() {
		ui.RenderExplainHeader(displayName)
		if !isJSON() {
			streamWriter = os.Stdout
		}
	}

	// 4. Execute deep dive
	result, err := explainApp.Explain(ctx, app.ExplainRequest{
		Query:        query,
		SymbolID:     explainSymbolID,
		Depth:        explainDepth,
		IncludeCode:  isVerbose(),
		StreamWriter: streamWriter,
	})
	if err != nil {
		return fmt.Errorf("explain failed: %w", err)
	}

	// 5. Output
	if isJSON() {
		return printJSON(result)
	}

	// Render streamed explanation newline
	if streamWriter != nil && result.Explanation != "" {
		fmt.Println()
	}

	// Render full results
	ui.RenderExplainResult(result, isVerbose())

	return nil
}
