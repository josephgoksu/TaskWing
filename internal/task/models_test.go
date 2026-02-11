package task

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTask_Validate(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name: "valid task",
			task: Task{
				Title:       "Valid Task",
				Description: "Valid Description",
				Priority:    50,
			},
			wantErr: false,
		},
		{
			name: "empty title",
			task: Task{
				Title:       "",
				Description: "Valid Description",
				Priority:    50,
			},
			wantErr: true, // title required
		},
		{
			name: "long title",
			task: Task{
				Title:       strings.Repeat("a", 201),
				Description: "Valid Description",
				Priority:    50,
			},
			wantErr: true, // max 200
		},
		{
			name: "empty description",
			task: Task{
				Title:       "Valid Task",
				Description: "",
				Priority:    50,
			},
			wantErr: true, // description required
		},
		{
			name: "priority too low",
			task: Task{
				Title:       "Valid Task",
				Description: "Valid Description",
				Priority:    -1,
			},
			wantErr: true, // 0-100
		},
		{
			name: "priority too high",
			task: Task{
				Title:       "Valid Task",
				Description: "Valid Description",
				Priority:    101,
			},
			wantErr: true, // 0-100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.task.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Task.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPlanIDJSONSchema_SnakeCase tests that plan_id is used in JSON output.
func TestPlanIDJSONSchema_SnakeCase(t *testing.T) {
	task := Task{
		ID:          "task-123",
		PlanID:      "plan-456",
		Title:       "Test Task",
		Description: "Test Description",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Failed to marshal task: %v", err)
	}

	jsonStr := string(data)

	// Should contain plan_id (snake_case)
	if !strings.Contains(jsonStr, `"plan_id"`) {
		t.Errorf("JSON output should use 'plan_id', got: %s", jsonStr)
	}

	// Should NOT contain planId (camelCase) in output
	if strings.Contains(jsonStr, `"planId"`) {
		t.Errorf("JSON output should NOT use 'planId', got: %s", jsonStr)
	}
}

// TestPlanIDJSONSchema_AcceptSnakeCase tests that plan_id is correctly unmarshaled.
func TestPlanIDJSONSchema_AcceptSnakeCase(t *testing.T) {
	jsonData := `{"id":"task-123","plan_id":"plan-456","title":"Test","description":"Test"}`

	var task Task
	if err := json.Unmarshal([]byte(jsonData), &task); err != nil {
		t.Fatalf("Failed to unmarshal task: %v", err)
	}

	if task.PlanID != "plan-456" {
		t.Errorf("PlanID = %q, want %q", task.PlanID, "plan-456")
	}
}

// TestPlanIDJSONSchema_RejectCamelCaseAlias tests that planId is rejected.
func TestPlanIDJSONSchema_RejectCamelCaseAlias(t *testing.T) {
	jsonData := `{"id":"task-123","planId":"plan-789","title":"Test","description":"Test"}`

	var task Task
	err := json.Unmarshal([]byte(jsonData), &task)
	if err == nil {
		t.Fatal("expected unmarshal error for legacy planId")
	}
	if !strings.Contains(err.Error(), "planId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPlanIDJSONSchema_RejectWhenBothKeysPresent tests strict rejection when legacy key exists.
func TestPlanIDJSONSchema_RejectWhenBothKeysPresent(t *testing.T) {
	jsonData := `{"id":"task-123","plan_id":"plan-primary","planId":"plan-alias","title":"Test","description":"Test"}`

	var task Task
	err := json.Unmarshal([]byte(jsonData), &task)
	if err == nil {
		t.Fatal("expected unmarshal error for legacy planId")
	}
	if !strings.Contains(err.Error(), "planId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPlanIDJSONSchema_RoundTrip tests that marshal -> unmarshal preserves PlanID.
func TestPlanIDJSONSchema_RoundTrip(t *testing.T) {
	original := Task{
		ID:          "task-123",
		PlanID:      "plan-456",
		Title:       "Test Task",
		Description: "Test Description",
		Priority:    50,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal task: %v", err)
	}

	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal task: %v", err)
	}

	if decoded.PlanID != original.PlanID {
		t.Errorf("PlanID after round-trip = %q, want %q", decoded.PlanID, original.PlanID)
	}
}

// TestPlanIDJSONSchema_EmptyValues tests edge cases with empty values.
func TestPlanIDJSONSchema_EmptyValues(t *testing.T) {
	tests := []struct {
		name       string
		jsonData   string
		wantPlanID string
	}{
		{
			name:       "empty plan_id",
			jsonData:   `{"id":"task-123","plan_id":"","title":"Test","description":"Test"}`,
			wantPlanID: "",
		},
		{
			name:       "null plan_id",
			jsonData:   `{"id":"task-123","plan_id":null,"title":"Test","description":"Test"}`,
			wantPlanID: "",
		},
		{
			name:       "both missing",
			jsonData:   `{"id":"task-123","title":"Test","description":"Test"}`,
			wantPlanID: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var task Task
			if err := json.Unmarshal([]byte(tc.jsonData), &task); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if task.PlanID != tc.wantPlanID {
				t.Errorf("PlanID = %q, want %q", task.PlanID, tc.wantPlanID)
			}
		})
	}
}

func TestPlanIDJSONSchema_LegacyAliasRejected(t *testing.T) {
	jsonData := `{"id":"task-123","planId":"plan-fallback","title":"Test","description":"Test"}`

	var task Task
	err := json.Unmarshal([]byte(jsonData), &task)
	if err == nil {
		t.Fatal("expected unmarshal error for legacy planId")
	}
	if !strings.Contains(err.Error(), "planId") {
		t.Fatalf("unexpected error: %v", err)
	}
}
