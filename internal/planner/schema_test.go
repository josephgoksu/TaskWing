package planner

import (
	"testing"
)

func TestPlanSchema_Valid(t *testing.T) {
	plan := LLMPlanResponse{
		GoalSummary:         "Implement user authentication with JWT",
		Rationale:           "JWT provides stateless authentication that scales well with our microservices architecture",
		EstimatedComplexity: "medium",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Create JWT token service",
				Description:        "Implement a service to generate and validate JWT tokens with refresh token support",
				Priority:           80,
				Complexity:         "medium",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Token generation works", "Token validation works"},
				ValidationSteps:    []string{"go test ./internal/auth/..."},
			},
		},
	}

	result := plan.Validate()
	if !result.Valid {
		t.Errorf("Expected valid plan, got errors: %s", result.ErrorSummary())
	}
}

func TestPlanSchema_MissingGoalSummary(t *testing.T) {
	plan := LLMPlanResponse{
		GoalSummary:         "", // Empty - should fail
		Rationale:           "This is a valid rationale with enough characters",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Task 1",
				Description:        "Valid description",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Done"},
			},
		},
	}

	result := plan.Validate()
	if result.Valid {
		t.Error("Expected validation to fail for empty GoalSummary")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "GoalSummary" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for GoalSummary field")
	}
}

func TestPlanSchema_MissingTasks(t *testing.T) {
	plan := LLMPlanResponse{
		GoalSummary:         "Valid goal summary",
		Rationale:           "This is a valid rationale with enough characters",
		EstimatedComplexity: "low",
		Tasks:               []LLMTaskSchema{}, // Empty - should fail
	}

	result := plan.Validate()
	if result.Valid {
		t.Error("Expected validation to fail for empty Tasks")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "Tasks" && e.Tag == "min" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected min error for Tasks field")
	}
}

func TestPlanSchema_InvalidComplexity(t *testing.T) {
	plan := LLMPlanResponse{
		GoalSummary:         "Valid goal summary",
		Rationale:           "This is a valid rationale with enough characters",
		EstimatedComplexity: "very_high", // Invalid - should fail
		Tasks: []LLMTaskSchema{
			{
				Title:              "Task 1",
				Description:        "Valid description here",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Done"},
			},
		},
	}

	result := plan.Validate()
	if result.Valid {
		t.Error("Expected validation to fail for invalid EstimatedComplexity")
	}
}

func TestTaskSchema_Valid(t *testing.T) {
	task := LLMTaskSchema{
		Title:              "Implement login endpoint",
		Description:        "Create a POST /api/auth/login endpoint that validates credentials",
		Priority:           75,
		Complexity:         "medium",
		AssignedAgent:      "coder",
		AcceptanceCriteria: []string{"Endpoint returns JWT on success", "Returns 401 on invalid credentials"},
		ValidationSteps:    []string{"curl -X POST http://localhost:8080/api/auth/login"},
		Scope:              "auth",
		Keywords:           []string{"login", "jwt", "auth"},
	}

	result := task.Validate()
	if !result.Valid {
		t.Errorf("Expected valid task, got errors: %s", result.ErrorSummary())
	}
}

func TestTaskSchema_InvalidPriority(t *testing.T) {
	task := LLMTaskSchema{
		Title:              "Test task",
		Description:        "Valid description here",
		Priority:           150, // Invalid - should be 0-100
		Complexity:         "low",
		AssignedAgent:      "coder",
		AcceptanceCriteria: []string{"Done"},
	}

	result := task.Validate()
	if result.Valid {
		t.Error("Expected validation to fail for priority > 100")
	}
}

func TestTaskSchema_NegativePriority(t *testing.T) {
	task := LLMTaskSchema{
		Title:              "Test task",
		Description:        "Valid description here",
		Priority:           -10, // Invalid - should be >= 0
		Complexity:         "low",
		AssignedAgent:      "coder",
		AcceptanceCriteria: []string{"Done"},
	}

	result := task.Validate()
	if result.Valid {
		t.Error("Expected validation to fail for negative priority")
	}
}

func TestTaskSchema_InvalidAgent(t *testing.T) {
	task := LLMTaskSchema{
		Title:              "Test task",
		Description:        "Valid description here",
		Priority:           50,
		Complexity:         "low",
		AssignedAgent:      "designer", // Invalid - not in allowed list
		AcceptanceCriteria: []string{"Done"},
	}

	result := task.Validate()
	if result.Valid {
		t.Error("Expected validation to fail for invalid AssignedAgent")
	}
}

func TestTaskSchema_MissingAcceptanceCriteria(t *testing.T) {
	task := LLMTaskSchema{
		Title:              "Test task",
		Description:        "Valid description here",
		Priority:           50,
		Complexity:         "low",
		AssignedAgent:      "coder",
		AcceptanceCriteria: []string{}, // Empty - should fail
	}

	result := task.Validate()
	if result.Valid {
		t.Error("Expected validation to fail for empty AcceptanceCriteria")
	}
}

func TestTaskSchema_WhitespaceOnlyTitle(t *testing.T) {
	task := LLMTaskSchema{
		Title:              "   ", // Whitespace only - should fail
		Description:        "Valid description here",
		Priority:           50,
		Complexity:         "low",
		AssignedAgent:      "coder",
		AcceptanceCriteria: []string{"Done"},
	}

	result := task.Validate()
	if result.Valid {
		t.Error("Expected validation to fail for whitespace-only Title")
	}
}

func TestClarificationSchema_ReadyToPlan(t *testing.T) {
	clarification := LLMClarificationResponse{
		IsReadyToPlan: true,
		EnrichedGoal:  "Implement a comprehensive authentication system using JWT tokens with refresh token rotation",
		GoalSummary:   "Implement JWT authentication with refresh tokens",
		Assumptions:   []string{"Using Go standard library for crypto"},
		Constraints:   []string{"Must be stateless"},
	}

	result := clarification.Validate()
	if !result.Valid {
		t.Errorf("Expected valid clarification, got errors: %s", result.ErrorSummary())
	}
}

func TestClarificationSchema_NeedsQuestions(t *testing.T) {
	clarification := LLMClarificationResponse{
		IsReadyToPlan: false,
		GoalSummary:   "Implement authentication",
		Questions:     []string{"What authentication method?", "Do you need MFA?"},
	}

	result := clarification.Validate()
	if !result.Valid {
		t.Errorf("Expected valid clarification with questions, got errors: %s", result.ErrorSummary())
	}
}

func TestValidationResult_ErrorSummary(t *testing.T) {
	result := ValidationResult{
		Valid: false,
		Errors: []ValidationError{
			{Field: "Title", Tag: "required", Message: "Title is required"},
			{Field: "Priority", Tag: "priority_range", Message: "Priority must be between 0 and 100"},
		},
	}

	summary := result.ErrorSummary()
	if summary == "" {
		t.Error("Expected non-empty error summary")
	}
	if !contains(summary, "Title is required") {
		t.Error("Expected error summary to contain 'Title is required'")
	}
	if !contains(summary, "Priority must be between") {
		t.Error("Expected error summary to contain priority error")
	}
}

func TestValidationResult_EmptySummaryWhenValid(t *testing.T) {
	result := ValidationResult{Valid: true}
	if result.ErrorSummary() != "" {
		t.Error("Expected empty error summary for valid result")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
