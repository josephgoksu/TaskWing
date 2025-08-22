package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/types"
	"github.com/spf13/cobra"
)

var analyzeComplexityCmd = &cobra.Command{
	Use:   "analyze-complexity",
	Short: "Analyze task complexity and generate a report",
	Long:  "Analyzes each task to assign a 1-10 complexity score and recommended subtask count. Saves JSON report.",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile, _ := cmd.Flags().GetString("file")
		defaultSubtasks, _ := cmd.Flags().GetInt("default-subtasks")

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
			fmt.Println("No tasks found to analyze.")
			return nil
		}

		byID := map[string]models.Task{}
		for _, t := range tasks {
			byID[t.ID] = t
		}

		entries := make([]types.TaskComplexity, 0, len(tasks))
		for _, t := range tasks {
			score, reason := scoreTaskComplexity(t, byID)
			rec := recommendSubtasks(score, defaultSubtasks)
			prompt := buildExpandPrompt(t)
			cmdStr := fmt.Sprintf("taskwing expand --id=%s", t.ID)
			entries = append(entries, types.TaskComplexity{
				TaskID:              t.ID,
				Title:               t.Title,
				Score:               score,
				RecommendedSubtasks: rec,
				Reason:              reason,
				ExpandPrompt:        prompt,
				ExpandCommand:       cmdStr,
			})
		}

		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Score > entries[j].Score })

		stats := types.ComplexityStats{Total: len(entries)}
		for _, e := range entries {
			switch {
			case e.Score <= 3:
				stats.Low++
			case e.Score <= 7:
				stats.Medium++
			default:
				stats.High++
			}
		}

		// Ensure directory
		if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
			return fmt.Errorf("failed to create report directory: %w", err)
		}

		report := types.ComplexityReport{
			GeneratedAtISO:  time.Now().UTC().Format(time.RFC3339),
			DefaultSubtasks: defaultSubtasks,
			Tasks:           entries,
			Stats:           stats,
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal report: %w", err)
		}
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}

		fmt.Printf("Complexity report generated: %s\n", outputFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(analyzeComplexityCmd)
	analyzeComplexityCmd.Flags().String("file", filepath.Join(".taskwing", "reports", "task-complexity-report.json"), "Path to save the complexity report JSON")
	analyzeComplexityCmd.Flags().Int("default-subtasks", 3, "Default recommended subtasks baseline")
}

// scoreTaskComplexity computes a 1-10 score using simple heuristics.
func scoreTaskComplexity(t models.Task, byID map[string]models.Task) (int, string) {
	score := 1
	reasons := []string{}

	// Priority weight
	switch t.Priority {
	case models.PriorityUrgent:
		score += 3
		reasons = append(reasons, "urgent priority +3")
	case models.PriorityHigh:
		score += 2
		reasons = append(reasons, "high priority +2")
	case models.PriorityMedium:
		score += 1
		reasons = append(reasons, "medium priority +1")
	}

	// Description length
	l := len(strings.TrimSpace(t.Description))
	switch {
	case l > 400:
		score += 3
		reasons = append(reasons, ">400 chars desc +3")
	case l > 200:
		score += 2
		reasons = append(reasons, ">200 chars desc +2")
	case l > 80:
		score += 1
		reasons = append(reasons, ">80 chars desc +1")
	}

	// Acceptance criteria
	if strings.TrimSpace(t.AcceptanceCriteria) != "" {
		score += 2
		reasons = append(reasons, "acceptance criteria +2")
	}

	// Dependencies
	if n := len(t.Dependencies); n > 0 {
		add := n
		if add > 3 {
			add = 3
		}
		score += add
		reasons = append(reasons, fmt.Sprintf("%d dependencies +%d", n, add))
		// If any dep not done, add slight complexity
		blocked := false
		for _, id := range t.Dependencies {
			if dep, ok := byID[id]; !ok || dep.Status != models.StatusDone {
				blocked = true
				break
			}
		}
		if blocked {
			score += 1
			reasons = append(reasons, "blocked +1")
		}
	}

	// Existing subtasks indicate breadth
	if len(t.SubtaskIDs) >= 2 {
		score += 2
		reasons = append(reasons, "has >=2 subtasks +2")
	} else if len(t.SubtaskIDs) == 1 {
		score += 1
		reasons = append(reasons, "has 1 subtask +1")
	}

	if score > 10 {
		score = 10
	}
	if score < 1 {
		score = 1
	}
	return score, strings.Join(reasons, "; ")
}

func recommendSubtasks(score int, defaultBase int) int {
	if score >= 8 {
		return max(defaultBase+2, defaultBase)
	}
	if score >= 5 {
		return max(defaultBase+1, defaultBase)
	}
	return defaultBase
}

func buildExpandPrompt(t models.Task) string {
	return fmt.Sprintf("Break down the task '%s' into actionable subtasks with clear acceptance criteria. Consider dependencies and parent context.", t.Title)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
