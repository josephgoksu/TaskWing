package llm

import "context"

// LLMConfig holds the specific configuration fields needed by the LLM package.
// This helps avoid circular dependencies with the cmd package.
type LLMConfig struct {
	Provider                   string
	ModelName                  string
	APIKey                     string // Resolved API key
	ProjectID                  string // For Google Cloud
	MaxOutputTokens            int
	Temperature                float64
	EstimationTemperature      float64 // Temperature for the estimation call
	EstimationMaxOutputTokens  int     // Max output tokens for the estimation call
	ImprovementTemperature     float64 // Temperature for the PRD improvement call
	ImprovementMaxOutputTokens int     // Max output tokens for the PRD improvement call
}

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

// Provider defines the interface for interacting with different LLM providers
// to generate tasks from a document.
type Provider interface {
	// GenerateTasks takes a system prompt, the content of a document (e.g., PRD),
	// model parameters, and returns a list of TaskOutput objects or an error.
	GenerateTasks(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]TaskOutput, error)

	// EstimateTaskParameters takes a system prompt, the content of a document and returns an estimation
	// of task count and complexity. This is used to dynamically adjust parameters for GenerateTasks.
	EstimateTaskParameters(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForEstimation int, temperatureForEstimation float64) (EstimationOutput, error)

	// ImprovePRD takes a system prompt, the content of a PRD, sends it to an LLM for refinement,
	// and returns the improved document content as a string.
	ImprovePRD(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForImprovement int, temperatureForImprovement float64) (string, error)
}
