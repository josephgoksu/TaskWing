package mcp

import (
	"context"
	"testing"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/types"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func seedSimple(t *testing.T, st interface {
	DeleteAllTasks() error
	CreateTask(models.Task) (models.Task, error)
},
) {
	t.Helper()
	_ = st.DeleteAllTasks()
	// Two TODOs and one DOING
	_, _ = st.CreateTask(models.Task{Title: "Task A", Description: "", Status: models.StatusTodo, Priority: models.PriorityMedium})
	_, _ = st.CreateTask(models.Task{Title: "Task B", Description: "", Status: models.StatusTodo, Priority: models.PriorityMedium})
	_, _ = st.CreateTask(models.Task{Title: "Task C", Description: "", Status: models.StatusDoing, Priority: models.PriorityHigh})
}

func TestBulkByFilter_PreviewAndConfirm_Complete(t *testing.T) {
	_ = SetupTestProject(t)
	st, _ := GetStore()
	defer func() { _ = st.Close() }()
	seedSimple(t, st)
	t.Logf("tasks file: %s", GetTaskFilePath())
	if tasks, _ := st.ListTasks(nil, nil); len(tasks) == 0 {
		t.Fatalf("no tasks after seeding (path=%s)", GetTaskFilePath())
	}
	handler := bulkByFilterHandler(st)

	// Sanity: ensure seeded tasks present
	if tasks, _ := st.ListTasks(nil, nil); len(tasks) == 0 {
		t.Fatalf("no tasks after seeding")
	}

	// Preview: match TODO tasks
	prevParams := &mcpsdk.CallToolParamsFor[types.BulkByFilterParams]{
		Arguments: types.BulkByFilterParams{Filter: "status=todo", Action: "complete", PreviewOnly: true},
	}
	prev, err := handler(context.Background(), nil, prevParams)
	if err != nil {
		t.Fatalf("preview err: %v", err)
	}
	if !prev.StructuredContent.Preview || prev.StructuredContent.Matched == 0 {
		t.Fatalf("expected preview with matches, got %+v", prev.StructuredContent)
	}

	// Confirm
	confParams := &mcpsdk.CallToolParamsFor[types.BulkByFilterParams]{
		Arguments: types.BulkByFilterParams{Filter: "status=todo", Action: "complete", Confirm: true},
	}
	conf, err := handler(context.Background(), nil, confParams)
	if err != nil {
		t.Fatalf("confirm err: %v", err)
	}
	if conf.StructuredContent.Acted == 0 {
		t.Fatalf("expected acted > 0, got %+v", conf.StructuredContent)
	}

	// Verify status updated
	stVerify, _ := GetStore()
	tasks, _ := stVerify.ListTasks(nil, nil)
	_ = stVerify.Close()
	doneCount := 0
	for _, tsk := range tasks {
		if tsk.Status == models.StatusDone {
			doneCount++
		}
	}
	if doneCount < conf.StructuredContent.Acted {
		t.Fatalf("expected at least %d done, got %d", conf.StructuredContent.Acted, doneCount)
	}
}
