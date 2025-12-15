package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
)

// UnifiedFakeProvider implements llm.Provider for MCP-specific tests.
type UnifiedFakeProvider struct {
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
		return f.EnhanceTaskResult, nil
	}

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

// testConfigMu guards access to the shared test configuration.
var testConfigMu sync.RWMutex

// activeTestConfig stores the configuration for the most recent test project.
var activeTestConfig *types.AppConfig

// SetupTestProject provisions an isolated TaskWing project structure for MCP tests.
func SetupTestProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	projectRoot := filepath.Join(root, ".taskwing")
	tasksDir := filepath.Join(projectRoot, "tasks")
	templatesDir := filepath.Join(projectRoot, "templates")

	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates dir: %v", err)
	}

	cfg := &types.AppConfig{}
	cfg.Project.RootDir = projectRoot
	cfg.Project.TemplatesDir = "templates"
	cfg.Project.TasksDir = "tasks"
	cfg.Project.OutputLogPath = filepath.Join(projectRoot, "taskwing.log")

	cfg.Data.File = "tasks.json"
	cfg.Data.Format = "json"

	cfg.LLM.Provider = "openai"
	cfg.LLM.ModelName = "test-model"
	cfg.LLM.APIKey = "dummy-key"
	cfg.LLM.MaxOutputTokens = 2048
	cfg.LLM.Temperature = 0.1
	cfg.LLM.ImprovementMaxOutputTokens = 1024
	cfg.LLM.ImprovementTemperature = 0.1

	testConfigMu.Lock()
	activeTestConfig = cfg
	testConfigMu.Unlock()

	ConfigureHooks(Hooks{
		GetConfig: func() *types.AppConfig {
			testConfigMu.RLock()
			defer testConfigMu.RUnlock()
			return activeTestConfig
		},
		CreateLLMProvider: func(cfg *types.LLMConfig) (llm.Provider, error) {
			return &UnifiedFakeProvider{}, nil
		},
		EnvPrefix: "TASKWING_TEST",
	})

	return projectRoot
}

// GetTaskFilePath mirrors the CLI helper for tests without importing cmd.
func GetTaskFilePath() string {
	testConfigMu.RLock()
	defer testConfigMu.RUnlock()

	if activeTestConfig == nil {
		return ""
	}

	return filepath.Join(
		activeTestConfig.Project.RootDir,
		activeTestConfig.Project.TasksDir,
		activeTestConfig.Data.File,
	)
}

// GetStore initializes a file-backed task store for MCP tests.
func GetStore() (store.TaskStore, error) {
	testConfigMu.RLock()
	cfg := activeTestConfig
	testConfigMu.RUnlock()

	if cfg == nil {
		return nil, fmt.Errorf("test configuration not initialized; call SetupTestProject first")
	}

	taskStore := store.NewFileTaskStore()
	if err := taskStore.Initialize(map[string]string{
		"dataFile":       filepath.Join(cfg.Project.RootDir, cfg.Project.TasksDir, cfg.Data.File),
		"dataFileFormat": cfg.Data.Format,
	}); err != nil {
		return nil, fmt.Errorf("initialize test store: %w", err)
	}

	return taskStore, nil
}
