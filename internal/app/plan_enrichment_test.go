package app

import (
	"context"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// TestPlanEnrichment_ContextSummaryPopulated verifies that tasks have ContextSummary
// populated when TaskEnricher is configured and queries are generated.
func TestPlanEnrichment_ContextSummaryPopulated(t *testing.T) {
	// Track which queries were executed
	queriesExecuted := []string{}

	// Mock repo that captures created tasks
	createdTasks := []*task.Task{}
	mockRepo := &MockRepository{
		CreatePlanFunc: func(p *task.Plan) error {
			return nil
		},
		CreateTaskFunc: func(tsk *task.Task) error {
			createdTasks = append(createdTasks, tsk)
			return nil
		},
		SetActivePlanFunc: func(id string) error {
			return nil
		},
	}

	appCtx := &Context{
		Repo:   nil,
		LLMCfg: llm.Config{},
	}

	app := NewPlanApp(appCtx)
	app.Repo = mockRepo

	// Mock context retriever
	app.ContextRetriever = func(ctx context.Context, ks *knowledge.Service, goal, memoryPath string) (impl.SearchStrategyResult, error) {
		return impl.SearchStrategyResult{
			Context:  "Mock Architecture Context",
			Strategy: "Mock Strategy",
		}, nil
	}

	// Mock TaskEnricher that tracks executed queries and returns context
	app.TaskEnricher = func(ctx context.Context, queries []string) (string, error) {
		queriesExecuted = append(queriesExecuted, queries...)
		if len(queries) == 0 {
			return "", nil
		}
		// Return enriched context based on queries
		return "## Relevant Architecture Context\n- **Test Pattern** (pattern): Use dependency injection for testability\n- **SQLite Constraint** (constraint): SQLite is the single source of truth", nil
	}

	// Mock clarifier that returns ready-to-plan
	app.ClarifierFactory = func(cfg llm.Config) GoalsClarifier {
		return &MockClarifier{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{
					Findings: []core.Finding{
						{
							Type: "clarification",
							Metadata: map[string]interface{}{
								"is_ready_to_plan": true,
								"enriched_goal":    "Implement user authentication with JWT tokens",
								"goal_summary":     "Auth Implementation",
								"questions":        []string{},
							},
						},
					},
				}, nil
			},
		}
	}

	// Mock planner that returns tasks with keywords (which will generate SuggestedRecallQueries)
	app.PlannerFactory = func(cfg llm.Config) TaskPlanner {
		return &MockPlanner{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				tasks := []impl.PlanningTask{
					{
						Title:       "Design authentication schema",
						Description: "Create database schema for user auth",
						Priority:    100,
						Keywords:    []string{"auth", "database", "schema"},
						Scope:       "api",
					},
					{
						Title:       "Implement JWT middleware",
						Description: "Create middleware for JWT validation",
						Priority:    90,
						Keywords:    []string{"jwt", "middleware", "security"},
						Scope:       "api",
					},
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

	// Execute clarify
	clarifyRes, err := app.Clarify(context.Background(), ClarifyOptions{Goal: "implement user auth"})
	if err != nil {
		t.Fatalf("Clarify failed: %v", err)
	}
	if !clarifyRes.IsReadyToPlan {
		t.Fatal("Expected IsReadyToPlan to be true")
	}

	// Execute generate with Save=true
	genRes, err := app.Generate(context.Background(), GenerateOptions{
		Goal:             "implement user auth",
		ClarifySessionID: clarifyRes.ClarifySessionID,
		EnrichedGoal:     clarifyRes.EnrichedGoal,
		Save:             true,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Assertions
	if !genRes.Success {
		t.Errorf("Expected Generate success, got failure: %s", genRes.Message)
	}
	if len(genRes.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(genRes.Tasks))
	}

	// Verify TaskEnricher was called (queries were executed)
	if len(queriesExecuted) == 0 {
		t.Error("TaskEnricher was never called - no recall queries were executed")
	}

	// Verify ContextSummary is populated on tasks
	for i, tsk := range genRes.Tasks {
		if tsk.ContextSummary == "" {
			t.Errorf("Task %d (%s) has empty ContextSummary", i, tsk.Title)
		}
		if !strings.Contains(tsk.ContextSummary, "Relevant Architecture Context") {
			t.Errorf("Task %d ContextSummary doesn't contain expected content: %s", i, tsk.ContextSummary)
		}
	}
}

// TestPlanEnrichment_MultipleQueriesAggregated verifies that multiple recall queries
// from different tasks are properly executed.
func TestPlanEnrichment_MultipleQueriesAggregated(t *testing.T) {
	// Track queries per task
	taskQueryCounts := make(map[int]int)
	callCount := 0

	mockRepo := &MockRepository{
		CreatePlanFunc:    func(p *task.Plan) error { return nil },
		CreateTaskFunc:    func(tsk *task.Task) error { return nil },
		SetActivePlanFunc: func(id string) error { return nil },
	}

	appCtx := &Context{
		Repo:   nil,
		LLMCfg: llm.Config{},
	}

	app := NewPlanApp(appCtx)
	app.Repo = mockRepo

	app.ContextRetriever = func(ctx context.Context, ks *knowledge.Service, goal, memoryPath string) (impl.SearchStrategyResult, error) {
		return impl.SearchStrategyResult{Context: "ctx", Strategy: "s"}, nil
	}

	// Track each call to TaskEnricher
	app.TaskEnricher = func(ctx context.Context, queries []string) (string, error) {
		taskQueryCounts[callCount] = len(queries)
		callCount++
		return "## Context\n- Mock result", nil
	}

	app.ClarifierFactory = func(cfg llm.Config) GoalsClarifier {
		return &MockClarifier{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{
					Findings: []core.Finding{{
						Type: "clarification",
						Metadata: map[string]interface{}{
							"is_ready_to_plan": true,
							"enriched_goal":    "Test goal",
							"goal_summary":     "Test",
							"questions":        []string{},
						},
					}},
				}, nil
			},
		}
	}

	// Tasks with varying numbers of keywords (which generate queries)
	app.PlannerFactory = func(cfg llm.Config) TaskPlanner {
		return &MockPlanner{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				tasks := []impl.PlanningTask{
					{Title: "Task 1", Description: "Task 1 desc", Priority: 100, Keywords: []string{"kw1", "kw2", "kw3"}, Scope: "api"},
					{Title: "Task 2", Description: "Task 2 desc", Priority: 90, Keywords: []string{"kw4"}, Scope: "cli"},
					{Title: "Task 3", Description: "Task 3 desc", Priority: 80, Keywords: []string{}, Scope: "test"}, // No keywords
				}
				return core.Output{
					Findings: []core.Finding{{
						Type:     "plan",
						Metadata: map[string]interface{}{"tasks": tasks},
					}},
				}, nil
			},
		}
	}

	clarifyRes, _ := app.Clarify(context.Background(), ClarifyOptions{Goal: "test"})
	_, err := app.Generate(context.Background(), GenerateOptions{
		Goal:             "test",
		ClarifySessionID: clarifyRes.ClarifySessionID,
		EnrichedGoal:     clarifyRes.EnrichedGoal,
		Save:             true,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify TaskEnricher was called for tasks with queries
	// Task 1 and Task 2 have keywords, Task 3 does not
	if callCount < 2 {
		t.Errorf("Expected TaskEnricher to be called at least 2 times, got %d", callCount)
	}
}

// TestPlanEnrichment_NilEnricherSkipsEnrichment verifies backward compatibility
// when TaskEnricher is nil (legacy behavior).
func TestPlanEnrichment_NilEnricherSkipsEnrichment(t *testing.T) {
	createdTasks := []*task.Task{}
	mockRepo := &MockRepository{
		CreatePlanFunc: func(p *task.Plan) error { return nil },
		CreateTaskFunc: func(tsk *task.Task) error {
			createdTasks = append(createdTasks, tsk)
			return nil
		},
		SetActivePlanFunc: func(id string) error { return nil },
	}

	appCtx := &Context{
		Repo:   nil,
		LLMCfg: llm.Config{},
	}

	app := NewPlanApp(appCtx)
	app.Repo = mockRepo
	// Explicitly set TaskEnricher to nil (simulating legacy behavior)
	app.TaskEnricher = nil

	app.ContextRetriever = func(ctx context.Context, ks *knowledge.Service, goal, memoryPath string) (impl.SearchStrategyResult, error) {
		return impl.SearchStrategyResult{Context: "ctx", Strategy: "s"}, nil
	}

	app.ClarifierFactory = func(cfg llm.Config) GoalsClarifier {
		return &MockClarifier{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{
					Findings: []core.Finding{{
						Type: "clarification",
						Metadata: map[string]interface{}{
							"is_ready_to_plan": true,
							"enriched_goal":    "Test",
							"goal_summary":     "Test",
							"questions":        []string{},
						},
					}},
				}, nil
			},
		}
	}

	app.PlannerFactory = func(cfg llm.Config) TaskPlanner {
		return &MockPlanner{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				tasks := []impl.PlanningTask{
					{Title: "Task 1", Description: "Task 1 description", Priority: 100, Keywords: []string{"kw1", "kw2"}, Scope: "api"},
				}
				return core.Output{
					Findings: []core.Finding{{
						Type:     "plan",
						Metadata: map[string]interface{}{"tasks": tasks},
					}},
				}, nil
			},
		}
	}

	clarifyRes, _ := app.Clarify(context.Background(), ClarifyOptions{Goal: "test"})
	genRes, err := app.Generate(context.Background(), GenerateOptions{
		Goal:             "test",
		ClarifySessionID: clarifyRes.ClarifySessionID,
		EnrichedGoal:     clarifyRes.EnrichedGoal,
		Save:             true,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Should still generate tasks successfully
	if !genRes.Success {
		t.Fatalf("Expected success even with nil TaskEnricher, got failure: %s", genRes.Message)
	}
	if len(genRes.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(genRes.Tasks))
	}

	// ContextSummary should be empty (legacy behavior)
	if genRes.Tasks[0].ContextSummary != "" {
		t.Errorf("Expected empty ContextSummary with nil TaskEnricher, got: %s", genRes.Tasks[0].ContextSummary)
	}

	// But SuggestedRecallQueries should still be generated (by EnrichAIFields)
	if len(genRes.Tasks[0].SuggestedRecallQueries) == 0 {
		t.Error("Expected SuggestedRecallQueries to be generated even without TaskEnricher")
	}
}

// TestPlanEnrichment_ContentLengthHandling verifies that very long context
// summaries are handled correctly (not testing truncation here as that's
// done in presentation layer, but ensuring no errors with long content).
func TestPlanEnrichment_ContentLengthHandling(t *testing.T) {
	mockRepo := &MockRepository{
		CreatePlanFunc:    func(p *task.Plan) error { return nil },
		CreateTaskFunc:    func(tsk *task.Task) error { return nil },
		SetActivePlanFunc: func(id string) error { return nil },
	}

	appCtx := &Context{
		Repo:   nil,
		LLMCfg: llm.Config{},
	}

	app := NewPlanApp(appCtx)
	app.Repo = mockRepo

	app.ContextRetriever = func(ctx context.Context, ks *knowledge.Service, goal, memoryPath string) (impl.SearchStrategyResult, error) {
		return impl.SearchStrategyResult{Context: "ctx", Strategy: "s"}, nil
	}

	// Return very long context
	longContent := strings.Repeat("This is a very long piece of content that tests our handling of large context summaries. ", 50)
	app.TaskEnricher = func(ctx context.Context, queries []string) (string, error) {
		return "## Context\n" + longContent, nil
	}

	app.ClarifierFactory = func(cfg llm.Config) GoalsClarifier {
		return &MockClarifier{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{
					Findings: []core.Finding{{
						Type: "clarification",
						Metadata: map[string]interface{}{
							"is_ready_to_plan": true,
							"enriched_goal":    "Test",
							"goal_summary":     "Test",
							"questions":        []string{},
						},
					}},
				}, nil
			},
		}
	}

	app.PlannerFactory = func(cfg llm.Config) TaskPlanner {
		return &MockPlanner{
			RunFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				tasks := []impl.PlanningTask{
					{Title: "Task 1", Description: "Task 1 description", Priority: 100, Keywords: []string{"test"}, Scope: "api"},
				}
				return core.Output{
					Findings: []core.Finding{{
						Type:     "plan",
						Metadata: map[string]interface{}{"tasks": tasks},
					}},
				}, nil
			},
		}
	}

	clarifyRes, _ := app.Clarify(context.Background(), ClarifyOptions{Goal: "test"})
	genRes, err := app.Generate(context.Background(), GenerateOptions{
		Goal:             "test",
		ClarifySessionID: clarifyRes.ClarifySessionID,
		EnrichedGoal:     clarifyRes.EnrichedGoal,
		Save:             true,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !genRes.Success {
		t.Fatalf("Expected success with long content, got: %s", genRes.Message)
	}

	// Verify full content is stored (truncation happens at presentation layer)
	if len(genRes.Tasks[0].ContextSummary) < len(longContent) {
		t.Error("Expected full context to be stored, but got truncated content")
	}
}
