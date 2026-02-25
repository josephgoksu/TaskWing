package project

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// ErrNoProjectFound is returned when no project root could be detected.
var ErrNoProjectFound = errors.New("no project root found")

// markerFiles defines the files/directories to check for project detection.
// Order matters for same-directory precedence within priority tiers.
var markerFiles = []struct {
	name       string
	markerType MarkerType
}{
	// Highest priority: explicit TaskWing context
	{".taskwing", MarkerTaskWing},

	// Medium priority: language manifests
	{"go.mod", MarkerGoMod},
	{"package.json", MarkerPackageJSON},
	{"Cargo.toml", MarkerCargoToml},
	{"pom.xml", MarkerPomXML},
	{"pyproject.toml", MarkerPyProjectToml},

	// Low priority: VCS root
	{".git", MarkerGit},
}

// Detect implements the Detector interface.
// It walks up the directory tree from startPath, looking for project markers.
//
// The detection algorithm:
//  1. For each directory from startPath upward to filesystem root:
//     - Check for markers in priority order
//     - If .taskwing found, return immediately (highest priority)
//     - Track best candidate based on marker priority
//  2. Continue until filesystem root or .taskwing found
//  3. Return the best candidate, or error if none found
//
// Constraint: Read-only detection using stat calls only. No files are created.
func (d *detector) Detect(startPath string) (*Context, error) {
	// Clean the path (handles . and .. but doesn't resolve symlinks)
	// For real filesystem, also convert to absolute path
	absPath := filepath.Clean(startPath)
	if !filepath.IsAbs(absPath) {
		var err error
		absPath, err = filepath.Abs(startPath)
		if err != nil {
			return nil, err
		}
	}

	// Track the best candidate found during traversal
	var bestCandidate *Context
	var gitRoot string

	// Walk up from startPath to filesystem root
	current := absPath
	for {
		// Check for markers at current directory
		marker := d.findMarkerAt(current)

		// Always check for .git to track git root FIRST (before .taskwing check)
		// This is critical: we need to know if we've passed a .git before considering
		// .taskwing directories that are above it.
		if gitRoot == "" && d.hasGit(current) {
			gitRoot = current
		}

		// If .taskwing found, check if it's a valid project marker
		if marker == MarkerTaskWing {
			// CRITICAL FIX: If we've already found a .git directory below this .taskwing,
			// don't use this .taskwing as the project root. It's likely global config
			// or belongs to a different project (e.g., ~/.taskwing).
			// A valid project .taskwing should be AT or BELOW the git root, not above it.
			if gitRoot != "" && gitRoot != current {
				// .taskwing is above gitRoot - skip it, continue looking for language manifests
				// or fall back to gitRoot
			} else {
				// .taskwing is at or below gitRoot (or no git found yet) - use it
				if gitRoot == "" {
					gitRoot = d.findGitRoot(current)
				}
				return &Context{
					RootPath:   current,
					MarkerType: MarkerTaskWing,
					GitRoot:    gitRoot,
					IsMonorepo: gitRoot != "" && (gitRoot != current || d.hasNestedProjects(current)),
				}, nil
			}
		}

		// Track language manifest as candidate (but continue upward looking for .taskwing)
		if marker.IsLanguageManifest() {
			// Only update if this is a higher priority or first candidate
			if bestCandidate == nil || marker.Priority() > bestCandidate.MarkerType.Priority() {
				bestCandidate = &Context{
					RootPath:   current,
					MarkerType: marker,
					GitRoot:    "", // Will be set after traversal completes
					IsMonorepo: false,
				}
			}
		}

		// Move to parent directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}
		current = parent
	}

	// If we found a language manifest candidate, use it
	if bestCandidate != nil {
		bestCandidate.GitRoot = gitRoot
		bestCandidate.IsMonorepo = gitRoot != "" && gitRoot != bestCandidate.RootPath
		return bestCandidate, nil
	}

	// Fall back to git root if found
	if gitRoot != "" {
		return &Context{
			RootPath:   gitRoot,
			MarkerType: MarkerGit,
			GitRoot:    gitRoot,
			IsMonorepo: d.hasNestedProjects(gitRoot),
		}, nil
	}

	// No project marker found, use startPath as fallback
	return &Context{
		RootPath:   absPath,
		MarkerType: MarkerNone,
		GitRoot:    "",
		IsMonorepo: false,
	}, nil
}

// findMarkerAt checks for project markers at the given directory.
// Returns the highest priority marker found, or MarkerNone if none found.
// Uses stat-only checks for performance (read-only, no file creation).
func (d *detector) findMarkerAt(dir string) MarkerType {
	for _, m := range markerFiles {
		path := filepath.Join(dir, m.name)
		if exists, _ := d.exists(path); exists {
			return m.markerType
		}
	}
	return MarkerNone
}

// exists checks if a file or directory exists using stat only.
// This is a read-only operation that doesn't create anything.
func (d *detector) exists(path string) (bool, error) {
	_, err := d.fs.Stat(path)
	if err == nil {
		return true, nil
	}
	// Check for actual errors vs "not exists"
	// afero wraps os errors, so we check for the common patterns
	return false, nil
}

// hasGit checks if a .git directory exists at the given path.
func (d *detector) hasGit(dir string) bool {
	path := filepath.Join(dir, ".git")
	exists, _ := d.exists(path)
	return exists
}

// findGitRoot walks up from the given path to find the nearest .git directory.
// Returns the path containing .git, or empty string if not found.
func (d *detector) findGitRoot(startPath string) string {
	current := startPath
	for {
		if d.hasGit(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}
		current = parent
	}
	return ""
}

// nestedProjectMarkers are the files that indicate a directory is a project.
// Shared with workspace.go's isProjectDir for consistency.
var nestedProjectMarkers = []string{
	"package.json",
	"go.mod",
	"Cargo.toml",
	"pom.xml",
	"pyproject.toml",
	"build.gradle",
	"requirements.txt",
	"Dockerfile",
}

// alwaysSkipDirs is a minimal safety-net for directories that should never be
// considered as project subdirectories, even when .gitignore is absent or unreadable.
// These are generated/vendored directories that commonly contain nested manifests.
var alwaysSkipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
}

// hasNestedProjects checks if a directory contains multiple subdirectories
// that look like independent projects (each with their own manifest file).
// This detects monorepo roots where RootPath == GitRoot but the directory
// structure clearly contains multiple services/packages.
//
// Directories are skipped if they match .gitignore patterns, the alwaysSkipDirs
// safety net, or are hidden (dot-prefixed).
//
// Requires at least 2 nested project directories to return true,
// reducing false positives for repos with a single nested manifest.
func (d *detector) hasNestedProjects(dir string) bool {
	ignored := d.loadGitignore(dir)

	entries, err := afero.ReadDir(d.fs, dir)
	if err != nil {
		return false
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || alwaysSkipDirs[name] || ignored[name] {
			continue
		}
		if d.isProjectDir(filepath.Join(dir, name)) {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

// loadGitignore reads .gitignore from dir and returns a set of directory names
// that should be ignored. It handles the common gitignore patterns:
//   - Simple names: "dist", "build"
//   - Directory markers: "dist/", "build/"
//   - Root-anchored: "/dist", "/build/"
//   - Comments (#) and blank lines are skipped
//   - Negation patterns (!) are skipped (conservative: don't un-ignore)
//
// Only simple name patterns are matched (no wildcards, no path separators
// mid-pattern). This covers the vast majority of real-world gitignore entries
// for top-level directories.
func (d *detector) loadGitignore(dir string) map[string]bool {
	ignored := make(map[string]bool)

	data, err := afero.ReadFile(d.fs, filepath.Join(dir, ".gitignore"))
	if err != nil {
		return ignored
	}

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip negation patterns (conservative: don't un-ignore anything)
		if strings.HasPrefix(line, "!") {
			continue
		}

		// Strip root anchor (leading /)
		line = strings.TrimPrefix(line, "/")
		// Strip directory indicator (trailing /)
		line = strings.TrimSuffix(line, "/")

		// Only match simple names (no path separators, no glob wildcards).
		// Patterns like "logs/*.log" or "src/generated" are path-based and
		// don't apply to top-level directory matching.
		if strings.ContainsAny(line, "/*?[") {
			continue
		}

		if line != "" {
			ignored[line] = true
		}
	}

	return ignored
}

// isProjectDir checks if a directory contains any project marker file.
func (d *detector) isProjectDir(dir string) bool {
	for _, marker := range nestedProjectMarkers {
		if exists, _ := d.exists(filepath.Join(dir, marker)); exists {
			return true
		}
	}
	return false
}
