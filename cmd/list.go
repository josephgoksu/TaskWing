/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/memory"
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
	store, err := memory.NewSQLiteStore(GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	var nodeType string
	if len(args) > 0 {
		nodeType = args[0]
	}

	nodes, err := store.ListNodes(nodeType)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if viper.GetBool("json") {
		output, _ := json.MarshalIndent(nodes, "", "  ")
		fmt.Println(string(output))
		return nil
	}

	if len(nodes) == 0 {
		if nodeType != "" {
			fmt.Printf("No %s nodes found.\n", nodeType)
		} else {
			fmt.Println("No knowledge nodes found.")
		}
		fmt.Println("Add one with: taskwing add \"Your text here\"")
		return nil
	}

	// Group by type
	byType := make(map[string][]memory.Node)
	for _, n := range nodes {
		t := n.Type
		if t == "" {
			t = "unknown"
		}
		byType[t] = append(byType[t], n)
	}

	// Print in order: decision, feature, plan, note, unknown
	typeOrder := []string{"decision", "feature", "plan", "note", "unknown"}
	totalCount := 0

	for _, t := range typeOrder {
		nodes := byType[t]
		if len(nodes) == 0 {
			continue
		}
		totalCount += len(nodes)

		icon := typeIcon(t)
		fmt.Printf("\n## %s %s (%d)\n\n", icon, capitalizeFirst(t), len(nodes))

		for _, n := range nodes {
			summary := n.Summary
			if summary == "" {
				summary = truncateSummary(n.Content, 80)
			}
			fmt.Printf("  • %s\n", summary)
			fmt.Printf("    ID: %s | %s\n", n.ID, n.CreatedAt.Format("2006-01-02"))
		}
	}

	fmt.Printf("\nTotal: %d nodes\n", totalCount)
	return nil
}

func typeIcon(t string) string {
	switch t {
	case "decision":
		return "🎯"
	case "feature":
		return "📦"
	case "plan":
		return "📋"
	case "note":
		return "📝"
	default:
		return "❓"
	}
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func truncateSummary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
