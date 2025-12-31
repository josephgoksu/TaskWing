package cmd

import (
	"fmt"
	"time"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage knowledge nodes",
	Long:  "Show, update, or delete knowledge nodes.",
}

var nodeShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID := args[0]
		repo, err := openRepo()
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		node, err := repo.GetNode(nodeID)
		if err != nil {
			return err
		}

		if isJSON() {
			return printJSON(node)
		}

		fmt.Printf("Node: %s\n", node.ID)
		fmt.Printf("Type: %s\n", node.Type)
		if node.Summary != "" {
			fmt.Printf("Summary: %s\n", node.Summary)
		}
		if node.Content != "" {
			fmt.Printf("Content: %s\n", node.Content)
		}
		if node.SourceAgent != "" {
			fmt.Printf("Source Agent: %s\n", node.SourceAgent)
		}
		fmt.Printf("Created: %s\n", node.CreatedAt.Format(time.RFC3339))
		return nil
	},
}

var nodeUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID := args[0]
		content, _ := cmd.Flags().GetString("content")
		summary, _ := cmd.Flags().GetString("summary")
		nodeType, _ := cmd.Flags().GetString("type")

		if content == "" && summary == "" && nodeType == "" {
			return fmt.Errorf("no fields to update")
		}
		if nodeType != "" && !isValidNodeType(nodeType) {
			return fmt.Errorf("invalid type: %s", nodeType)
		}

		repo, err := openRepo()
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		if err := repo.UpdateNode(nodeID, content, nodeType, summary); err != nil {
			return err
		}

		updated, err := repo.GetNode(nodeID)
		if err != nil {
			return err
		}

		if isJSON() {
			return printJSON(updated)
		}

		if !isQuiet() {
			fmt.Printf("✓ Updated node %s\n", nodeID)
		}
		return nil
	},
}

var nodeDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete node(s)",
	Long: `Delete a node by ID or bulk delete by type.

Examples:
  tw node delete n-abc12345
  tw node delete --type decision
  tw node delete --type feature --force`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		nodeType, _ := cmd.Flags().GetString("type")

		if len(args) == 0 && nodeType == "" {
			return fmt.Errorf("provide a node id or --type")
		}
		if len(args) > 0 && nodeType != "" {
			return fmt.Errorf("use either node id or --type, not both")
		}

		repo, err := openRepo()
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		if nodeType != "" {
			if !isValidNodeType(nodeType) {
				return fmt.Errorf("invalid type: %s", nodeType)
			}

			nodes, err := repo.ListNodes(nodeType)
			if err != nil {
				return fmt.Errorf("list nodes: %w", err)
			}
			if len(nodes) == 0 {
				if isJSON() {
					return printJSON(bulkDeleteResult{Type: nodeType, Deleted: 0})
				}
				fmt.Printf("No nodes found for type: %s\n", nodeType)
				return nil
			}

			if !force && !isJSON() {
				fmt.Printf("\n  Type:  %s\n", nodeType)
				fmt.Printf("  Count: %d\n\n", len(nodes))
				if !confirmOrAbort("⚠️  Delete ALL nodes of this type? [y/N]: ") {
					return nil
				}
			}

			deleted, err := repo.DeleteNodesByType(nodeType)
			if err != nil {
				return err
			}

			if isJSON() {
				return printJSON(bulkDeleteResult{
					Type:    nodeType,
					Deleted: deleted,
				})
			}

			if !isQuiet() {
				fmt.Printf("✓ Deleted %d node(s) of type %s\n", deleted, nodeType)
			}
			return nil
		}

		nodeID := args[0]
		node, err := repo.GetNode(nodeID)
		if err != nil {
			return err
		}

		if !force && !isJSON() {
			fmt.Printf("\n  Type:    %s\n", node.Type)
			fmt.Printf("  Summary: %s\n", node.Summary)
			if node.Content != "" && node.Content != node.Summary {
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

		if err := repo.DeleteNode(nodeID); err != nil {
			return err
		}

		if isJSON() {
			return printJSON(deletedResponse{
				Status: "deleted",
				ID:     nodeID,
				Type:   node.Type,
			})
		}

		if !isQuiet() {
			fmt.Printf("✓ Deleted [%s]: %s\n", node.Type, node.Summary)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeShowCmd)
	nodeCmd.AddCommand(nodeUpdateCmd)
	nodeCmd.AddCommand(nodeDeleteCmd)

	nodeUpdateCmd.Flags().String("summary", "", "Update the node summary")
	nodeUpdateCmd.Flags().String("content", "", "Update the node content")
	nodeUpdateCmd.Flags().String("type", "", "Update the node type (decision, feature, plan, note, unknown)")
	nodeDeleteCmd.Flags().String("type", "", "Delete all nodes of a given type")
	nodeDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

func isValidNodeType(nodeType string) bool {
	switch nodeType {
	case memory.NodeTypeDecision, memory.NodeTypeFeature, memory.NodeTypePlan, memory.NodeTypeNote, memory.NodeTypeUnknown:
		return true
	default:
		return false
	}
}
