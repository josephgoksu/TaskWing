/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	// 1. Get Shared LLM Config
	llmCfg, err := getLLMConfig(cmd)
	if err != nil {
		return err
	}

	// 2. Initialize Memory Repository (Source of Truth)
	basePath := config.GetMemoryBasePath()
	db, err := memory.NewSQLiteStore(basePath)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer func() { _ = db.Close() }()

	files := memory.NewMarkdownStore(basePath)
	repo := memory.NewRepository(db, files)

	// 3. Initialize Knowledge Service (Intelligence Layer)
	ks := knowledge.NewService(repo, llmCfg)

	if !viper.GetBool("quiet") {
		fmt.Fprint(os.Stderr, "🔍 Searching...")
	}

	// 4. Execute Search
	ctx := context.Background()
	scored, err := ks.Search(ctx, query, contextLimit)
	if err != nil {
		if !viper.GetBool("quiet") {
			fmt.Fprintln(os.Stderr, " failed")
		}
		// Fallback to simpler search or just error out?
		// For consistency, we error out and let user fix config
		return fmt.Errorf("search failed: %w", err)
	}

	if !viper.GetBool("quiet") {
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
		if !viper.GetBool("quiet") {
			fmt.Fprint(os.Stderr, "🧠 Generating answer...")
		}
		ans, err := ks.Ask(ctx, query, scored)
		if err != nil {
			return fmt.Errorf("ask failed: %w", err)
		}
		answer = ans
		if !viper.GetBool("quiet") {
			fmt.Fprintln(os.Stderr, " done")
		}
	}

	// 6. Output (JSON or TUI)
	if viper.GetBool("json") {
		type result struct {
			Query   string                 `json:"query"`
			Results []knowledge.ScoredNode `json:"results"`
			Answer  string                 `json:"answer,omitempty"`
		}
		output, _ := json.MarshalIndent(result{Query: query, Results: scored, Answer: answer}, "", "  ")
		fmt.Println(string(output))
		return nil
	}

	// TUI Output
	ui.RenderContextResults(query, scored, answer)

	return nil
}
