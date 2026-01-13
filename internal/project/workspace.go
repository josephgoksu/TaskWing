/*
Workspace detection and handling for multi-repo workspaces.

A workspace can be:
- Single: Normal git repository with unified codebase
- Monorepo: Single git root with multiple packages (nx, turborepo, lerna)
- MultiRepo: Directory containing multiple independent git repositories
*/
package project

import (
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceType represents the workspace structure type
type WorkspaceType int

const (
	// WorkspaceTypeSingle is a normal single-project repository
	WorkspaceTypeSingle WorkspaceType = iota
	// WorkspaceTypeMonorepo is a single git root with multiple packages
	WorkspaceTypeMonorepo
	// WorkspaceTypeMultiRepo is a directory containing multiple independent git repos
	WorkspaceTypeMultiRepo
)

func (t WorkspaceType) String() string {
	switch t {
	case WorkspaceTypeSingle:
		return "single"
	case WorkspaceTypeMonorepo:
		return "monorepo"
	case WorkspaceTypeMultiRepo:
		return "multi-repo"
	default:
		return "unknown"
	}
}

// WorkspaceInfo contains information about the detected workspace
type WorkspaceInfo struct {
	Type     WorkspaceType // The workspace type
	RootPath string        // The root path of the workspace
	Services []string      // List of service/repo paths (relative to root)
	Name     string        // Name of the project (from root directory or config)
}

// DetectWorkspace analyzes a directory and returns workspace information.
// It checks for git repositories, monorepo markers, and workspace configs.
func DetectWorkspace(basePath string) (*WorkspaceInfo, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	info := &WorkspaceInfo{
		RootPath: absPath,
		Name:     filepath.Base(absPath),
	}

	// Check for .git at root
	hasRootGit := hasGitDir(absPath)

	// Find nested projects (git repos or manifest-based services)
	nestedRepos := findNestedProjects(absPath)

	switch {
	case len(nestedRepos) > 0:
		// Monorepo or Multi-repo
		// If root has git, it's a Monorepo. If not, it's Multi-repo (conceptually, or just a non-git monorepo)
		if hasRootGit {
			info.Type = WorkspaceTypeMonorepo
		} else {
			info.Type = WorkspaceTypeMultiRepo
		}
		info.Services = nestedRepos

	default:
		// Normal single repo
		info.Type = WorkspaceTypeSingle
		info.Services = []string{"."}
	}

	return info, nil
}

// hasGitDir checks if a .git directory exists at the given path
func hasGitDir(path string) bool {
	gitPath := filepath.Join(path, ".git")
	stat, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// findNestedProjects finds all sub-directories that look like independent projects/services
func findNestedProjects(basePath string) []string {
	var projects []string

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return projects
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip hidden dirs and common non-repo dirs
		if strings.HasPrefix(name, ".") {
			continue
		}
		if isSkippableDir(name) {
			continue
		}

		dirPath := filepath.Join(basePath, name)
		if isProjectDir(dirPath) {
			projects = append(projects, name)
		}
	}

	return projects
}

// isProjectDir checks if a directory contains project markers
func isProjectDir(path string) bool {
	markers := []string{
		".git",
		"package.json",
		"go.mod",
		"pom.xml",
		"build.gradle",
		"requirements.txt",
		"pyproject.toml",
		"Cargo.toml",
		"Dockerfile",
	}

	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	return false
}

// isSkippableDir returns true for directories that shouldn't be treated as repos
func isSkippableDir(name string) bool {
	skippable := map[string]bool{
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		"out":          true,
		"target":       true,
		"bin":          true,
		"__pycache__":  true,
		".next":        true,
		"coverage":     true,
	}
	return skippable[name]
}

// IsMultiRepo returns true if this is a multi-repo workspace
func (w *WorkspaceInfo) IsMultiRepo() bool {
	return w.Type == WorkspaceTypeMultiRepo
}

// ServiceCount returns the number of services in the workspace
func (w *WorkspaceInfo) ServiceCount() int {
	return len(w.Services)
}

// GetServicePath returns the absolute path for a service
func (w *WorkspaceInfo) GetServicePath(serviceName string) string {
	return filepath.Join(w.RootPath, serviceName)
}
