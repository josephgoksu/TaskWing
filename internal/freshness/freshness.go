// Package freshness provides query-time validation of knowledge graph findings.
// It checks whether evidence files have changed since findings were last verified,
// adjusting confidence scores and adding staleness metadata to MCP responses.
package freshness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

		info, err := defaultCache.stat(fullPath)
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

	// Decay formula: smooth curve from 1.0 (no missing) to 0.2 (all missing).
	// decay = 1.0 - (missingRatio * 0.8)
	// At 0% missing: 1.0, at 50%: 0.6, at 100%: 0.2
	if len(missing) > 0 {
		missingRatio := float64(len(missing)) / float64(len(paths))
		decay := 1.0 - (missingRatio * 0.8)
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

// --- Stat cache with bounded size and TTL eviction ---

const (
	cacheTTL     = 60 * time.Second
	cacheMaxSize = 1000 // Evict oldest entries when exceeded
)

// statCache holds cached os.Stat results with TTL and bounded size.
type statCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	info      os.FileInfo
	err       error
	checkedAt time.Time
}

var defaultCache = &statCache{entries: make(map[string]cacheEntry)}

func (c *statCache) stat(path string) (os.FileInfo, error) {
	c.mu.RLock()
	entry, ok := c.entries[path]
	c.mu.RUnlock()

	if ok && time.Since(entry.checkedAt) < cacheTTL {
		return entry.info, entry.err
	}

	info, err := os.Stat(path)

	c.mu.Lock()
	// Re-check: another goroutine may have refreshed this entry while we were statting
	if existing, ok := c.entries[path]; ok && time.Since(existing.checkedAt) < cacheTTL {
		c.mu.Unlock()
		return existing.info, existing.err
	}
	c.entries[path] = cacheEntry{info: info, err: err, checkedAt: time.Now()}
	// Evict when cache grows too large
	if len(c.entries) > cacheMaxSize {
		c.evictExpired()
		// Fallback: if still over limit (burst scenario), evict oldest entries
		if len(c.entries) > cacheMaxSize {
			c.evictOldest(len(c.entries) - cacheMaxSize)
		}
	}
	c.mu.Unlock()

	return info, err
}

// evictExpired removes entries older than 2x TTL. Caller must hold write lock.
func (c *statCache) evictExpired() {
	cutoff := time.Now().Add(-2 * cacheTTL)
	for k, v := range c.entries {
		if v.checkedAt.Before(cutoff) {
			delete(c.entries, k)
		}
	}
}

// evictOldest removes the n oldest entries. Caller must hold write lock.
func (c *statCache) evictOldest(n int) {
	if n <= 0 {
		return
	}
	type aged struct {
		key string
		at  time.Time
	}
	items := make([]aged, 0, len(c.entries))
	for k, v := range c.entries {
		items = append(items, aged{key: k, at: v.checkedAt})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].at.Before(items[j].at) })
	for i := 0; i < n && i < len(items); i++ {
		delete(c.entries, items[i].key)
	}
}

func (c *statCache) reset() {
	c.mu.Lock()
	c.entries = make(map[string]cacheEntry)
	c.mu.Unlock()
}

// ResetCache clears the stat cache. Used in tests and after bootstrap.
func ResetCache() {
	defaultCache.reset()
}
