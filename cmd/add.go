/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <text>",
	Short: "Add knowledge to the project memory",
	Long: `Add any text to the project knowledge graph.

TaskWing uses AI to:
- Classify the type (decision, feature, plan, note)
- Extract a summary
- Identify relationships to existing knowledge

Examples:
  taskwing add "We chose Go because it's fast and deploys as a single binary"
  taskwing add "The auth module handles OAuth2 and session management"
  taskwing add "TODO: implement webhook retry with exponential backoff"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAdd,
}

var (
	addType   string // Manual override for type
	addSkipAI bool   // Skip AI classification
)

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVar(&addType, "type", "", "Manual type override (decision, feature, plan, note)")
	addCmd.Flags().BoolVar(&addSkipAI, "skip-ai", false, "Skip AI classification (store as-is)")
}

func runAdd(cmd *cobra.Command, args []string) error {
	content := strings.TrimSpace(strings.Join(args, " "))
	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	store, err := memory.NewSQLiteStore(GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	node := memory.Node{
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}

	// If manual type provided or skipping AI
	if addType != "" {
		node.Type = addType
		node.Summary = truncate(content, 100)
	} else if addSkipAI {
		node.Type = memory.NodeTypeUnknown
		node.Summary = truncate(content, 100)
	} else {
		// Use AI classification
		apiKey := viper.GetString("llm.apiKey")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}

		if apiKey == "" {
			// Fallback to manual if no API key
			node.Type = memory.NodeTypeUnknown
			node.Summary = truncate(content, 100)
			if !viper.GetBool("quiet") {
				fmt.Fprintln(os.Stderr, "⚠️  No API key found, storing without AI classification")
				fmt.Fprintln(os.Stderr, "   Set OPENAI_API_KEY or use --type to classify manually")
			}
		} else {
			// Classify with LLM
			ctx := context.Background()
			model := viper.GetString("llm.model")
			if model == "" {
				model = config.DefaultOpenAIModel
			}

			providerStr := viper.GetString("llm.provider")
			if providerStr == "" {
				providerStr = config.DefaultProvider
			}

			provider, err := llm.ValidateProvider(providerStr)
			if err != nil {
				return fmt.Errorf("invalid LLM provider: %w", err)
			}

			llmCfg := llm.Config{
				Provider: provider,
				Model:    model,
				APIKey:   apiKey,
				BaseURL:  viper.GetString("llm.baseURL"),
			}

			if !viper.GetBool("quiet") {
				fmt.Fprint(os.Stderr, "🧠 Classifying...")
			}

			result, err := knowledge.Classify(ctx, content, llmCfg)
			if err != nil {
				// Fallback on error
				node.Type = memory.NodeTypeUnknown
				node.Summary = truncate(content, 100)
				if !viper.GetBool("quiet") {
					fmt.Fprintf(os.Stderr, " failed (%v), storing as unknown\n", err)
				}
			} else {
				node.Type = result.Type
				node.Summary = result.Summary
				if !viper.GetBool("quiet") {
					fmt.Fprintf(os.Stderr, " %s\n", result.Type)
				}
			}
		}
	}

	// Generate embedding for semantic search (if API key available)
	apiKey := viper.GetString("llm.apiKey")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey != "" && !addSkipAI {
		ctx := context.Background()
		llmCfg := llm.Config{APIKey: apiKey}

		if !viper.GetBool("quiet") {
			fmt.Fprint(os.Stderr, "📊 Generating embedding...")
		}

		embedding, err := knowledge.GenerateEmbedding(ctx, content, llmCfg)
		if err != nil {
			if !viper.GetBool("quiet") {
				fmt.Fprintf(os.Stderr, " skipped (%v)\n", err)
			}
		} else {
			node.Embedding = embedding
			if !viper.GetBool("quiet") {
				fmt.Fprintln(os.Stderr, " done")
			}
		}
	}

	// Create the node
	if err := store.CreateNode(node); err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	if viper.GetBool("json") {
		output, _ := json.MarshalIndent(map[string]interface{}{
			"status":       "created",
			"id":           node.ID,
			"type":         node.Type,
			"summary":      node.Summary,
			"hasEmbedding": len(node.Embedding) > 0,
		}, "", "  ")
		fmt.Println(string(output))
	} else if !viper.GetBool("quiet") {
		embStatus := ""
		if len(node.Embedding) > 0 {
			embStatus = " 🔍"
		}
		fmt.Printf("✓ Added [%s]: %s%s\n", node.Type, node.Summary, embStatus)
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
