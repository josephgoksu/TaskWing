/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHookSessionJSON(t *testing.T) {
	// Test JSON serialization of HookSession with all fields
	session := &HookSession{
		SessionID:                    "test-session-123",
		StartedAt:                    time.Now().UTC(),
		TasksCompleted:               3,
		TasksStarted:                 4,
		CurrentTaskID:                "task-456",
		PlanID:                       "plan-789",
		LastTaskHadCriticalDeviation: true,
		LastDeviationSummary:         "Modified protected file",
		TotalDeviationsDetected:      2,
		LastTaskHadPolicyViolation:   true,
		LastPolicyViolations:         []string{"Cannot modify .env files", "Secrets directory protected"},
		TotalPolicyViolations:        2,
	}

	// Marshal to JSON
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Failed to marshal session: %v", err)
	}

	// Unmarshal back
	var decoded HookSession
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal session: %v", err)
	}

	// Verify fields
	if decoded.SessionID != session.SessionID {
		t.Errorf("SessionID: got %s, want %s", decoded.SessionID, session.SessionID)
	}
	if decoded.TasksCompleted != session.TasksCompleted {
		t.Errorf("TasksCompleted: got %d, want %d", decoded.TasksCompleted, session.TasksCompleted)
	}
	if decoded.LastTaskHadPolicyViolation != session.LastTaskHadPolicyViolation {
		t.Errorf("LastTaskHadPolicyViolation: got %v, want %v", decoded.LastTaskHadPolicyViolation, session.LastTaskHadPolicyViolation)
	}
	if len(decoded.LastPolicyViolations) != len(session.LastPolicyViolations) {
		t.Errorf("LastPolicyViolations length: got %d, want %d", len(decoded.LastPolicyViolations), len(session.LastPolicyViolations))
	}
}

func TestHookResponseJSON(t *testing.T) {
	tests := []struct {
		name      string
		response  HookResponse
		wantKey   string
		wantNoKey string
	}{
		{
			name:      "allow stop (no decision)",
			response:  HookResponse{Reason: "Plan complete"},
			wantKey:   "reason",
			wantNoKey: "decision",
		},
		{
			name: "block stop",
			response: func() HookResponse {
				block := "block"
				return HookResponse{Decision: &block, Reason: "Continue to next task"}
			}(),
			wantKey: "decision",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("Failed to marshal response: %v", err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if _, ok := decoded[tt.wantKey]; !ok {
				t.Errorf("Expected key %q in response, got: %s", tt.wantKey, string(data))
			}

			if tt.wantNoKey != "" {
				if _, ok := decoded[tt.wantNoKey]; ok {
					t.Errorf("Did not expect key %q in response, got: %s", tt.wantNoKey, string(data))
				}
			}
		})
	}
}

func TestHookSessionPersistence(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "hook_session.json")

	// Create test session
	session := &HookSession{
		SessionID:                  "test-session-persist",
		StartedAt:                  time.Now().UTC(),
		TasksCompleted:             2,
		TasksStarted:               3,
		LastTaskHadPolicyViolation: true,
		LastPolicyViolations:       []string{"Policy violation 1"},
		TotalPolicyViolations:      1,
	}

	// Save session manually
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal session: %v", err)
	}

	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		t.Fatalf("Failed to write session file: %v", err)
	}

	// Read and verify
	readData, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("Failed to read session file: %v", err)
	}

	var loaded HookSession
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal loaded session: %v", err)
	}

	if loaded.SessionID != session.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", loaded.SessionID, session.SessionID)
	}
	if !loaded.LastTaskHadPolicyViolation {
		t.Error("LastTaskHadPolicyViolation should be true")
	}
	if len(loaded.LastPolicyViolations) != 1 {
		t.Errorf("LastPolicyViolations length: got %d, want 1", len(loaded.LastPolicyViolations))
	}
}

func TestHookSessionDefaults(t *testing.T) {
	if DefaultMaxTasksPerSession != 5 {
		t.Errorf("DefaultMaxTasksPerSession: got %d, want 5", DefaultMaxTasksPerSession)
	}

	if DefaultMaxSessionMinutes != 30 {
		t.Errorf("DefaultMaxSessionMinutes: got %d, want 30", DefaultMaxSessionMinutes)
	}
}

func TestHookResponseBlock(t *testing.T) {
	block := "block"
	resp := HookResponse{
		Decision: &block,
		Reason:   "Continue to task 2/5: Add authentication",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded["decision"] != "block" {
		t.Errorf("Decision should be 'block', got: %v", decoded["decision"])
	}
}

func TestHookResponseAllow(t *testing.T) {
	resp := HookResponse{
		Reason: "Circuit breaker: Max tasks reached",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// When allowing stop, decision should be omitted
	if _, exists := decoded["decision"]; exists {
		t.Error("Decision should be omitted when allowing stop")
	}
}

func TestPolicyCircuitBreakerTracking(t *testing.T) {
	// Test that policy violations are properly tracked in session
	session := &HookSession{
		SessionID:                  "test-policy-circuit",
		LastTaskHadPolicyViolation: true,
		LastPolicyViolations:       []string{"Cannot modify .env files", "Secrets directory is protected"},
		TotalPolicyViolations:      2,
	}

	if !session.LastTaskHadPolicyViolation {
		t.Error("Policy violation flag should be set")
	}

	if len(session.LastPolicyViolations) != 2 {
		t.Errorf("Expected 2 violations, got %d", len(session.LastPolicyViolations))
	}

	if session.TotalPolicyViolations != 2 {
		t.Errorf("TotalPolicyViolations: got %d, want 2", session.TotalPolicyViolations)
	}

	// Serialize and verify
	data, _ := json.Marshal(session)
	var decoded HookSession
	_ = json.Unmarshal(data, &decoded)

	if !decoded.LastTaskHadPolicyViolation {
		t.Error("Policy violation flag should persist through JSON serialization")
	}
}

func TestDeviationCircuitBreakerTracking(t *testing.T) {
	// Test that deviations are properly tracked in session
	session := &HookSession{
		SessionID:                    "test-deviation-circuit",
		LastTaskHadCriticalDeviation: true,
		LastDeviationSummary:         "2 unexpected files, 1 missing file (requires review)",
		TotalDeviationsDetected:      3,
	}

	if !session.LastTaskHadCriticalDeviation {
		t.Error("Critical deviation flag should be set")
	}

	if session.LastDeviationSummary == "" {
		t.Error("Deviation summary should not be empty")
	}

	if session.TotalDeviationsDetected != 3 {
		t.Errorf("TotalDeviationsDetected: got %d, want 3", session.TotalDeviationsDetected)
	}
}

func TestHookSessionAllFieldsSerialization(t *testing.T) {
	// Verify all session fields survive JSON round-trip
	original := &HookSession{
		SessionID:                    "full-test-session",
		StartedAt:                    time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		TasksCompleted:               5,
		TasksStarted:                 6,
		CurrentTaskID:                "task-current",
		PlanID:                       "plan-active",
		LastTaskHadCriticalDeviation: true,
		LastDeviationSummary:         "deviation summary",
		TotalDeviationsDetected:      10,
		LastTaskHadPolicyViolation:   true,
		LastPolicyViolations:         []string{"v1", "v2", "v3"},
		TotalPolicyViolations:        3,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded HookSession
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Check all fields
	checks := []struct {
		name      string
		got, want any
	}{
		{"SessionID", decoded.SessionID, original.SessionID},
		{"TasksCompleted", decoded.TasksCompleted, original.TasksCompleted},
		{"TasksStarted", decoded.TasksStarted, original.TasksStarted},
		{"CurrentTaskID", decoded.CurrentTaskID, original.CurrentTaskID},
		{"PlanID", decoded.PlanID, original.PlanID},
		{"LastTaskHadCriticalDeviation", decoded.LastTaskHadCriticalDeviation, original.LastTaskHadCriticalDeviation},
		{"LastDeviationSummary", decoded.LastDeviationSummary, original.LastDeviationSummary},
		{"TotalDeviationsDetected", decoded.TotalDeviationsDetected, original.TotalDeviationsDetected},
		{"LastTaskHadPolicyViolation", decoded.LastTaskHadPolicyViolation, original.LastTaskHadPolicyViolation},
		{"TotalPolicyViolations", decoded.TotalPolicyViolations, original.TotalPolicyViolations},
		{"LastPolicyViolations length", len(decoded.LastPolicyViolations), len(original.LastPolicyViolations)},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}
