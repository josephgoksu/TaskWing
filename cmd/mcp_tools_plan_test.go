package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeProvider implements llm.Provider for tests
type fakeProvider struct{}

func (f *fakeProvider) GenerateTasks(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.TaskOutput, error) {
	return []types.TaskOutput{{
		Title:              "Test Task",
		Description:        "Description",
		AcceptanceCriteria: "- done",
		Priority:           "medium",
		TempID:             1,
	}}, nil
}

func (f *fakeProvider) ImprovePRD(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForImprovement int, temperatureForImprovement float64) (string, error) {
	return "IMPROVED:" + prdContent, nil
}

func (f *fakeProvider) EnhanceTask(ctx context.Context, systemPrompt, taskInput, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) (types.EnhancedTask, error) {
	return types.EnhancedTask{Title: taskInput, Description: taskInput, AcceptanceCriteria: "- ok", Priority: "medium"}, nil
}

func fakeProviderFactory(_ *types.LLMConfig) (llm.Provider, error) { return &fakeProvider{}, nil }

// setupTestProject configures a temporary project-scoped .taskwing directory and app config
func setupTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	proj := filepath.Join(root, ".taskwing")
	if err := os.MkdirAll(filepath.Join(proj, "tasks"), 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	cfg := GetConfig()
	cfg.Project.RootDir = proj
	cfg.Project.TemplatesDir = "templates"
	cfg.Project.TasksDir = "tasks"
	cfg.Data.File = "tasks.json"
	cfg.Data.Format = "json"
	cfg.LLM.Provider = "openai"
	cfg.LLM.ModelName = "test-model"
	cfg.LLM.APIKey = "dummy"
	cfg.LLM.MaxOutputTokens = 0
	cfg.LLM.Temperature = 0.1
	cfg.LLM.ImprovementMaxOutputTokens = 0
	cfg.LLM.ImprovementTemperature = 0.1
	return proj
}

func TestPlanFromDocument_Preview_NoImprove(t *testing.T) {
	// Override provider factory
	prev := newLLMProvider
	newLLMProvider = fakeProviderFactory
	defer func() { newLLMProvider = prev }()

	_ = setupTestProject(t)

	// Build server and handler
	_ = mcp.NewServer(&mcp.Implementation{Name: "taskwing", Version: "test"}, &mcp.ServerOptions{})
	handler := planFromDocumentHandler(nil) // store not used for preview

	// Call tool with content and skip_improve=true, confirm=false
	params := &mcp.CallToolParamsFor[types.PlanFromDocumentParams]{
		Arguments: types.PlanFromDocumentParams{
			Content:     "PRD content",
			SkipImprove: true,
			Confirm:     false,
		},
	}
	res, err := handler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	// Check structured content
	if res.StructuredContent.ProposedCount == 0 || !res.StructuredContent.Preview {
		t.Fatalf("unexpected response: %+v", res.StructuredContent)
	}
}

func TestPlanFromDocument_Confirm_CreatesTasks(t *testing.T) {
	prev := newLLMProvider
	newLLMProvider = fakeProviderFactory
	defer func() { newLLMProvider = prev }()

	_ = setupTestProject(t)

	_ = mcp.NewServer(&mcp.Implementation{Name: "taskwing", Version: "test"}, &mcp.ServerOptions{})
	handler := planFromDocumentHandler(nil)

	params := &mcp.CallToolParamsFor[types.PlanFromDocumentParams]{
		Arguments: types.PlanFromDocumentParams{
			Content:     "PRD content",
			SkipImprove: true,
			Confirm:     true,
		},
	}
	res, err := handler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res.StructuredContent.Created == 0 || res.StructuredContent.Preview {
		t.Fatalf("expected tasks to be created, got %+v", res.StructuredContent)
	}
}
