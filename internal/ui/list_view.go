package ui

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// RenderNodeList renders a list of knowledge nodes to stdout used by the list command.
func RenderNodeList(nodes []memory.Node) {
	// Group by type
	byType := make(map[string][]memory.Node)
	for _, n := range nodes {
		t := n.Type
		if t == "" {
			t = "unknown"
		}
		byType[t] = append(byType[t], n)
	}

	// Calculate stats - include all known types
	typeOrder := []string{"decision", "feature", "constraint", "pattern", "plan", "note", "unknown"}
	var stats []string
	totalCount := 0

	for _, t := range typeOrder {
		count := len(byType[t])
		if count > 0 {
			totalCount += count
			stats = append(stats, fmt.Sprintf("%s %d", TypeIcon(t), count))
		}
	}

	// Render Header Summary
	fmt.Printf(" 🧠 Knowledge: %d nodes (%s)\n", totalCount, strings.Join(stats, " • "))
	fmt.Println(StyleSubtle.Render(strings.Repeat("─", 50)))

	// Render Lists
	for _, t := range typeOrder {
		groupNodes := byType[t]
		if len(groupNodes) == 0 {
			continue
		}

		fmt.Println(StyleHeader.Render(fmt.Sprintf("%s %ss", TypeIcon(t), capitalizeFirst(t))))

		for _, n := range groupNodes {
			summary := n.Summary
			if summary == "" {
				summary = utils.Truncate(n.Content, 60)
			}

			dateStr := n.CreatedAt.Format("Jan 02")
			idStr := n.ID
			if len(idStr) > 6 {
				idStr = idStr[:6]
			}

			fmt.Printf(" • %s %s\n", StyleTitle.Render(summary), StyleSubtle.Render(fmt.Sprintf("[%s %s]", idStr, dateStr)))
		}
		fmt.Println()
	}
}

func TypeIcon(t string) string {
	switch t {
	case "decision":
		return "🎯"
	case "feature":
		return "📦"
	case "constraint":
		return "⚠️"
	case "pattern":
		return "🧩"
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
