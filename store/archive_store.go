package store

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "time"

    "github.com/gofrs/flock"
    "github.com/google/uuid"
    "github.com/josephgoksu/TaskWing/models"
)

// ArchiveStore defines the interface for archive persistence and search.
type ArchiveStore interface {
    Initialize(config map[string]string) error
    CreateFromTask(task models.Task, lessons string, tags []string) (models.ArchiveEntry, error)
    GetByID(id string) (models.ArchiveEntry, string, error)
    List() ([]models.ArchiveIndexItem, error)
    Search(query string, filters ArchiveSearchFilters) ([]models.ArchiveIndexItem, error)
    RestoreToTaskStore(id string, taskStore TaskStore) (models.Task, error)
    Export(path string) error
    Import(path string) error
    Purge(opts PurgeOptions) (PurgeResult, error)
    Close() error
}

// ArchiveSearchFilters contains optional filters for search.
type ArchiveSearchFilters struct {
    DateFrom *time.Time
    DateTo   *time.Time
    Tags     []string
    Assignee string // currently unused (no assignees in Task model)
}

// PurgeOptions controls retention behavior.
type PurgeOptions struct {
    DryRun       bool
    OlderThan    *time.Duration // e.g., 90*24h
    MaxTotalSize int64          // bytes, 0 to ignore
}

type PurgeResult struct {
    DryRun         bool
    FilesConsidered int
    FilesDeleted    int
    BytesFreed      int64
}

// FileArchiveStore is a file-based archive implementation using JSON files and an index.json
type FileArchiveStore struct {
    baseDir   string
    indexPath string
    flk       *flock.Flock
}

func NewFileArchiveStore() *FileArchiveStore {
    return &FileArchiveStore{}
}

func (s *FileArchiveStore) Initialize(config map[string]string) error {
    base, ok := config["archiveDir"]
    if !ok || base == "" {
        base = filepath.Join(".taskwing", "archive")
    }
    if err := os.MkdirAll(base, 0o755); err != nil {
        return fmt.Errorf("failed to create archive dir %s: %w", base, err)
    }
    s.baseDir = base
    s.indexPath = filepath.Join(base, "index.json")
    s.flk = flock.New(s.indexPath)
    // Ensure index exists
    if _, err := os.Stat(s.indexPath); errors.Is(err, os.ErrNotExist) {
        idx := models.ArchiveIndex{Archives: []models.ArchiveIndexItem{}}
        idx.Statistics.TotalArchives = 0
        idx.Statistics.TotalTasksArchived = 0
        if err := s.writeIndex(idx); err != nil {
            return err
        }
    }
    return nil
}

func (s *FileArchiveStore) Close() error {
    if s.flk != nil {
        return s.flk.Unlock()
    }
    return nil
}

func (s *FileArchiveStore) readIndex() (models.ArchiveIndex, error) {
    data, err := os.ReadFile(s.indexPath)
    if err != nil {
        return models.ArchiveIndex{}, fmt.Errorf("read index: %w", err)
    }
    var idx models.ArchiveIndex
    if len(data) == 0 {
        idx.Archives = []models.ArchiveIndexItem{}
        return idx, nil
    }
    if err := json.Unmarshal(data, &idx); err != nil {
        return models.ArchiveIndex{}, fmt.Errorf("parse index: %w", err)
    }
    return idx, nil
}

func (s *FileArchiveStore) writeIndex(idx models.ArchiveIndex) error {
    b, err := json.MarshalIndent(idx, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal index: %w", err)
    }
    if err := os.WriteFile(s.indexPath, b, 0o644); err != nil {
        return fmt.Errorf("write index: %w", err)
    }
    return nil
}

func slugify(title string) string {
    lower := strings.ToLower(strings.TrimSpace(title))
    // replace non-alphanumeric with hyphens
    re := regexp.MustCompile(`[^a-z0-9]+`)
    s := re.ReplaceAllString(lower, "-")
    s = strings.Trim(s, "-")
    if s == "" {
        s = "task"
    }
    if len(s) > 64 { // keep file paths readable
        // Truncate at word boundary if possible
        truncated := s[:64]
        lastDash := strings.LastIndex(truncated, "-")
        if lastDash > 40 { // Only use word boundary if we keep at least 40 chars
            s = truncated[:lastDash]
        } else {
            s = truncated
        }
        s = strings.Trim(s, "-")
    }
    return s
}

func (s *FileArchiveStore) entryPath(t time.Time, title, id string) (string, error) {
    y := t.Format("2006")
    m := t.Format("01")
    dirPath := filepath.Join(s.baseDir, y, m)
    if err := os.MkdirAll(dirPath, 0o755); err != nil {
        return "", fmt.Errorf("failed to create archive directory %s: %w", dirPath, err)
    }
    short := id
    if len(short) > 8 {
        short = id[:8]
    }
    name := fmt.Sprintf("%s_%s-%s.json", t.Format("2006-01-02"), slugify(title), short)
    return filepath.Join(s.baseDir, y, m, name), nil
}

func (s *FileArchiveStore) CreateFromTask(task models.Task, lessons string, tags []string) (models.ArchiveEntry, error) {
    if err := s.flk.Lock(); err != nil {
        return models.ArchiveEntry{}, fmt.Errorf("lock index: %w", err)
    }
    defer func() {
        if unlockErr := s.flk.Unlock(); unlockErr != nil {
            fmt.Fprintf(os.Stderr, "Warning: failed to unlock index: %v\n", unlockErr)
        }
    }()

    idx, err := s.readIndex()
    if err != nil {
        return models.ArchiveEntry{}, err
    }
    now := time.Now().UTC()
    entry := models.ArchiveEntry{
        ID:            uuid.NewString(),
        ArchivedAt:    now,
        TaskID:        task.ID,
        Title:         task.Title,
        Description:   task.Description,
        Priority:      task.Priority,
        CreatedAt:     task.CreatedAt,
        CompletedAt:   task.CompletedAt,
        Tags:          tags,
        LessonsLearned: strings.TrimSpace(lessons),
    }
    path, err := s.entryPath(now, task.Title, entry.ID)
    if err != nil {
        return models.ArchiveEntry{}, err
    }
    // persist entry
    if err := writeJSON(path, entry); err != nil {
        return models.ArchiveEntry{}, err
    }
    // update index
    item := models.ArchiveIndexItem{
        ID:         entry.ID,
        Date:       now.Format("2006-01-02"),
        Title:      task.Title,
        FilePath:   relPath(s.baseDir, path),
        Tags:       tags,
        Summary:    summarize(task.Description, 140),
        ArchivedAt: now,
    }
    idx.Archives = append(idx.Archives, item)
    // stable sort by ArchivedAt desc
    sort.SliceStable(idx.Archives, func(i, j int) bool { return idx.Archives[i].ArchivedAt.After(idx.Archives[j].ArchivedAt) })
    idx.Statistics.TotalArchives = len(idx.Archives)
    idx.Statistics.TotalTasksArchived = len(idx.Archives)
    if err := s.writeIndex(idx); err != nil {
        return models.ArchiveEntry{}, err
    }
    return entry, nil
}

func (s *FileArchiveStore) GetByID(id string) (models.ArchiveEntry, string, error) {
    if err := s.flk.Lock(); err != nil {
        return models.ArchiveEntry{}, "", fmt.Errorf("lock index: %w", err)
    }
    defer func() {
        if unlockErr := s.flk.Unlock(); unlockErr != nil {
            // Log unlock errors but don't override the main error
            fmt.Fprintf(os.Stderr, "Warning: failed to unlock index: %v\n", unlockErr)
        }
    }()
    idx, err := s.readIndex()
    if err != nil {
        return models.ArchiveEntry{}, "", err
    }
    for _, it := range idx.Archives {
        if it.ID == id || strings.HasPrefix(it.ID, id) {
            abs := filepath.Join(s.baseDir, it.FilePath)
            var e models.ArchiveEntry
            if err := readJSON(abs, &e); err != nil {
                return models.ArchiveEntry{}, "", err
            }
            return e, abs, nil
        }
    }
    return models.ArchiveEntry{}, "", fmt.Errorf("archive id not found: %s", id)
}

func (s *FileArchiveStore) List() ([]models.ArchiveIndexItem, error) {
    if err := s.flk.Lock(); err != nil {
        return nil, fmt.Errorf("lock index: %w", err)
    }
    defer func() {
        if unlockErr := s.flk.Unlock(); unlockErr != nil {
            fmt.Fprintf(os.Stderr, "Warning: failed to unlock index: %v\n", unlockErr)
        }
    }()
    idx, err := s.readIndex()
    if err != nil {
        return nil, err
    }
    return idx.Archives, nil
}

func (s *FileArchiveStore) Search(query string, filters ArchiveSearchFilters) ([]models.ArchiveIndexItem, error) {
    items, err := s.List()
    if err != nil {
        return nil, err
    }
    q := strings.ToLower(strings.TrimSpace(query))
    match := func(item models.ArchiveIndexItem) bool {
        // Filter by date
        if filters.DateFrom != nil && item.ArchivedAt.Before(*filters.DateFrom) {
            return false
        }
        if filters.DateTo != nil && item.ArchivedAt.After(*filters.DateTo) {
            return false
        }
        // Tags filter: require any match if provided
        if len(filters.Tags) > 0 {
            any := false
            for _, t := range item.Tags {
                for _, ft := range filters.Tags {
                    if strings.EqualFold(t, ft) { any = true; break }
                }
                if any { break }
            }
            if !any { return false }
        }
        if q == "" {
            return true
        }
        // quick check on index fields
        if strings.Contains(strings.ToLower(item.Title), q) || strings.Contains(strings.ToLower(item.Summary), q) {
            return true
        }
        // read entry for deeper search
        var e models.ArchiveEntry
        if err := readJSON(filepath.Join(s.baseDir, item.FilePath), &e); err == nil {
            if strings.Contains(strings.ToLower(e.Description), q) || strings.Contains(strings.ToLower(e.LessonsLearned), q) {
                return true
            }
        }
        return false
    }

    out := make([]models.ArchiveIndexItem, 0, len(items))
    for _, it := range items {
        if match(it) {
            out = append(out, it)
        }
    }
    // keep the same sort order as index (newest first)
    return out, nil
}

func (s *FileArchiveStore) RestoreToTaskStore(id string, taskStore TaskStore) (models.Task, error) {
    e, _, err := s.GetByID(id)
    if err != nil {
        return models.Task{}, err
    }
    // Re-create a new task from archive snapshot (status: todo)
    t := models.Task{
        Title:              e.Title,
        Description:        e.Description,
        AcceptanceCriteria: "",
        Status:             models.StatusTodo,
        ParentID:           nil,
        SubtaskIDs:         []string{},
        Dependencies:       []string{},
        Dependents:         []string{},
        Priority:           e.Priority,
        CreatedAt:          time.Now().UTC(),
        UpdatedAt:          time.Now().UTC(),
    }
    created, err := taskStore.CreateTask(t)
    if err != nil {
        return models.Task{}, fmt.Errorf("restore create task: %w", err)
    }
    return created, nil
}

func (s *FileArchiveStore) Export(path string) error {
    items, err := s.List()
    if err != nil { return err }
    // Pack all entries + index into a single JSON file for portability
    type exportBundle struct {
        Index   []models.ArchiveIndexItem `json:"index"`
        Entries []models.ArchiveEntry     `json:"entries"`
        ExportedAt time.Time              `json:"exportedAt"`
        Version string                    `json:"version"`
    }
    bundle := exportBundle{Index: items, Entries: []models.ArchiveEntry{}, ExportedAt: time.Now().UTC(), Version: "1"}
    for _, it := range items {
        var e models.ArchiveEntry
        if err := readJSON(filepath.Join(s.baseDir, it.FilePath), &e); err == nil {
            bundle.Entries = append(bundle.Entries, e)
        }
    }
    return writeJSON(path, bundle)
}

func (s *FileArchiveStore) Import(path string) error {
    // Accept bundle exported by Export
    var bundle struct{
        Index   []models.ArchiveIndexItem `json:"index"`
        Entries []models.ArchiveEntry     `json:"entries"`
    }
    if err := readJSON(path, &bundle); err != nil {
        return fmt.Errorf("import read bundle: %w", err)
    }
    if err := s.flk.Lock(); err != nil { return fmt.Errorf("lock index: %w", err) }
    defer func(){
        if unlockErr := s.flk.Unlock(); unlockErr != nil {
            fmt.Fprintf(os.Stderr, "Warning: failed to unlock index: %v\n", unlockErr)
        }
    }()

    idx, err := s.readIndex()
    if err != nil { return err }
    // write entries and append to index (skip duplicates by id)
    existing := map[string]bool{}
    for _, it := range idx.Archives { existing[it.ID] = true }
    for _, e := range bundle.Entries {
        if existing[e.ID] { continue }
        p, err := s.entryPath(e.ArchivedAt, e.Title, e.ID)
        if err != nil { return err }
        if err := writeJSON(p, e); err != nil { return err }
        item := models.ArchiveIndexItem{
            ID: e.ID, Date: e.ArchivedAt.Format("2006-01-02"), Title: e.Title, FilePath: relPath(s.baseDir, p), Tags: e.Tags, Summary: summarize(e.Description, 140), ArchivedAt: e.ArchivedAt,
        }
        idx.Archives = append(idx.Archives, item)
    }
    sort.SliceStable(idx.Archives, func(i, j int) bool { return idx.Archives[i].ArchivedAt.After(idx.Archives[j].ArchivedAt) })
    idx.Statistics.TotalArchives = len(idx.Archives)
    idx.Statistics.TotalTasksArchived = len(idx.Archives)
    return s.writeIndex(idx)
}

func (s *FileArchiveStore) Purge(opts PurgeOptions) (PurgeResult, error) {
    // Age-based deletion via index
    res := PurgeResult{DryRun: opts.DryRun}
    if err := s.flk.Lock(); err != nil { return res, fmt.Errorf("lock index: %w", err) }
    defer func(){
        if unlockErr := s.flk.Unlock(); unlockErr != nil {
            fmt.Fprintf(os.Stderr, "Warning: failed to unlock index: %v\n", unlockErr)
        }
    }()
    idx, err := s.readIndex()
    if err != nil { return res, err }
    kept := make([]models.ArchiveIndexItem, 0, len(idx.Archives))
    for _, it := range idx.Archives {
        res.FilesConsidered++
        deleteFile := false
        if opts.OlderThan != nil {
            cutoff := time.Now().UTC().Add(-*opts.OlderThan)
            if it.ArchivedAt.Before(cutoff) { deleteFile = true }
        }
        // MaxTotalSize handled after age pass (simple approach)
        if deleteFile {
            if !opts.DryRun {
                abs := filepath.Join(s.baseDir, it.FilePath)
                fi, _ := os.Stat(abs)
                if err := os.Remove(abs); err == nil && fi != nil {
                    res.FilesDeleted++
                    res.BytesFreed += fi.Size()
                }
            }
            continue
        }
        kept = append(kept, it)
    }
    // Size-based: if still over, remove oldest until under
    if opts.MaxTotalSize > 0 {
        // accumulate size of kept
        type fileWithSize struct { idx int; size int64 }
        sizes := make([]fileWithSize, 0, len(kept))
        var total int64
        for i, it := range kept {
            abs := filepath.Join(s.baseDir, it.FilePath)
            if fi, err := os.Stat(abs); err == nil {
                sizes = append(sizes, fileWithSize{i, fi.Size()})
                total += fi.Size()
            }
        }
        if total > opts.MaxTotalSize {
            // remove from end (oldest due to sort order)
            for total > opts.MaxTotalSize && len(kept) > 0 {
                victim := kept[len(kept)-1]
                abs := filepath.Join(s.baseDir, victim.FilePath)
                var sz int64
                if fi, err := os.Stat(abs); err == nil { sz = fi.Size() }
                if !opts.DryRun { _ = os.Remove(abs) }
                res.FilesDeleted++
                res.BytesFreed += sz
                kept = kept[:len(kept)-1]
                total -= sz
            }
        }
    }
    if !opts.DryRun {
        idx.Archives = kept
        idx.Statistics.TotalArchives = len(kept)
        idx.Statistics.TotalTasksArchived = len(kept)
        if err := s.writeIndex(idx); err != nil { return res, err }
    }
    return res, nil
}

// helpers
func summarize(s string, n int) string {
    s = strings.TrimSpace(s)
    if len(s) <= n { return s }
    return s[:n] + "…"
}

func relPath(base, p string) string {
    if r, err := filepath.Rel(base, p); err == nil { return r }
    return p
}

func writeJSON(path string, v any) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return err }
    b, err := json.MarshalIndent(v, "", "  ")
    if err != nil { return err }
    return os.WriteFile(path, b, 0o644)
}

func readJSON(path string, v any) error {
    b, err := os.ReadFile(path)
    if err != nil { return err }
    return json.Unmarshal(b, v)
}

