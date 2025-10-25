package cmd

import (
	"context"
	"testing"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/types"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func simplePlanProviderFactory(_ *types.LLMConfig) (llm.Provider, error) {
	return &UnifiedFakeProvider{
		EnhanceTaskResult: types.EnhancedTask{Title: "Better Title", Description: "Improved desc", AcceptanceCriteria: "- ok", Priority: "medium"},
		BreakdownTaskResult: []types.EnhancedTask{
			{Title: "Step A", Description: "A", AcceptanceCriteria: "- a", Priority: "high"},
			{Title: "Step B", Description: "B", AcceptanceCriteria: "- b", Priority: "medium"},
		},
	}, nil
}

func TestGeneratePlanPreviewAndConfirm(t *testing.T) {
	prev := newLLMProvider
	newLLMProvider = simplePlanProviderFactory
	defer func() { newLLMProvider = prev }()

	_ = SetupTestProject(t)

	st, err := GetStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer func() { _ = st.Close() }()
	parent, err := st.CreateTask(models.Task{Title: "Parent", Description: "Do X", Status: models.StatusTodo, Priority: models.PriorityMedium})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	_ = mcpsdk.NewServer(&mcpsdk.Implementation{Name: "taskwing", Version: "test"}, &mcpsdk.ServerOptions{})
	// Preview
	h := generatePlanHandler(st)
	res, err := h(context.Background(), nil, &mcpsdk.CallToolParamsFor[types.GeneratePlanParams]{Arguments: types.GeneratePlanParams{TaskID: parent.ID, Count: 2}})
	if err != nil {
		t.Fatalf("preview err: %v", err)
	}
	if res.StructuredContent.ProposedCount != 2 || res.StructuredContent.Created != 0 {
		t.Fatalf("unexpected preview: %+v", res.StructuredContent)
	}
	// Confirm
	res2, err := h(context.Background(), nil, &mcpsdk.CallToolParamsFor[types.GeneratePlanParams]{Arguments: types.GeneratePlanParams{TaskID: parent.ID, Count: 2, Confirm: true}})
	if err != nil {
		t.Fatalf("confirm err: %v", err)
	}
	if res2.StructuredContent.Created != 2 {
		t.Fatalf("expected 2 created, got %+v", res2.StructuredContent)
	}
}

func TestIteratePlanStepRefine(t *testing.T) {
	prev := newLLMProvider
	newLLMProvider = simplePlanProviderFactory
	defer func() { newLLMProvider = prev }()

	_ = SetupTestProject(t)
	st, err := GetStore()
	if err != nil {
		t.Fatalf("GetStore: %v", err)
	}
	defer func() { _ = st.Close() }()
	parent, err := st.CreateTask(models.Task{Title: "Parent Task", Status: models.StatusTodo, Priority: models.PriorityMedium})
	if err != nil {
		t.Fatalf("CreateTask parent: %v", err)
	}
	pid := parent.ID
	step, err := st.CreateTask(models.Task{Title: "Step 1", Status: models.StatusTodo, Priority: models.PriorityMedium, ParentID: &pid})
	if err != nil {
		t.Fatalf("CreateTask step: %v", err)
	}

	_ = mcpsdk.NewServer(&mcpsdk.Implementation{Name: "taskwing", Version: "test"}, &mcpsdk.ServerOptions{})
	h := iteratePlanStepHandler(st)
	// Preview refine
	t.Logf("Parent ID: %s, Step ID: %s", parent.ID, step.ID)
	if _, err := h(context.Background(), nil, &mcpsdk.CallToolParamsFor[types.IteratePlanStepParams]{Arguments: types.IteratePlanStepParams{TaskID: parent.ID, StepID: step.ID, Prompt: "refine"}}); err != nil {
		t.Fatalf("preview refine: %v", err)
	}
	// Confirm refine
	if _, err := h(context.Background(), nil, &mcpsdk.CallToolParamsFor[types.IteratePlanStepParams]{Arguments: types.IteratePlanStepParams{TaskID: parent.ID, StepID: step.ID, Prompt: "refine", Confirm: true}}); err != nil {
		t.Fatalf("confirm refine: %v", err)
	}
}
