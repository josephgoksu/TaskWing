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
// The implementation will be completed in task 2.
func (d *detector) Detect(startPath string) (*Context, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, err
	}

	// Placeholder: Return context at startPath
	// Full bubble-up traversal implemented in task 2
	return &Context{
		RootPath:   absPath,
		MarkerType: MarkerNone,
		GitRoot:    "",
		IsMonorepo: false,
	}, nil
}
