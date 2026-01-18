// Package tools provides shared tools for agent analysis.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/patterns"
)

// ChunkConfig configures the chunking behavior.
type ChunkConfig struct {
	MaxTokensPerChunk int  // Target tokens per chunk (default: 30000)
	MaxFilesPerChunk  int  // Max files per chunk (default: 50)
	IncludeLineNumbers bool // Add line numbers to file content
}

// DefaultChunkConfig returns sensible defaults for chunking.
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		MaxTokensPerChunk:  30000, // ~30k tokens per chunk - safe for all providers
		MaxFilesPerChunk:   50,    // Prevent overly fragmented chunks
		IncludeLineNumbers: true,
	}
}

// FileChunk represents a group of files to be analyzed together.
type FileChunk struct {
	Index       int          // Chunk index (0-based)
	Files       []ChunkFile  // Files in this chunk
	Content     string       // Formatted content ready for LLM
	TokenCount  int          // Estimated token count
	Description string       // Human-readable description of chunk contents
}

// ChunkFile represents a single file in a chunk.
type ChunkFile struct {
	RelPath    string // Relative path from base
	Content    string // File content (possibly truncated)
	TokenCount int    // Estimated tokens for this file
	Truncated  bool   // Whether content was truncated
}

// CodeChunker splits a codebase into LLM-friendly chunks.
type CodeChunker struct {
	basePath string
	config   ChunkConfig
	coverage CoverageStats
}

// NewCodeChunker creates a new code chunker.
func NewCodeChunker(basePath string) *CodeChunker {
	return &CodeChunker{
		basePath: basePath,
		config:   DefaultChunkConfig(),
		coverage: CoverageStats{
			FilesRead:    make([]FileRecord, 0),
			FilesSkipped: make([]SkipRecord, 0),
		},
	}
}

// SetConfig updates the chunking configuration.
// C6 Fix: Validates config values and falls back to defaults for invalid values.
func (c *CodeChunker) SetConfig(cfg ChunkConfig) {
	defaults := DefaultChunkConfig()

	// Validate and apply MaxTokensPerChunk
	if cfg.MaxTokensPerChunk > 0 {
		c.config.MaxTokensPerChunk = cfg.MaxTokensPerChunk
	} else {
		c.config.MaxTokensPerChunk = defaults.MaxTokensPerChunk
	}

	// Validate and apply MaxFilesPerChunk
	if cfg.MaxFilesPerChunk > 0 {
		c.config.MaxFilesPerChunk = cfg.MaxFilesPerChunk
	} else {
		c.config.MaxFilesPerChunk = defaults.MaxFilesPerChunk
	}

	c.config.IncludeLineNumbers = cfg.IncludeLineNumbers
}

// GetCoverage returns coverage stats after chunking.
func (c *CodeChunker) GetCoverage() CoverageStats {
	return c.coverage
}

// ChunkSourceCode splits the codebase into chunks suitable for LLM analysis.
// Returns chunks ordered by priority (most important first).
func (c *CodeChunker) ChunkSourceCode() ([]FileChunk, error) {
	// Collect and prioritize files
	files, err := c.collectFiles()
	if err != nil {
		return nil, fmt.Errorf("collect files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no source files found")
	}

	// Sort by priority (lower score = higher priority)
	sort.Slice(files, func(i, j int) bool {
		return files[i].priority < files[j].priority
	})

	// Group files into chunks
	chunks := c.groupIntoChunks(files)

	return chunks, nil
}

// prioritizedFile holds a file with its priority score.
type prioritizedFile struct {
	relPath  string
	priority int // Lower = more important
}

// collectFiles walks the codebase and collects source files with priorities.
func (c *CodeChunker) collectFiles() ([]prioritizedFile, error) {
	var files []prioritizedFile
	seen := make(map[string]bool)

	// Priority scoring maps (same as context.go for consistency)
	priorityDirs := map[string]int{
		"middleware": 1, "middlewares": 1, "auth": 1, "security": 1,
		"handler": 2, "handlers": 2, "controller": 2, "controllers": 2,
		"router": 2, "routers": 2, "routes": 2,
		"error": 3, "errors": 3, "exceptions": 3,
		"config": 4, "model": 4, "models": 4, "types": 4, "schema": 4,
		"service": 5, "services": 5, "repository": 5, "api": 5,
		"internal": 6, "pkg": 6, "src": 6, "lib": 6, "cmd": 6,
	}

	priorityFiles := map[string]int{
		"main": 1, "index": 1, "app": 1, "server": 1,
		"middleware": 1, "auth": 1, "cors": 1,
		"handler": 2, "controller": 2, "router": 2,
		"error": 3, "config": 4, "model": 4, "types": 4,
		"service": 5, "repository": 5,
	}

	err := filepath.WalkDir(c.basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if patterns.ShouldIgnoreDir(d.Name()) || patterns.ShouldSkipDotEntry(d.Name(), true) {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(c.basePath, path)
		relPathLower := strings.ToLower(relPath)

		if seen[relPathLower] {
			return nil
		}

		// Skip non-code files
		ext := filepath.Ext(d.Name())
		if !patterns.CodeExtensions[ext] {
			return nil
		}

		// Skip test files
		if isTestFile(d.Name()) {
			c.coverage.FilesSkipped = append(c.coverage.FilesSkipped, SkipRecord{
				Path:   relPath,
				Reason: "test file",
			})
			return nil
		}

		// Skip symlinks
		if isSymlink(path) {
			c.coverage.FilesSkipped = append(c.coverage.FilesSkipped, SkipRecord{
				Path:   relPath,
				Reason: "symlink",
			})
			return nil
		}

		seen[relPathLower] = true

		// Calculate priority score
		score := 100
		dir := filepath.Dir(relPath)
		for dirName, priority := range priorityDirs {
			if strings.Contains(strings.ToLower(dir), dirName) {
				if priority < score {
					score = priority
				}
			}
		}

		baseName := strings.TrimSuffix(d.Name(), ext)
		if priority, ok := priorityFiles[strings.ToLower(baseName)]; ok {
			if priority < score {
				score = priority
			}
		}

		files = append(files, prioritizedFile{
			relPath:  relPath,
			priority: score,
		})

		return nil
	})

	return files, err
}

// groupIntoChunks groups files into chunks respecting token limits.
func (c *CodeChunker) groupIntoChunks(files []prioritizedFile) []FileChunk {
	var chunks []FileChunk
	var currentChunk FileChunk
	currentChunk.Index = 0
	currentTokens := 0
	maxPerFile := 8000 // Max characters per file to prevent one huge file from dominating

	for _, pf := range files {
		fullPath := filepath.Join(c.basePath, pf.relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			c.coverage.FilesSkipped = append(c.coverage.FilesSkipped, SkipRecord{
				Path:   pf.relPath,
				Reason: fmt.Sprintf("read error: %v", err),
			})
			continue
		}

		// C8 Fix: Truncate large files at valid UTF-8 boundary
		truncated := false
		if len(content) > maxPerFile {
			content = content[:maxPerFile]
			// Ensure we don't cut in the middle of a multi-byte UTF-8 character
			for len(content) > 0 && !utf8.Valid(content) {
				content = content[:len(content)-1]
			}
			truncated = true
		}

		// Add line numbers if configured
		fileContent := string(content)
		if c.config.IncludeLineNumbers {
			fileContent = addLineNumbers(fileContent)
		}

		tokenCount := llm.EstimateTokens(fileContent)

		// Check if this file would exceed chunk limits
		wouldExceedTokens := currentTokens+tokenCount > c.config.MaxTokensPerChunk
		wouldExceedFiles := len(currentChunk.Files) >= c.config.MaxFilesPerChunk

		if (wouldExceedTokens || wouldExceedFiles) && len(currentChunk.Files) > 0 {
			// Finalize current chunk
			currentChunk.Content = c.formatChunkContent(currentChunk.Files)
			currentChunk.TokenCount = currentTokens
			currentChunk.Description = c.describeChunk(currentChunk.Files)
			chunks = append(chunks, currentChunk)

			// Start new chunk
			currentChunk = FileChunk{Index: len(chunks)}
			currentTokens = 0
		}

		// Add file to current chunk
		currentChunk.Files = append(currentChunk.Files, ChunkFile{
			RelPath:    pf.relPath,
			Content:    fileContent,
			TokenCount: tokenCount,
			Truncated:  truncated,
		})
		currentTokens += tokenCount

		// Track coverage
		c.coverage.FilesRead = append(c.coverage.FilesRead, FileRecord{
			Path:       pf.relPath,
			Characters: len(content),
			Lines:      strings.Count(string(content), "\n") + 1,
			Truncated:  truncated,
		})
	}

	// Don't forget the last chunk
	if len(currentChunk.Files) > 0 {
		currentChunk.Content = c.formatChunkContent(currentChunk.Files)
		currentChunk.TokenCount = currentTokens
		currentChunk.Description = c.describeChunk(currentChunk.Files)
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// formatChunkContent formats files into a single string for LLM consumption.
func (c *CodeChunker) formatChunkContent(files []ChunkFile) string {
	var sb strings.Builder

	for _, f := range files {
		fmt.Fprintf(&sb, "## FILE: %s\n```\n%s\n```\n\n", f.RelPath, f.Content)
	}

	return sb.String()
}

// describeChunk creates a human-readable description of chunk contents.
func (c *CodeChunker) describeChunk(files []ChunkFile) string {
	if len(files) == 0 {
		return "empty chunk"
	}

	// Group by directory
	dirs := make(map[string]int)
	for _, f := range files {
		dir := filepath.Dir(f.RelPath)
		if dir == "." {
			dir = "root"
		}
		dirs[dir]++
	}

	// Build description
	var parts []string
	for dir, count := range dirs {
		parts = append(parts, fmt.Sprintf("%s (%d files)", dir, count))
	}

	sort.Strings(parts)
	if len(parts) > 3 {
		parts = append(parts[:3], fmt.Sprintf("and %d more directories", len(parts)-3))
	}

	return strings.Join(parts, ", ")
}
