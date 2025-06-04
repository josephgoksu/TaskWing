/*
Copyright © 2025 NAME HERE josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/spf13/cobra"
)

// Helper to convert priority to an integer for sorting
func priorityToInt(p models.TaskPriority) int {
	switch p {
	case models.PriorityUrgent:
		return 4
	case models.PriorityHigh:
		return 3
	case models.PriorityMedium:
		return 2
	case models.PriorityLow:
		return 1
	default:
		return 0 // Should not happen with validated data
	}
}

// Helper to convert status to an integer for sorting (example order)
func statusToInt(s models.TaskStatus) int {
	switch s {
	case models.StatusPending:
		return 1
	case models.StatusInProgress:
		return 2
	case models.StatusBlocked:
		return 3
	case models.StatusNeedsReview:
		return 4
	case models.StatusOnHold:
		return 5
	case models.StatusCompleted:
		return 6
	case models.StatusCancelled:
		return 7
	default:
		return 0 // Should not happen
	}
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists tasks with filtering and sorting options",
	Long:  `Lists tasks with various filtering (status, priority, text, tags) and sorting capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := getStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get store: %v\n", err)
			os.Exit(1)
		}
		defer taskStore.Close()

		// Retrieve filter flag values
		statusFilter, _ := cmd.Flags().GetString("status")
		priorityFilter, _ := cmd.Flags().GetString("priority")
		titleContainsFilter, _ := cmd.Flags().GetString("title-contains")
		descContainsFilter, _ := cmd.Flags().GetString("description-contains")
		searchQuery, _ := cmd.Flags().GetString("search")

		// New flags for subtask filtering
		filterByParentID, _ := cmd.Flags().GetString("parent")
		filterTopLevel, _ := cmd.Flags().GetBool("top-level")
		renderAsTree, _ := cmd.Flags().GetBool("tree")
		showAllTasks, _ := cmd.Flags().GetBool("all")

		// Retrieve sorting flag values
		sortBy, _ := cmd.Flags().GetString("sort-by")
		sortOrder, _ := cmd.Flags().GetString("sort-order")

		var filterFns []func(models.Task) bool

		// Build filter functions based on flags
		if statusFilter != "" {
			statuses := strings.Split(strings.ToLower(statusFilter), ",")
			statusSet := make(map[models.TaskStatus]bool)
			for _, s := range statuses {
				statusSet[models.TaskStatus(s)] = true
			}
			filterFns = append(filterFns, func(t models.Task) bool {
				return statusSet[models.TaskStatus(strings.ToLower(string(t.Status)))]
			})
		}

		if priorityFilter != "" {
			priorities := strings.Split(strings.ToLower(priorityFilter), ",")
			prioSet := make(map[models.TaskPriority]bool)
			for _, p := range priorities {
				prioSet[models.TaskPriority(p)] = true
			}
			filterFns = append(filterFns, func(t models.Task) bool {
				return prioSet[models.TaskPriority(strings.ToLower(string(t.Priority)))]
			})
		}

		if titleContainsFilter != "" {
			lowerTitleFilter := strings.ToLower(titleContainsFilter)
			filterFns = append(filterFns, func(t models.Task) bool {
				return strings.Contains(strings.ToLower(t.Title), lowerTitleFilter)
			})
		}

		if descContainsFilter != "" {
			lowerDescFilter := strings.ToLower(descContainsFilter)
			filterFns = append(filterFns, func(t models.Task) bool {
				return strings.Contains(strings.ToLower(t.Description), lowerDescFilter)
			})
		}

		if searchQuery != "" {
			lowerSearchQuery := strings.ToLower(searchQuery)
			filterFns = append(filterFns, func(t models.Task) bool {
				return strings.Contains(strings.ToLower(t.Title), lowerSearchQuery) ||
					strings.Contains(strings.ToLower(t.Description), lowerSearchQuery) ||
					strings.Contains(strings.ToLower(t.ID), lowerSearchQuery)
			})
		}

		// New hierarchical filtering logic for non-tree view
		if !renderAsTree {
			if filterByParentID != "" {
				// --parent is specified, this takes precedence for hierarchy
				filterFns = append(filterFns, func(t models.Task) bool {
					return t.ParentID != nil && *t.ParentID == filterByParentID
				})
			} else if filterTopLevel {
				// --top-level is specified
				filterFns = append(filterFns, func(t models.Task) bool {
					return t.ParentID == nil || *t.ParentID == ""
				})
			} else if !showAllTasks {
				// DEFAULT table view: neither --parent, nor --top-level, nor --all is specified.
				// Show only top-level tasks.
				filterFns = append(filterFns, func(t models.Task) bool {
					return t.ParentID == nil || *t.ParentID == ""
				})
			}
			// If showAllTasks is true AND filterByParentID=="" AND !filterTopLevel, no *additional* hierarchical filter is added here.
			// This means list ALL tasks (subject to other filters like status, etc.).
		}

		// Ensure --top-level and --parent are not used together (this check should be effective).
		if filterTopLevel && filterByParentID != "" {
			fmt.Fprintf(os.Stderr, "Error: --top-level and --parent flags cannot be used together.\n")
			os.Exit(1)
		}

		// If --tree is used with a task ID argument, ensure other filters respect this context.
		// The task ID argument for list is not standard, so we will assume for now that
		// if a task ID is given (args[0]), and --tree is active, that ID is the root of the tree.
		var treeRootID string
		if len(args) > 0 && renderAsTree {
			treeRootID = args[0]
			// Potentially, one might want to clear other filters if a specific tree root is given,
			// or apply them only to the children of that root. For now, existing filters will select the pool of tasks
			// and if treeRootID is set, we start rendering from that task if it's in the pool.
		}

		// Composite filter function
		var finalFilterFn func(models.Task) bool
		if len(filterFns) > 0 {
			finalFilterFn = func(t models.Task) bool {
				for _, fn := range filterFns {
					if !fn(t) {
						return false
					}
				}
				return true
			}
		}

		// Sorting function
		var finalSortFn func([]models.Task) []models.Task
		if sortBy != "" {
			finalSortFn = func(tasks []models.Task) []models.Task {
				sort.SliceStable(tasks, func(i, j int) bool {
					t1 := tasks[i]
					t2 := tasks[j]
					var less bool
					switch strings.ToLower(sortBy) {
					case "id":
						less = t1.ID < t2.ID
					case "title":
						less = strings.ToLower(t1.Title) < strings.ToLower(t2.Title)
					case "status":
						less = statusToInt(t1.Status) < statusToInt(t2.Status)
					case "priority":
						less = priorityToInt(t1.Priority) < priorityToInt(t2.Priority)
					case "createdat":
						less = t1.CreatedAt.Before(t2.CreatedAt)
					case "updatedat":
						less = t1.UpdatedAt.Before(t2.UpdatedAt)
					default:
						// Default to createdAt if sort field is unknown
						less = t1.CreatedAt.Before(t2.CreatedAt)
					}
					if strings.ToLower(sortOrder) == "desc" {
						return !less
					}
					return less
				})
				return tasks
			}
		}

		tasks, err := taskStore.ListTasks(finalFilterFn, finalSortFn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list tasks with filters/sorting: %v\n", err)
			os.Exit(1)
		}

		if len(tasks) == 0 {
			fmt.Println("No tasks found matching your criteria.")
			return
		}

		if renderAsTree {
			displayTasksAsTree(tasks, treeRootID, taskStore, 0, finalFilterFn) // Pass taskStore for fetching children
		} else {
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleLight)
			t.AppendHeader(table.Row{"ID", "Title", "Status", "Priority", "ParentID", "Dependencies", "Dependents"})

			for _, task := range tasks {
				dependenciesStr := truncateUUIDList(task.Dependencies)
				dependentsStr := truncateUUIDList(task.Dependents)

				parentIDStr := "-"
				if task.ParentID != nil && *task.ParentID != "" {
					parentIDStr = truncateUUID(*task.ParentID)
				}

				t.AppendRow(table.Row{
					truncateUUID(task.ID),
					task.Title,
					task.Status,
					task.Priority,
					parentIDStr,
					dependenciesStr,
					dependentsStr,
				})
			}
			t.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Filtering flags
	listCmd.Flags().String("status", "", "Filter by status (comma-separated, e.g., pending,in-progress)")
	listCmd.Flags().String("priority", "", "Filter by priority (comma-separated, e.g., high,urgent)")
	listCmd.Flags().String("title-contains", "", "Filter by text in title (case-insensitive)")
	listCmd.Flags().String("description-contains", "", "Filter by text in description (case-insensitive)")
	listCmd.Flags().String("search", "", "Generic search across title, description, ID (case-insensitive)")

	// Sorting flags
	listCmd.Flags().String("sort-by", "createdAt", "Sort tasks by field (id, title, status, priority, createdAt, updatedAt)")
	listCmd.Flags().String("sort-order", "asc", "Sort order (asc or desc)")

	// Subtask filtering flags
	listCmd.Flags().String("parent", "", "Filter tasks by Parent ID (shows direct subtasks of the given parent)")
	listCmd.Flags().Bool("top-level", false, "Filter to show only top-level tasks (tasks without a parent)")
	listCmd.Flags().Bool("tree", false, "Display tasks in a hierarchical tree structure. If a task ID is provided as an argument, the tree starts from that task.")
	listCmd.Flags().Bool("all", false, "Show all tasks in a flat list, including subtasks (overrides default top-level filtering for table view).")
}

// displayTasksAsTree recursively prints tasks in a tree structure.
// allTasks is the initial pool of tasks (potentially pre-filtered).
// currentTaskID is the ID of the task to start the current branch from (if "", print all top-level trees in allTasks).
// store is needed to fetch children details if not in allTasks (though ideally allTasks contains everything needed).
// indentLevel is for pretty printing.
// originalFilterFn is the filter applied to the initial list, to potentially check if a child should be displayed.
func displayTasksAsTree(allTasks []models.Task, currentTaskID string, store store.TaskStore, indentLevel int, originalFilterFn func(models.Task) bool) {
	if currentTaskID != "" { // Displaying a specific subtree
		var rootTask models.Task
		found := false
		for _, t := range allTasks {
			if t.ID == currentTaskID {
				// If an original filter was applied, the root of the tree must also satisfy it.
				if originalFilterFn == nil || originalFilterFn(t) {
					rootTask = t
					found = true
				}
				break
			}
		}
		if !found {
			// If the specified root ID was not found in the filtered list, try to fetch it directly.
			// This allows `list <id> --tree` even if `<id>` itself doesn't match other filters (e.g. --status), showing its subtree.
			fetchedRoot, err := store.GetTask(currentTaskID)
			if err == nil {
				rootTask = fetchedRoot
				found = true
			} else {
				fmt.Fprintf(os.Stderr, "Error: Task ID %s specified for tree view not found.\n", currentTaskID)
				return
			}
		}
		if found {
			printTaskWithIndent(rootTask, indentLevel)
			for _, subID := range rootTask.SubtaskIDs {
				displayTasksAsTree(allTasks, subID, store, indentLevel+1, originalFilterFn)
			}
		}
	} else { // Displaying all top-level tasks in the provided list as trees
		parentMap := make(map[string][]models.Task)
		topLevelTasks := []models.Task{}

		for _, task := range allTasks {
			if originalFilterFn != nil && !originalFilterFn(task) {
				continue // Skip tasks that don't match the main filter
			}
			if task.ParentID == nil || *task.ParentID == "" {
				topLevelTasks = append(topLevelTasks, task)
			} else {
				parentMap[*task.ParentID] = append(parentMap[*task.ParentID], task)
			}
		}

		// A simple sort for top-level tasks, e.g., by CreatedAt or Title, can be added here if needed.
		// sort.Slice(topLevelTasks, func(i, j int) bool { return topLevelTasks[i].CreatedAt.Before(topLevelTasks[j].CreatedAt) })

		for _, rootTask := range topLevelTasks {
			printTaskWithIndent(rootTask, indentLevel)
			recursivelyPrintChildren(rootTask, parentMap, store, indentLevel+1, originalFilterFn)
		}
	}
}

// recursivelyPrintChildren is a helper for displayTasksAsTree when starting from all top-level tasks.
// It uses a pre-built parentMap for efficiency within the initial filtered list.
func recursivelyPrintChildren(parentTask models.Task, parentMap map[string][]models.Task, store store.TaskStore, indentLevel int, originalFilterFn func(models.Task) bool) {
	// Children directly from parentMap (already filtered by originalFilterFn during map construction)
	if children, ok := parentMap[parentTask.ID]; ok {
		// Can sort children here if needed, e.g. by title or creation date
		for _, childTask := range children {
			printTaskWithIndent(childTask, indentLevel)
			recursivelyPrintChildren(childTask, parentMap, store, indentLevel+1, originalFilterFn)
		}
	} else if len(parentTask.SubtaskIDs) > 0 {
		// If children were not in parentMap (e.g. not matching original filter, or --tree <id> scenario)
		// but SubtaskIDs exist, fetch them directly. This path is more for the specific treeRootID scenario.
		// In the all-tasks tree, parentMap should be comprehensive for tasks matching the filter.
		for _, subID := range parentTask.SubtaskIDs {
			// This part could be complex if we need to re-evaluate originalFilterFn for lazily fetched children.
			// For now, if we are in this branch for the general tree, it implies these children were not in the initial `allTasks` that matched the filter.
			// We might decide not to print them, or fetch and print them regardless of the original filter.
			// For --tree <id>, this is how we fetch the entire subtree.
			subTask, err := store.GetTask(subID) // This assumes store.GetTask is efficient enough.
			if err == nil {
				// Decide if originalFilterFn should apply to these dynamically fetched children.
				// If --tree <id> is used, we typically want the whole subtree of <id>.
				// If --tree is used with general filters, children not matching filter might be skipped.
				// Current logic for all-tasks tree relies on parentMap, which respects originalFilterFn.
				// This else-if branch is more for the `currentTaskID != ""` path of displayTasksAsTree.
				printTaskWithIndent(subTask, indentLevel)
				recursivelyPrintChildren(subTask, parentMap, store, indentLevel+1, originalFilterFn) // parentMap won't be used for these
			}
		}
	}
}

// Helper function to truncate UUIDs for display
func truncateUUID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Helper function to truncate a list of UUIDs for display
func truncateUUIDList(ids []string) string {
	if len(ids) == 0 {
		return "-"
	}
	truncated := make([]string, len(ids))
	for i, id := range ids {
		truncated[i] = truncateUUID(id)
	}
	return strings.Join(truncated, ", ")
}

func printTaskWithIndent(task models.Task, indentLevel int) {
	indent := strings.Repeat("  ", indentLevel) // 2 spaces per indent level
	prefix := "\u251C\u2500\u2500 "             // ├──
	if indentLevel == 0 {
		prefix = ""
	} else {
		// This part needs a way to know if it's the last child to use \u2514\u2500\u2500 (└──)
		// For simplicity, always using ├──. A more complex tree renderer would track siblings.
	}
	fmt.Printf("%s%s[%s] %s (%s) Subtasks: %d, Parent: %s\n",
		indent,
		prefix,
		truncateUUID(task.ID),
		task.Title,
		task.Status,
		len(task.SubtaskIDs),
		printParentID(task.ParentID),
	)
}

func printParentID(parentID *string) string {
	if parentID == nil || *parentID == "" {
		return "none"
	}
	return *parentID
}
