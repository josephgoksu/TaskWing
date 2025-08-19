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
	Status    string `json:"status,omitempty" mcp:"Filter by status: todo, doing, review, done (legacy: pending, in-progress, completed, cancelled, on-hold, blocked, needs-review)"`
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
	Status             string   `json:"status,omitempty" mcp:"New task status: todo, doing, review, done (legacy statuses also supported)"`
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

// Pattern suggestion types
type SuggestPatternParams struct {
	Description string `json:"description" mcp:"Description of work to find patterns for (required)"`
	ProjectType string `json:"projectType,omitempty" mcp:"Type of project (e.g., documentation, development, refactoring)"`
	Complexity  string `json:"complexity,omitempty" mcp:"Project complexity: simple, medium, complex"`
}

type PatternSuggestion struct {
	PatternID       string            `json:"pattern_id"`
	Name            string            `json:"name"`
	MatchScore      float64           `json:"match_score"`
	Category        string            `json:"category"`
	Description     string            `json:"description"`
	SuccessRate     float64           `json:"success_rate"`
	AverageDuration float64           `json:"average_duration_hours"`
	TaskBreakdown   []PatternPhase    `json:"task_breakdown"`
	SuccessFactors  []string          `json:"success_factors"`
	WhenToUse       []string          `json:"when_to_use"`
	AIGuidance      PatternAIGuidance `json:"ai_guidance"`
}

type PatternPhase struct {
	Phase                string   `json:"phase"`
	Tasks                []string `json:"tasks"`
	TypicalDurationHours float64  `json:"typical_duration_hours"`
	Priority             string   `json:"priority"`
}

type PatternAIGuidance struct {
	TaskGenerationHints []string          `json:"task_generation_hints"`
	PrioritySuggestions map[string]string `json:"priority_suggestions"`
	DependencyPatterns  []string          `json:"dependency_patterns"`
}

type SuggestPatternResponse struct {
	MatchingPatterns []PatternSuggestion    `json:"matching_patterns"`
	BestMatch        *PatternSuggestion     `json:"best_match,omitempty"`
	Suggestions      string                 `json:"suggestions"`
	LibraryStats     map[string]interface{} `json:"library_stats"`
}

// SetCurrentTaskParams for setting the current active task
type SetCurrentTaskParams struct {
	ID string `json:"id" mcp:"Task ID to set as current (required)"`
}

// GetCurrentTaskParams for retrieving the current active task
type GetCurrentTaskParams struct{}

// ClearCurrentTaskParams for clearing the current active task
type ClearCurrentTaskParams struct{}

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
	TempID             int      `json:"tempId,omitempty"`
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

// CurrentTaskResponse for current task operations
type CurrentTaskResponse struct {
	CurrentTask *TaskResponse `json:"current_task,omitempty"`
	Message     string        `json:"message"`
	Success     bool          `json:"success"`
}

// Task Resolution Tool Types

// FindTaskByTitleParams for fuzzy title matching
type FindTaskByTitleParams struct {
	Title string `json:"title" mcp:"Task title to search for (partial matches allowed)"`
	Limit int    `json:"limit,omitempty" mcp:"Maximum number of results to return (default: 5)"`
}

// ResolveTaskReferenceParams for smart task resolution
type ResolveTaskReferenceParams struct {
	Reference string `json:"reference" mcp:"Task reference - partial ID, title, or description"`
	Exact     bool   `json:"exact,omitempty" mcp:"Require exact match (default: false for fuzzy matching)"`
}

// TaskAutocompleteParams for predictive suggestions
type TaskAutocompleteParams struct {
	Input   string `json:"input" mcp:"Partial input to get suggestions for"`
	Context string `json:"context,omitempty" mcp:"Context for suggestions (current, related, etc.)"`
	Limit   int    `json:"limit,omitempty" mcp:"Maximum number of suggestions (default: 10)"`
}

// Task Resolution Response Types

// TaskMatch represents a task match with score
type TaskMatch struct {
	Task  TaskResponse `json:"task"`
	Score float64      `json:"score"`
	Type  string       `json:"match_type"` // "title", "id", "description"
}

// FindTaskByTitleResponse for fuzzy title search results
type FindTaskByTitleResponse struct {
	Matches []TaskMatch `json:"matches"`
	Query   string      `json:"query"`
	Count   int         `json:"count"`
}

// ResolveTaskReferenceResponse for task resolution results
type ResolveTaskReferenceResponse struct {
	Match     *TaskMatch  `json:"match,omitempty"`
	Matches   []TaskMatch `json:"matches,omitempty"`
	Reference string      `json:"reference"`
	Resolved  bool        `json:"resolved"`
	Message   string      `json:"message"`
}

// TaskAutocompleteResponse for suggestion results
type TaskAutocompleteResponse struct {
	Suggestions []TaskMatch `json:"suggestions"`
	Input       string      `json:"input"`
	Count       int         `json:"count"`
}

// JSON Processing Tool Types

// FilterTasksParams for advanced filtering with JSONPath expressions
type FilterTasksParams struct {
	Filter     string `json:"filter" mcp:"JSONPath-style filter expression (e.g., '$.status == \"pending\"')"`
	Expression string `json:"expression,omitempty" mcp:"Complex filter expression with AND/OR logic"`
	Fields     string `json:"fields,omitempty" mcp:"Comma-separated fields to return (default: all)"`
	Limit      int    `json:"limit,omitempty" mcp:"Maximum number of results (default: unlimited)"`
}

// ExtractTaskIDsParams for bulk ID extraction with criteria
type ExtractTaskIDsParams struct {
	Status   string `json:"status,omitempty" mcp:"Filter by status before extraction"`
	Priority string `json:"priority,omitempty" mcp:"Filter by priority before extraction"`
	Format   string `json:"format,omitempty" mcp:"Output format: array, space-separated, comma-separated (default: array)"`
	Search   string `json:"search,omitempty" mcp:"Search filter before extraction"`
}

// TaskAnalyticsParams for aggregation and statistics
type TaskAnalyticsParams struct {
	GroupBy   string `json:"group_by,omitempty" mcp:"Group by field: status, priority, created_date (default: status)"`
	DateRange string `json:"date_range,omitempty" mcp:"Date range: today, week, month, all (default: all)"`
	Metrics   string `json:"metrics,omitempty" mcp:"Comma-separated metrics: count, duration, completion_rate"`
	Format    string `json:"format,omitempty" mcp:"Output format: json, table, summary (default: json)"`
}

// JSON Processing Response Types

// FilterTasksResponse for filtered task results
type FilterTasksResponse struct {
	Tasks       []TaskResponse `json:"tasks"`
	Count       int            `json:"count"`
	Filter      string         `json:"filter_used"`
	Fields      []string       `json:"fields_returned,omitempty"`
	ExecutionMs int64          `json:"execution_time_ms"`
}

// ExtractTaskIDsResponse for bulk ID extraction results
type ExtractTaskIDsResponse struct {
	TaskIDs     []string `json:"task_ids"`
	Count       int      `json:"count"`
	Format      string   `json:"format"`
	Criteria    string   `json:"criteria_used"`
	ExecutionMs int64    `json:"execution_time_ms"`
}

// TaskAnalyticsResponse for aggregation results
type TaskAnalyticsResponse struct {
	Summary     string                 `json:"summary"`
	Metrics     map[string]interface{} `json:"metrics"`
	Groups      map[string]interface{} `json:"groups"`
	DateRange   string                 `json:"date_range"`
	ExecutionMs int64                  `json:"execution_time_ms"`
}

// Workflow Integration Tool Types

// SmartTaskTransitionParams for AI-powered next step suggestions
type SmartTaskTransitionParams struct {
	TaskID  string `json:"task_id,omitempty" mcp:"Task ID to analyze (uses current task if not provided)"`
	Context string `json:"context,omitempty" mcp:"Additional context for suggestions (e.g., 'completed', 'blocked', 'next')"`
	Limit   int    `json:"limit,omitempty" mcp:"Maximum number of suggestions (default: 5)"`
}

// WorkflowStatusParams for project lifecycle tracking
type WorkflowStatusParams struct {
	Depth  string `json:"depth,omitempty" mcp:"Detail level: summary, detailed, full (default: summary)"`
	Focus  string `json:"focus,omitempty" mcp:"Focus area: current, blockers, progress, timeline"`
	Format string `json:"format,omitempty" mcp:"Output format: text, json, visual (default: text)"`
}

// DependencyHealthParams for relationship validation
type DependencyHealthParams struct {
	TaskID      string `json:"task_id,omitempty" mcp:"Specific task to analyze (analyzes all if not provided)"`
	CheckType   string `json:"check_type,omitempty" mcp:"Type of check: circular, broken, orphaned, all (default: all)"`
	AutoFix     bool   `json:"auto_fix,omitempty" mcp:"Attempt to auto-fix issues where possible"`
	Suggestions bool   `json:"suggestions,omitempty" mcp:"Include resolution suggestions (default: true)"`
}

// Workflow Integration Response Types

// TaskTransition represents a suggested next step
type TaskTransition struct {
	Action       string            `json:"action"` // "create", "update", "complete", "start"
	TaskID       string            `json:"task_id,omitempty"`
	Title        string            `json:"title,omitempty"`
	Description  string            `json:"description"`
	Priority     string            `json:"priority,omitempty"`
	Confidence   float64           `json:"confidence"` // 0.0 to 1.0
	Reasoning    string            `json:"reasoning"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// SmartTaskTransitionResponse for AI-powered suggestions
type SmartTaskTransitionResponse struct {
	Suggestions     []TaskTransition `json:"suggestions"`
	CurrentTask     *TaskResponse    `json:"current_task,omitempty"`
	Context         string           `json:"context"`
	RecommendedNext *TaskTransition  `json:"recommended_next,omitempty"`
	Count           int              `json:"count"`
}

// ProjectPhase represents current project phase
type ProjectPhase struct {
	Phase       string   `json:"phase"`    // "planning", "development", "testing", "deployment", "maintenance"
	Progress    float64  `json:"progress"` // 0.0 to 1.0
	Description string   `json:"description"`
	Milestones  []string `json:"milestones,omitempty"`
	Blockers    []string `json:"blockers,omitempty"`
}

// WorkflowStatusResponse for project lifecycle information
type WorkflowStatusResponse struct {
	CurrentPhase    ProjectPhase           `json:"current_phase"`
	OverallProgress float64                `json:"overall_progress"`
	Timeline        map[string]interface{} `json:"timeline"`
	Bottlenecks     []string               `json:"bottlenecks"`
	Recommendations []string               `json:"recommendations"`
	Metrics         map[string]interface{} `json:"metrics"`
	Summary         string                 `json:"summary"`
}

// DependencyIssue represents a dependency problem
type DependencyIssue struct {
	Type          string   `json:"type"` // "circular", "broken", "orphaned", "missing"
	TaskID        string   `json:"task_id"`
	TaskTitle     string   `json:"task_title"`
	Description   string   `json:"description"`
	Severity      string   `json:"severity"` // "low", "medium", "high", "critical"
	AffectedTasks []string `json:"affected_tasks,omitempty"`
	Resolution    string   `json:"resolution,omitempty"`
	AutoFixable   bool     `json:"auto_fixable"`
}

// DependencyHealthResponse for relationship validation results
type DependencyHealthResponse struct {
	HealthScore   float64           `json:"health_score"` // 0.0 to 1.0
	Issues        []DependencyIssue `json:"issues"`
	FixedIssues   []DependencyIssue `json:"fixed_issues,omitempty"`
	Suggestions   []string          `json:"suggestions"`
	Summary       string            `json:"summary"`
	TasksAnalyzed int               `json:"tasks_analyzed"`
	IssuesFixed   int               `json:"issues_fixed,omitempty"`
}
