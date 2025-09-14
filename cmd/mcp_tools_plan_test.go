package cmd

import (
	"context"
	"testing"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Use unified fake provider with custom results for plan tests
func planTestProviderFactory(_ *types.LLMConfig) (llm.Provider, error) {
	return &UnifiedFakeProvider{
		GenerateTasksResult: []types.TaskOutput{{
			Title:              "Test Task",
			Description:        "Description",
			AcceptanceCriteria: "- done",
			Priority:           "medium",
			TempID:             1,
		}},
		ImprovePRDResult: "IMPROVED:content",
	}, nil
}

func TestPlanFromDocument_Preview_NoImprove(t *testing.T) {
	// Override provider factory
	prev := newLLMProvider
	newLLMProvider = planTestProviderFactory
	defer func() { newLLMProvider = prev }()

	_ = SetupTestProject(t)

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
	newLLMProvider = planTestProviderFactory
	defer func() { newLLMProvider = prev }()

	_ = SetupTestProject(t)

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
