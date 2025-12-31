/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
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
	if !isJSON() && !isQuiet() {
		ui.RenderPageHeader("TaskWing Context", fmt.Sprintf("Query: \"%s\"", query))
	}

	// 1. Get Shared LLM Config
	llmCfg, err := getLLMConfig(cmd)
	if err != nil {
		return err
	}

	// 2. Initialize Memory Repository (Source of Truth)
	// 2. Initialize Memory Repository (Source of Truth)
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// 3. Initialize Knowledge Service (Intelligence Layer)
	ks := knowledge.NewService(repo, llmCfg)

	if !isQuiet() {
		fmt.Fprint(os.Stderr, "🔍 Searching...")
	}

	// 4. Execute Search
	ctx := context.Background()
	scored, err := ks.Search(ctx, query, contextLimit)
	if err != nil {
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " failed")
		}
		// Fallback to simpler search or just error out?
		// For consistency, we error out and let user fix config
		return fmt.Errorf("search failed: %w", err)
	}

	if !isQuiet() {
		fmt.Fprintln(os.Stderr, " done")
	}

	if len(scored) == 0 {
		fmt.Println("No matching knowledge found.")
		fmt.Println("Try adding more context with: taskwing add \"...\"")
		return nil
	}

	// 5. Generate Answer (if requested)
	var answer string
	if contextAnswer {
		if !isQuiet() {
			fmt.Fprint(os.Stderr, "🧠 Generating answer...")
		}
		ans, err := ks.Ask(ctx, query, scored)
		if err != nil {
			return fmt.Errorf("ask failed: %w", err)
		}
		answer = ans
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " done")
		}
	}

	// 6. Output (JSON or TUI)
	if isJSON() {
		// Convert to NodeResponse for consistent format with MCP (no embeddings, has evidence)
		var nodeResponses []knowledge.NodeResponse
		for _, sn := range scored {
			nodeResponses = append(nodeResponses, knowledge.ScoredNodeToResponse(sn))
		}
		type result struct {
			Query   string                   `json:"query"`
			Results []knowledge.NodeResponse `json:"results"`
			Answer  string                   `json:"answer,omitempty"`
		}
		return printJSON(result{Query: query, Results: nodeResponses, Answer: answer})
	}

	// TUI Output
	ui.RenderContextResults(query, scored, answer)

	return nil
}
