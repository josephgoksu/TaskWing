// Package project provides detection and context for project boundaries.
//
// This package implements "Zero-Config Smart Defaults" to automatically detect
// the logical root of a project. It resolves ambiguities in monorepo setups
// and ensures TaskWing operates on the correct context without manual flags.
//
// Detection Strategy (Hierarchical Precedence):
//  1. Explicit Context (.taskwing/): Highest priority. Respects existing SSoT.
//  2. Language Manifests: go.mod, package.json, Cargo.toml, etc.
//  3. VCS Root (.git/): Medium priority fallback.
//  4. CWD: Lowest priority, used if unanchored.
package project

import "github.com/spf13/afero"

// MarkerType represents the type of project marker that was detected.
type MarkerType int

const (
	// MarkerNone indicates no project marker was found.
	MarkerNone MarkerType = iota

	// MarkerTaskWing indicates a .taskwing directory was found (highest priority).
	MarkerTaskWing

	// MarkerGoMod indicates a go.mod file was found.
	MarkerGoMod

	// MarkerPackageJSON indicates a package.json file was found.
	MarkerPackageJSON

	// MarkerCargoToml indicates a Cargo.toml file was found.
	MarkerCargoToml

	// MarkerPomXML indicates a pom.xml file was found.
	MarkerPomXML

	// MarkerPyProjectToml indicates a pyproject.toml file was found.
	MarkerPyProjectToml

	// MarkerGit indicates a .git directory was found.
	MarkerGit
)

// String returns a human-readable name for the marker type.
func (m MarkerType) String() string {
	switch m {
	case MarkerNone:
		return "none"
	case MarkerTaskWing:
		return ".taskwing"
	case MarkerGoMod:
		return "go.mod"
	case MarkerPackageJSON:
		return "package.json"
	case MarkerCargoToml:
		return "Cargo.toml"
	case MarkerPomXML:
		return "pom.xml"
	case MarkerPyProjectToml:
		return "pyproject.toml"
	case MarkerGit:
		return ".git"
	default:
		return "unknown"
	}
}

// Priority returns the detection priority for this marker type.
// Higher values indicate higher priority.
func (m MarkerType) Priority() int {
	switch m {
	case MarkerTaskWing:
		return 100 // Highest - explicit context
	case MarkerGoMod, MarkerPackageJSON, MarkerCargoToml, MarkerPomXML, MarkerPyProjectToml:
		return 50 // Medium - language manifests
	case MarkerGit:
		return 10 // Low - VCS fallback
	default:
		return 0
	}
}

// IsLanguageManifest returns true if this marker represents a language-specific manifest file.
func (m MarkerType) IsLanguageManifest() bool {
	switch m {
	case MarkerGoMod, MarkerPackageJSON, MarkerCargoToml, MarkerPomXML, MarkerPyProjectToml:
		return true
	default:
		return false
	}
}

// Context contains information about the detected project boundary.
// It provides all the context needed to correctly scope TaskWing operations.
type Context struct {
	// RootPath is the absolute path to the detected project root.
	RootPath string

	// MarkerType indicates which marker was used to identify the project root.
	MarkerType MarkerType

	// GitRoot is the absolute path to the nearest .git directory (may differ from RootPath in monorepos).
	// Empty string if no git repository was found.
	GitRoot string

	// IsMonorepo is true if the project appears to be within a larger monorepo.
	// This is detected when GitRoot differs from RootPath.
	IsMonorepo bool
}

// RelativeGitPath returns the relative path from GitRoot to RootPath.
// This is useful for scoping git operations to the project subdirectory.
// Returns "." if GitRoot equals RootPath or if either is empty.
func (c *Context) RelativeGitPath() string {
	if c.GitRoot == "" || c.RootPath == "" {
		return "."
	}
	if c.GitRoot == c.RootPath {
		return "."
	}
	// Calculate relative path from GitRoot to RootPath
	// This will be used for git log scoping
	rel, err := relativePath(c.GitRoot, c.RootPath)
	if err != nil {
		return "."
	}
	return rel
}

// relativePath returns the relative path from base to target.
// This is a simple implementation that doesn't rely on os.
func relativePath(base, target string) (string, error) {
	// For now, we use a simple string operation
	// The full implementation will use filepath.Rel
	if len(target) <= len(base) {
		return ".", nil
	}
	if target[:len(base)] != base {
		return ".", nil
	}
	rel := target[len(base):]
	if len(rel) > 0 && rel[0] == '/' {
		rel = rel[1:]
	}
	if rel == "" {
		return ".", nil
	}
	return rel, nil
}

// HasTaskWingDir returns true if the project already has a .taskwing directory.
func (c *Context) HasTaskWingDir() bool {
	return c.MarkerType == MarkerTaskWing
}

// Detector defines the interface for project detection.
// This abstraction allows for easy testing with mock filesystems.
type Detector interface {
	// Detect finds the project root starting from the given path.
	// It walks up the directory tree looking for project markers.
	Detect(startPath string) (*Context, error)
}

// detector implements Detector using an afero filesystem.
type detector struct {
	fs afero.Fs
}

// NewDetector creates a new Detector using the provided filesystem.
// Use afero.NewOsFs() for real filesystem operations,
// or afero.NewMemMapFs() for testing.
func NewDetector(fs afero.Fs) Detector {
	return &detector{fs: fs}
}

// NewOsDetector creates a Detector using the real operating system filesystem.
func NewOsDetector() Detector {
	return NewDetector(afero.NewOsFs())
}

// Detect is a convenience function that detects the project root from the given path
// using the real operating system filesystem.
func Detect(startPath string) (*Context, error) {
	return NewOsDetector().Detect(startPath)
}
