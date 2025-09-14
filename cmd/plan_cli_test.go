package cmd

import (
	"testing"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/types"
)

func cliTestProviderFactory(_ *types.LLMConfig) (llm.Provider, error) {
	return &UnifiedFakeProvider{
		EnhanceTaskResult: types.EnhancedTask{Title: "Better Title", Description: "Refined", AcceptanceCriteria: "- ok", Priority: "medium"},
		BreakdownTaskResult: []types.EnhancedTask{
			{Title: "Step 1", Description: "A", AcceptanceCriteria: "- a", Priority: "high"},
			{Title: "Step 2", Description: "B", AcceptanceCriteria: "- b", Priority: "medium"},
		},
	}, nil
}

func TestPlanCLI_PreviewAndConfirm(t *testing.T) {
	prev := newLLMProvider
	newLLMProvider = cliTestProviderFactory
	defer func() { newLLMProvider = prev }()

	_ = SetupTestProject(t)

	st, err := GetStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer func() { _ = st.Close() }()
	parent, err := st.CreateTask(models.Task{Title: "Parent", Status: models.StatusTodo, Priority: models.PriorityMedium})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Preview
	planTaskID = parent.ID
	planCount = 2
	planConfirm = false
	if err := planCmd.RunE(planCmd, nil); err != nil {
		t.Fatalf("plan preview: %v", err)
	}
	// Confirm
	planConfirm = true
	if err := planCmd.RunE(planCmd, nil); err != nil {
		t.Fatalf("plan confirm: %v", err)
	}
	// Verify created
	subs, _ := st.ListTasks(func(t models.Task) bool { return t.ParentID != nil && *t.ParentID == parent.ID }, nil)
	if len(subs) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(subs))
	}
}

func TestIterateCLI_RefineAndSplit(t *testing.T) {
	prev := newLLMProvider
	newLLMProvider = cliTestProviderFactory
	defer func() { newLLMProvider = prev }()

	_ = SetupTestProject(t)
	st, _ := GetStore()
	defer func() { _ = st.Close() }()
	parent, _ := st.CreateTask(models.Task{Title: "Parent Task", Status: models.StatusTodo, Priority: models.PriorityMedium})
	pid := parent.ID
	step, _ := st.CreateTask(models.Task{Title: "Step", Description: "D", Status: models.StatusTodo, Priority: models.PriorityMedium, ParentID: &pid})

	// Refine
	iterTaskID = parent.ID
	iterStep = step.ID
	iterPrompt = "refine"
	iterSplit = false
	iterConfirm = true
	if err := iterateCmd.RunE(iterateCmd, nil); err != nil {
		t.Fatalf("iterate refine: %v", err)
	}
	updated, _ := st.GetTask(step.ID)
	// Title should be enhanced (either configured or default "Enhanced" + original)
	if updated.Title == step.Title {
		t.Fatalf("title should be enhanced, got same title: %s", updated.Title)
	}

	// Split
	step2, _ := st.CreateTask(models.Task{Title: "Step2", Description: "d2", Status: models.StatusTodo, Priority: models.PriorityMedium, ParentID: &pid})
	iterTaskID = parent.ID
	iterStep = step2.ID
	iterSplit = true
	iterPrompt = "split"
	iterConfirm = true
	if err := iterateCmd.RunE(iterateCmd, nil); err != nil {
		t.Fatalf("iterate split: %v", err)
	}
	// The original step2 should be gone
	if _, err := st.GetTask(step2.ID); err == nil {
		t.Fatalf("expected step2 to be deleted")
	}
}
