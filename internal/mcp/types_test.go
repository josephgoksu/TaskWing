package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCodeAction_IsValid(t *testing.T) {
	tests := []struct {
		action CodeAction
		want   bool
	}{
		{CodeActionFind, true},
		{CodeActionSearch, true},
		{CodeActionExplain, true},
		{CodeActionCallers, true},
		{CodeActionImpact, true},
		{CodeActionSimplify, true},
		{"invalid", false},
		{"", false},
		{"FIND", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.want {
				t.Errorf("CodeAction(%q).IsValid() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestTaskAction_IsValid(t *testing.T) {
	tests := []struct {
		action TaskAction
		want   bool
	}{
		{TaskActionNext, true},
		{TaskActionCurrent, true},
		{TaskActionStart, true},
		{TaskActionComplete, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.want {
				t.Errorf("TaskAction(%q).IsValid() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestPlanAction_IsValid(t *testing.T) {
	tests := []struct {
		action PlanAction
		want   bool
	}{
		{PlanActionClarify, true},
		{PlanActionDecompose, true},
		{PlanActionExpand, true},
		{PlanActionGenerate, true},
		{PlanActionFinalize, true},
		{PlanActionAudit, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.want {
				t.Errorf("PlanAction(%q).IsValid() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestValidCodeActions(t *testing.T) {
	actions := ValidCodeActions()
	if len(actions) != 6 {
		t.Errorf("ValidCodeActions() returned %d actions, want 6", len(actions))
	}
}

func TestValidTaskActions(t *testing.T) {
	actions := ValidTaskActions()
	if len(actions) != 4 {
		t.Errorf("ValidTaskActions() returned %d actions, want 4", len(actions))
	}
}

func TestValidPlanActions(t *testing.T) {
	actions := ValidPlanActions()
	if len(actions) != 6 {
		t.Errorf("ValidPlanActions() returned %d actions, want 6", len(actions))
	}
}

// === PlanID JSON Schema Tests ===

// TestTaskToolParams_PlanIDSnakeCase tests that plan_id is correctly unmarshaled.
func TestTaskToolParams_PlanIDSnakeCase(t *testing.T) {
	jsonData := `{"action":"next","plan_id":"plan-123","session_id":"sess-456"}`

	var params TaskToolParams
	if err := json.Unmarshal([]byte(jsonData), &params); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if params.PlanID != "plan-123" {
		t.Errorf("PlanID = %q, want %q", params.PlanID, "plan-123")
	}
}

// TestTaskToolParams_RejectLegacyPlanIDAlias tests that planId is rejected.
func TestTaskToolParams_RejectLegacyPlanIDAlias(t *testing.T) {
	jsonData := `{"action":"next","planId":"plan-789","session_id":"sess-456"}`

	var params TaskToolParams
	err := json.Unmarshal([]byte(jsonData), &params)
	if err == nil {
		t.Fatal("expected unmarshal error for legacy planId")
	}
	if !strings.Contains(err.Error(), "planId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestTaskToolParams_RejectWhenBothPlanIDFormsProvided ensures strict rejection when legacy key is present.
func TestTaskToolParams_RejectWhenBothPlanIDFormsProvided(t *testing.T) {
	jsonData := `{"action":"next","plan_id":"plan-primary","planId":"plan-alias","session_id":"sess-456"}`

	var params TaskToolParams
	err := json.Unmarshal([]byte(jsonData), &params)
	if err == nil {
		t.Fatal("expected unmarshal error for legacy planId")
	}
	if !strings.Contains(err.Error(), "planId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPlanToolParams_PlanIDSnakeCase tests that plan_id is correctly unmarshaled.
func TestPlanToolParams_PlanIDSnakeCase(t *testing.T) {
	jsonData := `{"action":"audit","plan_id":"plan-123"}`

	var params PlanToolParams
	if err := json.Unmarshal([]byte(jsonData), &params); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if params.PlanID != "plan-123" {
		t.Errorf("PlanID = %q, want %q", params.PlanID, "plan-123")
	}
}

// TestPlanToolParams_RejectLegacyPlanIDAlias tests that planId is rejected.
func TestPlanToolParams_RejectLegacyPlanIDAlias(t *testing.T) {
	jsonData := `{"action":"audit","planId":"plan-789"}`

	var params PlanToolParams
	err := json.Unmarshal([]byte(jsonData), &params)
	if err == nil {
		t.Fatal("expected unmarshal error for legacy planId")
	}
	if !strings.Contains(err.Error(), "planId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPlanToolParams_RejectWhenBothPlanIDFormsProvided ensures strict rejection when legacy key is present.
func TestPlanToolParams_RejectWhenBothPlanIDFormsProvided(t *testing.T) {
	jsonData := `{"action":"audit","plan_id":"plan-primary","planId":"plan-alias"}`

	var params PlanToolParams
	err := json.Unmarshal([]byte(jsonData), &params)
	if err == nil {
		t.Fatal("expected unmarshal error for legacy planId")
	}
	if !strings.Contains(err.Error(), "planId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestMCPPlanIDEmptyValues tests edge cases with empty values.
func TestMCPPlanIDEmptyValues(t *testing.T) {
	tests := []struct {
		name       string
		jsonData   string
		wantPlanID string
	}{
		{
			name:       "empty plan_id",
			jsonData:   `{"action":"next","plan_id":"","session_id":"sess-1"}`,
			wantPlanID: "",
		},
		{
			name:       "null plan_id",
			jsonData:   `{"action":"next","plan_id":null,"session_id":"sess-1"}`,
			wantPlanID: "",
		},
		{
			name:       "both missing",
			jsonData:   `{"action":"next","session_id":"sess-1"}`,
			wantPlanID: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var params TaskToolParams
			if err := json.Unmarshal([]byte(tc.jsonData), &params); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if params.PlanID != tc.wantPlanID {
				t.Errorf("PlanID = %q, want %q", params.PlanID, tc.wantPlanID)
			}
		})
	}
}

func TestMCPPlanIDLegacyAliasRejected(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
	}{
		{
			name:     "task tool rejects planId",
			jsonData: `{"action":"next","planId":"plan-fallback","session_id":"sess-1"}`,
		},
		{
			name:     "plan tool rejects planId",
			jsonData: `{"action":"audit","planId":"plan-fallback"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.name, "task tool") {
				var params TaskToolParams
				err := json.Unmarshal([]byte(tc.jsonData), &params)
				if err == nil || !strings.Contains(err.Error(), "planId") {
					t.Fatalf("expected planId rejection, got: %v", err)
				}
				return
			}

			var params PlanToolParams
			err := json.Unmarshal([]byte(tc.jsonData), &params)
			if err == nil || !strings.Contains(err.Error(), "planId") {
				t.Fatalf("expected planId rejection, got: %v", err)
			}
		})
	}
}

// TestMCPParamsPreserveOtherFields ensures that custom UnmarshalJSON preserves other fields.
func TestMCPParamsPreserveOtherFields(t *testing.T) {
	t.Run("TaskToolParams", func(t *testing.T) {
		jsonData := `{"action":"complete","task_id":"task-abc","session_id":"sess-xyz","summary":"Done","files_modified":["a.go","b.go"]}`

		var params TaskToolParams
		if err := json.Unmarshal([]byte(jsonData), &params); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if params.Action != TaskActionComplete {
			t.Errorf("Action = %q, want %q", params.Action, TaskActionComplete)
		}
		if params.TaskID != "task-abc" {
			t.Errorf("TaskID = %q, want %q", params.TaskID, "task-abc")
		}
		if params.SessionID != "sess-xyz" {
			t.Errorf("SessionID = %q, want %q", params.SessionID, "sess-xyz")
		}
		if params.Summary != "Done" {
			t.Errorf("Summary = %q, want %q", params.Summary, "Done")
		}
		if len(params.FilesModified) != 2 {
			t.Errorf("FilesModified length = %d, want 2", len(params.FilesModified))
		}
	})

	t.Run("PlanToolParams", func(t *testing.T) {
		jsonData := `{"action":"generate","goal":"Add auth","enriched_goal":"Full auth spec","auto_answer":true}`

		var params PlanToolParams
		if err := json.Unmarshal([]byte(jsonData), &params); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if params.Action != PlanActionGenerate {
			t.Errorf("Action = %q, want %q", params.Action, PlanActionGenerate)
		}
		if params.Goal != "Add auth" {
			t.Errorf("Goal = %q, want %q", params.Goal, "Add auth")
		}
		if params.EnrichedGoal != "Full auth spec" {
			t.Errorf("EnrichedGoal = %q, want %q", params.EnrichedGoal, "Full auth spec")
		}
		if !params.AutoAnswer {
			t.Errorf("AutoAnswer = %v, want true", params.AutoAnswer)
		}
	})

}

func TestPlanToolParams_ClarifySessionAndAnswers(t *testing.T) {
	jsonData := `{
		"action":"clarify",
		"clarify_session_id":"clarify-123",
		"answers":[
			{"question":"Target users?","answer":"Backend team"},
			{"answer":"Must support monorepo"}
		]
	}`

	var params PlanToolParams
	if err := json.Unmarshal([]byte(jsonData), &params); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if params.ClarifySessionID != "clarify-123" {
		t.Fatalf("ClarifySessionID = %q, want %q", params.ClarifySessionID, "clarify-123")
	}
	if got := len(params.Answers); got != 2 {
		t.Fatalf("answers len = %d, want 2", got)
	}
	if params.Answers[0].Question != "Target users?" || params.Answers[0].Answer != "Backend team" {
		t.Fatalf("first answer mismatch: %+v", params.Answers[0])
	}
	if params.Answers[1].Question != "" || params.Answers[1].Answer != "Must support monorepo" {
		t.Fatalf("second answer mismatch: %+v", params.Answers[1])
	}
}

func TestPlanToolParams_RejectsLegacyHistory(t *testing.T) {
	jsonData := `{
		"action":"clarify",
		"goal":"Refactor API",
		"history":"Q: old? A: yes"
	}`

	var params PlanToolParams
	err := json.Unmarshal([]byte(jsonData), &params)
	if err == nil {
		t.Fatal("expected unmarshal error for legacy history field")
	}
	if got := err.Error(); got == "" || !containsAll(got, []string{"history", "clarify_session_id", "answers"}) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsAll(s string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(s, term) {
			return false
		}
	}
	return true
}
