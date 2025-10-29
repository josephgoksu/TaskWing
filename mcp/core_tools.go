package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/josephgoksu/TaskWing/store"
)

// RegisterCoreTools registers ALL essential MCP tools (15 total - optimized for token usage).
// This is the ONLY tool registration function used - all others are disabled.
func RegisterCoreTools(server *mcpsdk.Server, taskStore store.TaskStore) error {
	// ============ OVERVIEW & CONTEXT (2 tools) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task-summary",
		Description: "🎯 ALWAYS CALL FIRST: Get project overview with total tasks, active, completed today, and project health. Essential for understanding context before any operations.",
	}, taskSummaryHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "board-snapshot",
		Description: "Return Kanban-style snapshot grouped by status. Args: limit (int), include_tasks (bool). Columns: todo, doing, review, done.",
	}, boardSnapshotHandler(taskStore))

	// ============ CREATE & PLAN (3 tools) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "add-task",
		Description: "🎯 CREATE PROFESSIONAL TASK (use instead of simple todos): title, description, acceptanceCriteria, priority [low|medium|high|urgent], parentId, dependencies[]. Validates and maintains relationships.",
	}, addTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "batch-create-tasks",
		Description: "Create many tasks at once. Supports TempID-based parent-child linking, dependencies, and priorities. Returns created_tasks, errors, and success_count.",
	}, batchCreateTasksHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "generate-plan",
		Description: "Generate a concise set of subtasks for a parent task (preview by default; confirm to create).",
	}, generatePlanHandler(taskStore))

	// ============ READ & SEARCH (4 tools) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list-tasks",
		Description: "List tasks with filters. Args: status [todo|doing|review|done], priority [low|medium|high|urgent], search, parentId, sortBy [id|title|priority|createdAt|updatedAt], sortOrder [asc|desc]. Returns tasks+count.",
	}, listTasksHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get-task",
		Description: "Get one task by id or reference (partial ID/title). Returns full metadata, relationships, and timestamps.",
	}, getTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "find-task",
		Description: "🔍 FIND SINGLE TASK (use for specific task lookup): Find by partial ID, fuzzy title, or description. Handles typos. Best for: 'find task abc123' or 'find login task'.",
	}, findTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "query-tasks",
		Description: "🔍 NATURAL LANGUAGE SEARCH (preferred for general search): Examples: 'high priority unfinished', 'what needs review'. Supports fuzzy matching and smart interpretation.",
	}, queryTasksHandler(taskStore))

	// ============ UPDATE & EXECUTE (3 tools) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "update-task",
		Description: "Update a task by id or reference. Updatable: title, description, acceptanceCriteria, status [todo|doing|review|done], priority, parentId, dependencies[].",
	}, updateTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "mark-done",
		Description: "Complete a task by id or reference (partial ID/title). Sets status=done and completedAt timestamp.",
	}, markDoneHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "set-current-task",
		Description: "🎯 SET FOCUS TASK (essential for context): Set active task id used for context-aware responses. Persists in project config.",
	}, setCurrentTaskHandler(taskStore))

	// ============ DELETE & CLEANUP (2 tools) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "delete-task",
		Description: "Delete a task by id or reference. Blocks if task has dependents or subtasks. Use 'bulk-tasks' or 'clear-tasks' for batch deletes.",
	}, deleteTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "clear-tasks",
		Description: "Bulk delete with safety. Args: status (csv), priority (csv), completed (bool), all (bool), force (bool), no_backup (bool). Default clears status=done when no filters.",
	}, clearTasksHandler(taskStore))

	// ============ BULK OPERATIONS (1 tool) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "bulk-tasks",
		Description: "Bulk operate on task_ids (IDs or references). Actions: complete, delete, prioritize (requires priority [low|medium|high|urgent]). Returns succeeded/failed and updated_task_ids.",
	}, bulkTaskHandler(taskStore))

	// Note: get-current-task is accessible via hooks, not needed as separate tool

	return nil
}
