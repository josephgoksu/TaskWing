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

	// ============ CREATE (1 tool) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "add-task",
		Description: "🎯 CREATE PROFESSIONAL TASK (use instead of simple todos): title, description, acceptanceCriteria, priority [low|medium|high|urgent], parentId, dependencies[]. Validates and maintains relationships.",
	}, addTaskHandler(taskStore))

	// ============ READ & SEARCH (2 tools) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list-tasks",
		Description: "List tasks with filters. Args: status [todo|doing|review|done], priority [low|medium|high|urgent], search, parentId, sortBy [id|title|priority|createdAt|updatedAt], sortOrder [asc|desc]. Returns tasks+count.",
	}, listTasksHandler(taskStore))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get-task",
		Description: "Get one task by id or reference (partial ID/title). Returns full metadata, relationships, and timestamps.",
	}, getTaskHandler(taskStore))

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

	// ============ DELETE (1 tool) ============
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "delete-task",
		Description: "Delete a task by id or reference. Blocks if task has dependents or subtasks. Use 'bulk-tasks' or 'clear-tasks' for batch deletes.",
	}, deleteTaskHandler(taskStore))

	return nil
}
