package store

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/josephgoksu/TaskWing/models"
)

func TestArchiveStore_CreateListSearchExportImport(t *testing.T) {
    tmp := t.TempDir()
    archDir := filepath.Join(tmp, "archive")
    s := NewFileArchiveStore()
    if err := s.Initialize(map[string]string{"archiveDir": archDir}); err != nil {
        t.Fatalf("init archive store: %v", err)
    }
    defer s.Close()

    // Create a dummy completed task
    now := time.Now().UTC()
    comp := now.Add(-time.Hour)
    task := models.Task{
        ID: "00000000-0000-0000-0000-000000000000",
        Title: "Fix login bug",
        Description: "Resolved issue with token refresh",
        Status: models.StatusDone,
        Priority: models.PriorityHigh,
        CreatedAt: now.Add(-24 * time.Hour),
        UpdatedAt: now,
        CompletedAt: &comp,
    }

    entry, err := s.CreateFromTask(task, "Validate tokens early", []string{"bugfix", "auth"})
    if err != nil { t.Fatalf("create from task: %v", err) }
    if entry.ID == "" { t.Fatal("entry id empty") }

    items, err := s.List()
    if err != nil { t.Fatalf("list: %v", err) }
    if len(items) != 1 { t.Fatalf("expected 1 item, got %d", len(items)) }

    got, _, err := s.GetByID(entry.ID[:8])
    if err != nil { t.Fatalf("get by id prefix: %v", err) }
    if got.Title != task.Title { t.Fatalf("title mismatch: got %q", got.Title) }

    // Search by keyword
    res, err := s.Search("token", ArchiveSearchFilters{})
    if err != nil { t.Fatalf("search: %v", err) }
    if len(res) != 1 { t.Fatalf("expected 1 search result, got %d", len(res)) }

    // Export to bundle
    bundle := filepath.Join(tmp, "bundle.json")
    if err := s.Export(bundle); err != nil { t.Fatalf("export: %v", err) }
    if _, err := os.Stat(bundle); err != nil { t.Fatalf("bundle not written: %v", err) }

    // Import into a fresh store
    tmp2 := t.TempDir()
    s2 := NewFileArchiveStore()
    if err := s2.Initialize(map[string]string{"archiveDir": filepath.Join(tmp2, "archive")}); err != nil { t.Fatalf("init s2: %v", err) }
    defer s2.Close()
    if err := s2.Import(bundle); err != nil { t.Fatalf("import: %v", err) }
    items2, err := s2.List()
    if err != nil { t.Fatalf("list s2: %v", err) }
    if len(items2) != 1 { t.Fatalf("expected 1 item in s2, got %d", len(items2)) }
}

func TestArchiveStore_Purge_DryRun(t *testing.T) {
    tmp := t.TempDir()
    s := NewFileArchiveStore()
    if err := s.Initialize(map[string]string{"archiveDir": filepath.Join(tmp, "archive")}); err != nil {
        t.Fatalf("init: %v", err)
    }
    defer s.Close()
    // one old, one new
    now := time.Now().UTC()
    oldCompleted := now.Add(-90 * 24 * time.Hour)
    oldTask := models.Task{ID: "1", Title: "Old", Status: models.StatusDone, Priority: models.PriorityLow, CreatedAt: oldCompleted.Add(-2*time.Hour), UpdatedAt: oldCompleted, CompletedAt: &oldCompleted}
    if _, err := s.CreateFromTask(oldTask, "", nil); err != nil { t.Fatalf("create old: %v", err) }

    recentCompleted := now.Add(-24 * time.Hour)
    newTask := models.Task{ID: "2", Title: "New", Status: models.StatusDone, Priority: models.PriorityLow, CreatedAt: recentCompleted.Add(-2*time.Hour), UpdatedAt: recentCompleted, CompletedAt: &recentCompleted}
    if _, err := s.CreateFromTask(newTask, "", nil); err != nil { t.Fatalf("create new: %v", err) }

    d := 60 * 24 * time.Hour
    res, err := s.Purge(PurgeOptions{DryRun: true, OlderThan: &d})
    if err != nil { t.Fatalf("purge dry-run: %v", err) }
    if !res.DryRun { t.Fatal("expected dry-run true") }
    // nothing actually deleted
    items, _ := s.List()
    if len(items) != 2 { t.Fatalf("expected 2 items after dry-run, got %d", len(items)) }
}

