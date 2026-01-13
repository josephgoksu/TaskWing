package app

import (
	"context"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// MockClarifier
type MockClarifier struct {
	RunFunc        func(ctx context.Context, input core.Input) (core.Output, error)
	AutoAnswerFunc func(ctx context.Context, spec string, q []string, kg string) (string, error)
}

func (m *MockClarifier) Run(ctx context.Context, input core.Input) (core.Output, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, input)
	}
	return core.Output{}, nil
}
func (m *MockClarifier) AutoAnswer(ctx context.Context, spec string, q []string, kg string) (string, error) {
	if m.AutoAnswerFunc != nil {
		return m.AutoAnswerFunc(ctx, spec, q, kg)
	}
	return "", nil
}
func (m *MockClarifier) Close() error { return nil }

// MockPlanner
type MockPlanner struct {
	RunFunc func(ctx context.Context, input core.Input) (core.Output, error)
}

func (m *MockPlanner) Run(ctx context.Context, input core.Input) (core.Output, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, input)
	}
	return core.Output{}, nil
}
func (m *MockPlanner) Close() error { return nil }

func TestPlanApp_TUIFlow(t *testing.T) {
	// Invocation counters
	createPlanCalled := false
	setActivePlanCalled := false
	clarifierCalled := false
	plannerCalled := false

	// 1. Setup Mock Repo
	mockRepo := &MockRepository{
		CreatePlanFunc: func(p *task.Plan) error {
			createPlanCalled = true
			if p.Status != "active" {
				t.Errorf("expected plan status active, got %s", p.Status)
			}
			return nil
		},
		CreateTaskFunc: func(tsk *task.Task) error {
			if tsk.Title == "" {
				t.Error("created task has no title")
			}
			return nil
		},
		SetActivePlanFunc: func(id string) error {
			setActivePlanCalled = true
			return nil
		},
	}

	appCtx := &Context{
		Repo:   nil, // concrete repo not needed if we override
		LLMCfg: llm.Config{},
	}

	app := NewPlanApp(appCtx)
	// Inject dependencies
	app.Repo = mockRepo

	// Mock Context Retrieval
	app.ContextRetriever = func(ctx context.Context, ks *knowledge.Service, goal, memoryPath string) (impl.SearchStrategyResult, error) {
		return impl.SearchStrategyResult{
			Context:  "Mock Architecture Context",
			Strategy: "Mock Strategy",
		}, nil
	}

	// Mock Clarifier
	app.ClarifierFactory = func(cfg llm.Config) GoalsClarifier {
		return &MockClarifier{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				clarifierCalled = true
				return core.Output{
					Findings: []core.Finding{
						{
							Type: "clarification",
							Metadata: map[string]interface{}{
								"is_ready_to_plan": true,
								"enriched_goal":    "Build a flux capacitor",
								"goal_summary":     "Flux Capacitor",
								"questions":        []string{},
							},
						},
					},
				}, nil
			},
		}
	}

	// Mock Planner
	app.PlannerFactory = func(cfg llm.Config) TaskPlanner {
		return &MockPlanner{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				plannerCalled = true
				tasks := []impl.PlanningTask{
					{Title: "Task 1", Description: "Desc 1", Priority: 1, AssignedAgent: "engineer"},
					{Title: "Task 2", Description: "Desc 2", Priority: 2, AssignedAgent: "qa"},
				}
				return core.Output{
					Findings: []core.Finding{
						{
							Type: "plan",
							Metadata: map[string]interface{}{
								"tasks": tasks,
							},
						},
					},
				}, nil
			},
		}
	}

	// 2. Execute Clarify
	clarifyRes, err := app.Clarify(context.Background(), ClarifyOptions{Goal: "build time machine"})
	if err != nil {
		t.Fatalf("Clarify failed: %v", err)
	}
	if !clarifyRes.IsReadyToPlan {
		t.Error("Expected IsReadyToPlan to be true")
	}
	if clarifyRes.EnrichedGoal != "Build a flux capacitor" {
		t.Errorf("Expected enriched goal 'Build a flux capacitor', got '%s'", clarifyRes.EnrichedGoal)
	}

	// 3. Execute Generate
	genRes, err := app.Generate(context.Background(), GenerateOptions{
		Goal:         "build time machine",
		EnrichedGoal: clarifyRes.EnrichedGoal,
		Save:         true,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !genRes.Success {
		t.Errorf("Expected Generate success, got failure: %s", genRes.Message)
	}
	if len(genRes.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(genRes.Tasks))
	}

	// 4. Verify all mocks were called
	if !clarifierCalled {
		t.Error("ClarifierFactory agent was never called")
	}
	if !plannerCalled {
		t.Error("PlannerFactory agent was never called")
	}
	if !createPlanCalled {
		t.Error("CreatePlan was never called - plan was not saved")
	}
	if !setActivePlanCalled {
		t.Error("SetActivePlan was never called - plan was not activated")
	}
}
