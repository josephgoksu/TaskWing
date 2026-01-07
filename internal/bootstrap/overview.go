package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// OverviewAnalyzer generates project overviews by analyzing README and manifest files.
type OverviewAnalyzer struct {
	cfg         llm.Config
	projectPath string
}

// NewOverviewAnalyzer creates a new analyzer for generating project overviews.
func NewOverviewAnalyzer(cfg llm.Config, projectPath string) *OverviewAnalyzer {
	return &OverviewAnalyzer{
		cfg:         cfg,
		projectPath: projectPath,
	}
}

// overviewResponse is the expected JSON response from the LLM.
type overviewResponse struct {
	ShortDescription string `json:"short_description"`
	LongDescription  string `json:"long_description"`
}

// promptTemplateOverview is the prompt template for generating project overviews.
const promptTemplateOverview = `You are analyzing a software project to generate a concise overview.

## Project Files

{{.Content}}

## Task

Based on the project files above, generate:

1. **short_description**: A single sentence (max 100 characters) that captures what this project does. Be specific about the technology and purpose.

2. **long_description**: 2-3 paragraphs that explain:
   - What the project does and its main purpose
   - Key features or capabilities
   - Target users or use cases (if apparent)

## Output Format

Return ONLY valid JSON with this structure:
` + "```json" + `
{
  "short_description": "A one-sentence summary of the project.",
  "long_description": "A detailed description spanning 2-3 paragraphs.\n\nSecond paragraph with more details.\n\nThird paragraph about use cases."
}
` + "```"

// Analyze generates a ProjectOverview by reading project files and calling the LLM.
func (a *OverviewAnalyzer) Analyze(ctx context.Context) (*memory.ProjectOverview, error) {
	// Gather context from key project files
	content := a.gatherProjectContext()
	if content == "" {
		return nil, fmt.Errorf("no project files found (README.md, package.json, go.mod, etc.)")
	}

	// Create LLM chain
	chatModel, err := llm.NewCloseableChatModel(ctx, a.cfg)
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}
	defer chatModel.Close()

	chain, err := core.NewDeterministicChain[overviewResponse](
		ctx,
		"overview",
		chatModel.BaseChatModel,
		promptTemplateOverview,
	)
	if err != nil {
		return nil, fmt.Errorf("create chain: %w", err)
	}

	// Invoke the chain
	parsed, _, _, err := chain.Invoke(ctx, map[string]any{
		"Content": content,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke LLM: %w", err)
	}

	// Validate response
	if parsed.ShortDescription == "" || parsed.LongDescription == "" {
		return nil, fmt.Errorf("LLM returned empty description")
	}

	// Build the overview
	overview := &memory.ProjectOverview{
		ShortDescription: parsed.ShortDescription,
		LongDescription:  parsed.LongDescription,
		GeneratedAt:      time.Now().UTC(),
	}

	return overview, nil
}

// gatherProjectContext reads key project files and returns their concatenated content.
func (a *OverviewAnalyzer) gatherProjectContext() string {
	var sb strings.Builder

	// Priority order: README first, then manifests, then docs
	files := []struct {
		path    string
		heading string
	}{
		{"README.md", "README"},
		{"readme.md", "README"},
		{"README", "README"},
		{"package.json", "Package Manifest (Node.js)"},
		{"go.mod", "Go Module"},
		{"Cargo.toml", "Cargo Manifest (Rust)"},
		{"pyproject.toml", "Python Project"},
		{"setup.py", "Python Setup"},
		{"pom.xml", "Maven POM (Java)"},
		{"build.gradle", "Gradle Build (Java)"},
		{"Gemfile", "Ruby Gemfile"},
		{"composer.json", "Composer (PHP)"},
	}

	readmeFound := false
	for _, f := range files {
		// Skip duplicate README formats
		if strings.HasPrefix(f.heading, "README") && readmeFound {
			continue
		}

		fullPath := filepath.Join(a.projectPath, f.path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		if strings.HasPrefix(f.heading, "README") {
			readmeFound = true
		}

		// Truncate very long files
		text := string(content)
		if len(text) > 8000 {
			text = text[:8000] + "\n... [truncated]"
		}

		sb.WriteString(fmt.Sprintf("## %s (%s)\n\n%s\n\n", f.heading, f.path, text))
	}

	// Also check for docs folder
	docsPath := filepath.Join(a.projectPath, "docs")
	if entries, err := os.ReadDir(docsPath); err == nil {
		var docsContent strings.Builder
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			fullPath := filepath.Join(docsPath, entry.Name())
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			text := string(content)
			if len(text) > 2000 {
				text = text[:2000] + "\n... [truncated]"
			}
			docsContent.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", entry.Name(), text))
		}
		if docsContent.Len() > 0 {
			sb.WriteString("## Documentation (docs/)\n\n")
			sb.WriteString(docsContent.String())
		}
	}

	return sb.String()
}
