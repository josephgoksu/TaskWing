/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

// impactCmd represents the impact command
var impactCmd = &cobra.Command{
	Use:   "impact <symbol>",
	Short: "Analyze impact of changing a symbol",
	Long: `Analyze the impact of changing a code symbol.

Uses call graph traversal to find all downstream consumers that would be
affected by a change. This helps understand the "blast radius" of code changes.

Examples:
  taskwing impact CreateUser           # Analyze impact of changing CreateUser
  taskwing impact --id 42              # Analyze by symbol ID
  taskwing impact --depth 3            # Limit analysis depth
  taskwing impact CreateUser --json    # Output as JSON`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImpact,
}

var (
	impactID    uint32
	impactDepth int
)

func init() {
	rootCmd.AddCommand(impactCmd)
	impactCmd.Flags().Uint32Var(&impactID, "id", 0, "Symbol ID to analyze (alternative to name)")
	impactCmd.Flags().IntVar(&impactDepth, "depth", 5, "Maximum traversal depth")
}

func runImpact(cmd *cobra.Command, args []string) error {
	// Need either symbol name or ID
	symbolName := ""
	if len(args) > 0 {
		symbolName = args[0]
	}
	if symbolName == "" && impactID == 0 {
		return fmt.Errorf("provide a symbol name or use --id flag")
	}

	// Render header
	if !isJSON() && !isQuiet() {
		target := symbolName
		if impactID > 0 {
			target = fmt.Sprintf("ID:%d", impactID)
		}
		ui.RenderPageHeader("TaskWing Impact Analysis", fmt.Sprintf("Symbol: %s", target))
	}

	// Initialize repository
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	appCtx := app.NewContext(repo)
	codeIntelApp := app.NewCodeIntelApp(appCtx)

	// Check if index exists
	ctx := context.Background()
	stats, err := codeIntelApp.GetStats(ctx)
	if err != nil || stats.SymbolsFound == 0 {
		fmt.Println("⚠️  No symbols indexed. Run 'tw index' first to index your codebase.")
		return nil
	}

	if !isQuiet() {
		fmt.Fprint(os.Stderr, "🔍 Analyzing impact...")
	}

	result, err := codeIntelApp.AnalyzeImpact(ctx, app.AnalyzeImpactOptions{
		SymbolID:   impactID,
		SymbolName: symbolName,
		MaxDepth:   impactDepth,
	})
	if err != nil {
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return fmt.Errorf("impact analysis failed: %v", err)
	}

	if !result.Success {
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return fmt.Errorf("%s", result.Message)
	}

	if !isQuiet() {
		fmt.Fprintln(os.Stderr, " done")
	}

	if isJSON() {
		return printJSON(result)
	}

	// Render impact tree
	renderImpactTree(result)

	return nil
}

func renderImpactTree(result *app.AnalyzeImpactResult) {
	// Styles for the tree
	sourceStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")) // Pink

	depthStyles := []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("160")), // Red - depth 1
		lipgloss.NewStyle().Foreground(lipgloss.Color("214")), // Orange - depth 2
		lipgloss.NewStyle().Foreground(lipgloss.Color("226")), // Yellow - depth 3
		lipgloss.NewStyle().Foreground(lipgloss.Color("42")),  // Green - depth 4+
		lipgloss.NewStyle().Foreground(lipgloss.Color("87")),  // Cyan - depth 5+
	}

	subtleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	locationStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Source symbol
	fmt.Println()
	fmt.Printf("%s %s\n",
		sourceStyle.Render("◉"),
		sourceStyle.Render(result.Source.Name),
	)
	fmt.Printf("  %s\n", locationStyle.Render(fmt.Sprintf("%s:%d", result.Source.FilePath, result.Source.StartLine)))
	if result.Source.Signature != "" {
		fmt.Printf("  %s\n", subtleStyle.Render(result.Source.Signature))
	}

	if result.AffectedCount == 0 {
		fmt.Println()
		fmt.Println("✅ No downstream consumers found.")
		fmt.Println("   This symbol is not called by any other code in the index.")
		return
	}

	// Summary
	fmt.Println()
	fmt.Printf("📊 Impact Summary:\n")
	fmt.Printf("   • %d symbols affected\n", result.AffectedCount)
	fmt.Printf("   • %d files impacted\n", result.AffectedFiles)
	fmt.Printf("   • Max depth: %d\n", result.MaxDepth)
	fmt.Println()

	// Tree by depth
	fmt.Println("🌳 Dependency Tree:")
	fmt.Println()

	// Get sorted depth keys
	var depths []int
	for d := range result.ByDepth {
		depths = append(depths, d)
	}
	sort.Ints(depths)

	for _, depth := range depths {
		symbols := result.ByDepth[depth]
		if len(symbols) == 0 {
			continue
		}

		// Get style for this depth
		styleIdx := depth - 1
		if styleIdx >= len(depthStyles) {
			styleIdx = len(depthStyles) - 1
		}
		style := depthStyles[styleIdx]

		// Depth header
		indent := strings.Repeat("  ", depth)
		connector := "├─"
		if depth == 1 {
			connector = "└─"
		}

		fmt.Printf("%s%s Depth %d (%d symbols)\n", indent, subtleStyle.Render(connector), depth, len(symbols))

		for i, sym := range symbols {
			isLast := i == len(symbols)-1
			prefix := "├─"
			if isLast {
				prefix = "└─"
			}

			// Symbol entry
			fmt.Printf("%s  %s %s %s\n",
				indent,
				subtleStyle.Render(prefix),
				style.Render(symbolKindIcon(sym.Kind)),
				style.Render(sym.Name),
			)

			// Location
			location := fmt.Sprintf("%s:%d", sym.FilePath, sym.StartLine)
			continueLine := "│"
			if isLast {
				continueLine = " "
			}
			fmt.Printf("%s  %s   %s\n",
				indent,
				subtleStyle.Render(continueLine),
				locationStyle.Render(location),
			)
		}
	}

	// Risk assessment
	fmt.Println()
	renderRiskAssessment(result)
}

func renderRiskAssessment(result *app.AnalyzeImpactResult) {
	riskLevel := "Low"
	riskColor := lipgloss.Color("42") // Green

	if result.AffectedCount > 10 || result.AffectedFiles > 5 {
		riskLevel = "Medium"
		riskColor = lipgloss.Color("214") // Orange
	}
	if result.AffectedCount > 25 || result.AffectedFiles > 10 {
		riskLevel = "High"
		riskColor = lipgloss.Color("160") // Red
	}

	riskStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(riskColor)

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(riskColor).
		Padding(0, 1)

	var content strings.Builder
	content.WriteString(fmt.Sprintf("Risk Level: %s\n", riskStyle.Render(riskLevel)))
	content.WriteString("\n")

	switch riskLevel {
	case "Low":
		content.WriteString("This change has minimal impact.\n")
		content.WriteString("Proceed with standard testing.")
	case "Medium":
		content.WriteString("This change affects multiple components.\n")
		content.WriteString("Review affected files and run integration tests.")
	case "High":
		content.WriteString("This change has significant blast radius!\n")
		content.WriteString("Consider refactoring to reduce coupling,\n")
		content.WriteString("or ensure comprehensive test coverage.")
	}

	fmt.Println(boxStyle.Render(content.String()))
}
