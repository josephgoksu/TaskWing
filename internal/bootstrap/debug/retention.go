package debug

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PruneLogFiles removes old debug log files, keeping only the newest count files.
func PruneLogFiles(dir string, keepCount int) error {
	if keepCount <= 0 {
		keepCount = 5
	}

	// List all debug-*.log files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var logFiles []string
	for _, entry := range entries {
		name := entry.Name()
		// Match debug-*.log but not debug-latest.log (symlink)
		if strings.HasPrefix(name, "debug-") && strings.HasSuffix(name, ".log") && name != "debug-latest.log" {
			logFiles = append(logFiles, name)
		}
	}

	// Sort descending (newest first based on timestamp in filename)
	sort.Sort(sort.Reverse(sort.StringSlice(logFiles)))

	// Delete files beyond retention count
	for i := keepCount; i < len(logFiles); i++ {
		path := filepath.Join(dir, logFiles[i])
		if err := os.Remove(path); err != nil {
			// Log warning but continue
			continue
		}
	}

	return nil
}
