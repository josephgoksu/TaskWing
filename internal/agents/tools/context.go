/*
Package tools provides shared tools for agent analysis.
*/
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/patterns"
)

// Package-level config file map (avoid allocation on every call)
var knownConfigFiles = map[string]bool{
	"vite.config.ts": true, "vite.config.js": true, "vite.config.mjs": true,
	"next.config.js": true, "next.config.mjs": true, "next.config.ts": true,
	"nuxt.config.ts": true, "nuxt.config.js": true,
	"tailwind.config.js": true, "tailwind.config.ts": true,
	"webpack.config.js": true, "webpack.config.ts": true,
	"rollup.config.js": true, "rollup.config.ts": true,
	"tsconfig.json": true, "jsconfig.json": true,
	"package.json": true, "package-lock.json": true,
	"pyproject.toml": true, "setup.py": true, "setup.cfg": true,
	"Cargo.toml": true, "Cargo.lock": true,
	"go.mod": true, "go.sum": true,
	"Makefile": true, "makefile": true, "Justfile": true,
	"Dockerfile": true, "docker-compose.yml": true, "docker-compose.yaml": true,
	".env.example": true, ".env.sample": true,
}

// Package-level test file patterns (avoid allocation on every call)
var testFilePatterns = []string{
	"_test.",  // Go: foo_test.go
	".test.",  // JS/TS: foo.test.ts, foo.test.js
	".spec.",  // JS/TS/Angular: foo.spec.ts
	"_spec.",  // Ruby: foo_spec.rb
	"test_",   // Python: test_foo.py
	"_tests.", // Various
	".tests.", // Various
}

var testDirPatterns = []string{
	"__tests__", // Jest
	"__test__",  // Python
}

// FileRecord tracks a file that was read during context gathering.
type FileRecord struct {
	Path       string
	Characters int
	Lines      int
	Truncated  bool
}

// SkipRecord tracks a file that was skipped during context gathering.
type SkipRecord struct {
	Path   string
	Reason string
}

// CoverageStats tracks what files were analyzed during context gathering.
type CoverageStats struct {
	FilesRead    []FileRecord
	FilesSkipped []SkipRecord
}

// ContextGatherer provides methods for gathering file context.
type ContextGatherer struct {
	BasePath string
	coverage CoverageStats
}

// NewContextGatherer creates a new helper for gathering context.
func NewContextGatherer(basePath string) *ContextGatherer {
	return &ContextGatherer{
		BasePath: basePath,
		coverage: CoverageStats{
			FilesRead:    make([]FileRecord, 0),
			FilesSkipped: make([]SkipRecord, 0),
		},
	}
}

// GetCoverage returns the accumulated coverage stats.
func (g *ContextGatherer) GetCoverage() CoverageStats {
	return g.coverage
}

// recordRead tracks a file that was successfully read.
func (g *ContextGatherer) recordRead(relPath string, content []byte, truncated bool) {
	g.coverage.FilesRead = append(g.coverage.FilesRead, FileRecord{
		Path:       relPath,
		Characters: len(content),
		Lines:      strings.Count(string(content), "\n") + 1,
		Truncated:  truncated,
	})
}

// recordSkip tracks a file that was skipped.
func (g *ContextGatherer) recordSkip(relPath, reason string) {
	g.coverage.FilesSkipped = append(g.coverage.FilesSkipped, SkipRecord{
		Path:   relPath,
		Reason: reason,
	})
}

// GatherMarkdownDocs reads markdown files from root, docs/, and package-level READMEs.
// Includes line numbers so LLM can provide accurate evidence with start_line/end_line.
func (g *ContextGatherer) GatherMarkdownDocs() string {
	var sb strings.Builder
	seen := make(map[string]bool) // Key: relative path (lowercase) for consistent deduplication

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
			if !strings.HasSuffix(strings.ToLower(name), ".md") {
				continue
			}
			relPath := name
			if prefix != "" {
				relPath = filepath.Join(prefix, name)
			}
			// Use full relative path as key for deduplication
			key := strings.ToLower(relPath)
			if seen[key] {
				continue
			}
			content, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			truncated := len(content) > maxLen
			if truncated {
				content = content[:maxLen]
			}
			// Track this file read for coverage reporting
			g.recordRead(relPath, content, truncated)
			// Add line numbers to content for accurate evidence extraction
			numberedContent := addLineNumbers(string(content))
			sb.WriteString(fmt.Sprintf("## FILE: %s\n```\n%s\n```\n\n", relPath, numberedContent))
			seen[key] = true
		}
	}

	// Root and docs/ (existing behavior)
	gatherFromDir(g.BasePath, "", 4000)
	gatherFromDir(filepath.Join(g.BasePath, "docs"), "docs", 3000)

	// NEW: Recursively gather package-level READMEs from common source directories
	// These contain critical implementation details (auth patterns, error handling, etc.)
	packageDirs := []string{"internal", "pkg", "lib", "src", "app", "server", "api"}

	// Also check for monorepo subdirectories (e.g., backend-go/internal)
	entries, _ := os.ReadDir(g.BasePath)
	for _, entry := range entries {
		if entry.IsDir() && !patterns.ShouldIgnoreDir(entry.Name()) {
			// Check if subdir has its own internal/pkg/src
			for _, subPkgDir := range []string{"internal", "pkg", "src"} {
				subPath := filepath.Join(entry.Name(), subPkgDir)
				if info, err := os.Stat(filepath.Join(g.BasePath, subPath)); err == nil && info.IsDir() {
					packageDirs = append(packageDirs, subPath)
				}
			}
		}
	}

	// Important markdown filenames to capture (case-insensitive)
	importantMdFiles := map[string]bool{
		"readme.md":   true,
		"agents.md":   true,
		"claude.md":   true,
		"gemini.md":   true,
		"design.md":   true,
		"arch.md":     true,
		"api.md":      true,
		"schema.md":   true,
		"security.md": true,
	}

	for _, pkgDir := range packageDirs {
		searchDir := filepath.Join(g.BasePath, pkgDir)
		filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if patterns.ShouldIgnoreDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			nameLower := strings.ToLower(d.Name())
			if !importantMdFiles[nameLower] {
				return nil
			}
			relPath, _ := filepath.Rel(g.BasePath, path)
			// Use lowercase path as key for consistent deduplication
			key := strings.ToLower(relPath)
			if seen[key] {
				return nil
			}
			seen[key] = true

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			// Package docs can be longer - they contain implementation details
			maxLen := 3000
			truncated := len(content) > maxLen
			if truncated {
				content = append(content[:maxLen], []byte("\n...[truncated]")...)
			}
			// Track this file read for coverage reporting
			g.recordRead(relPath, content, truncated)
			numberedContent := addLineNumbers(string(content))
			sb.WriteString(fmt.Sprintf("## PACKAGE DOC: %s\n```\n%s\n```\n\n", relPath, numberedContent))
			return nil
		})
	}

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
		truncated := len(content) > 3000
		if truncated {
			content = append(content[:3000], []byte("\n...[truncated]")...)
		}
		// Track this file read for coverage reporting
		g.recordRead(relPath, content, truncated)
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
			truncated := len(content) > maxPerFile
			if truncated {
				content = append(content[:maxPerFile], []byte("\n...[truncated]")...)
			}
			relPath := filepath.Join(".github", "workflows", name)
			// Track this file read for coverage reporting
			g.recordRead(relPath, content, truncated)
			sb.WriteString(fmt.Sprintf("## %s\n```yaml\n%s\n```\n\n", relPath, string(content)))
		}
	}

	// GitLab CI
	gitlabCI := filepath.Join(g.BasePath, ".gitlab-ci.yml")
	if content, err := os.ReadFile(gitlabCI); err == nil {
		truncated := len(content) > maxPerFile
		if truncated {
			content = append(content[:maxPerFile], []byte("\n...[truncated]")...)
		}
		// Track this file read for coverage reporting
		g.recordRead(".gitlab-ci.yml", content, truncated)
		sb.WriteString(fmt.Sprintf("## .gitlab-ci.yml\n```yaml\n%s\n```\n\n", string(content)))
	}

	// CircleCI
	circleCI := filepath.Join(g.BasePath, ".circleci", "config.yml")
	if content, err := os.ReadFile(circleCI); err == nil {
		truncated := len(content) > maxPerFile
		if truncated {
			content = append(content[:maxPerFile], []byte("\n...[truncated]")...)
		}
		// Track this file read for coverage reporting
		g.recordRead(".circleci/config.yml", content, truncated)
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

// GatherSourceCode reads key source files: entry points, handlers, configs, middleware.
// Returns file contents with line numbers for evidence extraction.
// Handles both standard projects and monorepos.
// Uses a priority-based walk system since Go's filepath.Glob doesn't support ** patterns.
func (g *ContextGatherer) GatherSourceCode() string {
	var sb strings.Builder
	gathered := 0
	maxFiles := 50          // Need to see more of the codebase
	maxPerFile := 4000      // Chars per file
	maxTotalChars := 150000 // Budget cap to prevent token overflow
	totalChars := 0
	seen := make(map[string]bool)

	// Helper to add a file - returns false if budget exhausted
	addFile := func(relPath string, maxLines int) bool {
		if gathered >= maxFiles {
			g.recordSkip(relPath, "max files limit reached")
			return false
		}
		if seen[relPath] {
			return false // Already processed, don't record again
		}
		if totalChars >= maxTotalChars {
			g.recordSkip(relPath, "character budget exhausted")
			return false
		}
		fullPath := filepath.Join(g.BasePath, relPath)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			return false
		}
		// Only add code files (filter by extension)
		ext := filepath.Ext(relPath)
		if !patterns.CodeExtensions[ext] && !patterns.ConfigExtensions[ext] && !isConfigFile(filepath.Base(relPath)) {
			g.recordSkip(relPath, "not a recognized code/config file")
			return false
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			g.recordSkip(relPath, fmt.Sprintf("read error: %v", err))
			return false
		}
		seen[relPath] = true

		lines := strings.Split(string(content), "\n")
		truncated := false
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			truncated = true
		}
		var numbered strings.Builder
		for i, line := range lines {
			numbered.WriteString(fmt.Sprintf("%4d\t%s\n", i+1, line))
		}
		output := numbered.String()
		if len(output) > maxPerFile {
			output = output[:maxPerFile] + "\n...[truncated]"
			truncated = true
		}

		// Check budget before adding
		if totalChars+len(output) > maxTotalChars {
			g.recordSkip(relPath, "would exceed character budget")
			return false
		}

		sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", relPath, output))
		gathered++
		totalChars += len(output)
		g.recordRead(relPath, content, truncated)
		return true
	}

	// Priority directory names - files in these dirs are more important
	// Language-agnostic: concept-based directory names that exist across ecosystems
	priorityDirs := map[string]int{
		// Tier 1: Middleware & Security (highest priority - often missed)
		"middleware": 1, "middlewares": 1,
		"auth": 1, "authentication": 1, "authorization": 1,
		"security": 1, "guards": 1, "interceptors": 1,
		// Tier 2: Handlers & Routers
		"handler": 2, "handlers": 2,
		"controller": 2, "controllers": 2,
		"router": 2, "routers": 2,
		"routes": 2, "views": 2, "endpoints": 2,
		// Tier 3: Error handling & Resilience
		"error": 3, "errors": 3, "exceptions": 3,
		"resilience": 3, "circuit": 3, "retry": 3,
		// Tier 4: Config & Models
		"config": 4, "configuration": 4, "settings": 4,
		"model": 4, "models": 4,
		"entity": 4, "entities": 4,
		"types": 4, "schema": 4, "schemas": 4, "dto": 4,
		// Tier 5: Service layer
		"service": 5, "services": 5,
		"usecase": 5, "usecases": 5,
		"repository": 5, "repositories": 5,
		"api": 5, "server": 5,
		// Tier 6: Standard source dirs
		"internal": 6, "pkg": 6, "src": 6, "lib": 6,
		"cmd": 6, "app": 6, "core": 6, "domain": 6,
	}

	// Priority file base names (without extension)
	priorityFiles := map[string]int{
		"main": 1, "index": 1, "app": 1, "server": 1,
		"middleware": 1, "auth": 1, "cors": 1,
		"handler": 2, "controller": 2, "router": 2, "routes": 2,
		"error": 3, "errors": 3, "config": 4, "settings": 4,
		"model": 4, "models": 4, "types": 4, "schema": 4,
		"service": 5, "repository": 5,
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
			if patterns.ShouldIgnoreDir(entry.Name()) || patterns.ShouldSkipDotEntry(entry.Name(), true) {
				continue
			}
			subdir := entry.Name()
			markers := []string{"package.json", "go.mod", "Cargo.toml", "pyproject.toml", "pom.xml"}
			for _, marker := range markers {
				if _, err := os.Stat(filepath.Join(g.BasePath, subdir, marker)); err == nil {
					projectRoots = append(projectRoots, subdir)
					break
				}
			}
		}
	}

	// Phase 1: Add ONLY root-level entry points and main config (strict budget: max 5 files)
	// CI/CD and other config files will be added in Phase 2 with lower priority
	phase1Budget := 5
	phase1Added := 0

	// Entry point files to look for (in priority order)
	entryPoints := []string{
		"main.go", "main.ts", "main.py", "main.rs",
		"index.ts", "index.js", "app.ts", "app.py",
		"server.go", "server.ts", "server.py",
	}

	for _, root := range projectRoots {
		if phase1Added >= phase1Budget {
			break
		}
		searchRoot := g.BasePath
		if root != "" {
			searchRoot = filepath.Join(g.BasePath, root)
		}

		// Look for entry points in root, cmd/, src/
		searchDirs := []string{"", "cmd", "src"}
		for _, subdir := range searchDirs {
			if phase1Added >= phase1Budget {
				break
			}
			dir := searchRoot
			if subdir != "" {
				dir = filepath.Join(searchRoot, subdir)
			}
			for _, entry := range entryPoints {
				if phase1Added >= phase1Budget {
					break
				}
				fullPath := filepath.Join(dir, entry)
				if _, err := os.Stat(fullPath); err == nil {
					relPath, _ := filepath.Rel(g.BasePath, fullPath)
					if addFile(relPath, 200) {
						phase1Added++
					}
				}
			}
		}
	}

	// Phase 2: Walk and collect files with priority scoring
	type scoredFile struct {
		path  string
		score int // lower is higher priority
	}
	var candidates []scoredFile
	candidateSeen := make(map[string]bool) // Track already-added candidates to avoid duplicates in monorepos

	for _, root := range projectRoots {
		searchBase := g.BasePath
		if root != "" {
			searchBase = filepath.Join(g.BasePath, root)
		}
		filepath.WalkDir(searchBase, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if patterns.ShouldIgnoreDir(d.Name()) {
					return filepath.SkipDir
				}
				// Skip dot-directories (except allowed ones like .github)
				if patterns.ShouldSkipDotEntry(d.Name(), true) {
					return filepath.SkipDir
				}
				return nil
			}
			relPath, _ := filepath.Rel(g.BasePath, path)
			// Skip symlinks to avoid infinite loops
			if isSymlink(path) {
				g.recordSkip(relPath, "symlink")
				return nil
			}

			// Check for duplicates FIRST (before any recording) to prevent duplicate skip records
			if seen[relPath] || candidateSeen[relPath] {
				return nil
			}

			ext := filepath.Ext(d.Name())
			name := d.Name()
			nameLower := strings.ToLower(name)

			// Determine file category and base priority
			isCode := patterns.CodeExtensions[ext]
			isConfig := patterns.ConfigExtensions[ext] || isConfigFile(name)
			isCIFile := strings.Contains(relPath, ".github") || strings.Contains(relPath, ".gitlab") || strings.Contains(relPath, ".circleci")

			// Skip files that aren't code, config, or CI
			if !isCode && !isConfig && !isCIFile {
				return nil
			}

			// Mark as seen now (even if we skip it, to prevent duplicate processing)
			candidateSeen[relPath] = true

			// Skip test files (but only for code files)
			if isCode && isTestFile(name) {
				g.recordSkip(relPath, "test file")
				return nil
			}

			// Calculate priority score (lower = higher priority)
			// Source code: 1-100, Config: 101-150, CI/CD: 151-200
			var score int
			if isCode {
				score = 100 // Default for code files
				// Check if file is in a priority directory
				dir := filepath.Dir(relPath)
				for dirName, priority := range priorityDirs {
					if strings.Contains(strings.ToLower(dir), dirName) {
						if priority < score {
							score = priority
						}
					}
				}
				// Check if filename matches priority
				baseName := strings.TrimSuffix(name, ext)
				if priority, ok := priorityFiles[strings.ToLower(baseName)]; ok {
					if priority < score {
						score = priority
					}
				}
				// Boost files with important keywords in name
				for _, keyword := range []string{"middleware", "auth", "cors", "rate", "circuit", "error"} {
					if strings.Contains(nameLower, keyword) {
						score = min(score, 2)
					}
				}
			} else if isConfig {
				score = 120 // Config files get lower priority than source
				// But vite.config, tsconfig get slight boost
				if strings.Contains(nameLower, "vite.config") || strings.Contains(nameLower, "tsconfig") {
					score = 110
				}
			} else if isCIFile {
				score = 150 // CI/CD files get lowest priority
				// Main CI files get slight boost
				if strings.Contains(nameLower, "ci.yml") || strings.Contains(nameLower, "deploy") {
					score = 140
				}
			}

			candidates = append(candidates, scoredFile{path: relPath, score: score})
			return nil
		})
	}

	// Sort by priority score (lower first) - O(n log n)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	// Phase 3: Add files in priority order until budget exhausted
	for _, c := range candidates {
		if gathered >= maxFiles || totalChars >= maxTotalChars {
			break
		}
		addFile(c.path, 150)
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

// addLineNumbers prefixes each line with its 1-indexed line number.
// Format: "  1: content", "  2: content", etc.
// This enables the LLM to provide accurate start_line/end_line in evidence.
func addLineNumbers(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(fmt.Sprintf("%4d: %s\n", i+1, line))
	}
	return sb.String()
}

// isConfigFile returns true if the filename is a known config file.
// Uses package-level map to avoid allocation on every call.
func isConfigFile(name string) bool {
	return knownConfigFiles[name]
}

// isTestFile returns true if the filename appears to be a test file.
// Language-agnostic: checks common test file patterns across languages.
// Uses package-level slices to avoid allocation on every call.
func isTestFile(name string) bool {
	nameLower := strings.ToLower(name)

	for _, pattern := range testFilePatterns {
		if strings.Contains(nameLower, pattern) {
			return true
		}
	}

	for _, pattern := range testDirPatterns {
		if strings.Contains(nameLower, pattern) {
			return true
		}
	}

	return false
}

// isSymlink returns true if the file is a symbolic link.
func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
