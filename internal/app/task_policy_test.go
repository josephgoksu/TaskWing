package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// TestTaskComplete_PolicyEnforcement tests that policy violations block task completion.
func TestTaskComplete_PolicyEnforcement(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "taskwing-policy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .taskwing/policies directory
	policiesDir := filepath.Join(tmpDir, ".taskwing", "policies")
	if err := os.MkdirAll(policiesDir, 0755); err != nil {
		t.Fatalf("failed to create policies dir: %v", err)
	}

	// Create a test policy that blocks .env files
	testPolicy := `package taskwing.policy

import rego.v1

# Block environment files
deny contains msg if {
    some file in input.task.files_modified
    startswith(file, ".env")
    msg := sprintf("BLOCKED: Environment file '%s' is protected", [file])
}

# Block GOVERNANCE.md
deny contains msg if {
    some file in input.task.files_modified
    file == "GOVERNANCE.md"
    msg := "BLOCKED: GOVERNANCE.md is a protected file"
}
`
	policyPath := filepath.Join(policiesDir, "test.rego")
	if err := os.WriteFile(policyPath, []byte(testPolicy), 0644); err != nil {
		t.Fatalf("failed to write test policy: %v", err)
	}

	// Create .taskwing/memory directory for SQLite
	memoryDir := filepath.Join(tmpDir, ".taskwing", "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatalf("failed to create memory dir: %v", err)
	}

	// Initialize repository
	repo, err := memory.NewDefaultRepository(memoryDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer repo.Close()

	// Create a test plan and task
	testPlan := &task.Plan{
		ID:     "test-plan-001",
		Goal:   "Test policy enforcement",
		Status: task.PlanStatusActive,
	}
	if err := repo.CreatePlan(testPlan); err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	testTask := &task.Task{
		ID:          "test-task-001",
		PlanID:      testPlan.ID,
		Title:       "Test task",
		Description: "A test task for policy enforcement",
		Status:      task.StatusInProgress,
		Priority:    50,
	}
	if err := repo.CreateTask(testTask); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Change to temp directory so policy engine finds .taskwing/policies
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}
	defer os.Chdir(oldWd)

	// Create TaskApp
	appCtx := &Context{
		Repo:   repo,
		LLMCfg: llm.Config{},
	}
	taskApp := NewTaskApp(appCtx)

	tests := []struct {
		name          string
		filesModified []string
		expectSuccess bool
		expectMessage string
	}{
		{
			name:          "allowed_files_should_pass",
			filesModified: []string{"main.go", "README.md"},
			expectSuccess: true,
		},
		{
			name:          "env_file_should_be_blocked",
			filesModified: []string{"main.go", ".env"},
			expectSuccess: false,
			expectMessage: "BLOCKED: Environment file",
		},
		{
			name:          "env_local_should_be_blocked",
			filesModified: []string{".env.local"},
			expectSuccess: false,
			expectMessage: "BLOCKED: Environment file",
		},
		{
			name:          "governance_md_should_be_blocked",
			filesModified: []string{"GOVERNANCE.md"},
			expectSuccess: false,
			expectMessage: "BLOCKED: GOVERNANCE.md is a protected file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new task for each test case
			taskID := "task-" + tt.name
			newTask := &task.Task{
				ID:          taskID,
				PlanID:      testPlan.ID,
				Title:       "Test: " + tt.name,
				Description: "Test task",
				Status:      task.StatusInProgress,
				Priority:    50,
			}
			if err := repo.CreateTask(newTask); err != nil {
				t.Fatalf("failed to create task: %v", err)
			}

			// Attempt to complete the task
			result, err := taskApp.Complete(context.Background(), TaskCompleteOptions{
				TaskID:        taskID,
				Summary:       "Test completion",
				FilesModified: tt.filesModified,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("expected Success=%v, got %v. Message: %s", tt.expectSuccess, result.Success, result.Message)
			}

			if !tt.expectSuccess && tt.expectMessage != "" {
				if result.Message == "" || !contains(result.Message, tt.expectMessage) {
					t.Errorf("expected message to contain %q, got %q", tt.expectMessage, result.Message)
				}
			}

			// Verify task status in database
			taskFromDB, err := repo.GetTask(taskID)
			if err != nil {
				t.Fatalf("failed to get task: %v", err)
			}

			if tt.expectSuccess {
				if taskFromDB.Status != task.StatusCompleted {
					t.Errorf("expected task status %s, got %s", task.StatusCompleted, taskFromDB.Status)
				}
			} else {
				if taskFromDB.Status != task.StatusInProgress {
					t.Errorf("expected task status %s (unchanged), got %s", task.StatusInProgress, taskFromDB.Status)
				}
			}
		})
	}
}

// contains checks if s contains substr (case-sensitive).
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
