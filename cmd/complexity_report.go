package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/josephgoksu/taskwing.app/types"
	"github.com/spf13/cobra"
)

var complexityReportCmd = &cobra.Command{
	Use:   "complexity-report",
	Short: "Display complexity analysis report",
	Long:  "Shows a formatted view of task complexity scores, stats, and ready-to-use expand commands.",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		threshold, _ := cmd.Flags().GetInt("threshold")

		data, err := os.ReadFile(file)
		if err != nil {
			// Offer to generate on the spot
			fmt.Printf("Report not found at %s. Generating now...\n", file)
			// Reuse analyze-complexity default flags
			_ = analyzeComplexityCmd.Flags().Set("file", file)
			if err := analyzeComplexityCmd.RunE(analyzeComplexityCmd, nil); err != nil {
				return fmt.Errorf("failed to generate report: %w", err)
			}
			// If still missing, there were no tasks to analyze
			if _, statErr := os.Stat(file); statErr != nil {
				fmt.Println("No tasks found to analyze; report not created.")
				return nil
			}
			// Reload
			var readErr error
			data, readErr = os.ReadFile(file)
			if readErr != nil {
				return fmt.Errorf("failed to read generated report: %w", readErr)
			}
		}

		var report types.ComplexityReport
		if err := json.Unmarshal(data, &report); err != nil {
			return fmt.Errorf("failed to parse report: %w", err)
		}

		// Sort by score desc for display
		sort.SliceStable(report.Tasks, func(i, j int) bool { return report.Tasks[i].Score > report.Tasks[j].Score })

		// Summary
		fmt.Printf("Complexity Report (%s)\n", file)
		fmt.Printf("Generated: %s\n", report.GeneratedAtISO)
		fmt.Printf("Totals: %d | Low: %d | Medium: %d | High: %d\n\n", report.Stats.Total, report.Stats.Low, report.Stats.Medium, report.Stats.High)

		// Table
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleLight)
		t.AppendHeader(table.Row{"Task ID", "Title", "Score", "Rec Subtasks", "Expand Command"})
		for _, e := range report.Tasks {
			if threshold > 0 && e.Score < threshold {
				continue
			}
			t.AppendRow(table.Row{truncateUUID(e.TaskID), e.Title, e.Score, e.RecommendedSubtasks, fmt.Sprintf("taskwing expand --id=%s", e.TaskID)})
		}
		t.Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(complexityReportCmd)
	complexityReportCmd.Flags().String("file", filepath.Join(".taskwing", "reports", "task-complexity-report.json"), "Path to complexity report JSON")
	complexityReportCmd.Flags().Int("threshold", 0, "Only show tasks with score >= threshold")
}
