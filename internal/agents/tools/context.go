/*
Package tools provides shared tools for agent analysis.
*/
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/patterns"
)

// ContextGatherer provides methods for gathering file context.
type ContextGatherer struct {
	BasePath string
}

// NewContextGatherer creates a new helper for gathering context.
func NewContextGatherer(basePath string) *ContextGatherer {
	return &ContextGatherer{BasePath: basePath}
}

// GatherMarkdownDocs reads all markdown files in root and docs/ directory.
func (g *ContextGatherer) GatherMarkdownDocs() string {
	var sb strings.Builder
	seen := make(map[string]bool)

	gatherFromDir := func(dir, prefix string, maxLen int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".md") || seen[strings.ToLower(name)] {
				continue
			}
			content, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			if len(content) > maxLen {
				content = content[:maxLen]
			}
			relPath := name
			if prefix != "" {
				relPath = filepath.Join(prefix, name)
			}
			sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", relPath, string(content)))
			seen[strings.ToLower(name)] = true
		}
	}

	gatherFromDir(g.BasePath, "", 4000)
	gatherFromDir(filepath.Join(g.BasePath, "docs"), "docs", 3000)
	return sb.String()
}

// GatherKeyFiles reads critical key files like README.md, go.mod, package.json.
func (g *ContextGatherer) GatherKeyFiles() string {
	var sb strings.Builder
	// 1. Always gather Makefiles and critical config files
	keyFiles := []string{
		"README.md", "go.mod", "package.json", "Makefile", "makefile", "Justfile",
	}

	// 2. Add Rule Files (GEMINI.md, etc.)
	for name := range patterns.RuleFiles {
		keyFiles = append(keyFiles, name)
	}

	// 3. Add Important Dotfiles
	for name := range patterns.ImportantDotFiles {
		keyFiles = append(keyFiles, name)
	}

	seen := make(map[string]bool)
	for _, relPath := range keyFiles {
		if seen[relPath] {
			continue
		}
		seen[relPath] = true

		content, err := os.ReadFile(filepath.Join(g.BasePath, relPath))
		if err != nil {
			continue
		}
		if len(content) > 3000 {
			content = append(content[:3000], []byte("\n...[truncated]")...)
		}
		sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", relPath, string(content)))
	}
	return sb.String()
}

// GatherCIConfigs reads CI/CD workflow files from .github, .gitlab, .circleci.
func (g *ContextGatherer) GatherCIConfigs() string {
	var sb strings.Builder
	maxPerFile := 3000

	// GitHub Actions
	ghWorkflows := filepath.Join(g.BasePath, ".github", "workflows")
	if entries, err := os.ReadDir(ghWorkflows); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(ghWorkflows, name))
			if err != nil {
				continue
			}
			if len(content) > maxPerFile {
				content = content[:maxPerFile]
			}
			relPath := filepath.Join(".github/workflows", name)
			sb.WriteString(fmt.Sprintf("## %s\n```yaml\n%s\n```\n\n", relPath, string(content)))
		}
	}

	// GitLab CI
	gitlabCI := filepath.Join(g.BasePath, ".gitlab-ci.yml")
	if content, err := os.ReadFile(gitlabCI); err == nil {
		if len(content) > maxPerFile {
			content = content[:maxPerFile]
		}
		sb.WriteString(fmt.Sprintf("## .gitlab-ci.yml\n```yaml\n%s\n```\n\n", string(content)))
	}

	// CircleCI
	circleCI := filepath.Join(g.BasePath, ".circleci", "config.yml")
	if content, err := os.ReadFile(circleCI); err == nil {
		if len(content) > maxPerFile {
			content = content[:maxPerFile]
		}
		sb.WriteString(fmt.Sprintf("## .circleci/config.yml\n```yaml\n%s\n```\n\n", string(content)))
	}

	return sb.String()
}

// GatherSpecificFiles reads specific files given a list of relative paths.
func (g *ContextGatherer) GatherSpecificFiles(files []string) string {
	var sb strings.Builder
	for _, relPath := range files {
		content, err := os.ReadFile(filepath.Join(g.BasePath, relPath))
		if err != nil {
			continue
		}
		if len(content) > 8000 {
			content = content[:8000]
		}
		sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", relPath, string(content)))
	}
	return sb.String()
}

// GatherSourceCode reads key source files: entry points, handlers, configs.
// Returns file contents with line numbers for evidence extraction.
// Handles both standard projects and monorepos.
func (g *ContextGatherer) GatherSourceCode() string {
	var sb strings.Builder
	gathered := 0
	maxFiles := 10
	maxPerFile := 3000
	seen := make(map[string]bool)

	// Helper to add a file
	addFile := func(relPath string, maxLines int) bool {
		if gathered >= maxFiles || seen[relPath] {
			return false
		}
		fullPath := filepath.Join(g.BasePath, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return false
		}
		seen[relPath] = true

		lines := strings.Split(string(content), "\n")
		if len(lines) > maxLines {
			lines = lines[:maxLines]
		}
		var numbered strings.Builder
		for i, line := range lines {
			numbered.WriteString(fmt.Sprintf("%4d\t%s\n", i+1, line))
		}
		output := numbered.String()
		if len(output) > maxPerFile {
			output = output[:maxPerFile] + "\n...[truncated]"
		}

		sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", relPath, output))
		gathered++
		return true
	}

	// Priority file patterns (relative to any base)
	priorityPatterns := []string{
		"main.go", "cmd/main.go", "cmd/root.go",
		"internal/api/*.go", "internal/server/*.go", "internal/config/*.go",
		"src/index.ts", "src/main.ts", "src/App.tsx",
		"src/index.js", "src/main.js", "src/App.jsx",
		"vite.config.ts", "next.config.js", "tailwind.config.js",
		// CI/CD workflows
		".github/workflows/*.yml", ".github/workflows/*.yaml",
		".gitlab-ci.yml", ".circleci/config.yml",
	}

	// First, detect monorepo structure by finding package.json/go.mod in subdirs
	var projectRoots []string
	projectRoots = append(projectRoots, "") // Always include base

	entries, err := os.ReadDir(g.BasePath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			// Skip ignored dirs and non-allowed dot dirs
			if patterns.ShouldIgnoreDir(entry.Name()) {
				continue
			}
			if patterns.ShouldSkipDotEntry(entry.Name(), true) {
				continue
			}
			subdir := entry.Name()
			// Check if it's a project root (has package.json, go.mod, Cargo.toml, etc.)
			markers := []string{"package.json", "go.mod", "Cargo.toml", "pyproject.toml", "pom.xml"}
			for _, marker := range markers {
				if _, err := os.Stat(filepath.Join(g.BasePath, subdir, marker)); err == nil {
					projectRoots = append(projectRoots, subdir)
					break
				}
			}
		}
	}

	// For each project root, try priority patterns
	for _, root := range projectRoots {
		if gathered >= maxFiles {
			break
		}
		for _, pattern := range priorityPatterns {
			if gathered >= maxFiles {
				break
			}
			searchPath := filepath.Join(g.BasePath, root, pattern)
			matches, _ := filepath.Glob(searchPath)
			for _, match := range matches {
				if gathered >= maxFiles {
					break
				}
				relPath, _ := filepath.Rel(g.BasePath, match)
				addFile(relPath, 150)
			}
		}
	}

	// If still need more files, walk directories to find source files
	if gathered < maxFiles {
		extensions := map[string]bool{".go": true, ".ts": true, ".tsx": true, ".js": true, ".py": true, ".rs": true}
		scanDirs := []string{"internal", "src", "cmd", "pkg", "app", "lib", "api", "server"}

		for _, root := range projectRoots {
			if gathered >= maxFiles {
				break
			}
			for _, dir := range scanDirs {
				if gathered >= maxFiles {
					break
				}
				searchDir := filepath.Join(g.BasePath, root, dir)
				filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
					if err != nil || gathered >= maxFiles {
						return filepath.SkipAll
					}
					if d.IsDir() {
						if patterns.ShouldIgnoreDir(d.Name()) {
							return filepath.SkipDir
						}
						return nil
					}
					ext := filepath.Ext(d.Name())
					if !extensions[ext] {
						return nil
					}
					// Skip test files for brevity
					if strings.HasSuffix(d.Name(), "_test.go") || strings.HasSuffix(d.Name(), ".test.ts") {
						return nil
					}
					relPath, _ := filepath.Rel(g.BasePath, path)
					addFile(relPath, 100)
					return nil
				})
			}
		}
	}

	if gathered == 0 {
		return "No source code files found."
	}
	return sb.String()
}

// ListDirectoryTree returns a simple tree structure of the project.
func (g *ContextGatherer) ListDirectoryTree(maxDepth int) string {
	var sb strings.Builder
	if maxDepth == 0 {
		maxDepth = 2
	}
	filepath.WalkDir(g.BasePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(g.BasePath, path)
		if rel == "." {
			return nil
		}
		// Skip ignored directories and non-allowed dot entries
		if patterns.ShouldIgnoreDir(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if patterns.ShouldSkipDotEntry(d.Name(), d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator)) + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		indent := strings.Repeat("  ", depth-1)
		indicator := ""
		if d.IsDir() {
			indicator = "/"
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", indent, d.Name(), indicator))
		return nil
	})
	return sb.String()
}
