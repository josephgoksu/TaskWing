/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package types

// MCP Tool Parameter Types

// AddTaskParams for creating a new task
type AddTaskParams struct {
	Title              string   `json:"title" mcp:"Task title (required)"`
	Description        string   `json:"description,omitempty" mcp:"Task description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty" mcp:"Acceptance criteria for task completion"`
	Priority           string   `json:"priority,omitempty" mcp:"Task priority: low, medium, high, urgent"`
	ParentID           string   `json:"parentId,omitempty" mcp:"Parent task ID to create this as a subtask"`
	Dependencies       []string `json:"dependencies,omitempty" mcp:"List of task IDs this task depends on"`
}

// ListTasksParams for listing and filtering tasks
type ListTasksParams struct {
	Status    string `json:"status,omitempty" mcp:"Filter by status: pending, in-progress, completed, cancelled, on-hold, blocked, needs-review"`
	Priority  string `json:"priority,omitempty" mcp:"Filter by priority: low, medium, high, urgent"`
	Search    string `json:"search,omitempty" mcp:"Search in title and description"`
	ParentID  string `json:"parentId,omitempty" mcp:"Filter by parent task ID"`
	SortBy    string `json:"sortBy,omitempty" mcp:"Sort by: id, title, priority, createdAt, updatedAt"`
	SortOrder string `json:"sortOrder,omitempty" mcp:"Sort order: asc, desc"`
}

// UpdateTaskParams for updating an existing task
type UpdateTaskParams struct {
	ID                 string   `json:"id" mcp:"Task ID to update (required)"`
	Title              string   `json:"title,omitempty" mcp:"New task title"`
	Description        string   `json:"description,omitempty" mcp:"New task description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty" mcp:"New acceptance criteria"`
	Status             string   `json:"status,omitempty" mcp:"New task status"`
	Priority           string   `json:"priority,omitempty" mcp:"New task priority"`
	ParentID           string   `json:"parentId,omitempty" mcp:"New parent task ID"`
	Dependencies       []string `json:"dependencies,omitempty" mcp:"New dependencies list"`
}

// DeleteTaskParams for deleting a task
type DeleteTaskParams struct {
	ID string `json:"id" mcp:"Task ID to delete (required)"`
}

// MarkDoneParams for marking a task as completed
type MarkDoneParams struct {
	ID string `json:"id" mcp:"Task ID to mark as done (required)"`
}

// GetTaskParams for retrieving a specific task
type GetTaskParams struct {
	ID string `json:"id" mcp:"Task ID to retrieve (required)"`
}

// BulkTaskParams for bulk operations
type BulkTaskParams struct {
	TaskIDs  []string `json:"task_ids" mcp:"List of task IDs to operate on"`
	Action   string   `json:"action" mcp:"Action to perform: complete, cancel, delete, prioritize"`
	Priority string   `json:"priority,omitempty" mcp:"New priority for prioritize action"`
}

// TaskSearchParams for advanced search
type TaskSearchParams struct {
	Query       string   `json:"query" mcp:"Search query supporting AND, OR, NOT operators"`
	Tags        []string `json:"tags,omitempty" mcp:"Filter by tags"`
	DateFrom    string   `json:"date_from,omitempty" mcp:"Filter tasks created after this date (YYYY-MM-DD)"`
	DateTo      string   `json:"date_to,omitempty" mcp:"Filter tasks created before this date (YYYY-MM-DD)"`
	HasSubtasks *bool    `json:"has_subtasks,omitempty" mcp:"Filter tasks that have subtasks"`
}

// TaskCreationRequest for batch task creation
type TaskCreationRequest struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty"`
	Priority           string   `json:"priority,omitempty"`
	ParentID           string   `json:"parentId,omitempty"`
	Dependencies       []string `json:"dependencies,omitempty"`
}

// BatchCreateTasksParams for creating multiple tasks at once
type BatchCreateTasksParams struct {
	Tasks []TaskCreationRequest `json:"tasks" mcp:"List of tasks to create"`
}

// MCP Response Types

// TaskResponse represents a task in MCP responses
type TaskResponse struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria"`
	Status             string   `json:"status"`
	Priority           string   `json:"priority"`
	ParentID           *string  `json:"parentId,omitempty"`
	SubtaskIDs         []string `json:"subtaskIds,omitempty"`
	Dependencies       []string `json:"dependencies"`
	Dependents         []string `json:"dependents"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
	CompletedAt        *string  `json:"completedAt"`
}

// TaskListResponse for list operations
type TaskListResponse struct {
	Tasks []TaskResponse `json:"tasks"`
	Count int            `json:"count"`
}

// DeleteTaskResponse for delete operations
type DeleteTaskResponse struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

// BatchCreateTasksResponse for batch task creation
type BatchCreateTasksResponse struct {
	CreatedTasks []TaskResponse `json:"created_tasks"`
	Failed       []string       `json:"failed,omitempty"`
	Success      int            `json:"success_count"`
	Errors       []string       `json:"errors,omitempty"`
}

// TaskSummaryResponse provides a high-level summary
type TaskSummaryResponse struct {
	Summary        string       `json:"summary"`
	TotalTasks     int          `json:"total_tasks"`
	ActiveTasks    int          `json:"active_tasks"`
	CompletedToday int          `json:"completed_today"`
	DueToday       int          `json:"due_today"`
	Blocked        int          `json:"blocked"`
	Context        *TaskContext `json:"context"`
}

// BulkOperationResponse for bulk operations
type BulkOperationResponse struct {
	Succeeded    int      `json:"succeeded"`
	Failed       int      `json:"failed"`
	Errors       []string `json:"errors,omitempty"`
	UpdatedTasks []string `json:"updated_task_ids"`
}