/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package types

// MCP Tool Parameter Types

// AddTaskParams for creating a new task
type AddTaskParams struct {
	Title              string   `json:"title" mcp:"Title"`
	Description        string   `json:"description,omitempty" mcp:"Description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty" mcp:"Acceptance criteria"`
	Priority           string   `json:"priority,omitempty" mcp:"low|medium|high|urgent"`
	ParentID           string   `json:"parentId,omitempty" mcp:"Parent task ID"`
	Dependencies       []string `json:"dependencies,omitempty" mcp:"Dependency task IDs"`
}

// ListTasksParams for listing and filtering tasks
type ListTasksParams struct {
	Status    string `json:"status,omitempty" mcp:"todo|doing|review|done"`
	Priority  string `json:"priority,omitempty" mcp:"low|medium|high|urgent"`
	Search    string `json:"search,omitempty" mcp:"Search text"`
	ParentID  string `json:"parentId,omitempty" mcp:"Parent task ID"`
	SortBy    string `json:"sortBy,omitempty" mcp:"Sort field"`
	SortOrder string `json:"sortOrder,omitempty" mcp:"asc|desc"`
}

// UpdateTaskParams for updating an existing task
type UpdateTaskParams struct {
	ID                 string   `json:"id" mcp:"Task ID"`
	Reference          string   `json:"reference,omitempty" mcp:"Partial ID/title"`
	Title              string   `json:"title,omitempty" mcp:"Title"`
	Description        string   `json:"description,omitempty" mcp:"Description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty" mcp:"Criteria"`
	Status             string   `json:"status,omitempty" mcp:"todo|doing|review|done"`
	Priority           string   `json:"priority,omitempty" mcp:"low|medium|high|urgent"`
	ParentID           string   `json:"parentId,omitempty" mcp:"Parent ID"`
	Dependencies       []string `json:"dependencies,omitempty" mcp:"Dependency IDs"`
}

// DeleteTaskParams for deleting a task
type DeleteTaskParams struct {
	ID        string `json:"id" mcp:"Task ID"`
	Reference string `json:"reference,omitempty" mcp:"Partial ID/title"`
}

// MarkDoneParams for marking a task as completed
type MarkDoneParams struct {
	ID        string `json:"id" mcp:"Task ID"`
	Reference string `json:"reference,omitempty" mcp:"Partial ID/title"`
}

// GetTaskParams for retrieving a specific task
type GetTaskParams struct {
	ID        string `json:"id" mcp:"Task ID"`
	Reference string `json:"reference,omitempty" mcp:"Partial ID/title"`
}

// SetCurrentTaskParams for setting the current active task
type SetCurrentTaskParams struct {
	ID        string `json:"id" mcp:"Task ID"`
	Reference string `json:"reference,omitempty" mcp:"Partial ID/title"`
}

// GetCurrentTaskParams for retrieving the current active task
type GetCurrentTaskParams struct{}

// ClearCurrentTaskParams for clearing the current active task
type ClearCurrentTaskParams struct{}

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

// CurrentTaskResponse for current task operations
type CurrentTaskResponse struct {
	CurrentTask *TaskResponse `json:"current_task,omitempty"`
	Message     string        `json:"message"`
	Success     bool          `json:"success"`
}
