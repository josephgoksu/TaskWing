package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
	"github.com/spf13/cobra"
)

var expandCmd = &cobra.Command{
	Use:   "expand",
	Short: "Expand tasks into subtasks using recommendations or overrides",
	Long: `Expands a task (or all tasks) into subtasks. If a complexity report exists,
uses its recommended subtask counts by default. You can override with --count.

Examples:
  taskwing expand --id=<task-id>
  taskwing expand --all --threshold=7
  taskwing expand --id=<task-id> --count=4 --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID, _ := cmd.Flags().GetString("id")
		allFlag, _ := cmd.Flags().GetBool("all")
		countOverride, _ := cmd.Flags().GetInt("count")
		useReport, _ := cmd.Flags().GetBool("use-report")
		reportFile, _ := cmd.Flags().GetString("file")
		threshold, _ := cmd.Flags().GetInt("threshold")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if taskID == "" && !allFlag {
			return fmt.Errorf("provide --id or --all")
		}
		if taskID != "" && allFlag {
			return fmt.Errorf("use either --id or --all, not both")
		}

		var report *types.ComplexityReport
		if useReport {
			if data, err := os.ReadFile(reportFile); err == nil {
				var rep types.ComplexityReport
				if err := json.Unmarshal(data, &rep); err == nil {
					report = &rep
				}
			}
		}

		st, err := GetStore()
		if err != nil {
			return fmt.Errorf("failed to init store: %w", err)
		}
		defer func() { _ = st.Close() }()

		// Build list of targets
		var targets []models.Task
		if taskID != "" {
			t, err := st.GetTask(taskID)
			if err != nil {
				return fmt.Errorf("task '%s' not found: %w", taskID, err)
			}
			targets = []models.Task{t}
		} else {
			all, err := st.ListTasks(nil, nil)
			if err != nil {
				return fmt.Errorf("failed to list tasks: %w", err)
			}
			for _, t := range all {
				targets = append(targets, t)
			}
		}

		// If report present + --all, order by score desc and apply threshold
		if report != nil && allFlag {
			// map id->score, recommended
			scores := map[string]types.TaskComplexity{}
			for _, e := range report.Tasks {
				scores[e.TaskID] = e
			}
			sort.SliceStable(targets, func(i, j int) bool {
				si := scores[targets[i].ID].Score
				sj := scores[targets[j].ID].Score
				return si > sj
			})
			if threshold > 0 {
				filtered := targets[:0]
				for _, t := range targets {
					if scores[t.ID].Score >= threshold {
						filtered = append(filtered, t)
					}
				}
				targets = filtered
			}
		}

		// Execute expansion
		created := 0
		for _, parent := range targets {
			// Decide count
			n := countOverride
			if n == 0 {
				if report != nil {
					// default to report recommendation if present, else 3
					if e := findEntry(report, parent.ID); e != nil && e.RecommendedSubtasks > 0 {
						n = e.RecommendedSubtasks
					}
				}
				if n == 0 {
					n = 3
				}
			}

			// Dry-run preview
			if dryRun {
				fmt.Printf("Would create %d subtasks under %s - %s\n", n, parent.ID, parent.Title)
				continue
			}

			c, err := createUniformSubtasks(st, parent, n)
			if err != nil {
				return err
			}
			created += c
		}

		if !dryRun {
			fmt.Printf("Created %d subtasks in total.\n", created)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(expandCmd)
	expandCmd.Flags().String("id", "", "Task ID to expand")
	expandCmd.Flags().Bool("all", false, "Expand all tasks (ordered by report complexity when available)")
	expandCmd.Flags().Int("count", 0, "Override subtask count (default: from report or 3)")
	expandCmd.Flags().Bool("use-report", true, "Use complexity report recommendations when available")
	expandCmd.Flags().String("file", filepath.Join(".taskwing", "reports", "task-complexity-report.json"), "Complexity report path")
	expandCmd.Flags().Int("threshold", 0, "Minimum score from report when using --all")
	expandCmd.Flags().Bool("dry-run", false, "Preview without creating subtasks")
}

func findEntry(rep *types.ComplexityReport, id string) *types.TaskComplexity {
	if rep == nil {
		return nil
	}
	for i := range rep.Tasks {
		if rep.Tasks[i].TaskID == id {
			return &rep.Tasks[i]
		}
	}
	return nil
}

func createUniformSubtasks(st store.TaskStore, parent models.Task, n int) (int, error) {
	count := 0
	for i := 1; i <= n; i++ {
		title := fmt.Sprintf("%s — subtask %d", parent.Title, i)
		desc := fmt.Sprintf("Subtask %d for '%s'. Reference parent: %s.", i, parent.Title, parent.ID)
		t := models.Task{
			ID:          "",
			Title:       title,
			Description: desc,
			Status:      models.StatusTodo,
			Priority:    parent.Priority,
			ParentID:    &parent.ID,
		}
		created, err := st.CreateTask(t)
		if err != nil {
			return count, fmt.Errorf("failed creating subtask %d under %s: %w", i, parent.ID, err)
		}
		// Link to parent handled by store
		_ = created
		count++
	}
	return count, nil
}
