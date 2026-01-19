package policy

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with the policy_decisions table.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	// Create the policy_decisions table
	schema := `
	CREATE TABLE IF NOT EXISTS policy_decisions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		decision_id TEXT UNIQUE NOT NULL,
		policy_path TEXT NOT NULL,
		result TEXT NOT NULL,
		violations TEXT,
		input_json TEXT NOT NULL,
		task_id TEXT,
		session_id TEXT,
		evaluated_at TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_policy_decisions_task ON policy_decisions(task_id);
	CREATE INDEX IF NOT EXISTS idx_policy_decisions_session ON policy_decisions(session_id);
	CREATE INDEX IF NOT EXISTS idx_policy_decisions_result ON policy_decisions(result);
	CREATE INDEX IF NOT EXISTS idx_policy_decisions_evaluated_at ON policy_decisions(evaluated_at);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func TestAuditStore_SaveDecision(t *testing.T) {
	db := setupTestDB(t)
	store := NewAuditStore(db)

	tests := []struct {
		name     string
		decision *PolicyDecision
		wantErr  bool
	}{
		{
			name: "save allow decision",
			decision: &PolicyDecision{
				DecisionID: "test-allow-1",
				PolicyPath: "taskwing.policy",
				Result:     PolicyResultAllow,
				Input:      map[string]any{"task": map[string]any{"id": "task-1"}},
			},
			wantErr: false,
		},
		{
			name: "save deny decision with violations",
			decision: &PolicyDecision{
				DecisionID: "test-deny-1",
				PolicyPath: "taskwing.policy",
				Result:     PolicyResultDeny,
				Violations: []string{"Cannot modify protected file", "Raw SQL in controller"},
				Input:      map[string]any{"task": map[string]any{"id": "task-2"}},
				TaskID:     "task-2",
				SessionID:  "session-1",
			},
			wantErr: false,
		},
		{
			name: "auto-generate decision ID",
			decision: &PolicyDecision{
				PolicyPath: "taskwing.policy",
				Result:     PolicyResultAllow,
				Input:      map[string]any{},
			},
			wantErr: false,
		},
		{
			name:     "nil decision",
			decision: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.SaveDecision(tt.decision)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveDecision() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.decision != nil {
				// Verify the decision was saved
				saved, err := store.GetDecision(tt.decision.DecisionID)
				if err != nil {
					t.Errorf("GetDecision() error = %v", err)
					return
				}
				if saved.PolicyPath != tt.decision.PolicyPath {
					t.Errorf("PolicyPath = %v, want %v", saved.PolicyPath, tt.decision.PolicyPath)
				}
				if saved.Result != tt.decision.Result {
					t.Errorf("Result = %v, want %v", saved.Result, tt.decision.Result)
				}
			}
		})
	}
}

func TestAuditStore_GetDecision(t *testing.T) {
	db := setupTestDB(t)
	store := NewAuditStore(db)

	// Save a test decision
	decision := &PolicyDecision{
		DecisionID: "test-get-1",
		PolicyPath: "taskwing.policy.protected",
		Result:     PolicyResultDeny,
		Violations: []string{"Access denied to core/"},
		Input:      map[string]any{"file": "core/router.go"},
		TaskID:     "task-123",
		SessionID:  "session-456",
	}
	if err := store.SaveDecision(decision); err != nil {
		t.Fatalf("SaveDecision() error = %v", err)
	}

	// Retrieve it
	got, err := store.GetDecision("test-get-1")
	if err != nil {
		t.Fatalf("GetDecision() error = %v", err)
	}

	if got.DecisionID != decision.DecisionID {
		t.Errorf("DecisionID = %v, want %v", got.DecisionID, decision.DecisionID)
	}
	if got.PolicyPath != decision.PolicyPath {
		t.Errorf("PolicyPath = %v, want %v", got.PolicyPath, decision.PolicyPath)
	}
	if got.Result != decision.Result {
		t.Errorf("Result = %v, want %v", got.Result, decision.Result)
	}
	if len(got.Violations) != 1 || got.Violations[0] != "Access denied to core/" {
		t.Errorf("Violations = %v, want %v", got.Violations, decision.Violations)
	}
	if got.TaskID != decision.TaskID {
		t.Errorf("TaskID = %v, want %v", got.TaskID, decision.TaskID)
	}
	if got.SessionID != decision.SessionID {
		t.Errorf("SessionID = %v, want %v", got.SessionID, decision.SessionID)
	}

	// Test not found
	_, err = store.GetDecision("nonexistent")
	if err == nil {
		t.Error("GetDecision() expected error for nonexistent decision")
	}
}

func TestAuditStore_ListDecisions(t *testing.T) {
	db := setupTestDB(t)
	store := NewAuditStore(db)

	// Save multiple decisions
	decisions := []*PolicyDecision{
		{
			DecisionID: "list-1",
			PolicyPath: "taskwing.policy",
			Result:     PolicyResultAllow,
			Input:      map[string]any{},
			TaskID:     "task-1",
			SessionID:  "session-A",
		},
		{
			DecisionID: "list-2",
			PolicyPath: "taskwing.policy",
			Result:     PolicyResultDeny,
			Violations: []string{"violation 1"},
			Input:      map[string]any{},
			TaskID:     "task-2",
			SessionID:  "session-A",
		},
		{
			DecisionID: "list-3",
			PolicyPath: "taskwing.policy",
			Result:     PolicyResultDeny,
			Violations: []string{"violation 2"},
			Input:      map[string]any{},
			TaskID:     "task-3",
			SessionID:  "session-B",
		},
	}

	for _, d := range decisions {
		if err := store.SaveDecision(d); err != nil {
			t.Fatalf("SaveDecision() error = %v", err)
		}
	}

	tests := []struct {
		name      string
		opts      ListDecisionsOptions
		wantCount int
	}{
		{
			name:      "list all",
			opts:      ListDecisionsOptions{},
			wantCount: 3,
		},
		{
			name:      "filter by session A",
			opts:      ListDecisionsOptions{SessionID: "session-A"},
			wantCount: 2,
		},
		{
			name:      "filter by session B",
			opts:      ListDecisionsOptions{SessionID: "session-B"},
			wantCount: 1,
		},
		{
			name:      "filter by deny result",
			opts:      ListDecisionsOptions{Result: PolicyResultDeny},
			wantCount: 2,
		},
		{
			name:      "filter by allow result",
			opts:      ListDecisionsOptions{Result: PolicyResultAllow},
			wantCount: 1,
		},
		{
			name:      "filter by task",
			opts:      ListDecisionsOptions{TaskID: "task-2"},
			wantCount: 1,
		},
		{
			name:      "limit results",
			opts:      ListDecisionsOptions{Limit: 2},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.ListDecisions(tt.opts)
			if err != nil {
				t.Errorf("ListDecisions() error = %v", err)
				return
			}
			if len(got) != tt.wantCount {
				t.Errorf("ListDecisions() returned %d decisions, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestAuditStore_CountViolations(t *testing.T) {
	db := setupTestDB(t)
	store := NewAuditStore(db)

	// Save some decisions
	now := time.Now().UTC()
	decisions := []*PolicyDecision{
		{
			DecisionID:  "count-1",
			PolicyPath:  "taskwing.policy",
			Result:      PolicyResultDeny,
			Input:       map[string]any{},
			EvaluatedAt: now.Add(-1 * time.Hour),
		},
		{
			DecisionID:  "count-2",
			PolicyPath:  "taskwing.policy",
			Result:      PolicyResultDeny,
			Input:       map[string]any{},
			EvaluatedAt: now.Add(-30 * time.Minute),
		},
		{
			DecisionID:  "count-3",
			PolicyPath:  "taskwing.policy",
			Result:      PolicyResultAllow,
			Input:       map[string]any{},
			EvaluatedAt: now.Add(-15 * time.Minute),
		},
	}

	for _, d := range decisions {
		if err := store.SaveDecision(d); err != nil {
			t.Fatalf("SaveDecision() error = %v", err)
		}
	}

	count, err := store.CountViolations(now.Add(-2 * time.Hour))
	if err != nil {
		t.Fatalf("CountViolations() error = %v", err)
	}
	if count != 2 {
		t.Errorf("CountViolations() = %d, want 2", count)
	}
}

func TestAuditStore_DeleteDecision(t *testing.T) {
	db := setupTestDB(t)
	store := NewAuditStore(db)

	// Save a decision
	decision := &PolicyDecision{
		DecisionID: "delete-1",
		PolicyPath: "taskwing.policy",
		Result:     PolicyResultAllow,
		Input:      map[string]any{},
	}
	if err := store.SaveDecision(decision); err != nil {
		t.Fatalf("SaveDecision() error = %v", err)
	}

	// Delete it
	if err := store.DeleteDecision("delete-1"); err != nil {
		t.Errorf("DeleteDecision() error = %v", err)
	}

	// Verify it's gone
	_, err := store.GetDecision("delete-1")
	if err == nil {
		t.Error("GetDecision() expected error after deletion")
	}

	// Delete nonexistent
	err = store.DeleteDecision("nonexistent")
	if err == nil {
		t.Error("DeleteDecision() expected error for nonexistent decision")
	}
}

func TestAuditStore_PruneOldDecisions(t *testing.T) {
	db := setupTestDB(t)
	store := NewAuditStore(db)

	// Save decisions with different timestamps
	now := time.Now().UTC()
	decisions := []*PolicyDecision{
		{
			DecisionID:  "prune-1",
			PolicyPath:  "taskwing.policy",
			Result:      PolicyResultAllow,
			Input:       map[string]any{},
			EvaluatedAt: now.Add(-48 * time.Hour), // 2 days old
		},
		{
			DecisionID:  "prune-2",
			PolicyPath:  "taskwing.policy",
			Result:      PolicyResultAllow,
			Input:       map[string]any{},
			EvaluatedAt: now.Add(-1 * time.Hour), // 1 hour old
		},
	}

	for _, d := range decisions {
		if err := store.SaveDecision(d); err != nil {
			t.Fatalf("SaveDecision() error = %v", err)
		}
	}

	// Prune decisions older than 24 hours
	pruned, err := store.PruneOldDecisions(24 * time.Hour)
	if err != nil {
		t.Fatalf("PruneOldDecisions() error = %v", err)
	}
	if pruned != 1 {
		t.Errorf("PruneOldDecisions() pruned %d, want 1", pruned)
	}

	// Verify only one remains
	remaining, err := store.ListDecisions(ListDecisionsOptions{})
	if err != nil {
		t.Fatalf("ListDecisions() error = %v", err)
	}
	if len(remaining) != 1 {
		t.Errorf("Expected 1 remaining decision, got %d", len(remaining))
	}
}

func TestPolicyDecision_Methods(t *testing.T) {
	t.Run("IsAllowed", func(t *testing.T) {
		allow := &PolicyDecision{Result: PolicyResultAllow}
		deny := &PolicyDecision{Result: PolicyResultDeny}

		if !allow.IsAllowed() {
			t.Error("IsAllowed() = false for allow result")
		}
		if allow.IsDenied() {
			t.Error("IsDenied() = true for allow result")
		}
		if deny.IsAllowed() {
			t.Error("IsAllowed() = true for deny result")
		}
		if !deny.IsDenied() {
			t.Error("IsDenied() = false for deny result")
		}
	})

	t.Run("ViolationsJSON", func(t *testing.T) {
		d := &PolicyDecision{Violations: []string{"a", "b"}}
		got := d.ViolationsJSON()
		want := `["a","b"]`
		if got != want {
			t.Errorf("ViolationsJSON() = %v, want %v", got, want)
		}

		empty := &PolicyDecision{}
		if empty.ViolationsJSON() != "[]" {
			t.Errorf("ViolationsJSON() for empty = %v, want []", empty.ViolationsJSON())
		}
	})

	t.Run("InputJSON", func(t *testing.T) {
		d := &PolicyDecision{Input: map[string]any{"key": "value"}}
		got := d.InputJSON()
		want := `{"key":"value"}`
		if got != want {
			t.Errorf("InputJSON() = %v, want %v", got, want)
		}

		empty := &PolicyDecision{}
		if empty.InputJSON() != "{}" {
			t.Errorf("InputJSON() for nil = %v, want {}", empty.InputJSON())
		}
	})
}

func TestParseViolations(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "valid JSON array",
			input: `["a","b","c"]`,
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "empty array",
			input: "[]",
			want:  nil,
		},
		{
			name:  "invalid JSON",
			input: "not json",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseViolations(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseViolations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluationResult_IsBlocked(t *testing.T) {
	tests := []struct {
		name   string
		result EvaluationResult
		want   bool
	}{
		{
			name:   "allowed",
			result: EvaluationResult{Allowed: true, Denied: false},
			want:   false,
		},
		{
			name:   "denied",
			result: EvaluationResult{Allowed: false, Denied: true},
			want:   true,
		},
		{
			name:   "violations present",
			result: EvaluationResult{Allowed: true, Violations: []string{"violation"}},
			want:   true,
		},
		{
			name:   "warnings only",
			result: EvaluationResult{Allowed: true, Warnings: []string{"warning"}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsBlocked(); got != tt.want {
				t.Errorf("IsBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}
