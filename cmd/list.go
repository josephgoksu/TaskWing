/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

var listTypeFlag string

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List all knowledge nodes",
	Long: `List all knowledge in the project graph.

Without arguments, lists all nodes.
With a type argument or --type flag, filters to that type only.

Types: decision, feature, constraint, pattern, plan, note, metadata, documentation

Examples:
  taskwing list                  # All nodes
  taskwing list decision         # Only decisions (positional)
  taskwing list --type decision  # Only decisions (flag)
  taskwing list metadata         # Git stats and project info
  taskwing list documentation    # README, CLAUDE.md, etc.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listTypeFlag, "type", "t", "", "Filter by node type (decision, feature, constraint, pattern, plan, note, metadata, documentation)")
}

func runList(cmd *cobra.Command, args []string) error {
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Support both positional arg and --type flag (flag takes precedence)
	var nodeType string
	if listTypeFlag != "" {
		nodeType = listTypeFlag
	} else if len(args) > 0 {
		nodeType = args[0]
	}

	nodes, err := repo.ListNodes(nodeType)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if isJSON() {
		return printJSON(nodes)
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

	// Delegate rendering to UI package based on verbosity
	if isVerbose() {
		ui.RenderNodeListVerbose(nodes)
	} else {
		ui.RenderNodeList(nodes)
	}

	// Print version footer for human-readable output (not JSON, not quiet)
	if !isQuiet() {
		ver := version
		if ver == "" {
			ver = "dev"
		}
		fmt.Printf("TaskWing v%s\n", ver)
	}

	return nil
}
