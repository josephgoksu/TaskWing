package app

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// MockRepository implements task.Repository for testing
type MockRepository struct {
	CreatePlanFunc            func(p *task.Plan) error
	SetActivePlanFunc         func(id string) error
	GetActivePlanFunc         func() (*task.Plan, error)
	GetPlanFunc               func(id string) (*task.Plan, error)
	ListPlansFunc             func() ([]task.Plan, error)
	ListTasksFunc             func(planID string) ([]task.Task, error)
	CreateTaskFunc            func(t *task.Task) error
	GetTaskFunc               func(id string) (*task.Task, error)
	UpdateTaskStatusFunc      func(id string, status task.TaskStatus) error
	UpdatePlanFunc            func(id, goal, enrichedGoal string, status task.PlanStatus) error
	DeletePlanFunc            func(id string) error
	SearchPlansFunc           func(query string, status task.PlanStatus) ([]task.Plan, error)
	UpdatePlanAuditReportFunc func(id string, status task.PlanStatus, auditReportJSON string) error
}

func (m *MockRepository) CreatePlan(p *task.Plan) error {
	if m.CreatePlanFunc != nil {
		return m.CreatePlanFunc(p)
	}
	return nil
}
func (m *MockRepository) SetActivePlan(id string) error {
	if m.SetActivePlanFunc != nil {
		return m.SetActivePlanFunc(id)
	}
	return nil
}
func (m *MockRepository) GetActivePlan() (*task.Plan, error) {
	if m.GetActivePlanFunc != nil {
		return m.GetActivePlanFunc()
	}
	return nil, nil
}
func (m *MockRepository) GetPlan(id string) (*task.Plan, error) {
	if m.GetPlanFunc != nil {
		return m.GetPlanFunc(id)
	}
	return nil, nil
}
func (m *MockRepository) ListPlans() ([]task.Plan, error) {
	if m.ListPlansFunc != nil {
		return m.ListPlansFunc()
	}
	return nil, nil
}
func (m *MockRepository) SearchPlans(query string, status task.PlanStatus) ([]task.Plan, error) {
	if m.SearchPlansFunc != nil {
		return m.SearchPlansFunc(query, status)
	}
	return nil, nil
}
func (m *MockRepository) UpdatePlanAuditReport(id string, status task.PlanStatus, auditReportJSON string) error {
	if m.UpdatePlanAuditReportFunc != nil {
		return m.UpdatePlanAuditReportFunc(id, status, auditReportJSON)
	}
	return nil
}

func (m *MockRepository) ListTasks(planID string) ([]task.Task, error) {
	if m.ListTasksFunc != nil {
		return m.ListTasksFunc(planID)
	}
	return nil, nil
}
func (m *MockRepository) CreateTask(t *task.Task) error {
	if m.CreateTaskFunc != nil {
		return m.CreateTaskFunc(t)
	}
	return nil
}
func (m *MockRepository) GetTask(id string) (*task.Task, error) {
	if m.GetTaskFunc != nil {
		return m.GetTaskFunc(id)
	}
	return nil, nil
}
func (m *MockRepository) UpdateTaskStatus(id string, status task.TaskStatus) error {
	if m.UpdateTaskStatusFunc != nil {
		return m.UpdateTaskStatusFunc(id, status)
	}
	return nil
}
func (m *MockRepository) UpdatePlan(id, goal, enrichedGoal string, status task.PlanStatus) error {
	if m.UpdatePlanFunc != nil {
		return m.UpdatePlanFunc(id, goal, enrichedGoal, status)
	}
	return nil
}
func (m *MockRepository) DeletePlan(id string) error {
	if m.DeletePlanFunc != nil {
		return m.DeletePlanFunc(id)
	}
	return nil
}

func (m *MockRepository) AddDependency(taskID, dependsOn string) error {
	return nil
}

func (m *MockRepository) RemoveDependency(taskID, dependsOn string) error {
	return nil
}

func TestPlanApp_Generate_Failures(t *testing.T) {
	// Placeholder test
}

func TestPlanApp_Generate_SemanticValidation(t *testing.T) {
	// Test that semantic validation catches invalid file paths and shell commands
	t.Run("warns on missing file paths", func(t *testing.T) {
		// Create mock repo
		mockRepo := &MockRepository{
			CreatePlanFunc: func(p *task.Plan) error {
				p.ID = "test-plan-1"
				return nil
			},
			SetActivePlanFunc: func(id string) error {
				return nil
			},
		}

		// Create app with mocked planner that returns tasks with file references
		planApp := &PlanApp{
			ctx:  &Context{}, // Empty context - semantic validation doesn't need it
			Repo: mockRepo,
			PlannerFactory: func(cfg llm.Config) TaskPlanner {
				return &mockTaskPlanner{
					tasks: []impl.PlanningTask{
						{
							Title:              "Task with missing file",
							Description:        "Modify missing/path/to/file.go to add feature",
							Priority:           50,
							Complexity:         "medium",
							AssignedAgent:      "coder",
							AcceptanceCriteria: []string{"File is modified"},
							ValidationSteps:    []string{"go test ./..."},
						},
					},
				}
			},
			ContextRetriever: func(ctx context.Context, ks *knowledge.Service, goal, memoryPath string) (impl.SearchStrategyResult, error) {
				return impl.SearchStrategyResult{}, nil
			},
		}

		result, err := planApp.Generate(context.Background(), GenerateOptions{
			Goal:         "Test goal",
			EnrichedGoal: "Test enriched goal",
			Save:         true,
		})

		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if !result.Success {
			t.Fatalf("Expected success, got: %s", result.Message)
		}

		// Verify semantic warnings include the missing file
		if len(result.SemanticWarnings) == 0 {
			t.Error("Expected semantic warnings for missing file path, got none")
		}

		foundFileWarning := false
		for _, w := range result.SemanticWarnings {
			lower := strings.ToLower(w)
			if strings.Contains(lower, "missing_file") || strings.Contains(lower, "missing") {
				foundFileWarning = true
				break
			}
		}
		if !foundFileWarning {
			t.Errorf("Expected warning about missing file, got: %v", result.SemanticWarnings)
		}

		// Verify stats were populated
		if result.ValidationStats == nil {
			t.Error("Expected ValidationStats to be populated")
		} else if result.ValidationStats.PathsChecked == 0 {
			t.Error("Expected PathsChecked > 0")
		}
	})

	t.Run("reports invalid shell commands", func(t *testing.T) {
		if _, err := exec.LookPath("bash"); err != nil {
			t.Skip("bash not available; skipping shell validation test")
		}

		mockRepo := &MockRepository{
			CreatePlanFunc: func(p *task.Plan) error {
				p.ID = "test-plan-2"
				return nil
			},
			SetActivePlanFunc: func(id string) error {
				return nil
			},
		}

		planApp := &PlanApp{
			ctx:  &Context{}, // Empty context - semantic validation doesn't need it
			Repo: mockRepo,
			PlannerFactory: func(cfg llm.Config) TaskPlanner {
				return &mockTaskPlanner{
					tasks: []impl.PlanningTask{
						{
							Title:              "Task with invalid command",
							Description:        "Run the build",
							Priority:           50,
							Complexity:         "low",
							AssignedAgent:      "coder",
							AcceptanceCriteria: []string{"Build passes"},
							ValidationSteps:    []string{"if [ -f test.txt then echo ok fi"}, // Invalid syntax - missing ]
						},
					},
				}
			},
			ContextRetriever: func(ctx context.Context, ks *knowledge.Service, goal, memoryPath string) (impl.SearchStrategyResult, error) {
				return impl.SearchStrategyResult{}, nil
			},
		}

		result, err := planApp.Generate(context.Background(), GenerateOptions{
			Goal:         "Test goal",
			EnrichedGoal: "Test enriched goal",
			Save:         true,
		})

		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if !result.Success {
			t.Fatalf("Expected success, got: %s", result.Message)
		}

		// Verify semantic errors include the invalid command
		if len(result.SemanticErrors) == 0 {
			t.Error("Expected semantic errors for invalid shell command, got none")
		}

		foundCommandError := false
		for _, e := range result.SemanticErrors {
			if strings.Contains(strings.ToLower(e), "invalid_command") {
				foundCommandError = true
				break
			}
		}
		if !foundCommandError {
			t.Errorf("Expected error about invalid command, got: %v", result.SemanticErrors)
		}

		// Verify stats
		if result.ValidationStats == nil {
			t.Error("Expected ValidationStats to be populated")
		} else if result.ValidationStats.CommandsValidated == 0 {
			t.Error("Expected CommandsValidated > 0")
		}
	})
}

// mockTaskPlanner is a mock TaskPlanner for testing
type mockTaskPlanner struct {
	tasks []impl.PlanningTask
}

func (m *mockTaskPlanner) Run(ctx context.Context, input core.Input) (core.Output, error) {
	return core.Output{
		AgentName: "mock-planner",
		Findings: []core.Finding{
			{
				Type:        "plan",
				Title:       "Test Plan",
				Description: "Test rationale",
				Metadata: map[string]any{
					"tasks": m.tasks,
				},
			},
		},
	}, nil
}

func (m *mockTaskPlanner) Close() error {
	return nil
}
