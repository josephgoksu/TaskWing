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
	DependsOnIDs       []int        `json:"dependsOnIds,omitempty"` // List of TempIDs of other tasks it depends on.
}

// EstimationOutput holds the LLM's estimation of task parameters from a document.
type EstimationOutput struct {
	EstimatedTaskCount  int    `json:"estimatedTaskCount"`
	EstimatedComplexity string `json:"estimatedComplexity"` // e.g., "low", "medium", "high"
}
