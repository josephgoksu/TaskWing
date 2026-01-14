// Package planner provides semantic validation middleware for plans.
package planner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// SemanticValidationResult contains the results of semantic validation.
type SemanticValidationResult struct {
	Valid    bool                    `json:"valid"`
	Warnings []SemanticWarning       `json:"warnings,omitempty"`
	Errors   []SemanticError         `json:"errors,omitempty"`
	Stats    SemanticValidationStats `json:"stats"`
}

// SemanticWarning represents a non-blocking issue.
type SemanticWarning struct {
	TaskIndex int    `json:"task_index"`
	TaskTitle string `json:"task_title"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Path      string `json:"path,omitempty"`
}

// SemanticError represents a blocking issue.
type SemanticError struct {
	TaskIndex int    `json:"task_index"`
	TaskTitle string `json:"task_title"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Path      string `json:"path,omitempty"`
	Command   string `json:"command,omitempty"`
}

// SemanticValidationStats tracks validation statistics.
type SemanticValidationStats struct {
	TotalTasks        int `json:"total_tasks"`
	PathsChecked      int `json:"paths_checked"`
	PathsMissing      int `json:"paths_missing"`
	CommandsValidated int `json:"commands_validated"`
	CommandsInvalid   int `json:"commands_invalid"`
}

// MiddlewareConfig configures semantic validation behavior.
type MiddlewareConfig struct {
	// BasePath is the project root for resolving relative paths
	BasePath string
	// SkipFileValidation disables file existence checks
	SkipFileValidation bool
	// SkipCommandValidation disables shell command validation
	SkipCommandValidation bool
	// AllowMissingFiles treats missing files as warnings, not errors
	AllowMissingFiles bool
}

// SemanticMiddleware validates plans for semantic correctness.
type SemanticMiddleware struct {
	cfg            MiddlewareConfig
	shellAvailable bool
	shellChecked   bool
}

// NewSemanticMiddleware creates a new semantic validation middleware.
func NewSemanticMiddleware(cfg MiddlewareConfig) *SemanticMiddleware {
	if cfg.BasePath == "" {
		cfg.BasePath, _ = os.Getwd()
	}
	m := &SemanticMiddleware{cfg: cfg}
	if !cfg.SkipCommandValidation {
		if _, err := exec.LookPath("bash"); err == nil {
			m.shellAvailable = true
		}
		m.shellChecked = true
	}
	return m
}

// Validate performs semantic validation on a plan.
func (m *SemanticMiddleware) Validate(plan *LLMPlanResponse) SemanticValidationResult {
	result := SemanticValidationResult{
		Valid: true,
		Stats: SemanticValidationStats{
			TotalTasks: len(plan.Tasks),
		},
	}

	for i, task := range plan.Tasks {
		// Validate file paths in task description and acceptance criteria
		if !m.cfg.SkipFileValidation {
			m.validateFilePaths(&result, i, &task)
		}

		// Validate shell commands in validation steps
		if !m.cfg.SkipCommandValidation {
			m.validateCommands(&result, i, &task)
		}
	}

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	return result
}

// validateFilePaths checks for file path references in the task.
func (m *SemanticMiddleware) validateFilePaths(result *SemanticValidationResult, taskIdx int, task *LLMTaskSchema) {
	// Extract paths from description, acceptance criteria, and validation steps
	allText := task.Description
	for _, criterion := range task.AcceptanceCriteria {
		allText += " " + criterion
	}
	for _, step := range task.ValidationSteps {
		allText += " " + step
	}

	paths := extractFilePaths(allText)
	result.Stats.PathsChecked += len(paths)

	for _, path := range paths {
		// Skip if it looks like it's being created
		if isCreationContext(allText, path) {
			continue
		}

		// Resolve relative paths
		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(m.cfg.BasePath, path)
		}

		// Check existence
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			result.Stats.PathsMissing++

			if m.cfg.AllowMissingFiles {
				result.Warnings = append(result.Warnings, SemanticWarning{
					TaskIndex: taskIdx,
					TaskTitle: task.Title,
					Type:      "missing_file",
					Message:   fmt.Sprintf("Referenced file does not exist: %s", path),
					Path:      path,
				})
			} else {
				result.Errors = append(result.Errors, SemanticError{
					TaskIndex: taskIdx,
					TaskTitle: task.Title,
					Type:      "missing_file",
					Message:   fmt.Sprintf("Referenced file does not exist: %s", path),
					Path:      path,
				})
			}
		}
	}
}

// validateCommands checks shell commands for syntax validity.
func (m *SemanticMiddleware) validateCommands(result *SemanticValidationResult, taskIdx int, task *LLMTaskSchema) {
	if !m.shellChecked {
		if _, err := exec.LookPath("bash"); err == nil {
			m.shellAvailable = true
		}
		m.shellChecked = true
	}
	if !m.shellAvailable {
		if len(task.ValidationSteps) > 0 {
			result.Warnings = append(result.Warnings, SemanticWarning{
				TaskIndex: taskIdx,
				TaskTitle: task.Title,
				Type:      "command_validation_skipped",
				Message:   "bash not available; skipping shell syntax validation",
			})
		}
		return
	}

	for _, step := range task.ValidationSteps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		result.Stats.CommandsValidated++

		if err := validateShellSyntax(step); err != nil {
			result.Stats.CommandsInvalid++
			result.Errors = append(result.Errors, SemanticError{
				TaskIndex: taskIdx,
				TaskTitle: task.Title,
				Type:      "invalid_command",
				Message:   fmt.Sprintf("Shell syntax error: %s", err.Error()),
				Command:   step,
			})
		}
	}
}

// Regular expressions for extracting file paths
var (
	// Match common file path patterns
	filePathRegex = regexp.MustCompile(`(?:^|[^a-zA-Z0-9])(/[a-zA-Z0-9_\-./]+\.[a-z]+)(?:[^a-zA-Z0-9]|$)`)
	// Match relative paths with extensions
	relativePathRegex = regexp.MustCompile(`(?:^|[^a-zA-Z0-9./])([a-zA-Z0-9_\-]+(?:/[a-zA-Z0-9_\-]+)*\.[a-z]{1,5})(?:[^a-zA-Z0-9]|$)`)
	// Match paths in backticks or quotes
	quotedPathRegex = regexp.MustCompile("(?:`|\")'?([a-zA-Z0-9_\\-./]+\\.[a-z]{1,5})'?(?:`|\")")
)

// extractFilePaths finds file path references in text.
func extractFilePaths(text string) []string {
	pathSet := make(map[string]bool)

	// Find absolute paths
	for _, match := range filePathRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 && isLikelyFilePath(match[1]) {
			pathSet[match[1]] = true
		}
	}

	// Find relative paths
	for _, match := range relativePathRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 && isLikelyFilePath(match[1]) {
			pathSet[match[1]] = true
		}
	}

	// Find quoted paths
	for _, match := range quotedPathRegex.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 && isLikelyFilePath(match[1]) {
			pathSet[match[1]] = true
		}
	}

	var paths []string
	for path := range pathSet {
		paths = append(paths, path)
	}
	return paths
}

// isLikelyFilePath filters out false positives.
func isLikelyFilePath(path string) bool {
	// Skip common non-file patterns
	if strings.HasPrefix(path, "http") || strings.HasPrefix(path, "www.") {
		return false
	}

	// Skip URLs disguised as paths (e.g., //example.com)
	if strings.Contains(path, "//") || strings.Contains(path, ".com") || strings.Contains(path, ".org") {
		return false
	}

	// Must have a recognizable extension
	ext := filepath.Ext(path)
	if ext == "" {
		return false
	}

	// Common source code extensions
	validExts := map[string]bool{
		".go": true, ".ts": true, ".js": true, ".tsx": true, ".jsx": true,
		".py": true, ".rs": true, ".java": true, ".cpp": true, ".c": true,
		".h": true, ".hpp": true, ".rb": true, ".php": true, ".swift": true,
		".kt": true, ".scala": true, ".vue": true, ".svelte": true,
		".css": true, ".scss": true, ".less": true, ".html": true,
		".yaml": true, ".yml": true, ".json": true, ".xml": true,
		".md": true, ".txt": true, ".toml": true, ".sh": true,
		".sql": true, ".proto": true, ".graphql": true,
	}

	return validExts[ext]
}

// isCreationContext checks if a path is mentioned in a "create" context.
func isCreationContext(text, path string) bool {
	// Find position of path in text
	idx := strings.Index(text, path)
	if idx == -1 {
		return false
	}

	// Check text before the path for creation indicators
	before := strings.ToLower(text[max(0, idx-50):idx])
	creationKeywords := []string{
		"create", "add", "new", "generate", "write",
		"initialize", "scaffold", "setup", "make",
	}

	for _, keyword := range creationKeywords {
		if strings.Contains(before, keyword) {
			return true
		}
	}

	return false
}

// validateShellSyntax checks if a shell command is syntactically valid.
func validateShellSyntax(command string) error {
	if command == "" {
		return fmt.Errorf("empty command")
	}

	// Use bash -n for syntax checking (dry run)
	cmd := exec.Command("bash", "-n", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Extract meaningful error from bash output
		errMsg := strings.TrimSpace(string(output))
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// ErrorSummary returns a human-readable summary of validation errors.
func (r SemanticValidationResult) ErrorSummary() string {
	if r.Valid {
		return ""
	}

	var parts []string
	for _, e := range r.Errors {
		parts = append(parts, fmt.Sprintf("[Task %d: %s] %s", e.TaskIndex, e.TaskTitle, e.Message))
	}
	return strings.Join(parts, "; ")
}

// WarningSummary returns a human-readable summary of warnings.
func (r SemanticValidationResult) WarningSummary() string {
	if len(r.Warnings) == 0 {
		return ""
	}

	var parts []string
	for _, w := range r.Warnings {
		parts = append(parts, fmt.Sprintf("[Task %d: %s] %s", w.TaskIndex, w.TaskTitle, w.Message))
	}
	return strings.Join(parts, "; ")
}
