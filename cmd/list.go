/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	// Styles
	var (
		subtle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		title     = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
		header    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true) // Pinkish header
	)

	// Group by type
	byType := make(map[string][]memory.Node)
	for _, n := range nodes {
		t := n.Type
		if t == "" {
			t = "unknown"
		}
		byType[t] = append(byType[t], n)
	}

	// Calculate stats
	typeOrder := []string{"decision", "feature", "plan", "note", "unknown"}
	var stats []string
	totalCount := 0

	for _, t := range typeOrder {
		count := len(byType[t])
		if count > 0 {
			totalCount += count
			stats = append(stats, fmt.Sprintf("%s %d", typeIcon(t), count))
		}
	}

	// Render Header Summary
	fmt.Printf(" 🧠 Knowledge: %d nodes (%s)\n", totalCount, strings.Join(stats, " • "))
	fmt.Println(subtle.Render(strings.Repeat("─", 50)))

	// Render Lists
	for _, t := range typeOrder {
		groupNodes := byType[t]
		if len(groupNodes) == 0 {
			continue
		}

		fmt.Println(header.Render(fmt.Sprintf("%s %ss", typeIcon(t), capitalizeFirst(t))))

		for _, n := range groupNodes {
			summary := n.Summary
			if summary == "" {
				summary = truncateSummary(n.Content, 60)
			}
			
			dateStr := n.CreatedAt.Format("Jan 02")
			idStr := n.ID
			if len(idStr) > 6 {
				idStr = idStr[:6]
			}
			
			fmt.Printf(" • %s %s\n", title.Render(summary), subtle.Render(fmt.Sprintf("[%s %s]", idStr, dateStr)))
		}
		fmt.Println()
	}

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
