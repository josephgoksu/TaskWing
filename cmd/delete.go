/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a knowledge node",
	Long: `Delete a knowledge node by its ID.

Use 'taskwing list' to find node IDs.

Examples:
  taskwing delete n-abc12345
  taskwing delete n-abc12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	nodeID := args[0]

	repo, err := memory.NewDefaultRepository(config.GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Check if node exists
	node, err := repo.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Delete the node
	if err := repo.DeleteNode(nodeID); err != nil {
		return fmt.Errorf("delete node: %w", err)
	}

	if viper.GetBool("json") {
		output, _ := json.MarshalIndent(map[string]string{
			"status":  "deleted",
			"id":      nodeID,
			"type":    node.Type,
			"summary": node.Summary,
		}, "", "  ")
		fmt.Println(string(output))
	} else if !viper.GetBool("quiet") {
		fmt.Printf("✓ Deleted [%s]: %s\n", node.Type, node.Summary)
	}

	return nil
}
