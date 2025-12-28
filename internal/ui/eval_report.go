package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// EvalModelSummary holds summary data for a single model
type EvalModelSummary struct {
	Total    int
	HardFail int
}

// EvalTaskResult holds result data for a single task
type EvalTaskResult struct {
	Task        string
	Model       string
	Pass        bool
	FailedRules []string
}

// EvalReportData holds all data needed for rendering the report
type EvalReportData struct {
	Models  map[string]EvalModelSummary
	Results []EvalTaskResult
	Tasks   []string // Unique task IDs in order
}

var (
	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				Padding(0, 1)

	tableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	passStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	failStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	modelNameStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	scoreBarFull = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	scoreBarEmpty = lipgloss.NewStyle().
			Foreground(lipgloss.Color("237"))

	sectionStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginTop(1)

	dimStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)
)

// RenderEvalReport renders a styled evaluation report to stdout
func RenderEvalReport(data EvalReportData) {
	// Header
	fmt.Println()
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Render("EVAL RESULTS")
	fmt.Println(header)
	fmt.Println()

	// Model Summary Table
	renderModelSummary(data)

	// Task Breakdown
	renderTaskBreakdown(data)

	fmt.Println()
}

func renderModelSummary(data EvalReportData) {
	// Sort models
	models := make([]string, 0, len(data.Models))
	for m := range data.Models {
		models = append(models, m)
	}
	sort.Strings(models)

	fmt.Println(sectionStyle.Render("Model Summary"))
	fmt.Println()

	// Find max model name length for alignment
	maxLen := 0
	for _, m := range models {
		if len(m) > maxLen {
			maxLen = len(m)
		}
	}

	for _, model := range models {
		summary := data.Models[model]
		passed := summary.Total - summary.HardFail
		score := float64(passed) / float64(summary.Total) * 100

		// Progress bar (10 chars)
		barLen := 10
		filledLen := int(float64(barLen) * float64(passed) / float64(summary.Total))
		bar := scoreBarFull.Render(strings.Repeat("█", filledLen)) +
			scoreBarEmpty.Render(strings.Repeat("░", barLen-filledLen))

		// Score text
		scoreText := fmt.Sprintf("%3.0f%% (%d/%d pass)", score, passed, summary.Total)
		if passed == summary.Total {
			scoreText = passStyle.Render(scoreText)
		} else if passed == 0 {
			scoreText = failStyle.Render(scoreText)
		}

		// Model name (truncate if too long)
		displayName := model
		if len(displayName) > 30 {
			displayName = displayName[:27] + "..."
		}
		paddedName := fmt.Sprintf("%-30s", displayName)

		fmt.Printf("  %s %s %s\n", modelNameStyle.Render(paddedName), bar, scoreText)
	}
}

func renderTaskBreakdown(data EvalReportData) {
	if len(data.Tasks) == 0 || len(data.Results) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(sectionStyle.Render("Task Breakdown"))
	fmt.Println()

	// Get unique models
	modelSet := make(map[string]bool)
	for _, r := range data.Results {
		modelSet[r.Model] = true
	}
	models := make([]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, m)
	}
	sort.Strings(models)

	// Create result lookup
	resultMap := make(map[string]map[string]EvalTaskResult) // task -> model -> result
	for _, r := range data.Results {
		if resultMap[r.Task] == nil {
			resultMap[r.Task] = make(map[string]EvalTaskResult)
		}
		resultMap[r.Task][r.Model] = r
	}

	// Header row
	headerRow := fmt.Sprintf("  %-6s", "Task")
	for _, m := range models {
		shortName := shortenModelName(m)
		headerRow += fmt.Sprintf(" %-12s", shortName)
	}
	fmt.Println(dimStyle.Render(headerRow))

	// Task rows
	for _, task := range data.Tasks {
		row := fmt.Sprintf("  %-6s", task)
		for _, model := range models {
			result, ok := resultMap[task][model]
			if !ok {
				row += fmt.Sprintf(" %-12s", "-")
			} else if result.Pass {
				row += " " + passStyle.Render("✓ pass") + "      "
			} else {
				row += " " + failStyle.Render("✗ fail") + "      "
			}
		}
		fmt.Println(row)
	}

	// Failure details
	var failures []EvalTaskResult
	for _, r := range data.Results {
		if !r.Pass {
			failures = append(failures, r)
		}
	}

	if len(failures) > 0 {
		fmt.Println()
		fmt.Println(sectionStyle.Render("Failure Details"))
		fmt.Println()

		sort.Slice(failures, func(i, j int) bool {
			if failures[i].Model == failures[j].Model {
				return failures[i].Task < failures[j].Task
			}
			return failures[i].Model < failures[j].Model
		})

		for _, f := range failures {
			shortModel := shortenModelName(f.Model)
			rules := strings.Join(f.FailedRules, ", ")
			fmt.Printf("  %s %s: %s\n",
				failStyle.Render(f.Task),
				dimStyle.Render(shortModel),
				dimStyle.Render(rules))
		}
	}
}

func shortenModelName(name string) string {
	// Remove common prefixes
	name = strings.TrimPrefix(name, "openai_")
	name = strings.TrimPrefix(name, "anthropic_")

	// Truncate long names
	if len(name) > 12 {
		return name[:10] + ".."
	}
	return name
}
