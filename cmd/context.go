/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
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

Examples:
  taskwing context "how does authentication work"
  taskwing context "error handling"
  taskwing context "what decisions were made about the API"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runContext,
}

var contextLimit int

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.Flags().IntVar(&contextLimit, "limit", 5, "Maximum number of nodes to return")
}

type scoredNode struct {
	Node  memory.Node
	Score float32
}

func runContext(cmd *cobra.Command, args []string) error {
	query := args[0]

	store, err := memory.NewSQLiteStore(GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	// Get API key for embeddings
	apiKey := viper.GetString("llm.apiKey")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		// Fallback to keyword search if no API key
		return runKeywordSearch(store, query)
	}

	ctx := context.Background()
	llmCfg := llm.Config{
		APIKey: apiKey,
	}

	// Get all nodes with embeddings
	nodes, err := store.ListNodes("")
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if len(nodes) == 0 {
		fmt.Println("No knowledge nodes found.")
		fmt.Println("Add some with: taskwing add \"Your text here\"")
		return nil
	}

	// Generate query embedding
	if !viper.GetBool("quiet") {
		fmt.Fprint(os.Stderr, "🔍 Searching...")
	}

	queryEmbedding, err := knowledge.GenerateEmbedding(ctx, query, llmCfg)
	if err != nil {
		// Fallback to keyword search
		if !viper.GetBool("quiet") {
			fmt.Fprintln(os.Stderr, " falling back to keyword search")
		}
		return runKeywordSearch(store, query)
	}

	// Score each node by similarity
	var scored []scoredNode
	for _, n := range nodes {
		// Load full node with embedding
		fullNode, err := store.GetNode(n.ID)
		if err != nil {
			continue
		}

		if len(fullNode.Embedding) == 0 {
			// Node has no embedding, skip for now
			continue
		}

		score := knowledge.CosineSimilarity(queryEmbedding, fullNode.Embedding)
		scored = append(scored, scoredNode{Node: *fullNode, Score: score})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Limit results
	if len(scored) > contextLimit {
		scored = scored[:contextLimit]
	}

	if !viper.GetBool("quiet") {
		fmt.Fprintln(os.Stderr, " done")
	}

	if len(scored) == 0 {
		fmt.Println("No matching knowledge found.")
		fmt.Println("Try adding more context with: taskwing add \"...\"")
		return nil
	}

	// Output
	if viper.GetBool("json") {
		type result struct {
			Query   string       `json:"query"`
			Results []scoredNode `json:"results"`
		}
		output, _ := json.MarshalIndent(result{Query: query, Results: scored}, "", "  ")
		fmt.Println(string(output))
		return nil
	}

	fmt.Printf("Context for: \"%s\"\n\n", query)
	for i, s := range scored {
		icon := typeIcon(s.Node.Type)
		fmt.Printf("%d. %s [%s] (%.0f%% match)\n", i+1, icon, s.Node.Type, s.Score*100)
		if s.Node.Summary != "" {
			fmt.Printf("   %s\n", s.Node.Summary)
		} else {
			fmt.Printf("   %s\n", truncateSummary(s.Node.Content, 80))
		}
		fmt.Printf("   ID: %s\n\n", s.Node.ID)
	}

	return nil
}

func runKeywordSearch(store *memory.SQLiteStore, query string) error {
	fmt.Println("⚠️  Semantic search requires API key. Showing all nodes instead.")
	fmt.Println()

	nodes, err := store.ListNodes("")
	if err != nil {
		return err
	}

	for _, n := range nodes {
		icon := typeIcon(n.Type)
		fmt.Printf("%s [%s] %s\n", icon, n.Type, n.Summary)
	}

	return nil
}
