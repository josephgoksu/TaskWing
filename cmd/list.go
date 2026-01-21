/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

var (
	listTypeFlag      string
	listWorkspaceFlag string
	listAllFlag       bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List all knowledge nodes",
	Long: `List all knowledge in the project graph.

Without arguments, lists all nodes.
With a type argument or --type flag, filters to that type only.

Types: decision, feature, constraint, pattern, plan, note, metadata, documentation

Workspace Filtering (monorepo support):
  By default, lists nodes from all workspaces.
  Use --workspace to filter by a specific service/workspace.
  Use --all to explicitly show all workspaces (ignores auto-detection).

  Note: Nodes without a workspace are treated as 'root' (global knowledge).

Examples:
  taskwing list                      # All nodes (all workspaces)
  taskwing list decision             # Only decisions (positional)
  taskwing list --type decision      # Only decisions (flag)
  taskwing list --workspace=osprey   # Only osprey workspace + root
  taskwing list --all                # Explicitly all workspaces
  taskwing list metadata             # Git stats and project info
  taskwing list documentation        # README, CLAUDE.md, etc.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listTypeFlag, "type", "t", "", "Filter by node type (decision, feature, constraint, pattern, plan, note, metadata, documentation)")
	listCmd.Flags().StringVarP(&listWorkspaceFlag, "workspace", "w", "", "Filter by workspace name (e.g., 'osprey', 'api'). Includes 'root' nodes by default.")
	listCmd.Flags().BoolVar(&listAllFlag, "all", false, "Show all workspaces (ignores workspace auto-detection)")
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

	// Resolve workspace: --all overrides --workspace
	// --workspace explicitly sets workspace; --all returns all workspaces
	var workspace string
	if listAllFlag {
		// Explicitly all workspaces - empty string means no filtering
		workspace = ""
	} else if listWorkspaceFlag != "" {
		// Validate explicit workspace
		if err := app.ValidateWorkspace(listWorkspaceFlag); err != nil {
			return err
		}
		workspace = listWorkspaceFlag
	}
	// If neither --all nor --workspace specified, workspace stays empty (all workspaces)

	// Build filter
	filter := memory.NodeFilter{
		Type:        nodeType,
		Workspace:   workspace,
		IncludeRoot: true, // Always include root knowledge when filtering by workspace
	}

	// Use filtered query if workspace is specified, otherwise use simple ListNodes
	var nodes []memory.Node
	if workspace != "" {
		nodes, err = repo.ListNodesFiltered(filter)
	} else {
		nodes, err = repo.ListNodes(nodeType)
	}
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if isJSON() {
		return printJSON(nodes)
	}

	if len(nodes) == 0 {
		if nodeType != "" {
			cmd.Printf("No %s nodes found", nodeType)
		} else {
			cmd.Print("No knowledge nodes found")
		}
		if workspace != "" {
			cmd.Printf(" in workspace '%s'", workspace)
		}
		cmd.Println(".")
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
