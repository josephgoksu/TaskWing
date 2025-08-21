package cmd

// Board MCP tools: snapshot and reconcile

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterBoardTools registers board-related MCP tools
func RegisterBoardTools(server *mcp.Server, taskStore store.TaskStore) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "board-snapshot",
		Description: "Return Kanban-style snapshot grouped by status. Args: limit (int), include_tasks (bool). Columns: todo, doing, review, done.",
	}, boardSnapshotHandler(taskStore))
	mcp.AddTool(server, &mcp.Tool{
		Name:        "board-reconcile",
		Description: "Apply multiple operations by reference (complete/delete/prioritize/update). Supports dry_run preview and returns final snapshot.",
	}, boardReconcileHandler(taskStore))
	return nil
}

func boardSnapshotHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.BoardSnapshotParams, types.BoardSnapshotResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.BoardSnapshotParams]) (*mcp.CallToolResultFor[types.BoardSnapshotResponse], error) {
		args := params.Arguments

		limit := args.Limit
		if limit <= 0 {
			limit = 10
		}
		// Default true; only false when explicitly set
		includeTasks := args.IncludeTasks

		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Group by status
		groups := map[models.TaskStatus][]models.Task{
			models.StatusTodo:   {},
			models.StatusDoing:  {},
			models.StatusReview: {},
			models.StatusDone:   {},
		}
		for _, t := range tasks {
			groups[t.Status] = append(groups[t.Status], t)
		}

		// Sort each group by UpdatedAt desc for relevance
		for k := range groups {
			sort.SliceStable(groups[k], func(i, j int) bool {
				return groups[k][i].UpdatedAt.After(groups[k][j].UpdatedAt)
			})
		}

		// Build response columns
		columns := []types.BoardColumn{}
		order := []models.TaskStatus{models.StatusTodo, models.StatusDoing, models.StatusReview, models.StatusDone}
		for _, st := range order {
			colTasks := groups[st]
			col := types.BoardColumn{Status: string(st), Count: len(colTasks)}
			if includeTasks && len(colTasks) > 0 {
				// limit tasks
				end := len(colTasks)
				if end > limit {
					end = limit
				}
				col.Tasks = make([]types.TaskResponse, end)
				for i := 0; i < end; i++ {
					col.Tasks[i] = taskToResponse(colTasks[i])
				}
			}
			columns = append(columns, col)
		}

		summary := fmt.Sprintf("Board: %d total | todo:%d doing:%d review:%d done:%d",
			len(tasks), len(groups[models.StatusTodo]), len(groups[models.StatusDoing]), len(groups[models.StatusReview]), len(groups[models.StatusDone]))

		resp := types.BoardSnapshotResponse{
			Total:   len(tasks),
			Columns: columns,
			Summary: summary,
		}

		text := summary
		return &mcp.CallToolResultFor[types.BoardSnapshotResponse]{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: resp,
		}, nil
	}
}

func boardReconcileHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.BoardReconcileParams, types.BoardReconcileResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.BoardReconcileParams]) (*mcp.CallToolResultFor[types.BoardReconcileResponse], error) {
		args := params.Arguments

		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		results := make([]types.BoardReconcileOpResult, 0, len(args.Ops))
		succeeded := 0
		failed := 0

		for _, op := range args.Ops {
			ref := strings.TrimSpace(op.Reference)
			if ref == "" {
				results = append(results, types.BoardReconcileOpResult{Reference: ref, Success: false, Error: "missing reference"})
				failed++
				continue
			}
			resolvedID := ref
			// Resolve to ID if necessary
			if id, _, ok := resolveReference(ref, tasks); ok {
				resolvedID = id
			}

			if args.DryRun {
				results = append(results, types.BoardReconcileOpResult{Reference: ref, ResolvedID: resolvedID, Success: true, Message: "preview"})
				continue
			}

			// Apply action
			var applyErr error
			switch strings.ToLower(op.Action) {
			case "complete", "done", "mark-done":
				_, applyErr = taskStore.MarkTaskDone(resolvedID)
			case "delete":
				applyErr = taskStore.DeleteTask(resolvedID)
			case "prioritize":
				if op.Priority == "" {
					applyErr = fmt.Errorf("priority required for prioritize action")
				} else {
					_, applyErr = taskStore.UpdateTask(resolvedID, map[string]interface{}{"priority": op.Priority})
				}
			case "update":
				updates := map[string]interface{}{}
				if op.Title != "" {
					updates["title"] = op.Title
				}
				if op.Description != "" {
					updates["description"] = op.Description
				}
				if op.AcceptanceCriteria != "" {
					updates["acceptanceCriteria"] = op.AcceptanceCriteria
				}
				if op.Status != "" {
					updates["status"] = op.Status
				}
				if op.ParentID != "" {
					updates["parentId"] = op.ParentID
				}
				if op.Dependencies != nil {
					updates["dependencies"] = op.Dependencies
				}
				if len(updates) == 0 {
					applyErr = fmt.Errorf("no updates provided")
				} else {
					_, applyErr = taskStore.UpdateTask(resolvedID, updates)
				}
			default:
				applyErr = fmt.Errorf("invalid action: %s", op.Action)
			}

			if applyErr != nil {
				failed++
				results = append(results, types.BoardReconcileOpResult{Reference: ref, ResolvedID: resolvedID, Success: false, Error: applyErr.Error()})
			} else {
				succeeded++
				results = append(results, types.BoardReconcileOpResult{Reference: ref, ResolvedID: resolvedID, Success: true})
			}
		}

		var snapshot *types.BoardSnapshotResponse
		if !args.DryRun {
			// Build a fresh snapshot after changes
			// Reuse handler logic by calling internal function
			snap, _ := buildBoardSnapshot(taskStore, 5, true)
			snapshot = &snap
		}

		resp := types.BoardReconcileResponse{
			DryRun:    args.DryRun,
			Results:   results,
			Succeeded: succeeded,
			Failed:    failed,
			Snapshot:  snapshot,
		}

		text := fmt.Sprintf("Reconcile: %d succeeded, %d failed", succeeded, failed)
		return &mcp.CallToolResultFor[types.BoardReconcileResponse]{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: resp,
		}, nil
	}
}

// buildBoardSnapshot provides reusable snapshot assembly
func buildBoardSnapshot(taskStore store.TaskStore, limit int, includeTasks bool) (types.BoardSnapshotResponse, error) {
	tasks, err := taskStore.ListTasks(nil, nil)
	if err != nil {
		return types.BoardSnapshotResponse{}, err
	}
	groups := map[models.TaskStatus][]models.Task{
		models.StatusTodo: {}, models.StatusDoing: {}, models.StatusReview: {}, models.StatusDone: {},
	}
	for _, t := range tasks {
		groups[t.Status] = append(groups[t.Status], t)
	}
	for k := range groups {
		sort.SliceStable(groups[k], func(i, j int) bool { return groups[k][i].UpdatedAt.After(groups[k][j].UpdatedAt) })
	}
	columns := []types.BoardColumn{}
	order := []models.TaskStatus{models.StatusTodo, models.StatusDoing, models.StatusReview, models.StatusDone}
	for _, st := range order {
		colTasks := groups[st]
		col := types.BoardColumn{Status: string(st), Count: len(colTasks)}
		if includeTasks && len(colTasks) > 0 {
			end := len(colTasks)
			if end > limit {
				end = limit
			}
			col.Tasks = make([]types.TaskResponse, end)
			for i := 0; i < end; i++ {
				col.Tasks[i] = taskToResponse(colTasks[i])
			}
		}
		columns = append(columns, col)
	}
	summary := fmt.Sprintf("Board: %d total | todo:%d doing:%d review:%d done:%d",
		len(tasks), len(groups[models.StatusTodo]), len(groups[models.StatusDoing]), len(groups[models.StatusReview]), len(groups[models.StatusDone]))
	return types.BoardSnapshotResponse{Total: len(tasks), Columns: columns, Summary: summary}, nil
}
