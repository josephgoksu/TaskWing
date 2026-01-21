/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/project"
	"github.com/spf13/cobra"
)

// workspacesCmd represents the workspaces command
var workspacesCmd = &cobra.Command{
	Use:   "workspaces",
	Short: "List detected workspaces in the current project",
	Long: `List all workspaces detected in the current project.

In a monorepo, workspaces are subdirectories that contain separate services
or packages (identified by markers like package.json, go.mod, etc.).

The workspace scoping feature allows filtering knowledge by workspace:
- 'root' workspace contains global/shared knowledge
- Service workspaces (e.g., 'api', 'web') contain service-specific knowledge

Examples:
  taskwing workspaces              # List all detected workspaces
  taskwing workspaces --counts     # Show node counts per workspace`,
	RunE: runWorkspaces,
}

var workspacesShowCounts bool

func init() {
	rootCmd.AddCommand(workspacesCmd)
	workspacesCmd.Flags().BoolVar(&workspacesShowCounts, "counts", false, "Show node counts per workspace")
}

// WorkspaceInfo represents information about a workspace for display
type WorkspaceInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`       // "root", "service", "monorepo"
	NodeCount int    `json:"node_count"` // Number of knowledge nodes
}

func runWorkspaces(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	// Detect workspace structure
	ws, err := project.DetectWorkspace(cwd)
	if err != nil {
		return fmt.Errorf("detect workspace: %w", err)
	}

	// Detect current workspace from cwd
	currentWorkspace, _ := project.DetectWorkspaceFromCwd()

	// Build workspace list
	var workspaces []WorkspaceInfo

	// Always include root
	rootInfo := WorkspaceInfo{
		Name: "root",
		Type: "global",
	}

	// Get node counts if requested
	if workspacesShowCounts {
		repo, err := openRepo()
		if err == nil {
			defer func() { _ = repo.Close() }()

			// Count nodes per workspace
			nodes, err := repo.ListNodes("")
			if err == nil {
				counts := make(map[string]int)
				for _, n := range nodes {
					ws := n.Workspace
					if ws == "" {
						ws = "root"
					}
					counts[ws]++
				}
				rootInfo.NodeCount = counts["root"]

				// Update service counts
				for _, svc := range ws.Services {
					if svc != "." {
						workspaces = append(workspaces, WorkspaceInfo{
							Name:      svc,
							Type:      "service",
							NodeCount: counts[svc],
						})
					}
				}
			}
		}
	} else {
		// No counts - just list services
		for _, svc := range ws.Services {
			if svc != "." {
				workspaces = append(workspaces, WorkspaceInfo{
					Name: svc,
					Type: "service",
				})
			}
		}
	}

	// Sort services alphabetically
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].Name < workspaces[j].Name
	})

	// Prepend root
	workspaces = append([]WorkspaceInfo{rootInfo}, workspaces...)

	// JSON output
	if isJSON() {
		return printJSON(struct {
			WorkspaceType string          `json:"workspace_type"`
			RootPath      string          `json:"root_path"`
			Current       string          `json:"current_workspace"`
			Workspaces    []WorkspaceInfo `json:"workspaces"`
		}{
			WorkspaceType: ws.Type.String(),
			RootPath:      ws.RootPath,
			Current:       currentWorkspace,
			Workspaces:    workspaces,
		})
	}

	// Human-readable output
	fmt.Printf("Project: %s\n", ws.Name)
	fmt.Printf("Type: %s\n", ws.Type.String())
	fmt.Printf("Current workspace: %s\n", currentWorkspace)
	fmt.Println()

	if len(workspaces) == 1 && workspaces[0].Name == "root" {
		fmt.Println("This is a single-repo project (no sub-workspaces detected).")
		fmt.Println("All knowledge is stored in the 'root' workspace.")
	} else {
		fmt.Println("Workspaces:")
		for _, w := range workspaces {
			marker := " "
			if w.Name == currentWorkspace {
				marker = "*"
			}

			if workspacesShowCounts {
				fmt.Printf("  %s %-20s [%s] (%d nodes)\n", marker, w.Name, w.Type, w.NodeCount)
			} else {
				fmt.Printf("  %s %-20s [%s]\n", marker, w.Name, w.Type)
			}
		}
	}

	fmt.Println()
	fmt.Println("Use --workspace=<name> with list/context commands to filter by workspace.")
	if !strings.Contains(currentWorkspace, "root") && currentWorkspace != "root" {
		fmt.Printf("Tip: You're in '%s' - use 'tw list --workspace=%s' to see service-specific knowledge.\n", currentWorkspace, currentWorkspace)
	}

	return nil
}
