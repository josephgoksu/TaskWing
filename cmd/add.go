/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/cobra"
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

	// 1. Initialize repository
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// 2. Create app context with LLM config
	llmCfg, err := getLLMConfigForRole(cmd, llm.RoleQuery)
	if err != nil {
		// Log but continue - AddNode handles missing key gracefully
		if !isQuiet() {
			fmt.Fprintf(os.Stderr, "⚠️  Config warning: %v\n", err)
		}
	}
	appCtx := app.NewContextWithConfig(repo, llmCfg)
	memoryApp := app.NewMemoryApp(appCtx)

	// 3. Show progress (CLI-specific)
	if !isQuiet() {
		fmt.Fprint(os.Stderr, "🧠 Processing...")
	}

	// 4. Execute add via app layer (ALL business logic here)
	ctx := context.Background()
	result, err := memoryApp.Add(ctx, content, app.AddOptions{
		Type:   addType,
		SkipAI: addSkipAI,
	})
	if err != nil {
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return fmt.Errorf("add node failed: %w", err)
	}

	// 5. Output (JSON or CLI)
	if isJSON() {
		return printJSON(nodeCreatedResponse{
			Status:       "created",
			ID:           result.ID,
			Type:         result.Type,
			Summary:      result.Summary,
			HasEmbedding: result.HasEmbedding,
		})
	}

	if !isQuiet() {
		fmt.Fprintln(os.Stderr, " done")
		embStatus := ""
		if result.HasEmbedding {
			embStatus = " 🔍"
		}
		fmt.Printf("✓ Added [%s]: %s%s\n", result.Type, result.Summary, embStatus)
	}

	return nil
}
