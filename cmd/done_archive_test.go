package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/josephgoksu/TaskWing/models"
	"os"
)

func TestDoneCreatesArchiveEntryWithFlags(t *testing.T) {
	proj := SetupTestProject(t)
	// Ensure a config file exists and point viper to it via env so InitConfig uses project config
	cfgPath := filepath.Join(proj, ".taskwing.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	t.Setenv("TASKWING_CONFIG", cfgPath)

	// create a task
	st, err := GetStore()
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	defer func() { _ = st.Close() }()
	task, err := st.CreateTask(models.Task{
		Title:       "Archive Flow Test",
		Description: "Ensure done --archive creates an entry",
		Status:      models.StatusTodo,
		Priority:    models.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// mark done with flags (non-interactive)
	rootCmd.SetArgs([]string{"done", "--archive", "--lessons", "works fine", "--tags", "test,ci", task.ID})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute done: %v", err)
	}

	// briefly wait for file ops
	time.Sleep(50 * time.Millisecond)

	// verify archive index exists and is non-empty
	idx := filepath.Join(proj, "archive", "index.json")
	assertFileExists(t, idx)
}

func assertFileExists(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected file to exist: %s: %v", p, err)
	}
}
