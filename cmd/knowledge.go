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
	"github.com/spf13/viper"
)

var (
	knowledgeTypeFlag      string
	knowledgeWorkspaceFlag string
	knowledgeAllFlag       bool
)

var knowledgeCmd = &cobra.Command{
	Use:          "knowledge [type]",
	Short:        "View project knowledge nodes",
	SilenceUsage: true,
	Long: `View project knowledge captured in memory.

Without arguments, lists all nodes.
With a type argument or --type flag, filters to that type only.

Types: decision, feature, constraint, pattern, plan, note, metadata, documentation

Workspace Filtering (monorepo support):
  By default, lists nodes from all workspaces.
  Use --workspace to filter by a specific service/workspace.
  Use --all to explicitly show all workspaces.

Examples:
  taskwing knowledge
  taskwing knowledge decision
  taskwing knowledge --type decision
  taskwing knowledge --workspace=osprey`,
	Args: cobra.MaximumNArgs(1),
	RunE: runKnowledge,
}

func init() {
	rootCmd.AddCommand(knowledgeCmd)
	knowledgeCmd.Flags().StringVarP(&knowledgeTypeFlag, "type", "t", "", "Filter by node type (decision, feature, constraint, pattern, plan, note, metadata, documentation)")
	knowledgeCmd.Flags().StringVarP(&knowledgeWorkspaceFlag, "workspace", "w", "", "Filter by workspace name (e.g., 'osprey', 'api'). Includes root nodes by default.")
	knowledgeCmd.Flags().BoolVar(&knowledgeAllFlag, "all", false, "Show all workspaces")
}

func runKnowledge(cmd *cobra.Command, args []string) error {
	// Support both positional arg and --type flag (flag takes precedence)
	var nodeType string
	if knowledgeTypeFlag != "" {
		nodeType = knowledgeTypeFlag
	} else if len(args) > 0 {
		nodeType = args[0]
	}

	repo, err := openRepoOrHandleMissingMemory()
	if err != nil {
		return err
	}
	if repo == nil {
		return nil
	}
	defer func() { _ = repo.Close() }()

	// Resolve workspace: --all overrides --workspace
	var workspace string
	if knowledgeAllFlag {
		workspace = ""
	} else if knowledgeWorkspaceFlag != "" {
		if err := app.ValidateWorkspace(knowledgeWorkspaceFlag); err != nil {
			return err
		}
		workspace = knowledgeWorkspaceFlag
	}

	filter := memory.NodeFilter{
		Type:        nodeType,
		Workspace:   workspace,
		IncludeRoot: true, // Include root/global nodes with workspace filters.
	}

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
		cmd.Println("Run 'taskwing bootstrap' to populate project memory.")
		return nil
	}

	if viper.GetBool("verbose") {
		ui.RenderNodeListVerbose(nodes)
	} else {
		ui.RenderNodeList(nodes)
	}

	if !isQuiet() {
		ver := version
		if ver == "" {
			ver = "dev"
		}
		fmt.Printf("TaskWing v%s\n", ver)
	}

	return nil
}
