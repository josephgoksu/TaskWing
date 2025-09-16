package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/types"
)

// UnifiedFakeProvider implements llm.Provider for all test scenarios
type UnifiedFakeProvider struct {
	// Configure behavior for different test scenarios
	GenerateTasksResult      []types.TaskOutput
	GenerateTasksError       error
	ImprovePRDResult         string
	ImprovePRDError          error
	EnhanceTaskResult        types.EnhancedTask
	EnhanceTaskError         error
	BreakdownTaskResult      []types.EnhancedTask
	BreakdownTaskError       error
	SuggestNextTaskResult    []types.TaskSuggestion
	SuggestNextTaskError     error
	DetectDependenciesResult []types.DependencySuggestion
	DetectDependenciesError  error
}

func (f *UnifiedFakeProvider) GenerateTasks(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.TaskOutput, error) {
	if f.GenerateTasksError != nil {
		return nil, f.GenerateTasksError
	}
	if len(f.GenerateTasksResult) > 0 {
		return f.GenerateTasksResult, nil
	}
	// Default result
	return []types.TaskOutput{{
		Title:              "Generated Task",
		Description:        "Generated Description",
		AcceptanceCriteria: "- completed",
		Priority:           "medium",
		TempID:             1,
	}}, nil
}

func (f *UnifiedFakeProvider) ImprovePRD(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForImprovement int, temperatureForImprovement float64) (string, error) {
	if f.ImprovePRDError != nil {
		return "", f.ImprovePRDError
	}
	if f.ImprovePRDResult != "" {
		return f.ImprovePRDResult, nil
	}
	return "IMPROVED:" + prdContent, nil
}

func (f *UnifiedFakeProvider) EnhanceTask(ctx context.Context, systemPrompt, taskInput, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) (types.EnhancedTask, error) {
	if f.EnhanceTaskError != nil {
		return types.EnhancedTask{}, f.EnhanceTaskError
	}
	if f.EnhanceTaskResult.Title != "" {
		// Return the configured result - including priority
		result := f.EnhanceTaskResult
		return result, nil
	}
	// Extract just the title from taskInput (which might contain title + description)
	lines := strings.Split(taskInput, "\n")
	title := lines[0]
	return types.EnhancedTask{
		Title:              "Enhanced " + title,
		Description:        "Enhanced description for " + title,
		AcceptanceCriteria: "- enhanced criteria",
		Priority:           "medium",
	}, nil
}

func (f *UnifiedFakeProvider) BreakdownTask(ctx context.Context, systemPrompt, taskTitle, taskDescription, acceptanceCriteria, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.EnhancedTask, error) {
	if f.BreakdownTaskError != nil {
		return nil, f.BreakdownTaskError
	}
	if len(f.BreakdownTaskResult) > 0 {
		return f.BreakdownTaskResult, nil
	}
	return []types.EnhancedTask{
		{Title: "Subtask 1", Description: "First subtask", AcceptanceCriteria: "- done", Priority: "high"},
		{Title: "Subtask 2", Description: "Second subtask", AcceptanceCriteria: "- completed", Priority: "medium"},
	}, nil
}

func (f *UnifiedFakeProvider) SuggestNextTask(ctx context.Context, systemPrompt, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.TaskSuggestion, error) {
	if f.SuggestNextTaskError != nil {
		return nil, f.SuggestNextTaskError
	}
	if len(f.SuggestNextTaskResult) > 0 {
		return f.SuggestNextTaskResult, nil
	}
	return []types.TaskSuggestion{
		{TaskID: "task-123", Reasoning: "High priority task", ConfidenceScore: 0.9, EstimatedEffort: "2 hours", ProjectPhase: "Development", RecommendedActions: []string{"Start coding", "Write tests"}},
	}, nil
}

func (f *UnifiedFakeProvider) DetectDependencies(ctx context.Context, systemPrompt, taskInfo, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.DependencySuggestion, error) {
	if f.DetectDependenciesError != nil {
		return nil, f.DetectDependenciesError
	}
	if len(f.DetectDependenciesResult) > 0 {
		return f.DetectDependenciesResult, nil
	}
	return []types.DependencySuggestion{
		{SourceTaskID: "task-123", TargetTaskID: "task-456", Reasoning: "API dependency", ConfidenceScore: 0.95, DependencyType: "technical"},
	}, nil
}

// UnifiedFakeProviderFactory creates a new UnifiedFakeProvider
func UnifiedFakeProviderFactory(_ *types.LLMConfig) (llm.Provider, error) {
	return &UnifiedFakeProvider{}, nil
}

// SetupTestProjectWithProvider configures a temporary project and sets up fake LLM provider
func SetupTestProjectWithProvider(t *testing.T, provider llm.Provider) string {
	t.Helper()

	// Setup temporary project directory
	root := t.TempDir()
	proj := filepath.Join(root, ".taskwing")
	if err := os.MkdirAll(filepath.Join(proj, "tasks"), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// Configure application
	cfg := GetConfig()
	cfg.Project.RootDir = proj
	cfg.Project.TemplatesDir = "templates"
	cfg.Project.TasksDir = "tasks"
	cfg.Data.File = "tasks.json"
	cfg.Data.Format = "json"
	cfg.LLM.Provider = "openai"
	cfg.LLM.ModelName = "test-model"
	cfg.LLM.APIKey = "dummy-key"
	cfg.LLM.MaxOutputTokens = 2048
	cfg.LLM.Temperature = 0.1
	cfg.LLM.ImprovementMaxOutputTokens = 1024
	cfg.LLM.ImprovementTemperature = 0.1

	// Set provider factory if provided
	if provider != nil {
		prev := newLLMProvider
		newLLMProvider = func(_ *types.LLMConfig) (llm.Provider, error) {
			return provider, nil
		}
		t.Cleanup(func() {
			newLLMProvider = prev
		})
	}

	return proj
}

// SetupTestProject configures a temporary project with default fake provider
func SetupTestProject(t *testing.T) string {
	t.Helper()
	return SetupTestProjectWithProvider(t, &UnifiedFakeProvider{})
}

// SeedTestTasks creates a set of test tasks for use in tests
func SeedTestTasks(t *testing.T, st interface {
	DeleteAllTasks() error
	CreateTask(task interface{}) (interface{}, error)
}) {
	t.Helper()

	if err := st.DeleteAllTasks(); err != nil {
		t.Fatalf("DeleteAllTasks error: %v", err)
	}

	// Import the models package types - this will need to be adjusted based on actual interface
	// For now, this is a placeholder that shows the pattern
	// The actual implementation would need to match the store interface
}

// AssertTaskExists verifies that a task exists in the store
func AssertTaskExists(t *testing.T, st interface {
	GetTask(id string) (interface{}, error)
}, taskID string) {
	t.Helper()

	_, err := st.GetTask(taskID)
	if err != nil {
		t.Fatalf("Expected task %s to exist, but got error: %v", taskID, err)
	}
}

// AssertTaskNotExists verifies that a task does not exist in the store
func AssertTaskNotExists(t *testing.T, st interface {
	GetTask(id string) (interface{}, error)
}, taskID string) {
	t.Helper()

	_, err := st.GetTask(taskID)
	if err == nil {
		t.Fatalf("Expected task %s to not exist, but it was found", taskID)
	}
}
