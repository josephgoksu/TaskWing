package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/josephgoksu/TaskWing/store"
)

// RegisterCoreTools registers the base MCP tools (summary, CRUD, current task, clear).
func RegisterCoreTools(server *mcpsdk.Server, taskStore store.TaskStore) error {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task-summary",
		Description: "🎯 ALWAYS CALL FIRST: Get project overview with total tasks, active, completed today, and project health. Essential for understanding context before any operations.",
	}, taskSummaryHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "add-task",
		Description: "🎯 CREATE PROFESSIONAL TASK (use instead of simple todos): title, description, acceptanceCriteria, priority [low|medium|high|urgent], parentId, dependencies[]. Validates and maintains relationships.",
	}, addTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list-tasks",
		Description: "List tasks with filters. Args: status [todo|doing|review|done], priority [low|medium|high|urgent], search, parentId, sortBy [id|title|priority|createdAt|updatedAt], sortOrder [asc|desc]. Returns tasks+count.",
	}, listTasksHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "update-task",
		Description: "Update a task by id or reference. Updatable: title, description, acceptanceCriteria, status [todo|doing|review|done], priority, parentId, dependencies[].",
	}, updateTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "delete-task",
		Description: "Delete a task by id or reference. Blocks if task has dependents or subtasks. Use 'bulk-tasks' or 'clear-tasks' for batch deletes.",
	}, deleteTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "mark-done",
		Description: "Complete a task by id or reference (partial ID/title). Sets status=done and completedAt timestamp.",
	}, markDoneHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get-task",
		Description: "Get one task by id or reference (partial ID/title). Returns full metadata, relationships, and timestamps.",
	}, getTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "set-current-task",
		Description: "🎯 SET FOCUS TASK (essential for context): Set active task id used for context-aware responses. Persists in project config.",
	}, setCurrentTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get-current-task",
		Description: "Return current active task (if set) with full details for context.",
	}, getCurrentTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "clear-current-task",
		Description: "Clear the active task reference from project config.",
	}, clearCurrentTaskHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "clear-tasks",
		Description: "Bulk delete with safety. Args: status (csv), priority (csv), completed (bool), all (bool), force (bool), no_backup (bool). Default clears status=done when no filters.",
	}, clearTasksHandler(taskStore))

	return nil
}
