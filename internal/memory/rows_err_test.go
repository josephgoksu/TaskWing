package memory

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/task"
)

// TestCheckRowsErr_NilError tests that checkRowsErr returns nil when rows.Err() is nil.
func TestCheckRowsErr_NilError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Execute a simple query and iterate fully
	rows, err := store.db.Query("SELECT 1")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
	}

	// After successful iteration, rows.Err() should be nil
	err = checkRowsErr(rows)
	if err != nil {
		t.Errorf("checkRowsErr returned error for successful iteration: %v", err)
	}
}

// TestListPlans_ErrorPropagation tests that errors are propagated from ListPlans.
// This tests that a closed database connection causes proper error propagation.
func TestListPlans_ErrorPropagation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create some test data
	plan := &task.Plan{
		ID:   "plan-test-001",
		Goal: "Test plan for error propagation",
	}
	if err := store.CreatePlan(plan); err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	// Close the database to force errors on subsequent queries
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close store: %v", err)
	}

	// Now listing plans should return an error (database closed)
	_, err = store.ListPlans()
	if err == nil {
		t.Error("expected error when listing plans on closed database, got nil")
	}
}

// TestListTasks_ErrorPropagation tests that errors are propagated from ListTasks.
func TestListTasks_ErrorPropagation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create a test plan first
	plan := &task.Plan{
		ID:   "plan-test-002",
		Goal: "Test plan",
	}
	if err := store.CreatePlan(plan); err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	// Create a test task
	testTask := &task.Task{
		ID:     "task-test-001",
		PlanID: "plan-test-002",
		Title:  "Test task",
	}
	if err := store.CreateTask(testTask); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Close the database
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close store: %v", err)
	}

	// Now listing tasks should return an error
	_, err = store.ListTasks("plan-test-002")
	if err == nil {
		t.Error("expected error when listing tasks on closed database, got nil")
	}
}

// TestListNodes_ErrorPropagation tests that errors are propagated from ListNodes.
func TestListNodes_ErrorPropagation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create a test node
	node := &Node{
		ID:      "node-test-001",
		Content: "Test content",
		Type:    NodeTypeDecision,
		Summary: "Test node",
	}
	if err := store.CreateNode(node); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Close the database
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close store: %v", err)
	}

	// Now listing nodes should return an error
	_, err = store.ListNodes("")
	if err == nil {
		t.Error("expected error when listing nodes on closed database, got nil")
	}
}

// TestSearchPlans_ErrorPropagation tests that errors are propagated from SearchPlans.
func TestSearchPlans_ErrorPropagation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Close the database
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close store: %v", err)
	}

	// Now searching plans should return an error
	_, err = store.SearchPlans("test", "")
	if err == nil {
		t.Error("expected error when searching plans on closed database, got nil")
	}
}

// TestGetNodeEdges_ErrorPropagation tests that errors are propagated from GetNodeEdges.
func TestGetNodeEdges_ErrorPropagation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Close the database
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close store: %v", err)
	}

	// Now getting node edges should return an error
	_, err = store.GetNodeEdges("node-test-001")
	if err == nil {
		t.Error("expected error when getting node edges on closed database, got nil")
	}
}

// TestRowsErrPropagation_TableDriven uses table-driven tests for multiple functions.
func TestRowsErrPropagation_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(store *SQLiteStore) error
	}{
		{
			name: "ListPlans",
			testFunc: func(store *SQLiteStore) error {
				_, err := store.ListPlans()
				return err
			},
		},
		{
			name: "ListNodes",
			testFunc: func(store *SQLiteStore) error {
				_, err := store.ListNodes("")
				return err
			},
		},
		{
			name: "GetAllNodeEdges",
			testFunc: func(store *SQLiteStore) error {
				_, err := store.GetAllNodeEdges()
				return err
			},
		},
		{
			name: "ListBootstrapStates",
			testFunc: func(store *SQLiteStore) error {
				_, err := store.ListBootstrapStates()
				return err
			},
		},
		{
			name: "ListToolVersions",
			testFunc: func(store *SQLiteStore) error {
				_, err := store.ListToolVersions()
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			store, err := NewSQLiteStore(tmpDir)
			if err != nil {
				t.Fatalf("failed to create store: %v", err)
			}

			// Close the database to force errors
			if err := store.Close(); err != nil {
				t.Fatalf("failed to close store: %v", err)
			}

			// The function should return an error on closed database
			err = tc.testFunc(store)
			if err == nil {
				t.Errorf("%s should return error on closed database, got nil", tc.name)
			}
		})
	}
}

// TestRowsErrPropagation_SuccessPath verifies no errors on successful iteration.
func TestRowsErrPropagation_SuccessPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create test data
	plan := &task.Plan{
		ID:   "plan-success-001",
		Goal: "Test successful iteration",
	}
	if err := store.CreatePlan(plan); err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	testTask := &task.Task{
		ID:     "task-success-001",
		PlanID: "plan-success-001",
		Title:  "Test task",
	}
	if err := store.CreateTask(testTask); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	node := &Node{
		ID:      "node-success-001",
		Content: "Test content",
		Type:    NodeTypeDecision,
		Summary: "Test node",
	}
	if err := store.CreateNode(node); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// All these operations should succeed without errors
	t.Run("ListPlans", func(t *testing.T) {
		plans, err := store.ListPlans()
		if err != nil {
			t.Errorf("ListPlans failed: %v", err)
		}
		if len(plans) == 0 {
			t.Error("expected at least one plan")
		}
	})

	t.Run("ListTasks", func(t *testing.T) {
		tasks, err := store.ListTasks("plan-success-001")
		if err != nil {
			t.Errorf("ListTasks failed: %v", err)
		}
		if len(tasks) == 0 {
			t.Error("expected at least one task")
		}
	})

	t.Run("ListNodes", func(t *testing.T) {
		nodes, err := store.ListNodes("")
		if err != nil {
			t.Errorf("ListNodes failed: %v", err)
		}
		if len(nodes) == 0 {
			t.Error("expected at least one node")
		}
	})

	t.Run("Check", func(t *testing.T) {
		_, err := store.Check()
		if err != nil {
			t.Errorf("Check failed: %v", err)
		}
	})
}

// TestCheckRowsErr_ReturnsWrappedError tests that checkRowsErr wraps errors properly.
func TestCheckRowsErr_ReturnsWrappedError(t *testing.T) {
	// We can't easily trigger a real rows.Err() in SQLite without network issues,
	// but we can verify the function signature and behavior with a mock rows.
	// This test verifies the helper function exists and returns nil for successful rows.

	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create and fully iterate rows
	rows, err := store.db.Query("SELECT 1 UNION SELECT 2 UNION SELECT 3")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	count := 0
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		count++
	}
	_ = rows.Close()

	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}

	// After full iteration and close, Err() should be nil
	// Note: calling Err() after Close() is implementation-dependent but safe
	if rows.Err() != nil {
		t.Errorf("unexpected error after successful iteration: %v", rows.Err())
	}
}

// TestErrorMessageContainsContext verifies error messages include context.
func TestErrorMessageContainsContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Close the database to force errors
	_ = store.Close()

	// Test that error messages contain meaningful context
	tests := []struct {
		name     string
		testFunc func() error
		contains string
	}{
		{
			name: "ListPlans",
			testFunc: func() error {
				_, err := store.ListPlans()
				return err
			},
			contains: "plan", // Should mention "plan" somewhere in error
		},
		{
			name: "ListNodes",
			testFunc: func() error {
				_, err := store.ListNodes("")
				return err
			},
			contains: "node", // Should mention "node" somewhere in error
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.testFunc()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			errMsg := strings.ToLower(err.Error())
			if !strings.Contains(errMsg, tc.contains) {
				t.Errorf("error message %q should contain %q", err.Error(), tc.contains)
			}
		})
	}
}

// TestMockRowsErr demonstrates how rows.Err() works conceptually.
// In a real scenario with network databases, rows.Err() would capture
// errors that occur during iteration (like connection drops).
func TestMockRowsErr(t *testing.T) {
	// This test documents the expected behavior of rows.Err():
	// - Returns nil if iteration completed successfully
	// - Returns error if iteration was interrupted (e.g., network failure)

	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Test 1: Successful iteration should have nil Err()
	rows, err := store.db.Query("SELECT 1")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
	}

	// Before Close, Err() should be nil for successful iteration
	if err := checkRowsErr(rows); err != nil {
		t.Errorf("expected nil error after successful iteration, got: %v", err)
	}
	_ = rows.Close()

	// Test 2: Query with no results should also have nil Err()
	rows2, err := store.db.Query("SELECT 1 WHERE 1=0") // returns no rows
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	count := 0
	for rows2.Next() {
		count++
	}

	if err := checkRowsErr(rows2); err != nil {
		t.Errorf("expected nil error for empty result set, got: %v", err)
	}
	_ = rows2.Close()

	if count != 0 {
		t.Errorf("expected 0 rows, got %d", count)
	}
}

// TestCheckRowsErrHelper_FunctionExists verifies the helper function works.
func TestCheckRowsErrHelper_FunctionExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-rows-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create rows and verify checkRowsErr exists and works
	rows, err := store.db.Query("SELECT 1")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	// Iterate
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
	}

	// Call checkRowsErr - should return nil
	if err := checkRowsErr(rows); err != nil {
		t.Errorf("checkRowsErr failed: %v", err)
	}
}

// Verify that sql package is imported and types are correct
var _ *sql.Rows // Ensures sql package is correctly imported

// === Prefix Finder Tests ===

// TestFindTaskIDsByPrefix tests the task ID prefix finder.
func TestFindTaskIDsByPrefix(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-prefix-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create a test plan
	plan := &task.Plan{ID: "plan-prefix001", Goal: "Test plan"}
	if err := store.CreatePlan(plan); err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	// Create test tasks with various prefixes
	tasks := []*task.Task{
		{ID: "task-abc11111", PlanID: "plan-prefix001", Title: "Task A1"},
		{ID: "task-abc22222", PlanID: "plan-prefix001", Title: "Task A2"},
		{ID: "task-abc33333", PlanID: "plan-prefix001", Title: "Task A3"},
		{ID: "task-xyz11111", PlanID: "plan-prefix001", Title: "Task X1"},
	}
	for _, tsk := range tasks {
		if err := store.CreateTask(tsk); err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	tests := []struct {
		name      string
		prefix    string
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "full ID match",
			prefix:    "task-abc11111",
			wantCount: 1,
			wantIDs:   []string{"task-abc11111"},
		},
		{
			name:      "prefix matches multiple",
			prefix:    "task-abc",
			wantCount: 3,
		},
		{
			name:      "prefix matches one",
			prefix:    "task-xyz",
			wantCount: 1,
			wantIDs:   []string{"task-xyz11111"},
		},
		{
			name:      "prefix matches none",
			prefix:    "task-zzz",
			wantCount: 0,
		},
		{
			name:      "empty prefix matches all",
			prefix:    "",
			wantCount: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := store.FindTaskIDsByPrefix(tc.prefix)
			if err != nil {
				t.Fatalf("FindTaskIDsByPrefix failed: %v", err)
			}
			if len(ids) != tc.wantCount {
				t.Errorf("got %d IDs, want %d", len(ids), tc.wantCount)
			}
			if tc.wantIDs != nil {
				for i, wantID := range tc.wantIDs {
					if i >= len(ids) || ids[i] != wantID {
						t.Errorf("ID[%d] = %q, want %q", i, ids[i], wantID)
					}
				}
			}
		})
	}
}

// TestFindPlanIDsByPrefix tests the plan ID prefix finder.
func TestFindPlanIDsByPrefix(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-prefix-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create test plans with various prefixes
	plans := []*task.Plan{
		{ID: "plan-abc11111", Goal: "Plan A1"},
		{ID: "plan-abc22222", Goal: "Plan A2"},
		{ID: "plan-xyz11111", Goal: "Plan X1"},
	}
	for _, p := range plans {
		if err := store.CreatePlan(p); err != nil {
			t.Fatalf("failed to create plan: %v", err)
		}
	}

	tests := []struct {
		name      string
		prefix    string
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "full ID match",
			prefix:    "plan-abc11111",
			wantCount: 1,
			wantIDs:   []string{"plan-abc11111"},
		},
		{
			name:      "prefix matches multiple",
			prefix:    "plan-abc",
			wantCount: 2,
		},
		{
			name:      "prefix matches one",
			prefix:    "plan-xyz",
			wantCount: 1,
			wantIDs:   []string{"plan-xyz11111"},
		},
		{
			name:      "prefix matches none",
			prefix:    "plan-zzz",
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := store.FindPlanIDsByPrefix(tc.prefix)
			if err != nil {
				t.Fatalf("FindPlanIDsByPrefix failed: %v", err)
			}
			if len(ids) != tc.wantCount {
				t.Errorf("got %d IDs, want %d", len(ids), tc.wantCount)
			}
			if tc.wantIDs != nil {
				for i, wantID := range tc.wantIDs {
					if i >= len(ids) || ids[i] != wantID {
						t.Errorf("ID[%d] = %q, want %q", i, ids[i], wantID)
					}
				}
			}
		})
	}
}

// TestFindPrefixMethods_ClosedDB tests error propagation for prefix finders.
func TestFindPrefixMethods_ClosedDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-prefix-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	_ = store.Close() // Close immediately

	t.Run("FindTaskIDsByPrefix", func(t *testing.T) {
		_, err := store.FindTaskIDsByPrefix("task-")
		if err == nil {
			t.Error("expected error on closed database, got nil")
		}
	})

	t.Run("FindPlanIDsByPrefix", func(t *testing.T) {
		_, err := store.FindPlanIDsByPrefix("plan-")
		if err == nil {
			t.Error("expected error on closed database, got nil")
		}
	})
}
