package project

import (
	"errors"
	"path/filepath"
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
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, err
	}

	// Track the best candidate found during traversal
	var bestCandidate *Context
	var gitRoot string

	// Walk up from startPath to filesystem root
	current := absPath
	for {
		// Check for markers at current directory
		marker := d.findMarkerAt(current)

		// If .taskwing found, return immediately (highest priority)
		if marker == MarkerTaskWing {
			return &Context{
				RootPath:   current,
				MarkerType: MarkerTaskWing,
				GitRoot:    gitRoot,
				IsMonorepo: gitRoot != "" && gitRoot != current,
			}, nil
		}

		// Track git root for monorepo detection
		if marker == MarkerGit && gitRoot == "" {
			gitRoot = current
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
			IsMonorepo: false,
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
