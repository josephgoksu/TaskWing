/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/utils"
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

	// 1. Get Unified Config
	llmCfg, err := getLLMConfigForRole(cmd, llm.RoleQuery)
	if err != nil {
		// Log but continue if no API key (AddNode handles missing key gracefully)
		if !isQuiet() {
			fmt.Fprintf(os.Stderr, "⚠️  Config warning: %v\n", err)
		}
	}

	// 2. Initialize Repo
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// 3. Initialize Service
	ks := knowledge.NewService(repo, llmCfg)

	// 4. Prepare Input
	input := knowledge.NodeInput{
		Content: content,
		Type:    addType, // from flag
	}
	if addSkipAI {
		if input.Type == "" {
			input.Type = memory.NodeTypeUnknown
		}
		input.Summary = utils.Truncate(content, 100)
	}

	if !isQuiet() {
		fmt.Fprint(os.Stderr, "🧠 Processing...")
	}

	// 5. Execute
	node, err := ks.AddNode(context.Background(), input)
	if err != nil {
		return fmt.Errorf("add node failed: %w", err)
	}

	if isJSON() {
		return printJSON(nodeCreatedResponse{
			Status:       "created",
			ID:           node.ID,
			Type:         node.Type,
			Summary:      node.Summary,
			HasEmbedding: len(node.Embedding) > 0,
		})
	} else if !isQuiet() {
		fmt.Fprintln(os.Stderr, " done")
		embStatus := ""
		if len(node.Embedding) > 0 {
			embStatus = " 🔍"
		}
		fmt.Printf("✓ Added [%s]: %s%s\n", node.Type, node.Summary, embStatus)
	}

	return nil
}
