/*
Package patterns provides centralized file and directory patterns for the codebase.
This is the SINGLE source of truth for ignore lists, file extensions, and path patterns.
*/
package patterns

import "strings"

// IgnoredDirs contains directories that should be skipped during traversal.
var IgnoredDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	"dist":         true,
	"build":        true,
	"__pycache__":  true,
	".next":        true,
	".nuxt":        true,
	"coverage":     true,
}

// AllowedDotDirs contains dot-directories that SHOULD be analyzed (exceptions to the dot-skip rule).
// These often contain important architectural decisions (CI/CD, IDE configs, etc.)
var AllowedDotDirs = map[string]bool{
	".github":   true, // GitHub Actions workflows, issue templates, etc.
	".gitlab":   true, // GitLab CI configs
	".circleci": true, // CircleCI configs
	".vscode":   true, // VS Code workspace settings (can reveal project conventions)
	".devcontainer": true, // Dev container configs
}

// ImportantDotFiles contains dotfiles that should be gathered for analysis.
// These often contain important project configuration decisions.
var ImportantDotFiles = map[string]bool{
	".eslintrc":       true,
	".eslintrc.js":    true,
	".eslintrc.json":  true,
	".eslintrc.yaml":  true,
	".prettierrc":     true,
	".prettierrc.js":  true,
	".prettierrc.json": true,
	".dockerignore":   true,
	".gitignore":      true,
	".env.example":    true,
	".editorconfig":   true,
	".nvmrc":          true,
	".node-version":   true,
	".python-version": true,
	".ruby-version":   true,
	".tool-versions":  true, // asdf version manager
}

// ShouldIgnoreDir returns true if the directory should be skipped.
func ShouldIgnoreDir(name string) bool {
	return IgnoredDirs[name]
}

// IsAllowedDotDir returns true if this dot-directory should be analyzed.
func IsAllowedDotDir(name string) bool {
	return AllowedDotDirs[name]
}

// ShouldSkipDotEntry returns true if a dot-prefixed entry should be skipped.
// Returns false for allowed dot-directories and important dotfiles.
func ShouldSkipDotEntry(name string, isDir bool) bool {
	if !strings.HasPrefix(name, ".") {
		return false // Not a dot-entry, don't skip
	}
	if isDir {
		return !AllowedDotDirs[name] // Skip unless in allowlist
	}
	return !ImportantDotFiles[name] // Skip unless in allowlist
}

// CodeExtensions contains extensions for source code files.
var CodeExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rs": true, ".java": true, ".kt": true, ".swift": true,
	".c": true, ".cpp": true, ".h": true, ".hpp": true, ".cs": true,
	".rb": true, ".php": true, ".vue": true, ".svelte": true,
}

// ConfigExtensions contains extensions for configuration files.
var ConfigExtensions = map[string]bool{
	".yaml": true, ".yml": true, ".toml": true, ".json": true, ".ini": true,
}

// ConfigFiles contains specific configuration filenames.
var ConfigFiles = map[string]bool{
	"Dockerfile": true, "docker-compose.yaml": true, "docker-compose.yml": true,
	"Makefile": true, "justfile": true, ".env.example": true,
}

// DependencyFiles contains dependency manifest filenames.
var DependencyFiles = map[string]bool{
	"go.mod": true, "go.sum": true,
	"package.json": true, "package-lock.json": true,
	"yarn.lock": true, "pnpm-lock.yaml": true,
	"Cargo.toml": true, "Cargo.lock": true,
	"requirements.txt": true, "Pipfile": true, "pyproject.toml": true,
}

// IsCodeFile returns true if the extension belongs to a code file.
func IsCodeFile(ext string) bool {
	return CodeExtensions[ext]
}

// IsConfigFile returns true if the filename or extension is a config file.
func IsConfigFile(name, ext string) bool {
	return ConfigExtensions[ext] || ConfigFiles[name]
}

// IsDependencyFile returns true if the filename is a dependency manifest.
func IsDependencyFile(name string) bool {
	return DependencyFiles[name]
}
