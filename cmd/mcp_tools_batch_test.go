package cmd

import (
    "context"
    "testing"

    "github.com/josephgoksu/TaskWing/models"
    "github.com/josephgoksu/TaskWing/types"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBatchCreate_WithTempIDs_ResolvesDependenciesAndParents(t *testing.T) {
    _ = setupTestProject(t)
    st, err := GetStore()
    if err != nil { t.Fatalf("store: %v", err) }
    defer func() { _ = st.Close() }()
    _ = st.DeleteAllTasks()

    handler := batchCreateTasksHandler(st)

    params := &mcp.CallToolParamsFor[types.BatchCreateTasksParams]{
        Arguments: types.BatchCreateTasksParams{Tasks: []types.TaskCreationRequest{
            {TempID: 1, Title: "Root A", Description: ""},
            {TempID: 2, Title: "Depends on A", Description: "", Dependencies: []string{"1"}},
            {TempID: 3, Title: "Child of A", Description: "", ParentID: "1"},
        }},
    }

    res, err := handler(context.Background(), nil, params)
    if err != nil { t.Fatalf("batch-create error: %v", err) }

    if res.StructuredContent.Success != 3 || len(res.StructuredContent.Failed) != 0 || len(res.StructuredContent.Errors) != 0 {
        t.Fatalf("unexpected response: %+v", res.StructuredContent)
    }

    // Verify relationships in store
    all, err := st.ListTasks(nil, nil)
    if err != nil { t.Fatalf("list: %v", err) }

    var aID, depID, childID string
    for _, tsk := range all {
        switch tsk.Title {
        case "Root A":
            aID = tsk.ID
        case "Depends on A":
            depID = tsk.ID
        case "Child of A":
            childID = tsk.ID
        }
    }
    if aID == "" || depID == "" || childID == "" {
        t.Fatalf("missing created tasks: a=%s dep=%s child=%s", aID, depID, childID)
    }

    // Check that dependency was resolved to UUID, not the string "1"
    depTask, err := st.GetTask(depID)
    if err != nil { t.Fatalf("get dep: %v", err) }
    if len(depTask.Dependencies) != 1 || depTask.Dependencies[0] != aID {
        t.Fatalf("dependency not resolved: %+v (expected %s)", depTask.Dependencies, aID)
    }

    // Check back-reference (Dependents)
    aTask, err := st.GetTask(aID)
    if err != nil { t.Fatalf("get a: %v", err) }
    if len(aTask.Dependents) == 0 || aTask.Dependents[0] != depID {
        t.Fatalf("dependents not updated on A: %+v", aTask.Dependents)
    }

    // Check parent-child linkage
    childTask, err := st.GetTask(childID)
    if err != nil { t.Fatalf("get child: %v", err) }
    if childTask.ParentID == nil || *childTask.ParentID != aID {
        t.Fatalf("parent not linked on child: parent=%v expected=%s", childTask.ParentID, aID)
    }
    // And A lists child as subtask
    found := false
    for _, sid := range aTask.SubtaskIDs { if sid == childID { found = true; break } }
    if !found { t.Fatalf("A missing child subtask id: %+v", aTask.SubtaskIDs) }
}

func TestBatchCreate_RejectsPlaceholderDependency(t *testing.T) {
    _ = setupTestProject(t)
    st, err := GetStore()
    if err != nil { t.Fatalf("store: %v", err) }
    defer func() { _ = st.Close() }()
    _ = st.DeleteAllTasks()

    handler := batchCreateTasksHandler(st)

    params := &mcp.CallToolParamsFor[types.BatchCreateTasksParams]{
        Arguments: types.BatchCreateTasksParams{Tasks: []types.TaskCreationRequest{
            {TempID: 1, Title: "X", Description: ""},
            {TempID: 2, Title: "Y", Description: "", Dependencies: []string{"task_placeholder"}},
        }},
    }

    _, err = handler(context.Background(), nil, params)
    if err == nil {
        t.Fatalf("expected error for placeholder dependency, got nil")
    }
}

func TestBatchCreate_WithUUIDDependenciesAndParent(t *testing.T) {
    _ = setupTestProject(t)
    st, err := GetStore()
    if err != nil { t.Fatalf("store: %v", err) }
    defer func() { _ = st.Close() }()
    _ = st.DeleteAllTasks()

    // Seed an existing task to reference by UUID
    existing, err := st.CreateTask(models.Task{Title: "Existing P", Description: "", Status: models.StatusTodo, Priority: models.PriorityMedium})
    if err != nil { t.Fatalf("seed create: %v", err) }

    handler := batchCreateTasksHandler(st)

    params := &mcp.CallToolParamsFor[types.BatchCreateTasksParams]{
        Arguments: types.BatchCreateTasksParams{Tasks: []types.TaskCreationRequest{
            {TempID: 10, Title: "A depends on existing", Description: "", Dependencies: []string{existing.ID}},
            {TempID: 11, Title: "B child of existing", Description: "", ParentID: existing.ID},
        }},
    }

    res, err := handler(context.Background(), nil, params)
    if err != nil { t.Fatalf("batch-create error: %v", err) }
    if res.StructuredContent.Success != 2 {
        t.Fatalf("expected 2 created, got %+v", res.StructuredContent)
    }

    // Verify relationships
    // Find A and B
    all, _ := st.ListTasks(nil, nil)
    var aID, bID string
    for _, tsk := range all {
        switch tsk.Title {
        case "A depends on existing": aID = tsk.ID
        case "B child of existing": bID = tsk.ID
        }
    }
    if aID == "" || bID == "" { t.Fatalf("missing created tasks a=%s b=%s", aID, bID) }

    a, _ := st.GetTask(aID)
    if len(a.Dependencies) != 1 || a.Dependencies[0] != existing.ID {
        t.Fatalf("A dependencies incorrect: %+v", a.Dependencies)
    }
    // Existing should have A as dependent
    ex, _ := st.GetTask(existing.ID)
    foundDep := false
    for _, d := range ex.Dependents { if d == aID { foundDep = true; break } }
    if !foundDep { t.Fatalf("existing task missing dependent %s: %+v", aID, ex.Dependents) }

    // B should have parent existing
    b, _ := st.GetTask(bID)
    if b.ParentID == nil || *b.ParentID != existing.ID {
        t.Fatalf("B parent incorrect: %v", b.ParentID)
    }
    // Existing should have B in SubtaskIDs
    foundChild := false
    for _, sid := range ex.SubtaskIDs { if sid == bID { foundChild = true; break } }
    if !foundChild { t.Fatalf("existing missing child subtask id: %+v", ex.SubtaskIDs) }
}
