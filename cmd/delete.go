/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a knowledge node",
	Long: `Delete a knowledge node by its ID.

Use 'taskwing list' to find node IDs.
By default, prompts for confirmation. Use --force to skip.

Examples:
  taskwing delete n-abc12345
  taskwing delete n-abc12345 --force   # Skip confirmation
  taskwing delete n-abc12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

func runDelete(cmd *cobra.Command, args []string) error {
	nodeID := args[0]

	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Check if node exists
	node, err := repo.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Confirmation prompt (unless --force or --json)
	force, _ := cmd.Flags().GetBool("force")
	if !force && !isJSON() {
		// Show what will be deleted
		fmt.Printf("\n  Type:    %s\n", node.Type)
		fmt.Printf("  Summary: %s\n", node.Summary)
		if node.Content != "" && node.Content != node.Summary {
			// Show truncated content preview
			preview := node.Content
			if len(preview) > 100 {
				preview = preview[:97] + "..."
			}
			fmt.Printf("  Content: %s\n", preview)
		}
		fmt.Println()

		if !confirmOrAbort("⚠️  Delete this node? [y/N]: ") {
			return nil
		}
	}

	// Delete the node
	if err := repo.DeleteNode(nodeID); err != nil {
		return fmt.Errorf("delete node: %w", err)
	}

	if isJSON() {
		return printJSON(deletedResponse{
			Status: "deleted",
			ID:     nodeID,
			Type:   node.Type,
		})
	} else if !isQuiet() {
		fmt.Printf("✓ Deleted [%s]: %s\n", node.Type, node.Summary)
	}

	return nil
}
