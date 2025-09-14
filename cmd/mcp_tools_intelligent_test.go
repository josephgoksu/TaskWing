package cmd

import (
	"context"
	"testing"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// seedIntelligentTasks adds sample tasks for intelligent MCP tests
func seedIntelligentTasks(t *testing.T) {
	t.Helper()
	st, err := GetStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Clear any existing
	_ = st.DeleteAllTasks()

	must := func(task models.Task) {
		if _, err := st.CreateTask(task); err != nil {
			t.Fatalf("seed create: %v", err)
		}
	}

	must(models.Task{Title: "Build login screen", Description: "UI for login", Status: models.StatusTodo, Priority: models.PriorityHigh})
	must(models.Task{Title: "Implement auth backend", Description: "API endpoints", Status: models.StatusDoing, Priority: models.PriorityMedium})
	must(models.Task{Title: "Write tests for auth", Description: "unit tests", Status: models.StatusTodo, Priority: models.PriorityHigh})
}

func TestQueryTasks_NaturalLanguage(t *testing.T) {
	_ = SetupTestProject(t)
	seedIntelligentTasks(t)

	st, _ := GetStore()
	defer func() { _ = st.Close() }()
	handler := queryTasksHandler(st)
	params := &mcp.CallToolParamsFor[types.FilterTasksParams]{
		Arguments: types.FilterTasksParams{
			Query:      "high priority unfinished",
			FuzzyMatch: true,
			Limit:      10,
		},
	}
	res, err := handler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("query handler error: %v", err)
	}
	if res.StructuredContent.Count == 0 {
		t.Fatalf("expected some results, got 0")
	}
}

func TestResolveTaskReference_ExactAndFuzzy(t *testing.T) {
	_ = SetupTestProject(t)
	seedIntelligentTasks(t)

	// Fetch a task ID by listing tasks
	st, _ := GetStore()
	tasks, _ := st.ListTasks(nil, nil)
	if len(tasks) == 0 {
		t.Fatalf("no tasks after seed")
	}
	exactID := tasks[0].ID
	partial := exactID[:8]
	_ = st.Close()

	st2, _ := GetStore()
	defer func() { _ = st2.Close() }()
	handler := resolveTaskReferenceHandler(st2)

	// Exact ID
	paramsExact := &mcp.CallToolParamsFor[types.ResolveTaskReferenceParams]{
		Arguments: types.ResolveTaskReferenceParams{Reference: exactID, Exact: true},
	}
	rexact, err := handler(context.Background(), nil, paramsExact)
	if err != nil {
		t.Fatalf("resolve exact err: %v", err)
	}
	if !rexact.StructuredContent.Resolved {
		t.Fatalf("expected resolved for exact id")
	}

	// Fuzzy partial ID
	paramsFuzzy := &mcp.CallToolParamsFor[types.ResolveTaskReferenceParams]{
		Arguments: types.ResolveTaskReferenceParams{Reference: partial, Exact: false},
	}
	rfuzzy, err := handler(context.Background(), nil, paramsFuzzy)
	if err != nil {
		t.Fatalf("resolve fuzzy err: %v", err)
	}
	if rfuzzy.StructuredContent.Resolved == false && len(rfuzzy.StructuredContent.Matches) == 0 {
		t.Fatalf("expected matches for partial id")
	}
}
