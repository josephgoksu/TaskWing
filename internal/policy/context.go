// Package policy provides policy-as-code enforcement using OPA (Open Policy Agent).
// This file contains context builders for constructing OPA input from TaskWing state.
package policy

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// ContextBuilder constructs PolicyInput objects from TaskWing state.
// It provides a fluent API for building policy evaluation contexts.
type ContextBuilder struct {
	input   *PolicyInput
	workDir string
	fs      afero.Fs
}

// NewContextBuilder creates a new ContextBuilder with the given working directory.
// The working directory is used for resolving relative paths and loading context.
func NewContextBuilder(workDir string) *ContextBuilder {
	return &ContextBuilder{
		input:   &PolicyInput{},
		workDir: workDir,
		fs:      afero.NewOsFs(),
	}
}

// NewContextBuilderWithFs creates a new ContextBuilder with a custom filesystem.
// This is useful for testing with in-memory filesystems.
func NewContextBuilderWithFs(workDir string, fs afero.Fs) *ContextBuilder {
	return &ContextBuilder{
		input:   &PolicyInput{},
		workDir: workDir,
		fs:      fs,
	}
}

// WithTask adds task information to the policy input.
// This includes task ID, title, and file modification information.
func (b *ContextBuilder) WithTask(id, title string) *ContextBuilder {
	if b.input.Task == nil {
		b.input.Task = &TaskInput{}
	}
	b.input.Task.ID = id
	b.input.Task.Title = title
	return b
}

// WithTaskFiles adds file modification information to the task.
// filesModified and filesCreated are lists of file paths.
func (b *ContextBuilder) WithTaskFiles(filesModified, filesCreated []string) *ContextBuilder {
	if b.input.Task == nil {
		b.input.Task = &TaskInput{}
	}
	b.input.Task.FilesModified = filesModified
	b.input.Task.FilesCreated = filesCreated
	return b
}

// WithPlan adds plan information to the policy input.
// This includes plan ID and the goal/description.
func (b *ContextBuilder) WithPlan(id, goal string) *ContextBuilder {
	if id != "" || goal != "" {
		b.input.Plan = &PlanInput{
			ID:   id,
			Goal: goal,
		}
	}
	return b
}

// WithProtectedZones adds protected zone patterns to the context.
// These are glob patterns for files/directories that should be protected.
func (b *ContextBuilder) WithProtectedZones(zones []string) *ContextBuilder {
	if b.input.Context == nil {
		b.input.Context = &ContextInput{}
	}
	b.input.Context.ProtectedZones = zones
	return b
}

// WithProjectType adds the project type to the context.
// Examples: "drupal", "laravel", "nextjs", etc.
func (b *ContextBuilder) WithProjectType(projectType string) *ContextBuilder {
	if b.input.Context == nil {
		b.input.Context = &ContextInput{}
	}
	b.input.Context.ProjectType = projectType
	return b
}

// Build returns the constructed PolicyInput.
func (b *ContextBuilder) Build() *PolicyInput {
	return b.input
}

// BuildForTask is a convenience method that creates a PolicyInput for task evaluation.
// It combines task and optional plan information into a single input.
func BuildForTask(taskID, taskTitle string, filesModified, filesCreated []string, planID, planGoal string) *PolicyInput {
	input := &PolicyInput{
		Task: &TaskInput{
			ID:            taskID,
			Title:         taskTitle,
			FilesModified: filesModified,
			FilesCreated:  filesCreated,
		},
	}
	if planID != "" || planGoal != "" {
		input.Plan = &PlanInput{
			ID:   planID,
			Goal: planGoal,
		}
	}
	return input
}

// BuildForFiles is a convenience method that creates a PolicyInput for file checking.
// It creates a minimal input suitable for checking file modifications.
func BuildForFiles(filesModified, filesCreated []string) *PolicyInput {
	return &PolicyInput{
		Task: &TaskInput{
			ID:            "file-check",
			Title:         "File modification check",
			FilesModified: filesModified,
			FilesCreated:  filesCreated,
		},
	}
}

// DetectProjectType attempts to detect the project type based on files in the directory.
// Returns an empty string if the project type cannot be determined.
func DetectProjectType(workDir string) string {
	fs := afero.NewOsFs()
	return DetectProjectTypeWithFs(workDir, fs)
}

// DetectProjectTypeWithFs detects project type using a custom filesystem.
func DetectProjectTypeWithFs(workDir string, fs afero.Fs) string {
	// Check for various project markers
	markers := map[string]string{
		"composer.json":   "php",
		"package.json":    "node",
		"go.mod":          "go",
		"Cargo.toml":      "rust",
		"requirements.txt": "python",
		"pyproject.toml":  "python",
		"Gemfile":         "ruby",
		"pom.xml":         "java",
		"build.gradle":    "java",
	}

	// Check for specific frameworks
	frameworkMarkers := map[string]string{
		"core/modules":         "drupal",
		"artisan":              "laravel",
		"next.config.js":       "nextjs",
		"next.config.mjs":      "nextjs",
		"nuxt.config.js":       "nuxt",
		"nuxt.config.ts":       "nuxt",
		"angular.json":         "angular",
		"vue.config.js":        "vue",
		".taskwing":            "taskwing",
	}

	// Check framework markers first (more specific)
	for marker, projectType := range frameworkMarkers {
		path := filepath.Join(workDir, marker)
		if exists, _ := afero.Exists(fs, path); exists {
			return projectType
		}
	}

	// Check general markers
	for marker, projectType := range markers {
		path := filepath.Join(workDir, marker)
		if exists, _ := afero.Exists(fs, path); exists {
			return projectType
		}
	}

	return ""
}

// LoadProtectedZonesFromConfig loads protected zones from a configuration file.
// Looks for .taskwing/protected_zones.txt or similar configuration.
// Returns nil if no configuration is found.
func LoadProtectedZonesFromConfig(workDir string) []string {
	fs := afero.NewOsFs()
	return LoadProtectedZonesFromConfigWithFs(workDir, fs)
}

// LoadProtectedZonesFromConfigWithFs loads protected zones using a custom filesystem.
func LoadProtectedZonesFromConfigWithFs(workDir string, fs afero.Fs) []string {
	configPath := filepath.Join(workDir, ".taskwing", "protected_zones.txt")

	content, err := afero.ReadFile(fs, configPath)
	if err != nil {
		return nil
	}

	var zones []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		zones = append(zones, line)
	}

	return zones
}

// NormalizePath normalizes a file path for consistent policy evaluation.
// It converts Windows paths to forward slashes and removes leading ./
func NormalizePath(path string) string {
	// Convert backslashes to forward slashes
	path = strings.ReplaceAll(path, "\\", "/")
	// Remove leading ./
	path = strings.TrimPrefix(path, "./")
	return path
}

// NormalizePaths normalizes a slice of file paths.
func NormalizePaths(paths []string) []string {
	if paths == nil {
		return nil
	}
	normalized := make([]string, len(paths))
	for i, p := range paths {
		normalized[i] = NormalizePath(p)
	}
	return normalized
}

// GetRelativePaths converts absolute paths to relative paths based on workDir.
// Returns the original paths if they are already relative.
func GetRelativePaths(workDir string, paths []string) []string {
	if paths == nil {
		return nil
	}

	relative := make([]string, len(paths))
	for i, p := range paths {
		if filepath.IsAbs(p) {
			rel, err := filepath.Rel(workDir, p)
			if err == nil {
				relative[i] = NormalizePath(rel)
				continue
			}
		}
		relative[i] = NormalizePath(p)
	}
	return relative
}

// GetStagedFiles returns the list of git staged files in the working directory.
// Returns nil if not in a git repository or on error.
func GetStagedFiles(workDir string) []string {
	// Read git index to get staged files
	// This is a simplified implementation - for production, use git plumbing
	cmd := filepath.Join(workDir, ".git")
	if _, err := os.Stat(cmd); os.IsNotExist(err) {
		return nil
	}

	// For now, return nil - the actual implementation would run git diff --cached
	// This is handled by the CLI command instead
	return nil
}
