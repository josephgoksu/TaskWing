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
	resolvedID, node, err := resolveNodeID(repo, nodeID)
	if err != nil {
		return err
	}

	// Confirmation prompt (unless --force or --json)
	force, _ := cmd.Flags().GetBool("force")
	if !force && !isJSON() {
		// Show what will be deleted
		fmt.Printf("\n  Type:    %s\n", node.Type)
		fmt.Printf("  Summary: %s\n", node.Summary)
		if node.Content != "" && node.Content != node.Summary {
			// Show truncated content preview
			contentPreview := node.Content
			if len(contentPreview) > 100 {
				contentPreview = contentPreview[:97] + "..."
			}
			fmt.Printf("  Content: %s\n", contentPreview)
		}
		fmt.Println()

		if !confirmOrAbort("⚠️  Delete this node? [y/N]: ") {
			return nil
		}
	}

	// Check for preview mode - exit before making changes
	if isPreview() {
		if isJSON() {
			return printJSON(map[string]any{
				"status":  "preview",
				"id":      resolvedID,
				"type":    node.Type,
				"summary": node.Summary,
				"message": "Would delete this node (dry run)",
			})
		}
		fmt.Printf("[PREVIEW] Would delete [%s]: %s\n", node.Type, node.Summary)
		return nil
	}

	// Delete the node
	if err := repo.DeleteNode(resolvedID); err != nil {
		return fmt.Errorf("delete node: %w", err)
	}

	if isJSON() {
		return printJSON(deletedResponse{
			Status: "deleted",
			ID:     resolvedID,
			Type:   node.Type,
		})
	} else if !isQuiet() {
		fmt.Printf("✓ Deleted [%s]: %s\n", node.Type, node.Summary)
	}

	return nil
}
