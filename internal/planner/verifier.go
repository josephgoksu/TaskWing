// Package planner provides plan verification using code intelligence.
package planner

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
)

// Verifier defines the interface for plan verification.
type Verifier interface {
	// Verify validates and optionally corrects tasks using code intelligence.
	// Returns the verified (and potentially corrected) tasks.
	Verify(ctx context.Context, tasks []LLMTaskSchema) ([]LLMTaskSchema, error)
}

// PlanVerifier validates plan tasks against the actual codebase using code intelligence.
// It uses the codeintel.QueryService to verify file paths, symbol references,
// and dependency relationships mentioned in tasks.
type PlanVerifier struct {
	query *codeintel.QueryService
}

// NewPlanVerifier creates a new PlanVerifier with the given query service.
func NewPlanVerifier(query *codeintel.QueryService) *PlanVerifier {
	return &PlanVerifier{
		query: query,
	}
}

// Verify validates tasks against the codebase and attempts to correct any issues.
// It checks:
// - File paths referenced in task descriptions
// - Symbol names mentioned in tasks
// - Dependency relationships
//
// Returns the verified (and potentially corrected) tasks.
func (v *PlanVerifier) Verify(ctx context.Context, tasks []LLMTaskSchema) ([]LLMTaskSchema, error) {
	// TODO: Implement verification logic in subsequent tasks
	// For now, pass through unchanged
	return tasks, nil
}

// Ensure PlanVerifier implements Verifier
var _ Verifier = (*PlanVerifier)(nil)

// === Path Extraction Logic ===

// PathReference represents a file path or directory found in task text.
type PathReference struct {
	Path     string // The extracted path
	IsDir    bool   // True if this is a directory reference (ends with /... or /)
	Original string // The original matched text
}

// Regular expressions for path extraction
var (
	// Match standard project directory patterns: internal/..., cmd/..., pkg/...
	// Handles both file paths and directory patterns like ./internal/...
	projectPathRegex = regexp.MustCompile(`(?:^|[^\w./])(\./)?((internal|cmd|pkg|src|lib|test|tests|api|web|app)/[\w/.-]+)`)

	// Match file paths with common extensions
	fileExtRegex = regexp.MustCompile(`(?:^|[^\w./])(\./)?([a-zA-Z0-9_][\w/.-]*\.(go|ts|tsx|js|jsx|py|rs|java|md|json|yaml|yml|toml|sql|proto|graphql|sh|css|scss|html))(?:[^\w.]|$)`)

	// Match go test patterns like ./... or ./internal/...
	goTestDirRegex = regexp.MustCompile(`\./[\w/.-]*\.\.\.`)

	// Match backtick-quoted paths
	backtickPathRegex = regexp.MustCompile("`([a-zA-Z0-9_][\\w/.-]+)`")
)

// validExtensions maps file extensions to their validity
var validExtensions = map[string]bool{
	".go": true, ".ts": true, ".js": true, ".tsx": true, ".jsx": true,
	".py": true, ".rs": true, ".java": true, ".cpp": true, ".c": true,
	".h": true, ".hpp": true, ".rb": true, ".php": true, ".swift": true,
	".kt": true, ".scala": true, ".vue": true, ".svelte": true,
	".css": true, ".scss": true, ".less": true, ".html": true,
	".yaml": true, ".yml": true, ".json": true, ".xml": true,
	".md": true, ".txt": true, ".toml": true, ".sh": true,
	".sql": true, ".proto": true, ".graphql": true,
}

// commonWords that should not be treated as paths even if they match patterns
var commonWords = map[string]bool{
	"error.go": true, "main.go": true, // too generic without directory
	"test.go":  true, "doc.go": true,
}

// ExtractPaths extracts file paths and directory references from text.
// It identifies:
// - File paths like internal/ui/presenter.go
// - Directory patterns like ./internal/...
// - Paths in backticks like `cmd/server.go`
func ExtractPaths(text string) []PathReference {
	pathSet := make(map[string]PathReference)

	// Extract go test directory patterns first (highest priority)
	for _, match := range goTestDirRegex.FindAllString(text, -1) {
		path := strings.TrimSuffix(match, "/...")
		path = strings.TrimPrefix(path, "./")
		if path == "" || path == "." {
			path = "."
		}
		pathSet[match] = PathReference{
			Path:     path,
			IsDir:    true,
			Original: match,
		}
	}

	// Extract project directory paths
	for _, match := range projectPathRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 {
			path := match[2]
			if isValidPath(path) {
				isDir := strings.HasSuffix(path, "/") || !strings.Contains(filepath.Base(path), ".")
				pathSet[path] = PathReference{
					Path:     path,
					IsDir:    isDir,
					Original: match[0],
				}
			}
		}
	}

	// Extract file paths with extensions
	for _, match := range fileExtRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 {
			path := match[2]
			if isValidPath(path) && !commonWords[filepath.Base(path)] {
				pathSet[path] = PathReference{
					Path:     path,
					IsDir:    false,
					Original: match[0],
				}
			}
		}
	}

	// Extract backtick-quoted paths
	for _, match := range backtickPathRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			path := match[1]
			if isValidPath(path) {
				ext := filepath.Ext(path)
				isDir := ext == "" || !validExtensions[ext]
				pathSet[path] = PathReference{
					Path:     path,
					IsDir:    isDir,
					Original: match[0],
				}
			}
		}
	}

	// Convert to slice
	var refs []PathReference
	for _, ref := range pathSet {
		refs = append(refs, ref)
	}
	return refs
}

// isValidPath checks if a path looks valid and isn't a false positive.
func isValidPath(path string) bool {
	// Skip URLs
	if strings.HasPrefix(path, "http") || strings.Contains(path, "://") {
		return false
	}

	// Skip paths that look like domain names
	if strings.Contains(path, ".com") || strings.Contains(path, ".org") || strings.Contains(path, ".io") {
		return false
	}

	// Skip very short paths that are likely false positives
	if len(path) < 3 {
		return false
	}

	// Must have at least one path separator or be a file with extension
	ext := filepath.Ext(path)
	hasSlash := strings.Contains(path, "/")
	hasValidExt := ext != "" && validExtensions[ext]

	return hasSlash || hasValidExt
}

// ExtractPathsFromTask extracts all path references from a task's fields.
func ExtractPathsFromTask(task *LLMTaskSchema) []PathReference {
	var allText strings.Builder

	allText.WriteString(task.Title)
	allText.WriteString(" ")
	allText.WriteString(task.Description)

	for _, criterion := range task.AcceptanceCriteria {
		allText.WriteString(" ")
		allText.WriteString(criterion)
	}

	for _, step := range task.ValidationSteps {
		allText.WriteString(" ")
		allText.WriteString(step)
	}

	return ExtractPaths(allText.String())
}
