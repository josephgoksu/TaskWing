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
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/logger"
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

Workspace Filtering (monorepo support):
  By default, searches all workspaces.
  Use --workspace to filter results by a specific service/workspace.
  Use --all to explicitly search all workspaces (ignores auto-detection).

  Note: Nodes without a workspace are treated as 'root' (global knowledge).

Examples:
  taskwing context "how does authentication work"
  taskwing context "why lancedb" --answer
  taskwing context "what decisions were made about the API" --answer
  taskwing context "api patterns" --workspace=osprey  # Only osprey + root`,
	Args: cobra.MinimumNArgs(1),
	RunE: runContext,
}

var (
	contextLimit     int
	contextAnswer    bool
	contextNoRewrite bool
	contextDeep      bool
	contextDepth     int
	contextOffline   bool
	contextWorkspace string
	contextAll       bool
)

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.Flags().IntVar(&contextLimit, "limit", 5, "Maximum number of nodes to return")
	contextCmd.Flags().BoolVar(&contextAnswer, "answer", false, "Generate an LLM answer from retrieved context (RAG)")
	contextCmd.Flags().BoolVar(&contextNoRewrite, "no-rewrite", false, "Disable LLM query rewriting (faster, no API call)")
	contextCmd.Flags().BoolVar(&contextDeep, "deep", false, "Deep dive: show call graph, impact analysis, and related architecture")
	contextCmd.Flags().IntVar(&contextDepth, "depth", 2, "Call graph traversal depth for --deep mode (1-5)")
	contextCmd.Flags().BoolVar(&contextOffline, "offline", false, "Disable all LLM usage (FTS-only, no rewrite, no answer)")
	contextCmd.Flags().BoolVar(&contextOffline, "no-llm", false, "Alias for --offline")
	contextCmd.Flags().StringVarP(&contextWorkspace, "workspace", "w", "", "Filter by workspace name (e.g., 'osprey', 'api'). Includes 'root' nodes by default.")
	contextCmd.Flags().BoolVar(&contextAll, "all", false, "Search all workspaces (ignores workspace auto-detection)")
}

func runContext(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Track user input for crash logging
	logger.SetLastInput(fmt.Sprintf("context %q", query))

	// Handle --deep mode: route to ExplainApp for symbol deep dive
	if contextDeep && contextOffline {
		return fmt.Errorf("--deep requires LLM access; rerun without --offline")
	}
	if contextDeep {
		return runDeepDive(cmd, query)
	}

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

	if contextOffline {
		if contextAnswer && !isQuiet() && !isJSON() {
			fmt.Fprintln(os.Stderr, "⚠️  --offline disables --answer; continuing without LLM answer.")
		}
		contextAnswer = false
		contextNoRewrite = true
	}

	// 3. Create app context with LLM config
	var llmCfg llm.Config
	if contextOffline {
		llmCfg = llm.Config{}
	} else {
		llmCfg, err = getLLMConfigForRole(cmd, llm.RoleQuery)
		if err != nil {
			return err
		}
	}
	appCtx := app.NewContextWithConfig(repo, llmCfg)
	recallApp := app.NewRecallApp(appCtx)

	// 4. Determine if we'll be streaming the answer
	ctx := context.Background()
	var streamWriter io.Writer
	willStream := contextAnswer && !isJSON()

	// 5. Show progress (CLI-specific)
	// For streaming mode, use a different message since answer will appear inline
	if !isQuiet() && !isJSON() {
		if willStream {
			fmt.Fprintln(os.Stderr, "🔍 Searching and generating answer...")
			fmt.Fprintln(os.Stderr, "")
			fmt.Println("💬 Answer:")
			streamWriter = os.Stdout
		} else {
			fmt.Fprint(os.Stderr, "🔍 Searching...")
		}
	} else if willStream {
		streamWriter = os.Stdout
	}

	// Resolve workspace filtering
	// --all overrides --workspace, empty string means all workspaces
	var workspace string
	if contextAll {
		workspace = "" // Explicitly all workspaces
	} else if contextWorkspace != "" {
		if err := app.ValidateWorkspace(contextWorkspace); err != nil {
			return err
		}
		workspace = contextWorkspace
	}

	// 6. Execute query via app layer (ALL business logic here)
	result, err := recallApp.Query(ctx, query, app.RecallOptions{
		Limit:          contextLimit,
		SymbolLimit:    contextLimit, // Use same limit for symbols
		GenerateAnswer: contextAnswer,
		IncludeSymbols: true, // Include code symbols in search
		NoRewrite:      contextNoRewrite,
		DisableVector:  contextOffline,
		DisableRerank:  contextOffline,
		StreamWriter:   streamWriter,
		Workspace:      workspace,
		IncludeRoot:    true, // Always include root knowledge when filtering by workspace
	})
	if err != nil {
		if !isQuiet() && !willStream {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return fmt.Errorf("search failed: %w", err)
	}

	// 7. Show completion (CLI-specific)
	if !isQuiet() && !isJSON() && !willStream {
		fmt.Fprintln(os.Stderr, " done")
		// Show query rewriting info if it happened
		if result.RewrittenQuery != "" {
			fmt.Fprintf(os.Stderr, "✨ Query improved: \"%s\"\n", result.RewrittenQuery)
		}
		fmt.Fprintf(os.Stderr, "📊 Pipeline: %s\n", result.Pipeline)
	}
	if result.Warning != "" && !isJSON() {
		fmt.Fprintf(os.Stderr, "⚠️  %s\n", result.Warning)
	}

	// 8. Output (JSON or TUI)
	if isJSON() {
		return printJSON(result)
	}

	// 8. Handle empty results (non-JSON)
	if len(result.Results) == 0 && len(result.Symbols) == 0 {
		fmt.Println("No matching knowledge or code symbols found.")
		fmt.Println("Try adding more context with: taskwing add \"...\"")
		fmt.Println("Or run: taskwing bootstrap to index your codebase")
		return nil
	}

	// When streaming was used, answer was already printed - just add a newline
	if willStream && result.Answer != "" {
		fmt.Println() // End the streamed answer with a newline
		fmt.Println() // Blank line before results
	}

	// TUI Output - convert back to ScoredNodes for UI rendering
	// (UI layer expects ScoredNode for detailed rendering)
	scored := nodeResponsesToScoredNodes(result.Results)
	symbols := result.Symbols

	// If we streamed the answer, pass empty string to avoid re-printing
	answerToRender := result.Answer
	if willStream {
		answerToRender = "" // Already printed during streaming
	}

	if isVerbose() {
		ui.RenderContextResultsWithSymbolsVerbose(query, scored, symbols, answerToRender)
	} else {
		ui.RenderContextResultsWithSymbols(query, scored, symbols, answerToRender)
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

// runDeepDive handles the --deep flag for symbol explanation.
func runDeepDive(cmd *cobra.Command, query string) error {
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
	if !isQuiet() {
		ui.RenderExplainHeader(query)
		if !isJSON() {
			streamWriter = os.Stdout
		}
	}

	// 4. Execute deep dive
	result, err := explainApp.Explain(ctx, app.ExplainRequest{
		Query:        query,
		Depth:        contextDepth,
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
