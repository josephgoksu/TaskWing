package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
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

	// Mock TaskEnricher to avoid calling real RecallApp
	app.TaskEnricher = func(ctx context.Context, queries []string) (string, error) {
		return "## Mock Context\n- Test decision: Use mock pattern", nil
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
		Goal:             "build time machine",
		ClarifySessionID: clarifyRes.ClarifySessionID,
		EnrichedGoal:     clarifyRes.EnrichedGoal,
		Save:             true,
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

func TestPlanApp_ClarifySessionizedRoundsPersistTurns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	repo, err := memory.NewDefaultRepository(dbPath)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	appCtx := &Context{
		Repo:   repo,
		LLMCfg: llm.Config{},
	}
	planApp := NewPlanApp(appCtx)

	runCount := 0
	planApp.ClarifierFactory = func(cfg llm.Config) GoalsClarifier {
		return &MockClarifier{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				runCount++
				if runCount == 2 {
					history, _ := input.ExistingContext["history"].(string)
					if !strings.Contains(history, "Q1?") || !strings.Contains(history, "Use streaming") {
						t.Fatalf("continuation clarify call missing persisted Q/A context: %q", history)
					}
				}

				if runCount == 1 {
					return core.Output{
						Findings: []core.Finding{{
							Type: "clarification",
							Metadata: map[string]any{
								"is_ready_to_plan": false,
								"enriched_goal":    "Draft spec v1",
								"goal_summary":     "Draft summary",
								"questions":        []string{"Q1?"},
							},
						}},
					}, nil
				}

				return core.Output{
					Findings: []core.Finding{{
						Type: "clarification",
						Metadata: map[string]any{
							"is_ready_to_plan": true,
							"enriched_goal":    "Final enriched spec",
							"goal_summary":     "Final summary",
							"questions":        []string{},
						},
					}},
				}, nil
			},
		}
	}

	first, err := planApp.Clarify(context.Background(), ClarifyOptions{
		Goal: "Ship onboarding revamp",
	})
	if err != nil {
		t.Fatalf("first clarify failed: %v", err)
	}
	if first.ClarifySessionID == "" {
		t.Fatal("expected clarify_session_id on first clarify round")
	}
	if first.IsReadyToPlan {
		t.Fatal("expected first clarify round to be unresolved")
	}
	if first.RoundIndex != 1 {
		t.Fatalf("expected first round index 1, got %d", first.RoundIndex)
	}

	second, err := planApp.Clarify(context.Background(), ClarifyOptions{
		Goal:             "Ship onboarding revamp",
		ClarifySessionID: first.ClarifySessionID,
		Answers: []ClarifyAnswer{
			{Question: "Q1?", Answer: "Use streaming"},
		},
	})
	if err != nil {
		t.Fatalf("second clarify failed: %v", err)
	}
	if second.ClarifySessionID != first.ClarifySessionID {
		t.Fatalf("expected stable session id %q, got %q", first.ClarifySessionID, second.ClarifySessionID)
	}
	if !second.IsReadyToPlan {
		t.Fatal("expected second clarify round to be ready_to_plan")
	}
	if second.RoundIndex != 2 {
		t.Fatalf("expected second round index 2, got %d", second.RoundIndex)
	}

	session, err := repo.GetClarifySession(first.ClarifySessionID)
	if err != nil {
		t.Fatalf("load clarify session: %v", err)
	}
	if !session.IsReadyToPlan {
		t.Fatal("expected persisted session to be ready_to_plan")
	}
	if session.RoundIndex != 2 {
		t.Fatalf("expected persisted round index 2, got %d", session.RoundIndex)
	}

	turns, err := repo.ListClarifyTurns(first.ClarifySessionID)
	if err != nil {
		t.Fatalf("list clarify turns: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected 2 persisted turns, got %d", len(turns))
	}
	if len(turns[0].Questions) != 1 || turns[0].Questions[0] != "Q1?" {
		t.Fatalf("unexpected round 1 questions: %+v", turns[0].Questions)
	}
	if len(turns[1].Answers) != 1 || turns[1].Answers[0] != "Use streaming" {
		t.Fatalf("unexpected round 2 answers: %+v", turns[1].Answers)
	}
}

func TestPlanApp_GenerateBlockedUntilClarifyReady(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	repo, err := memory.NewDefaultRepository(dbPath)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	session := &task.ClarifySession{
		ID:                   "clarify-awaiting",
		Goal:                 "Improve planner",
		EnrichedGoal:         "Draft goal",
		State:                task.ClarifySessionStateAwaitingAnswers,
		RoundIndex:           1,
		MaxRounds:            5,
		MaxQuestionsPerRound: 3,
		CurrentQuestions:     []string{"Q1?"},
		IsReadyToPlan:        false,
	}
	if err := repo.CreateClarifySession(session); err != nil {
		t.Fatalf("create clarify session: %v", err)
	}

	planApp := NewPlanApp(&Context{
		Repo:   repo,
		LLMCfg: llm.Config{},
	})

	result, err := planApp.Generate(context.Background(), GenerateOptions{
		Goal:             "Improve planner",
		ClarifySessionID: session.ID,
		EnrichedGoal:     "Draft goal",
		Save:             false,
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected generate to be blocked while clarify is unresolved")
	}
	if !strings.Contains(strings.ToLower(result.Message), "clarification is not complete") {
		t.Fatalf("expected clarification gate message, got %q", result.Message)
	}
}
