package cmd

import (
	"fmt"
	"sort"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/spf13/cobra"
)

// validateDependenciesCmd checks for missing references and circular dependency chains.
var validateDependenciesCmd = &cobra.Command{
	Use:   "validate-dependencies",
	Short: "Validate dependency graph for missing refs and cycles",
	Long:  "Checks all tasks for missing dependency references and circular dependencies. Returns non-zero on issues.",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := GetStore()
		if err != nil {
			return fmt.Errorf("failed to initialize store: %w", err)
		}
		defer func() { _ = store.Close() }()

		tasks, err := store.ListTasks(nil, nil)
		if err != nil {
			return fmt.Errorf("failed to list tasks: %w", err)
		}
		if len(tasks) == 0 {
			fmt.Println("No tasks to validate.")
			return nil
		}

		byID := make(map[string]models.Task, len(tasks))
		for _, t := range tasks {
			byID[t.ID] = t
		}

		issues := []string{}

		// Missing dependency references
		for _, t := range tasks {
			for _, dep := range t.Dependencies {
				if _, ok := byID[dep]; !ok {
					issues = append(issues, fmt.Sprintf("Missing dependency: task %s depends on unknown %s", t.ID, dep))
				}
				if dep == t.ID {
					issues = append(issues, fmt.Sprintf("Self-dependency: task %s depends on itself", t.ID))
				}
			}
		}

		// Cycle detection via DFS
		const (
			white = 0
			gray  = 1
			black = 2
		)
		color := map[string]int{}
		parent := map[string]string{}
		for id := range byID {
			color[id] = white
		}

		var findCyclePath = func(start, end string) []string {
			// reconstruct simple path from end back to start using parent map
			path := []string{end}
			cur := end
			for cur != start && parent[cur] != "" {
				cur = parent[cur]
				path = append(path, cur)
			}
			// reverse
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			return path
		}

		var dfs func(string) bool
		dfs = func(u string) bool {
			color[u] = gray
			for _, v := range byID[u].Dependencies {
				if _, ok := byID[v]; !ok {
					// already reported as missing
					continue
				}
				if color[v] == white {
					parent[v] = u
					if dfs(v) {
						return true
					}
				} else if color[v] == gray {
					// cycle found u -> v
					path := findCyclePath(v, u)
					issues = append(issues, fmt.Sprintf("Cycle detected: %v -> %s", path, v))
					return true
				}
			}
			color[u] = black
			return false
		}

		// Iterate deterministically for stable output
		ids := make([]string, 0, len(byID))
		for id := range byID {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if color[id] == white {
				if dfs(id) {
					// keep checking to collect more issues; reset not required for colors
				}
			}
		}

		if len(issues) == 0 {
			fmt.Println("✅ Dependency graph is valid (no issues found).")
			return nil
		}

		fmt.Printf("❌ Validation failed with %d issue(s):\n", len(issues))
		for i, msg := range issues {
			fmt.Printf("  %d) %s\n", i+1, msg)
		}
		return fmt.Errorf("dependency validation failed")
	},
}

func init() {
	rootCmd.AddCommand(validateDependenciesCmd)
}
