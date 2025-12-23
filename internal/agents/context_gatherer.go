package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ContextGatherer provides shared methods for gathering file context
type ContextGatherer struct {
	BasePath string
}

// NewContextGatherer creates a new helper for gathering context
func NewContextGatherer(basePath string) *ContextGatherer {
	return &ContextGatherer{BasePath: basePath}
}

// GatherMarkdownDocs reads all markdown files in root and docs/ directory
func (g *ContextGatherer) GatherMarkdownDocs() string {
	var sb strings.Builder
	seen := make(map[string]bool)

	// Helper to gather from a directory
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

			// Avoid duplicates if same filename exists in both (unlikely but safe)
			// Actually we probably want to see both if paths differ, but let's just track by name for now as per original logic?
			// Original logic tracked 'seen' by filename.
			if seen[strings.ToLower(name)] {
				continue
			}

			path := filepath.Join(dir, name)
			content, err := os.ReadFile(path)
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

	// Root directory
	gatherFromDir(g.BasePath, "", 4000)

	// Docs directory
	gatherFromDir(filepath.Join(g.BasePath, "docs"), "docs", 3000)

	// .taskwing/docs directory (optional, but good practice)
	// gatherFromDir(filepath.Join(g.BasePath, ".taskwing", "docs"), ".taskwing/docs", 3000)

	return sb.String()
}

// GatherKeyFiles reads critical key files like README.md, go.mod, package.json
func (g *ContextGatherer) GatherKeyFiles() string {
	var sb strings.Builder
	keyFiles := []string{"README.md", "go.mod", "package.json", "Makefile"}

	for _, relPath := range keyFiles {
		path := filepath.Join(g.BasePath, relPath)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		maxLen := 2500
		if len(content) > maxLen {
			content = append(content[:maxLen], []byte("\n...[truncated]")...)
		}

		sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", relPath, string(content)))
	}

	return sb.String()
}

// GatherSpecificFiles reads specific files given a list of relative paths
func (g *ContextGatherer) GatherSpecificFiles(files []string) string {
	var sb strings.Builder
	for _, relPath := range files {
		path := filepath.Join(g.BasePath, relPath)
		content, err := os.ReadFile(path)
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

// ListDirectoryTree returns a simple tree structure of the project (ignoring hidden/vendor/node_modules)
// ListDirectoryTree returns a simple tree structure of the project (ignoring hidden/vendor/node_modules)
func (g *ContextGatherer) ListDirectoryTree(maxDepth int) string {
	var sb strings.Builder
	root := g.BasePath

	if maxDepth == 0 {
		maxDepth = 2
	}

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}

		// Skip hidden and noise
		if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "vendor" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Calculate depth
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
