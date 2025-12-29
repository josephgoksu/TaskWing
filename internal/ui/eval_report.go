package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

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

// BenchmarkRun holds data for a single model in a single run
type BenchmarkRun struct {
	RunID        string    `json:"run_id"`
	RunDate      time.Time `json:"run_date"`
	Model        string    `json:"model"`
	Score        float64   `json:"score"` // 0.0-1.0
	Pass         int       `json:"pass"`
	Total        int       `json:"total"`
	InputTokens  int       `json:"input_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	CostUSD      float64   `json:"cost_usd,omitempty"`
	Label        string    `json:"label,omitempty"`
}

// BenchmarkData holds aggregated data across multiple runs
type BenchmarkData struct {
	Runs   []string                           `json:"runs"`
	Models []string                           `json:"models"`
	Matrix map[string]map[string]BenchmarkRun `json:"matrix"` // [model][runID]
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

// RenderBenchmark renders historical benchmark data with trend arrows
func RenderBenchmark(data BenchmarkData) {
	if len(data.Runs) == 0 {
		fmt.Println("No benchmark data.")
		return
	}

	// Header
	fmt.Println()
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Render(fmt.Sprintf("BENCHMARK RESULTS · %d runs · %d models", len(data.Runs), len(data.Models)))
	fmt.Println(header)
	fmt.Println()

	// Build column headers (run dates with time)
	fmt.Println(sectionStyle.Render("Score History"))
	fmt.Println()

	// Header row with run dates and times
	headerRow := fmt.Sprintf("  %-42s", "Model")
	for _, runID := range data.Runs {
		// Format: show date+time (20251228-193648 -> 12-28 19:36)
		dateStr := runID
		if len(runID) >= 13 {
			dateStr = runID[4:6] + "-" + runID[6:8] + " " + runID[9:11] + ":" + runID[11:13]
		} else if len(runID) >= 8 {
			dateStr = runID[4:6] + "-" + runID[6:8]
		}
		headerRow += fmt.Sprintf(" %-16s", dateStr)
	}
	fmt.Println(dimStyle.Render(headerRow))

	// Model rows
	for _, model := range data.Models {
		shortModel := formatModelName(model, 40)
		row := fmt.Sprintf("  %-42s", shortModel)

		var prevScore float64 = -1
		var hadPrevData bool = false
		for i, runID := range data.Runs {
			runData, ok := data.Matrix[model][runID]
			if !ok {
				row += fmt.Sprintf(" %-16s", "") // blank instead of -
				continue
			}

			scorePercent := runData.Score * 100
			var scoreStr string

			// Color based on score
			if runData.Score >= 0.8 {
				scoreStr = passStyle.Render(fmt.Sprintf("%.0f%%", scorePercent))
			} else if runData.Score == 0 {
				scoreStr = failStyle.Render(fmt.Sprintf("%.0f%%", scorePercent))
			} else {
				scoreStr = fmt.Sprintf("%.0f%%", scorePercent)
			}
			scoreStr += fmt.Sprintf(" (%d/%d)", runData.Pass, runData.Total)

			// Add trend arrow comparing to previous data point
			if i > 0 && hadPrevData {
				if runData.Score > prevScore {
					scoreStr = passStyle.Render("▲") + " " + scoreStr
				} else if runData.Score < prevScore {
					scoreStr = failStyle.Render("▼") + " " + scoreStr
				} else {
					scoreStr = dimStyle.Render("─") + " " + scoreStr
				}
			} else {
				scoreStr = "  " + scoreStr
			}

			row += fmt.Sprintf(" %-16s", scoreStr)
			prevScore = runData.Score
			hadPrevData = true
		}

		fmt.Println(row)
	}

	// Summary: Show best model overall
	fmt.Println()
	fmt.Println(sectionStyle.Render("Overall Best"))
	fmt.Println()

	// Calculate average score per model across all runs
	type modelAvg struct {
		model    string
		avgScore float64
		runCount int
	}
	var averages []modelAvg
	for _, model := range data.Models {
		var total float64
		var count int
		for _, runID := range data.Runs {
			if runData, ok := data.Matrix[model][runID]; ok {
				total += runData.Score
				count++
			}
		}
		if count > 0 {
			averages = append(averages, modelAvg{model: model, avgScore: total / float64(count), runCount: count})
		}
	}
	sort.Slice(averages, func(i, j int) bool {
		return averages[i].avgScore > averages[j].avgScore
	})

	for i, a := range averages {
		if i >= 5 { // Top 5 only
			break
		}
		shortModel := formatModelName(a.model, 40)
		scoreStr := fmt.Sprintf("%.0f%% avg", a.avgScore*100)
		if a.avgScore >= 0.8 {
			scoreStr = passStyle.Render(scoreStr)
		} else if a.avgScore == 0 {
			scoreStr = failStyle.Render(scoreStr)
		}
		runsNote := dimStyle.Render(fmt.Sprintf("(%d runs)", a.runCount))
		fmt.Printf("  %d. %-42s %s %s\n", i+1, shortModel, scoreStr, runsNote)
	}

	// Cost Summary section (only show if cost data exists)
	hasCosts := false
	for _, model := range data.Models {
		for _, runID := range data.Runs {
			if run, ok := data.Matrix[model][runID]; ok && run.CostUSD > 0 {
				hasCosts = true
				break
			}
		}
		if hasCosts {
			break
		}
	}

	if hasCosts {
		fmt.Println()
		fmt.Println(sectionStyle.Render("Cost Summary"))
		fmt.Println()

		// Header
		costHeader := fmt.Sprintf("  %-42s %-12s %-14s %s", "Model", "Total Cost", "Total Tokens", "Avg/Run")
		fmt.Println(dimStyle.Render(costHeader))

		// Calculate totals per model
		type modelCosts struct {
			model       string
			totalCost   float64
			totalTokens int
			runCount    int
		}
		var costs []modelCosts
		for _, model := range data.Models {
			var mc modelCosts
			mc.model = model
			for _, runID := range data.Runs {
				if run, ok := data.Matrix[model][runID]; ok {
					mc.totalCost += run.CostUSD
					mc.totalTokens += run.InputTokens + run.OutputTokens
					mc.runCount++
				}
			}
			if mc.runCount > 0 {
				costs = append(costs, mc)
			}
		}

		// Sort by cost (lowest first for value comparison)
		sort.Slice(costs, func(i, j int) bool {
			return costs[i].totalCost < costs[j].totalCost
		})

		for _, mc := range costs {
			shortModel := formatModelName(mc.model, 40)
			costStr := fmt.Sprintf("$%.2f", mc.totalCost)
			tokensStr := fmt.Sprintf("%dk", mc.totalTokens/1000)
			avgCost := mc.totalCost / float64(mc.runCount)
			avgStr := fmt.Sprintf("$%.3f", avgCost)
			fmt.Printf("  %-42s %-12s %-14s %s\n", shortModel, costStr, tokensStr, avgStr)
		}
	}

	fmt.Println()
}

// formatModelName formats a model name to a fixed width
func formatModelName(name string, maxLen int) string {
	// Remove common prefixes
	name = strings.TrimPrefix(name, "openai_")
	name = strings.TrimPrefix(name, "anthropic_")
	name = strings.TrimPrefix(name, "google_")

	if len(name) > maxLen {
		return name[:maxLen-2] + ".."
	}
	return name
}
