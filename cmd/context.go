/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

// contextCmd represents the context command
var contextCmd = &cobra.Command{
	Use:   "context <query>",
	Short: "Find relevant knowledge for a query",
	Long: `Search the knowledge graph semantically.

Uses embeddings to find the most relevant nodes for your query.
Returns context optimized for AI consumption (~500-1000 tokens).

Use --answer to get an LLM-generated answer based on the retrieved context.

Examples:
  taskwing context "how does authentication work"
  taskwing context "why lancedb" --answer
  taskwing context "what decisions were made about the API" --answer`,
	Args: cobra.MinimumNArgs(1),
	RunE: runContext,
}

var (
	contextLimit  int
	contextAnswer bool
)

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.Flags().IntVar(&contextLimit, "limit", 5, "Maximum number of nodes to return")
	contextCmd.Flags().BoolVar(&contextAnswer, "answer", false, "Generate an LLM answer from retrieved context (RAG)")
}

func runContext(cmd *cobra.Command, args []string) error {
	query := args[0]

	// 1. Render header (CLI-specific)
	if !isJSON() && !isQuiet() {
		ui.RenderPageHeader("TaskWing Context", fmt.Sprintf("Query: \"%s\"", query))
	}

	// 2. Initialize repository
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// 3. Create app context with LLM config
	llmCfg, err := getLLMConfigForRole(cmd, llm.RoleQuery)
	if err != nil {
		return err
	}
	appCtx := app.NewContextWithConfig(repo, llmCfg)
	recallApp := app.NewRecallApp(appCtx)

	// 4. Show progress (CLI-specific)
	if !isQuiet() {
		fmt.Fprint(os.Stderr, "🔍 Searching...")
	}

	// 5. Execute query via app layer (ALL business logic here)
	ctx := context.Background()
	result, err := recallApp.Query(ctx, query, app.RecallOptions{
		Limit:          contextLimit,
		SymbolLimit:    contextLimit, // Use same limit for symbols
		GenerateAnswer: contextAnswer,
		IncludeSymbols: true, // Include code symbols in search
	})
	if err != nil {
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return fmt.Errorf("search failed: %w", err)
	}

	// 6. Show completion (CLI-specific)
	if !isQuiet() {
		fmt.Fprintln(os.Stderr, " done")
		// Show query rewriting info if it happened
		if result.RewrittenQuery != "" {
			fmt.Fprintf(os.Stderr, "✨ Query improved: \"%s\"\n", result.RewrittenQuery)
		}
		fmt.Fprintf(os.Stderr, "📊 Pipeline: %s\n", result.Pipeline)
	}

	// 7. Handle empty results
	if len(result.Results) == 0 && len(result.Symbols) == 0 {
		fmt.Println("No matching knowledge or code symbols found.")
		fmt.Println("Try adding more context with: taskwing add \"...\"")
		fmt.Println("Or run: taskwing bootstrap to index your codebase")
		return nil
	}

	// 8. Output (JSON or TUI)
	if isJSON() {
		return printJSON(result)
	}

	// TUI Output - convert back to ScoredNodes for UI rendering
	// (UI layer expects ScoredNode for detailed rendering)
	scored := nodeResponsesToScoredNodes(result.Results)
	symbols := result.Symbols
	if isVerbose() {
		ui.RenderContextResultsWithSymbolsVerbose(query, scored, symbols, result.Answer)
	} else {
		ui.RenderContextResultsWithSymbols(query, scored, symbols, result.Answer)
	}

	return nil
}

// nodeResponsesToScoredNodes converts NodeResponse slice back to ScoredNode slice
// for compatibility with existing UI rendering functions.
func nodeResponsesToScoredNodes(responses []knowledge.NodeResponse) []knowledge.ScoredNode {
	result := make([]knowledge.ScoredNode, len(responses))
	for i, r := range responses {
		result[i] = knowledge.ScoredNode{
			Node: &memory.Node{
				ID:                 r.ID,
				Content:            r.Content,
				Type:               r.Type,
				Summary:            r.Summary,
				ConfidenceScore:    r.ConfidenceScore,
				VerificationStatus: r.VerificationStatus,
			},
			Score: r.MatchScore,
		}
	}
	return result
}
