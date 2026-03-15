// Package freshness provides query-time validation of knowledge graph findings.
// It checks whether evidence files have changed since findings were last verified,
// adjusting confidence scores and adding staleness metadata to MCP responses.
package freshness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Status describes the freshness of a finding's evidence.
type Status string

const (
	StatusFresh      Status = "fresh"       // All evidence files unchanged
	StatusStale      Status = "stale"       // Some evidence files changed since last check
	StatusMissing    Status = "missing"     // Some evidence files no longer exist
	StatusNoEvidence Status = "no_evidence" // Finding has no file-based evidence
	StatusUnknown    Status = "unknown"     // Could not determine (permission error, timeout, etc.)
)

// Result captures the outcome of a freshness check for a single node.
type Result struct {
	Status       Status   `json:"status"`
	StaleFiles   []string `json:"staleFiles,omitempty"`
	MissingFiles []string `json:"missingFiles,omitempty"`
	DecayFactor  float64  `json:"decayFactor"`
	CheckedAt    time.Time
}

// evidenceItem is the subset of evidence fields we need for freshness checks.
type evidenceItem struct {
	FilePath string `json:"file_path"`
}

// skipPatterns are path prefixes for build artifacts and generated files.
// Changes to these should not trigger staleness.
var skipPatterns = []string{
	"dist/", "build/", "node_modules/", "vendor/",
	".next/", "__pycache__/", "target/", ".gradle/",
	".taskwing/", ".git/",
}

// Check performs a Level 1 (stat-based) freshness check on a node's evidence.
// It compares file mtimes against the reference time (typically node creation or last verification).
// Designed to be fast enough to run inline on every MCP query (<1ms per node).
func Check(basePath string, evidenceJSON string, referenceTime time.Time) Result {
	if evidenceJSON == "" {
		return Result{Status: StatusNoEvidence, DecayFactor: 1.0, CheckedAt: time.Now()}
	}

	var evidence []evidenceItem
	if err := json.Unmarshal([]byte(evidenceJSON), &evidence); err != nil {
		return Result{Status: StatusNoEvidence, DecayFactor: 1.0, CheckedAt: time.Now()}
	}

	// Filter to evidence items with file paths
	var paths []string
	for _, e := range evidence {
		if e.FilePath == "" {
			continue
		}
		if shouldSkip(e.FilePath) {
			continue
		}
		paths = append(paths, e.FilePath)
	}

	if len(paths) == 0 {
		return Result{Status: StatusNoEvidence, DecayFactor: 1.0, CheckedAt: time.Now()}
	}

	var stale, missing []string

	for _, p := range paths {
		fullPath := p
		if !filepath.IsAbs(p) {
			fullPath = filepath.Join(basePath, p)
		}

		info, err := statCached(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, p)
			}
			// Permission errors and other failures: skip, don't count as stale
			continue
		}

		if info.ModTime().After(referenceTime) {
			stale = append(stale, p)
		}
	}

	now := time.Now()

	// All evidence files deleted
	if len(missing) == len(paths) {
		return Result{
			Status:       StatusMissing,
			MissingFiles: missing,
			DecayFactor:  0.2,
			CheckedAt:    now,
		}
	}

	// Some files missing
	if len(missing) > 0 {
		missingRatio := float64(len(missing)) / float64(len(paths))
		decay := 1.0 - (missingRatio * 0.6) // 0.4 at 100% missing
		return Result{
			Status:       StatusMissing,
			StaleFiles:   stale,
			MissingFiles: missing,
			DecayFactor:  decay,
			CheckedAt:    now,
		}
	}

	// Some files changed
	if len(stale) > 0 {
		return Result{
			Status:      StatusStale,
			StaleFiles:  stale,
			DecayFactor: 0.7,
			CheckedAt:   now,
		}
	}

	// All fresh
	return Result{
		Status:      StatusFresh,
		DecayFactor: 1.0,
		CheckedAt:   now,
	}
}

func shouldSkip(path string) bool {
	normalized := filepath.ToSlash(path)
	for _, pattern := range skipPatterns {
		// Match at start (dist/...) or as a path segment (api/dist/...)
		if strings.HasPrefix(normalized, pattern) || strings.Contains(normalized, "/"+pattern) {
			return true
		}
	}
	return false
}

// FormatStatus returns a human-readable annotation for MCP responses.
func FormatStatus(r Result, lastVerified *time.Time) string {
	switch r.Status {
	case StatusFresh:
		if lastVerified != nil {
			age := time.Since(*lastVerified)
			return fmt.Sprintf("[verified %s ago]", formatDuration(age))
		}
		return "[fresh]"
	case StatusStale:
		files := strings.Join(r.StaleFiles, ", ")
		if len(files) > 60 {
			files = fmt.Sprintf("%d files changed", len(r.StaleFiles))
		}
		return fmt.Sprintf("[STALE: %s]", files)
	case StatusMissing:
		return "[WARNING: evidence file deleted]"
	case StatusNoEvidence:
		return ""
	default:
		return ""
	}
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// --- Stat cache (avoids re-statting the same file within a session) ---

var (
	cache   = make(map[string]cacheEntry)
	cacheMu sync.RWMutex
	cacheTTL = 60 * time.Second
)

type cacheEntry struct {
	info      os.FileInfo
	err       error
	checkedAt time.Time
}

func statCached(path string) (os.FileInfo, error) {
	cacheMu.RLock()
	entry, ok := cache[path]
	cacheMu.RUnlock()

	if ok && time.Since(entry.checkedAt) < cacheTTL {
		return entry.info, entry.err
	}

	info, err := os.Stat(path)

	cacheMu.Lock()
	cache[path] = cacheEntry{info: info, err: err, checkedAt: time.Now()}
	cacheMu.Unlock()

	return info, err
}

// ResetCache clears the stat cache. Used in tests and after bootstrap.
func ResetCache() {
	cacheMu.Lock()
	cache = make(map[string]cacheEntry)
	cacheMu.Unlock()
}
