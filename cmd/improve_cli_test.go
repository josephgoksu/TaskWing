package cmd

import (
	"testing"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/types"
)

// fake provider producing deterministic enhancement and 2 subtasks
func improveTestProviderFactory(_ *types.LLMConfig) (llm.Provider, error) {
	// Ensure we return the configured result with high priority
	return &UnifiedFakeProvider{
		EnhanceTaskResult: types.EnhancedTask{
			Title:              "Enhanced Old Title",
			Description:        "Enhanced description for Old Title",
			AcceptanceCriteria: "- enhanced criteria",
			Priority:           "high", // This should be returned
		},
		BreakdownTaskResult: []types.EnhancedTask{
			{Title: "Breakdown 1", Description: "A", AcceptanceCriteria: "- a", Priority: "high"},
			{Title: "Breakdown 2", Description: "B", AcceptanceCriteria: "- b", Priority: "medium"},
		},
	}, nil
}

func TestImproveCLI_ApplyAndPlan(t *testing.T) {
	_ = SetupTestProject(t)

	// Override the provider AFTER SetupTestProject
	prev := newLLMProvider
	newLLMProvider = improveTestProviderFactory
	defer func() { newLLMProvider = prev }()

	st, err := GetStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Create a base task
	base, err := st.CreateTask(models.Task{Title: "Old Title", Description: "D", Status: models.StatusTodo, Priority: models.PriorityMedium})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Apply improvements and plan
	improveTaskID = base.ID
	improveApply = true
	improvePlan = true
	if err := improveCmd.RunE(improveCmd, nil); err != nil {
		t.Fatalf("improve run: %v", err)
	}

	updated, err := st.GetTask(base.ID)
	if err != nil {
		t.Fatalf("get updated: %v", err)
	}
	if updated.Title == base.Title {
		t.Fatalf("expected title to change, got %s", updated.Title)
	}
	if updated.Priority != models.PriorityHigh {
		t.Fatalf("expected priority high, got %s", updated.Priority)
	}

	// Subtasks should be created
	subs, _ := st.ListTasks(func(t models.Task) bool { return t.ParentID != nil && *t.ParentID == base.ID }, nil)
	if len(subs) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(subs))
	}
}
