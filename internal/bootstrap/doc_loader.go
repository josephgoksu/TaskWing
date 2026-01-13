package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// DocFile represents a documentation file loaded from the repository.
type DocFile struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	Size     int    `json:"size"`
	Category string `json:"category"` // readme, architecture, api, guide, etc.
}

// DocLoader extracts documentation files without LLM processing.
// Files are stored as-is for RAG retrieval.
type DocLoader struct {
	basePath string
	maxSize  int64 // Maximum file size to load (bytes)
}

// NewDocLoader creates a new documentation loader.
func NewDocLoader(basePath string) *DocLoader {
	return &DocLoader{
		basePath: basePath,
		maxSize:  512 * 1024, // 512KB max per file
	}
}

// Load finds and reads all documentation files in the repository.
func (l *DocLoader) Load() ([]DocFile, error) {
	var docs []DocFile

	// Priority files in root
	rootDocs := []struct {
		pattern  string
		category string
	}{
		{"README.md", "readme"},
		{"README.markdown", "readme"},
		{"README.txt", "readme"},
		{"README", "readme"},
		{"ARCHITECTURE.md", "architecture"},
		{"DESIGN.md", "architecture"},
		{"CONTRIBUTING.md", "contributing"},
		{"CHANGELOG.md", "changelog"},
		{"HISTORY.md", "changelog"},
		{"API.md", "api"},
		{"CLAUDE.md", "ai-instructions"},
		{"GEMINI.md", "ai-instructions"},
		{"CURSOR.md", "ai-instructions"},
	}

	// Load root-level priority docs
	for _, doc := range rootDocs {
		path := filepath.Join(l.basePath, doc.pattern)
		if content, err := l.loadFile(path); err == nil {
			docs = append(docs, DocFile{
				Path:     doc.pattern,
				Name:     doc.pattern,
				Content:  content,
				Size:     len(content),
				Category: doc.category,
			})
		}
	}

	// Scan docs/ directory if exists
	docsDir := filepath.Join(l.basePath, "docs")
	if info, err := os.Stat(docsDir); err == nil && info.IsDir() {
		docsDirFiles, err := l.scanDocsDirectory(docsDir)
		if err == nil {
			docs = append(docs, docsDirFiles...)
		}
	}

	// Scan .taskwing/ directory for existing docs
	taskwingDir := filepath.Join(l.basePath, ".taskwing")
	if info, err := os.Stat(taskwingDir); err == nil && info.IsDir() {
		// Load ARCHITECTURE.md if generated
		archPath := filepath.Join(taskwingDir, "ARCHITECTURE.md")
		if content, err := l.loadFile(archPath); err == nil {
			docs = append(docs, DocFile{
				Path:     ".taskwing/ARCHITECTURE.md",
				Name:     "ARCHITECTURE.md",
				Content:  content,
				Size:     len(content),
				Category: "architecture",
			})
		}
	}

	return docs, nil
}

func (l *DocLoader) loadFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.Size() > l.maxSize {
		return "", fmt.Errorf("file too large: %d bytes", info.Size())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (l *DocLoader) scanDocsDirectory(docsDir string) ([]DocFile, error) {
	var docs []DocFile

	err := filepath.WalkDir(docsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if d.IsDir() {
			// Skip hidden directories and node_modules
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process markdown and text files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".markdown" && ext != ".txt" && ext != ".rst" {
			return nil
		}

		content, err := l.loadFile(path)
		if err != nil {
			return nil // Skip files that can't be loaded
		}

		relPath, _ := filepath.Rel(l.basePath, path)
		category := l.categorizeDoc(relPath, d.Name())

		docs = append(docs, DocFile{
			Path:     relPath,
			Name:     d.Name(),
			Content:  content,
			Size:     len(content),
			Category: category,
		})

		return nil
	})

	return docs, err
}

func (l *DocLoader) categorizeDoc(path, name string) string {
	lower := strings.ToLower(name)
	pathLower := strings.ToLower(path)

	// Check filename patterns
	switch {
	case strings.Contains(lower, "readme"):
		return "readme"
	case strings.Contains(lower, "architecture") || strings.Contains(lower, "design"):
		return "architecture"
	case strings.Contains(lower, "api"):
		return "api"
	case strings.Contains(lower, "contributing"):
		return "contributing"
	case strings.Contains(lower, "changelog") || strings.Contains(lower, "history"):
		return "changelog"
	case strings.Contains(lower, "setup") || strings.Contains(lower, "install"):
		return "setup"
	case strings.Contains(lower, "guide") || strings.Contains(lower, "tutorial"):
		return "guide"
	case strings.Contains(lower, "config"):
		return "configuration"
	}

	// Check path patterns
	switch {
	case strings.Contains(pathLower, "api"):
		return "api"
	case strings.Contains(pathLower, "architecture"):
		return "architecture"
	case strings.Contains(pathLower, "guide"):
		return "guide"
	case strings.Contains(pathLower, "development"):
		return "development"
	}

	return "documentation"
}

// ToMarkdownIndex creates a markdown index of all loaded docs.
func ToMarkdownIndex(docs []DocFile) string {
	var sb strings.Builder

	sb.WriteString("# Documentation Index\n\n")

	// Group by category
	byCategory := make(map[string][]DocFile)
	for _, doc := range docs {
		byCategory[doc.Category] = append(byCategory[doc.Category], doc)
	}

	// Order categories
	categoryOrder := []string{
		"readme", "architecture", "api", "guide", "setup",
		"configuration", "contributing", "changelog", "ai-instructions",
		"development", "documentation",
	}

	for _, category := range categoryOrder {
		catDocs, ok := byCategory[category]
		if !ok || len(catDocs) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n\n", cases.Title(language.English).String(category)))
		for _, doc := range catDocs {
			sb.WriteString(fmt.Sprintf("- **%s** (%s, %d bytes)\n", doc.Name, doc.Path, doc.Size))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
