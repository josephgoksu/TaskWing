/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package agents provides activity logging for watch mode.
*/
package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ActivityLog records watch mode activity
type ActivityLog struct {
	logPath string
	entries []ActivityEntry
	mu      sync.RWMutex
	maxSize int
}

// ActivityEntry represents a single activity log entry
type ActivityEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // file_change, agent_run, finding, error
	Category  string    `json:"category,omitempty"`
	Path      string    `json:"path,omitempty"`
	Agent     string    `json:"agent,omitempty"`
	Message   string    `json:"message"`
	Details   any       `json:"details,omitempty"`
}

// NewActivityLog creates a new activity logger
func NewActivityLog(basePath string) *ActivityLog {
	logPath := filepath.Join(basePath, ".taskwing", "activity.json")

	log := &ActivityLog{
		logPath: logPath,
		entries: make([]ActivityEntry, 0),
		maxSize: 500, // Keep last 500 entries
	}

	// Try to load existing log
	log.load()

	return log
}

// LogFileChange records a file change event
func (l *ActivityLog) LogFileChange(path string, operation string, category FileCategory) {
	l.addEntry(ActivityEntry{
		Type:     "file_change",
		Category: string(category),
		Path:     path,
		Message:  fmt.Sprintf("%s: %s", operation, path),
	})
}

// LogAgentRun records an agent execution
func (l *ActivityLog) LogAgentRun(agent string, findingCount int, duration time.Duration, err error) {
	msg := fmt.Sprintf("%s completed: %d findings in %.1fs", agent, findingCount, duration.Seconds())
	details := map[string]any{
		"findings":    findingCount,
		"duration_ms": duration.Milliseconds(),
	}

	entryType := "agent_run"
	if err != nil {
		entryType = "error"
		msg = fmt.Sprintf("%s error: %v", agent, err)
		details["error"] = err.Error()
	}

	l.addEntry(ActivityEntry{
		Type:    entryType,
		Agent:   agent,
		Message: msg,
		Details: details,
	})
}

// LogFinding records a new finding
func (l *ActivityLog) LogFinding(agent string, title string, findingType string) {
	l.addEntry(ActivityEntry{
		Type:    "finding",
		Agent:   agent,
		Message: title,
		Details: map[string]string{"finding_type": findingType},
	})
}

// LogError records an error
func (l *ActivityLog) LogError(source string, err error) {
	l.addEntry(ActivityEntry{
		Type:    "error",
		Agent:   source,
		Message: err.Error(),
	})
}

// addEntry adds an entry to the log
func (l *ActivityLog) addEntry(entry ActivityEntry) {
	l.mu.Lock()

	entry.ID = time.Now().UnixNano()
	entry.Timestamp = time.Now()

	l.entries = append(l.entries, entry)

	// Trim to max size
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}

	// Copy data for async save to avoid holding lock
	entriesCopy := make([]ActivityEntry, len(l.entries))
	copy(entriesCopy, l.entries)
	l.mu.Unlock()

	// Async save with copied data
	go l.saveEntries(entriesCopy)
}

// GetRecent returns the most recent N entries
func (l *ActivityLog) GetRecent(count int) []ActivityEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if count > len(l.entries) {
		count = len(l.entries)
	}

	start := len(l.entries) - count
	result := make([]ActivityEntry, count)
	copy(result, l.entries[start:])

	// Return in reverse order (newest first)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetSince returns entries since a given timestamp
func (l *ActivityLog) GetSince(since time.Time) []ActivityEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []ActivityEntry
	for _, e := range l.entries {
		if e.Timestamp.After(since) {
			result = append(result, e)
		}
	}

	// Reverse for newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// save writes the log to disk (reads from entries with lock)
func (l *ActivityLog) save() {
	l.mu.RLock()
	entriesCopy := make([]ActivityEntry, len(l.entries))
	copy(entriesCopy, l.entries)
	l.mu.RUnlock()

	l.saveEntries(entriesCopy)
}

// saveEntries writes pre-copied entries to disk (no lock needed)
func (l *ActivityLog) saveEntries(entries []ActivityEntry) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}

	// Ensure directory exists
	dir := filepath.Dir(l.logPath)
	_ = os.MkdirAll(dir, 0755)

	_ = os.WriteFile(l.logPath, data, 0644)
}

// load reads the log from disk
func (l *ActivityLog) load() {
	data, err := os.ReadFile(l.logPath)
	if err != nil {
		return
	}

	var entries []ActivityEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}

	l.mu.Lock()
	l.entries = entries
	l.mu.Unlock()
}

// Clear clears the activity log
func (l *ActivityLog) Clear() {
	l.mu.Lock()
	l.entries = make([]ActivityEntry, 0)
	l.mu.Unlock()
	l.save()
}

// Count returns the total number of entries
func (l *ActivityLog) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Summary returns a summary of activity
func (l *ActivityLog) Summary() ActivitySummary {
	l.mu.RLock()
	defer l.mu.RUnlock()

	summary := ActivitySummary{
		TotalEntries: len(l.entries),
	}

	for _, e := range l.entries {
		switch e.Type {
		case "file_change":
			summary.FileChanges++
		case "agent_run":
			summary.AgentRuns++
		case "finding":
			summary.Findings++
		case "error":
			summary.Errors++
		}
	}

	if len(l.entries) > 0 {
		summary.OldestEntry = l.entries[0].Timestamp
		summary.NewestEntry = l.entries[len(l.entries)-1].Timestamp
	}

	return summary
}

// ActivitySummary provides a summary of activity
type ActivitySummary struct {
	TotalEntries int       `json:"total_entries"`
	FileChanges  int       `json:"file_changes"`
	AgentRuns    int       `json:"agent_runs"`
	Findings     int       `json:"findings"`
	Errors       int       `json:"errors"`
	OldestEntry  time.Time `json:"oldest_entry,omitempty"`
	NewestEntry  time.Time `json:"newest_entry,omitempty"`
}
