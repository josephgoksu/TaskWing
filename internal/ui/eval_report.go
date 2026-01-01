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
	JudgeReason string // LLM judge's reasoning for the score
	Score       int    // LLM judge score (0-10)
}

// EvalReportData holds all data needed for rendering the report
type EvalReportData struct {
	Models  map[string]EvalModelSummary
	Results []EvalTaskResult
	Tasks   []string // Unique task IDs in order
}

// BenchmarkRun holds data for a single model in a single run
type BenchmarkRun struct {
	RunID        string         `json:"run_id"`
	RunDate      time.Time      `json:"run_date"`
	Model        string         `json:"model"`
	PassRate     float64        `json:"pass_rate"` // 0.0-1.0
	AvgScore     float64        `json:"avg_score"` // 0.0-10.0
	TaskScores   map[string]int `json:"task_scores,omitempty"`
	Pass         int            `json:"pass"`
	Total        int            `json:"total"`
	InputTokens  int            `json:"input_tokens,omitempty"`
	OutputTokens int            `json:"output_tokens,omitempty"`
	CostUSD      float64        `json:"cost_usd,omitempty"`
	Label        string         `json:"label,omitempty"`
}

// BenchmarkData holds aggregated data across multiple runs
type BenchmarkData struct {
	Runs    []string                           `json:"runs"`
	Models  []string                           `json:"models"`
	TaskIDs []string                           `json:"task_ids"` // Union of all task IDs found
	Matrix  map[string]map[string]BenchmarkRun `json:"matrix"`   // [model][runID]
}

var (
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

			// Determine what to display: failed rules or judge reason
			var detail string
			if len(f.FailedRules) > 0 {
				detail = strings.Join(f.FailedRules, ", ")
			} else if f.JudgeReason != "" {
				// Truncate long reasons for display
				reason := f.JudgeReason
				if len(reason) > 120 {
					reason = reason[:117] + "..."
				}
				detail = fmt.Sprintf("score=%d: %s", f.Score, reason)
			} else {
				detail = fmt.Sprintf("score=%d (no details)", f.Score)
			}

			fmt.Printf("  %s %s: %s\n",
				failStyle.Render(f.Task),
				dimStyle.Render(shortModel),
				dimStyle.Render(detail))
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

// RenderBenchmark renders historical benchmark data with detailed task scores
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

	// Scale Legend
	legend := "Scale: 0-10 (" +
		passStyle.Render("≥7 Pass") + ", " +
		"5-6 Marginal, " +
		failStyle.Render("<5 Fail") + ")"
	fmt.Println(legend)
	fmt.Println()

	// Score History Table Headers
	fmt.Println(sectionStyle.Render("Score History"))
	fmt.Println()

	// Build headers: "Model | Date | Avg | T1 | T2 | ... "
	// Since we might have multiple runs per model, we list them all chronologically?
	// Existing UI grouped by Model then iterated Runs. That spreads wide horizontally.
	// User request "individual scores" suggests we need columns for tasks.
	// If we have many runs, columns for tasks AND runs won't fit.
	// Let's Pivot: Row = Run (Model + Date), Cols = Avg + T1 + T2...

	// Header Row
	headerRow := fmt.Sprintf("  %-35s | %-12s | %-8s |", "Model", "Date", "Avg")
	for _, taskID := range data.TaskIDs {
		headerRow += fmt.Sprintf(" %-4s", taskID)
	}
	fmt.Println(dimStyle.Render(headerRow))
	fmt.Println(dimStyle.Render(strings.Repeat("-", len(headerRow)+10)))

	for _, model := range data.Models {
		// Iterate runs for this model
		var modelRuns []BenchmarkRun
		for _, runID := range data.Runs {
			if run, ok := data.Matrix[model][runID]; ok {
				modelRuns = append(modelRuns, run)
			}
		}

		if len(modelRuns) == 0 {
			continue
		}

		// Print Model Name (use the row key directly, which already includes label)
		// Truncate if too long
		shortModel := model
		if len(shortModel) > 35 {
			shortModel = shortModel[:33] + ".."
		}

		fmt.Printf("  %-35s", shortModel)

		// If multiple runs, print data on subsequent lines or same line if single?
		// Let's do:
		// ModelName                 Date1        5.2   8    2    ...
		//                           Date2        6.5   8    5    ...

		for i, run := range modelRuns {
			prefix := ""
			if i > 0 {
				prefix = fmt.Sprintf("  %-35s", "") // Indent for subsequent runs
			}

			// Date
			runID := run.RunID
			dateStr := runID
			if len(runID) >= 13 {
				dateStr = runID[4:6] + "-" + runID[6:8] + " " + runID[9:11] + ":" + runID[11:13]
			}

			// Avg Score
			avgStr := ""
			if run.AvgScore > 0 {
				// Pad first so ANSI codes don't confuse Sprintf width later
				rawAvg := fmt.Sprintf("%-8.1f", run.AvgScore)
				if run.AvgScore >= 7.0 {
					avgStr = passStyle.Render(rawAvg)
				} else if run.AvgScore < 5.0 {
					avgStr = failStyle.Render(rawAvg)
				} else {
					avgStr = rawAvg
				}
			} else {
				// Legacy %
				// Pad to 8 chars
				rawPct := fmt.Sprintf("%-8s", fmt.Sprintf("%.0f%%", run.PassRate*100))
				avgStr = rawPct
			}

			// Render row part 1 with separators
			sep := dimStyle.Render("|")
			// Layout: [Prefix] [Sep] [Date] [Sep] [Avg] [Sep] [Tasks...]
			// Note: avgStr is already padded to 8 chars visually (and colored). Use %s.

			rowStr := fmt.Sprintf(" %s %-12s %s %s %s", sep, dateStr, sep, avgStr, sep)
			if i > 0 {
				rowStr = fmt.Sprintf("%s %s %-12s %s %s %s", prefix, sep, dateStr, sep, avgStr, sep)
			}

			// Render Task Scores
			for _, taskID := range data.TaskIDs {
				score, exists := run.TaskScores[taskID]
				scoreStr := " -"
				if exists {
					scoreStr = fmt.Sprintf("%2d", score) // right align number
					if score >= 7 {
						scoreStr = passStyle.Render(scoreStr)
					} else if score < 5 {
						scoreStr = failStyle.Render(scoreStr)
					}
					// Add padding to match header %-4s (space + 4 chars = 5 total)
					// We output space + 2 chars + 2 spaces = 5 chars
					scoreStr = " " + scoreStr + "  "
				} else if run.AvgScore == 0 && run.PassRate > 0 {
					scoreStr = " ?   "
				} else {
					scoreStr = " -   "
				}
				rowStr += scoreStr
			}

			fmt.Println(rowStr)
		}
		// Spacer between models
		fmt.Println()
	}

	// Overall Best Summary (Keep existing logic but simplified)
	// Calculate weighted average per model
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
				if runData.AvgScore > 0 {
					total += runData.AvgScore
				} else {
					total += runData.PassRate * 10.0
				}
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

	fmt.Println(sectionStyle.Render("Overall Best (Avg)"))
	fmt.Println()
	for i, a := range averages {
		if i >= 5 {
			break
		}
		shortModel := formatModelName(a.model, 30)
		scoreStr := fmt.Sprintf("%.1f", a.avgScore)
		if a.avgScore >= 7.0 {
			scoreStr = passStyle.Render(scoreStr)
		} else if a.avgScore < 5.0 {
			scoreStr = failStyle.Render(scoreStr)
		}
		fmt.Printf("  %d. %-32s %s (%d runs)\n", i+1, shortModel, scoreStr, a.runCount)
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
