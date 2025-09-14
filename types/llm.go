/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package types

// TaskOutput is the expected structure for tasks extracted by an LLM.
// This structure is designed to be easily convertible to models.Task.
type TaskOutput struct {
	Title              string       `json:"title"`
	Description        string       `json:"description"`
	AcceptanceCriteria string       `json:"acceptanceCriteria"`
	Priority           string       `json:"priority"` // e.g., "high", "medium", "low", "urgent"
	TempID             int          `json:"tempId"`   // A temporary, unique ID for this task within the generation context.
	Subtasks           []TaskOutput `json:"subtasks,omitempty"`
	DependsOnIDs       []int        `json:"dependsOnIds,omitempty"`    // List of TempIDs of other tasks it depends on.
	DependsOnTitles    []string     `json:"dependsOnTitles,omitempty"` // List of task titles it depends on (for iterative generation).
}

// GetAcceptanceCriteriaAsString returns the acceptance criteria as a string.
// Since the API now returns a string, this just returns the field directly.
func (t *TaskOutput) GetAcceptanceCriteriaAsString() string {
	return t.AcceptanceCriteria
}

// EnhancedTask holds the AI-enhanced task details for single task creation.
type EnhancedTask struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptanceCriteria"`
	Priority           string `json:"priority"` // e.g., "high", "medium", "low", "urgent"
}

// TaskSuggestion represents a context-aware task suggestion for the next command.
type TaskSuggestion struct {
	TaskID             string   `json:"taskId"`
	Reasoning          string   `json:"reasoning"`
	ConfidenceScore    float64  `json:"confidenceScore"` // 0.0 to 1.0
	EstimatedEffort    string   `json:"estimatedEffort"` // e.g., "30 minutes", "2 hours"
	ProjectPhase       string   `json:"projectPhase"`    // e.g., "Planning", "Development", "Testing"
	RecommendedActions []string `json:"recommendedActions"`
}

// DependencySuggestion represents a suggested dependency relationship between tasks.
type DependencySuggestion struct {
	SourceTaskID    string  `json:"sourceTaskId"`    // The task that depends on another
	TargetTaskID    string  `json:"targetTaskId"`    // The task that should be completed first
	Reasoning       string  `json:"reasoning"`       // Why this dependency makes sense
	ConfidenceScore float64 `json:"confidenceScore"` // 0.0 to 1.0
	DependencyType  string  `json:"dependencyType"`  // e.g., "technical", "logical", "sequential"
}
