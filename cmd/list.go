/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "Lists tasks with filtering and sorting options",
	Long:    `Lists tasks with various filtering (status, priority, text, tags) and sorting capabilities.`,
	Example: `  # List all todo tasks
  taskwing ls --status todo
  
  # List high priority tasks  
  taskwing ls --priority high
  
  # List ready tasks (no blockers)
  taskwing ls --ready
  
  # Simple format for scripts
  taskwing ls --format simple
  
  # JSON output for integration
  taskwing ls --json`,
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := GetStore()
		if err != nil {
			HandleFatalError("Error: could not get the task store", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleFatalError("Failed to close task store", err)
			}
		}()

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
		jsonOutput, _ := cmd.Flags().GetBool("json")
		readyOnly, _ := cmd.Flags().GetBool("ready")
		blockedOnly, _ := cmd.Flags().GetBool("blocked")
		noLegend, _ := cmd.Flags().GetBool("no-legend")

		if readyOnly && blockedOnly {
			fmt.Fprintln(os.Stderr, "Error: --ready and --blocked flags cannot be used together.")
			os.Exit(1)
		}

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
			raw := strings.Split(priorityFilter, ",")
			prioSet := make(map[models.TaskPriority]bool)
			for _, p := range raw {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if canon, err := normalizePriorityString(p); err == nil {
					prioSet[models.TaskPriority(canon)] = true
				} else {
					// Also try literal value for backward-compat in case of new values
					prioSet[models.TaskPriority(strings.ToLower(p))] = true
				}
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
				// Resolve partial parent ID to full ID
				resolvedParentID, err := resolveTaskID(taskStore, filterByParentID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: parent task '%s' not found: %v\n", filterByParentID, err)
					os.Exit(1)
				}
				filterFns = append(filterFns, func(t models.Task) bool {
					return t.ParentID != nil && *t.ParentID == resolvedParentID
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
			// This is a user input error, not a technical one.
			fmt.Fprintln(os.Stderr, "Error: --top-level and --parent flags cannot be used together.")
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
			HandleFatalError("Failed to list tasks with filters/sorting", err)
		}

		// Apply readiness filters post-fetch if requested
		if readyOnly || blockedOnly {
			all, err := taskStore.ListTasks(nil, nil)
			if err != nil {
				HandleFatalError("Failed to list all tasks for readiness evaluation", err)
			}
			byID := make(map[string]models.Task, len(all))
			for _, t := range all {
				byID[t.ID] = t
			}
			isReady := func(t models.Task) bool {
				if t.Status != models.StatusTodo && t.Status != models.StatusDoing {
					return false
				}
				for _, depID := range t.Dependencies {
					dep, ok := byID[depID]
					if !ok || dep.Status != models.StatusDone {
						return false
					}
				}
				return true
			}
			filtered := make([]models.Task, 0, len(tasks))
			for _, t := range tasks {
				r := isReady(t)
				if readyOnly && r {
					filtered = append(filtered, t)
				}
				if blockedOnly && !r && (t.Status == models.StatusTodo || t.Status == models.StatusDoing) {
					filtered = append(filtered, t)
				}
			}
			tasks = filtered
		}

		if len(tasks) == 0 {
			if jsonOutput {
				fmt.Println("[]")
			} else {
				fmt.Println("No tasks found matching your criteria.")

				// Command discovery hints for empty results
				fmt.Printf("\n💡 Get started:\n")
				fmt.Printf("   • Create a task:     taskwing add \"Your task description\"\n")
				fmt.Printf("   • Interactive mode:  taskwing interactive\n")
				fmt.Printf("   • Quick start:       taskwing quickstart\n")
			}
			return
		}

		if jsonOutput {
			// Output as JSON
			jsonData, err := json.MarshalIndent(tasks, "", "  ")
			if err != nil {
				HandleFatalError("Failed to marshal tasks to JSON", err)
				return
			}
			fmt.Println(string(jsonData))
		} else if renderAsTree {
			// Show current task banner for consistency with table view
			showCurrentTaskBanner(taskStore)
			printLegendUnlessHidden(noLegend)

			// Build a quick lookup for dependency summaries (use all tasks)
			allForDeps, err := taskStore.ListTasks(nil, nil)
			if err != nil {
				HandleFatalError("Failed to list all tasks for dependency summary", err)
			}
			depByID := make(map[string]models.Task, len(allForDeps))
			for _, tsk := range allForDeps {
				depByID[tsk.ID] = tsk
			}

			// Render improved tree view with nice guides and icons
			displayTasksAsTreeEnhanced(tasks, treeRootID, taskStore, depByID, finalFilterFn)
		} else {
			// Show current task information if available
			showCurrentTaskBanner(taskStore)
			printLegendUnlessHidden(noLegend)

			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleLight)
			t.AppendHeader(table.Row{"ID", "Title", "Status", "Priority", "ParentID", "Deps", "Dependents"})

			// Prepare lookup for dependency status summary
			allForDeps, err := taskStore.ListTasks(nil, nil)
			if err != nil {
				HandleFatalError("Failed to list all tasks for dependency summary", err)
			}
			depByID := make(map[string]models.Task, len(allForDeps))
			for _, tsk := range allForDeps {
				depByID[tsk.ID] = tsk
			}

			currentTaskID := GetCurrentTask()

			for _, task := range tasks {
				dependenciesStr := dependencySummaryIcons(task, depByID)
				dependentsStr := truncateUUIDList(task.Dependents)

				parentIDStr := "-"
				if task.ParentID != nil && *task.ParentID != "" {
					parentIDStr = truncateUUID(*task.ParentID)
				}

				// Consistent UI with tree: icons and current marker
				titleDisplay := task.Title
				if currentTaskID != "" && task.ID == currentTaskID {
					titleDisplay = "📌 " + titleDisplay
				}
				statusDisplay := statusIcon(task.Status)
				priorityDisplay := priorityIcon(task.Priority)

				// Wrap title to avoid breaking table layout
				wrappedTitle := wrapText(titleDisplay, 80)
				t.AppendRow(table.Row{
					truncateUUID(task.ID),
					wrappedTitle,
					statusDisplay,
					priorityDisplay,
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

	// Output format flags
	listCmd.Flags().Bool("json", false, "Output results in JSON format for automation and scripting")

	// Readiness filters
	listCmd.Flags().Bool("ready", false, "Show only tasks that are ready (all dependencies done; status todo/doing)")
	listCmd.Flags().Bool("blocked", false, "Show only tasks that are blocked (some dependencies not done; status todo/doing)")

	// UI options
	listCmd.Flags().Bool("no-legend", false, "Hide the status/priority legend in output")
}

// displayTasksAsTreeEnhanced renders a compact, readable tree with proper guides and icons.
// It respects the provided filtered set (allTasks). If currentTaskID is set, it prints that subtree
// regardless of filters for descendants (to show full context of the selected task).
func displayTasksAsTreeEnhanced(allTasks []models.Task, currentTaskID string, store store.TaskStore, depByID map[string]models.Task, originalFilterFn func(models.Task) bool) {
	// Build parent map and index of tasks in scope
	byID := make(map[string]models.Task, len(allTasks))
	parentMap := make(map[string][]models.Task)
	top := make([]models.Task, 0)
	for _, t := range allTasks {
		// Respect original filter for the general tree view
		if originalFilterFn != nil && !originalFilterFn(t) {
			continue
		}
		byID[t.ID] = t
		if t.ParentID == nil || *t.ParentID == "" {
			top = append(top, t)
		} else {
			parentMap[*t.ParentID] = append(parentMap[*t.ParentID], t)
		}
	}

	// Helper to sort siblings for a pleasant order: priority desc, then status, then title
	sortSiblings := func(list []models.Task) {
		sort.SliceStable(list, func(i, j int) bool {
			li, lj := list[i], list[j]
			pi, pj := priorityToInt(li.Priority), priorityToInt(lj.Priority)
			if pi != pj {
				return pi > pj // higher priority first
			}
			si, sj := statusToInt(li.Status), statusToInt(lj.Status)
			if si != sj {
				return si < sj // workflow order
			}
			return strings.ToLower(li.Title) < strings.ToLower(lj.Title)
		})
	}
	sortSiblings(top)
	for k := range parentMap {
		sortSiblings(parentMap[k])
	}

	// Current task marker
	current := GetCurrentTask()

	// Node printer using classic guide drawing algorithm
	var printNode func(t models.Task, prefix string, isLast bool, useParentMap bool)
	printNode = func(t models.Task, prefix string, isLast bool, useParentMap bool) {
		// Line guide
		connector := "├── "
		nextPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			nextPrefix = prefix + "    "
		}

		// Left gutter (omit connector for the very first printed root when prefix == "")
		left := prefix
		if prefix != "" || connector == "└── " || connector == "├── " {
			left += connector
		}

		// Compose badges and content
		curMark := ""
		if current != "" && t.ID == current {
			curMark = "📌 "
		}
		status := statusIcon(t.Status)
		prio := priorityIcon(t.Priority)

		deps := dependencySummaryIcons(t, depByID)
		depPart := ""
		if deps != "-" {
			depPart = "  deps: " + deps
		}

		fmt.Printf("%s%s%s [%s] %s  %s%s\n", left, curMark, status, truncateUUID(t.ID), t.Title, prio, depPart)

		// Children
		var children []models.Task
		if useParentMap {
			children = parentMap[t.ID]
		} else {
			// Subtree mode: fetch via SubtaskIDs (full subtree for a specific root)
			for _, cid := range t.SubtaskIDs {
				if ct, ok := byID[cid]; ok {
					children = append(children, ct)
				} else if fetched, err := store.GetTask(cid); err == nil {
					children = append(children, fetched)
				}
			}
			sortSiblings(children)
		}
		for i, c := range children {
			printNode(c, nextPrefix, i == len(children)-1, useParentMap)
		}
	}

	if currentTaskID != "" {
		// Subtree mode: start at the specified root (try filtered set, then store)
		if r, ok := byID[currentTaskID]; ok {
			printNode(r, "", true, false)
			return
		}
		if fetched, err := store.GetTask(currentTaskID); err == nil {
			printNode(fetched, "", true, false)
			return
		}
		HandleFatalError(fmt.Sprintf("Error: Task ID %s specified for tree view not found.", currentTaskID), fmt.Errorf("not found"))
		return
	}

	// General tree: print all top-level tasks within filtered set
	for i, r := range top {
		printNode(r, "", i == len(top)-1, true)
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

// Legacy helper removed in favor of enhanced tree renderer.

// wrapText wraps the input string to the specified width using spaces as breakpoints.
// It preserves existing newlines and returns a multi-line string suitable for table cells.
func wrapText(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			if line == "" {
				line = word
				continue
			}
			if len(line)+1+len(word) <= width {
				line += " " + word
			} else {
				out = append(out, line)
				// If a single word is longer than width, hard-break it
				for len(word) > width {
					out = append(out, word[:width])
					word = word[width:]
				}
				line = word
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// showCurrentTaskBanner displays current task information if set
func showCurrentTaskBanner(taskStore store.TaskStore) {
	currentTaskID := GetCurrentTask()
	if currentTaskID == "" {
		return
	}

	task, err := taskStore.GetTask(currentTaskID)
	if err != nil {
		fmt.Printf("⚠️  Current task '%s' not found (may have been deleted)\n\n", truncateUUID(currentTaskID))
		return
	}

	fmt.Printf("📌 Current Task: %s - %s (%s, %s)\n\n",
		truncateUUID(task.ID),
		task.Title,
		task.Status,
		task.Priority)
}

// dependencySummaryIcons returns compact dependency readiness like "✅ x/y" or "⏱️ x/y"; "-" when none.
func dependencySummaryIcons(task models.Task, byID map[string]models.Task) string {
	if len(task.Dependencies) == 0 {
		return "-"
	}
	total := len(task.Dependencies)
	done := 0
	for _, id := range task.Dependencies {
		if dep, ok := byID[id]; ok && dep.Status == models.StatusDone {
			done++
		}
	}
	if done == total {
		return fmt.Sprintf("✅ %d/%d", done, total)
	}
	return fmt.Sprintf("⏱️ %d/%d", done, total)
}

// printLegendUnlessHidden prints a compact legend for icons if not suppressed
func printLegendUnlessHidden(noLegend bool) {
	if noLegend {
		return
	}
	fmt.Println("Legend: ⭕ todo  🔄 doing  🔍 review  ✅ done | 🟩 low  🟨 medium  🟧 high  🟥 urgent | deps: ✅ x/y (done/total) | 📌 current task")
	fmt.Println("")
}
