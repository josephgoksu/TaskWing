package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MarkdownStore handles filesystem operations for human-readable files.
type MarkdownStore struct {
	basePath string
}

func NewMarkdownStore(basePath string) *MarkdownStore {
	return &MarkdownStore{basePath: basePath}
}

func (s *MarkdownStore) WriteFeature(f Feature, decisions []Decision) error {
	featuresDir := filepath.Join(s.basePath, "features")
	if err := os.MkdirAll(featuresDir, 0755); err != nil {
		return fmt.Errorf("create features dir: %w", err)
	}

	// Use the feature's FilePath if set, otherwise generate one
	filePath := f.FilePath
	if filePath == "" {
		safeName := strings.ToLower(strings.ReplaceAll(f.Name, " ", "-"))
		filePath = filepath.Join(featuresDir, safeName+".md")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", f.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", f.OneLiner))

	if len(decisions) > 0 {
		sb.WriteString("## Decisions\n\n")
		for _, d := range decisions {
			sb.WriteString(fmt.Sprintf("### %s\n", d.Title))
			sb.WriteString(fmt.Sprintf("- **Summary:** %s\n", d.Summary))
			if d.Reasoning != "" {
				sb.WriteString(fmt.Sprintf("- **Why:** %s\n", d.Reasoning))
			}
			if d.Tradeoffs != "" {
				sb.WriteString(fmt.Sprintf("- **Trade-offs:** %s\n", d.Tradeoffs))
			}
			sb.WriteString(fmt.Sprintf("- **Date:** %s\n\n", d.CreatedAt.Format("2006-01-02")))
		}
	}

	sb.WriteString("## Notes\n\n")
	sb.WriteString("<!-- Add notes here -->\n")

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

func (s *MarkdownStore) RemoveFile(path string) error {
	return os.Remove(path)
}
