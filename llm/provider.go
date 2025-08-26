package llm

import (
	"context"

	"github.com/josephgoksu/TaskWing/types"
)

// Provider defines the interface for interacting with different LLM providers
// to generate tasks from a document.
type Provider interface {
	// GenerateTasks takes a system prompt, the content of a document (e.g., PRD),
	// model parameters, and returns a list of TaskOutput objects or an error.
	GenerateTasks(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.TaskOutput, error)

	// EstimateTaskParameters takes a system prompt, the content of a document and returns an estimation
	// of task count and complexity. This is used to dynamically adjust parameters for GenerateTasks.
	EstimateTaskParameters(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForEstimation int, temperatureForEstimation float64) (types.EstimationOutput, error)

	// ImprovePRD takes a system prompt, the content of a PRD, sends it to an LLM for refinement,
	// and returns the improved document content as a string.
	ImprovePRD(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForImprovement int, temperatureForImprovement float64) (string, error)

	// EnhanceTask takes a system prompt, task input, and returns an enhanced task with improved details.
	EnhanceTask(ctx context.Context, systemPrompt, taskInput, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) (types.EnhancedTask, error)
}
