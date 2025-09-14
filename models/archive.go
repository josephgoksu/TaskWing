package models

import "time"

// ArchiveEntry represents a snapshot of a completed task with enriched metadata.
type ArchiveEntry struct {
	ID          string       `json:"id"`
	ArchivedAt  time.Time    `json:"archivedAt"`
	TaskID      string       `json:"taskId"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Priority    TaskPriority `json:"priority"`
	CreatedAt   time.Time    `json:"createdAt"`
	CompletedAt *time.Time   `json:"completedAt,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Assignees   []string     `json:"assignees,omitempty"`
	// Free text learnings captured at completion time
	LessonsLearned string `json:"lessonsLearned,omitempty"`
	// Optional artifact references
	Artifacts struct {
		Files      []string `json:"files,omitempty"`
		URLs       []string `json:"urls,omitempty"`
		CLIOutputs []string `json:"cliOutputs,omitempty"`
	} `json:"artifacts,omitempty"`
}

// ArchiveIndex summarizes archive entries for fast listing and basic search.
type ArchiveIndex struct {
	Archives   []ArchiveIndexItem `json:"archives"`
	Statistics struct {
		TotalArchives      int `json:"totalArchives"`
		TotalTasksArchived int `json:"totalTasksArchived"`
	} `json:"statistics"`
}

// ArchiveIndexItem is a compact record of an archive entry stored on disk.
type ArchiveIndexItem struct {
	ID         string    `json:"id"`
	Date       string    `json:"date"`
	Title      string    `json:"title"`
	FilePath   string    `json:"filePath"`
	Tags       []string  `json:"tags,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	ArchivedAt time.Time `json:"archivedAt"`
}
