// Package planner provides plan verification using code intelligence.
package planner

import (
	"context"
	"fmt"
	"os"
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
	query    *codeintel.QueryService
	basePath string // Project root for resolving relative paths
}

// VerifierConfig configures the PlanVerifier behavior.
type VerifierConfig struct {
	BasePath string // Project root directory
}

// NewPlanVerifier creates a new PlanVerifier with the given query service.
func NewPlanVerifier(query *codeintel.QueryService) *PlanVerifier {
	basePath, _ := os.Getwd()
	return &PlanVerifier{
		query:    query,
		basePath: basePath,
	}
}

// NewPlanVerifierWithConfig creates a PlanVerifier with custom configuration.
func NewPlanVerifierWithConfig(query *codeintel.QueryService, cfg VerifierConfig) *PlanVerifier {
	basePath := cfg.BasePath
	if basePath == "" {
		basePath, _ = os.Getwd()
	}
	return &PlanVerifier{
		query:    query,
		basePath: basePath,
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

// === Path Validation and Auto-Recovery ===

// PathValidationResult represents the outcome of validating a single path.
type PathValidationResult struct {
	Original    string   // The original path from the task
	Valid       bool     // True if path exists
	Corrected   string   // The corrected path (if auto-recovered)
	Suggestions []string // Alternative paths if ambiguous
	Note        string   // Additional context (e.g., "Did you mean...?")
}

// VerificationResult contains the full verification outcome for a task.
type VerificationResult struct {
	TaskIndex   int
	TaskTitle   string
	PathResults []PathValidationResult
	Corrections int // Number of auto-corrections applied
	Warnings    int // Number of suggestions/notes added
}

// ValidatePath checks if a path exists and attempts recovery if not.
func (v *PlanVerifier) ValidatePath(ctx context.Context, ref PathReference) PathValidationResult {
	result := PathValidationResult{
		Original: ref.Path,
	}

	// Resolve full path
	fullPath := ref.Path
	if !filepath.IsAbs(ref.Path) {
		fullPath = filepath.Join(v.basePath, ref.Path)
	}

	// Check if path exists
	if _, err := os.Stat(fullPath); err == nil {
		result.Valid = true
		return result
	}

	// Path doesn't exist - attempt recovery
	recovery := v.recoverPath(ctx, ref)
	result.Valid = recovery.found
	result.Corrected = recovery.correctedPath
	result.Suggestions = recovery.suggestions
	result.Note = recovery.note

	return result
}

// pathRecovery holds the result of a path recovery attempt.
type pathRecovery struct {
	found         bool
	correctedPath string
	suggestions   []string
	note          string
}

// recoverPath attempts to find the correct path using codeintel.
func (v *PlanVerifier) recoverPath(ctx context.Context, ref PathReference) pathRecovery {
	basename := filepath.Base(ref.Path)

	// Skip directory references - can't recover those easily
	if ref.IsDir {
		return pathRecovery{
			note: fmt.Sprintf("Directory %s not found", ref.Path),
		}
	}

	// If no query service, we can't search
	if v.query == nil {
		return pathRecovery{
			note: fmt.Sprintf("File %s not found (no code index available)", ref.Path),
		}
	}

	// Search for files matching the basename using HybridSearch
	results, err := v.query.HybridSearch(ctx, basename, 10)
	if err != nil || len(results) == 0 {
		return pathRecovery{
			note: fmt.Sprintf("File %s not found in codebase", ref.Path),
		}
	}

	// Filter results to those with matching basename
	var candidates []string
	for _, r := range results {
		if filepath.Base(r.Symbol.FilePath) == basename {
			// Deduplicate
			found := false
			for _, c := range candidates {
				if c == r.Symbol.FilePath {
					found = true
					break
				}
			}
			if !found {
				candidates = append(candidates, r.Symbol.FilePath)
			}
		}
	}

	// If no exact basename matches, try fuzzy matches
	if len(candidates) == 0 {
		nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
		for _, r := range results {
			fileBase := filepath.Base(r.Symbol.FilePath)
			fileNameWithoutExt := strings.TrimSuffix(fileBase, filepath.Ext(fileBase))
			if strings.Contains(strings.ToLower(fileNameWithoutExt), strings.ToLower(nameWithoutExt)) {
				found := false
				for _, c := range candidates {
					if c == r.Symbol.FilePath {
						found = true
						break
					}
				}
				if !found {
					candidates = append(candidates, r.Symbol.FilePath)
				}
			}
		}
	}

	// Apply recovery logic
	switch len(candidates) {
	case 0:
		return pathRecovery{
			note: fmt.Sprintf("File %s not found in codebase", ref.Path),
		}
	case 1:
		// Unique match - auto-correct
		return pathRecovery{
			found:         true,
			correctedPath: candidates[0],
			note:          fmt.Sprintf("Auto-corrected: %s -> %s", ref.Path, candidates[0]),
		}
	default:
		// Multiple candidates - provide suggestions
		return pathRecovery{
			suggestions: candidates,
			note:        fmt.Sprintf("Did you mean one of: %s?", strings.Join(candidates, ", ")),
		}
	}
}

// VerifyTaskPaths validates all paths in a task and returns the results.
func (v *PlanVerifier) VerifyTaskPaths(ctx context.Context, taskIdx int, task *LLMTaskSchema) VerificationResult {
	result := VerificationResult{
		TaskIndex: taskIdx,
		TaskTitle: task.Title,
	}

	paths := ExtractPathsFromTask(task)
	for _, ref := range paths {
		pathResult := v.ValidatePath(ctx, ref)
		result.PathResults = append(result.PathResults, pathResult)

		if pathResult.Corrected != "" {
			result.Corrections++
		}
		if len(pathResult.Suggestions) > 0 || (pathResult.Note != "" && !pathResult.Valid) {
			result.Warnings++
		}
	}

	return result
}

// ApplyCorrections applies path corrections to task text.
func ApplyCorrections(text string, corrections map[string]string) string {
	result := text
	for original, corrected := range corrections {
		result = strings.ReplaceAll(result, original, corrected)
	}
	return result
}
