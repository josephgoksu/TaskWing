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

	// ImprovePRD takes a system prompt, the content of a PRD, sends it to an LLM for refinement,
	// and returns the improved document content as a string.
	ImprovePRD(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForImprovement int, temperatureForImprovement float64) (string, error)

	// EnhanceTask takes a system prompt, task input, and returns an enhanced task with improved details.
	EnhanceTask(ctx context.Context, systemPrompt, taskInput, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) (types.EnhancedTask, error)

	// BreakdownTask analyzes a task and suggests relevant subtasks
	BreakdownTask(ctx context.Context, systemPrompt, taskTitle, taskDescription, acceptanceCriteria, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.EnhancedTask, error)

	// SuggestNextTask provides context-aware suggestions for which task to work on next
	SuggestNextTask(ctx context.Context, systemPrompt, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.TaskSuggestion, error)

	// DetectDependencies analyzes tasks and suggests dependency relationships
	DetectDependencies(ctx context.Context, systemPrompt, taskInfo, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.DependencySuggestion, error)
}
