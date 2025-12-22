/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List all knowledge nodes",
	Long: `List all knowledge in the project graph.

Without arguments, lists all nodes.
With a type argument, filters to that type only.

Types: decision, feature, plan, note

Examples:
  taskwing list              # All nodes
  taskwing list decision     # Only decisions
  taskwing list plan         # Only plans`,
	Args: cobra.MaximumNArgs(1),
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	repo, err := memory.NewDefaultRepository(config.GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	var nodeType string
	if len(args) > 0 {
		nodeType = args[0]
	}

	nodes, err := repo.ListNodes(nodeType)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if viper.GetBool("json") {
		output, _ := json.MarshalIndent(nodes, "", "  ")
		cmd.Println(string(output))
		return nil
	}

	if len(nodes) == 0 {
		if nodeType != "" {
			cmd.Printf("No %s nodes found.\n", nodeType)
		} else {
			cmd.Println("No knowledge nodes found.")
		}
		cmd.Println("Add one with: taskwing add \"Your text here\"")
		return nil
	}

	// Styles are now imported from internal/ui

	// Delegate rendering to UI package
	ui.RenderNodeList(nodes)
	return nil
}
