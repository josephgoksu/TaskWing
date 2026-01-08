package bootstrap

import (
	"context"
	"encoding/json"
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

	// 1. Root Level Context
	// Priority order: README, Architecture, other MDs, manifests
	files := []struct {
		pattern string
		heading string
	}{
		{"README.md", "README"},
		{"README", "README"},
		{"readme.md", "README"},
		{"ARCHITECTURE.md", "Architecture"}, // Existing architecture doc
		{"CONTRIBUTING.md", "Contributing"},
		{"STRATEGIC_ANALYSIS.md", "Strategic Analysis"},
		{"package.json", "Root Package Manifest"},
		{"go.mod", "Root Go Module"},
		{"Cargo.toml", "Root Cargo Manifest"},
		{"pom.xml", "Root Maven POM"},
		{"build.gradle", "Root Gradle Build"},
	}

	foundRootContext := false

	// Check specific priority files
	for _, f := range files {
		fullPath := filepath.Join(a.projectPath, f.pattern)
		if content, err := os.ReadFile(fullPath); err == nil {
			foundRootContext = true
			text := string(content)
			if len(text) > 8000 {
				text = text[:8000] + "\n... [truncated]"
			}
			sb.WriteString(fmt.Sprintf("## %s (%s)\n\n%s\n\n", f.heading, f.pattern, text))
		}
	}

	// 2. Docs Folder (as before)
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
			foundRootContext = true
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

	// 3. Workspace / Monorepo Discovery (Fallback or Addition)
	// If root context is thin (or even if not), listing sub-projects is crucial for monorepos
	subProjects := findSubProjects(a.projectPath)
	if len(subProjects) > 0 {
		foundRootContext = true
		sb.WriteString("## Workspace Structure (Sub-projects)\n\n")
		sb.WriteString("The project appears to be a monorepo containing the following services:\n\n")

		for _, p := range subProjects {
			sb.WriteString(fmt.Sprintf("- **%s**", p.Name))
			if p.Description != "" {
				sb.WriteString(fmt.Sprintf(": %s", p.Description))
			} else if p.Type != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", p.Type))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")

		// If we have very little root context, grab READMEs from top 3 sub-projects
		if sb.Len() < 1000 && len(subProjects) > 0 {
			sb.WriteString("## Sub-project Context\n\n")
			limit := 3
			if len(subProjects) < limit {
				limit = len(subProjects)
			}
			for i := 0; i < limit; i++ {
				p := subProjects[i]
				readmePath := filepath.Join(a.projectPath, p.Name, "README.md")
				if content, err := os.ReadFile(readmePath); err == nil {
					text := string(content)
					if len(text) > 3000 {
						text = text[:3000] + "\n... [truncated]"
					}
					sb.WriteString(fmt.Sprintf("### %s/README.md\n\n%s\n\n", p.Name, text))
				}
			}
		}
	}

	// 4. Directory Listing (Last Resort)
	if !foundRootContext {
		entries, err := os.ReadDir(a.projectPath)
		if err == nil {
			sb.WriteString("## Directory Listing\n\n")
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, ".") {
					continue
				}
				if e.IsDir() {
					sb.WriteString(fmt.Sprintf("- %s/\n", name))
				} else {
					sb.WriteString(fmt.Sprintf("- %s\n", name))
				}
			}
		}
	}

	return sb.String()
}

type subProjectInfo struct {
	Name        string
	Type        string
	Description string // Parsed from package.json if possible
}

// findSubProjects scans 1 level deep for project markers
func findSubProjects(root string) []subProjectInfo {
	var projects []subProjectInfo
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		dirName := entry.Name()
		dirPath := filepath.Join(root, dirName)

		// Check for markers
		if _, err := os.Stat(filepath.Join(dirPath, "package.json")); err == nil {
			desc := ""
			// Try to read description from package.json
			if content, err := os.ReadFile(filepath.Join(dirPath, "package.json")); err == nil {
				// Very low-tech regex scan to avoid struct definition overhead for just one field
				// or strictly, we do a quick map unmarshal
				var pkg map[string]any
				if json.Unmarshal(content, &pkg) == nil {
					if d, ok := pkg["description"].(string); ok {
						desc = d
					}
				}
			}
			projects = append(projects, subProjectInfo{Name: dirName, Type: "Node.js", Description: desc})
		} else if _, err := os.Stat(filepath.Join(dirPath, "go.mod")); err == nil {
			projects = append(projects, subProjectInfo{Name: dirName, Type: "Go"})
		} else if _, err := os.Stat(filepath.Join(dirPath, "pom.xml")); err == nil {
			projects = append(projects, subProjectInfo{Name: dirName, Type: "Java/Maven"})
		} else if _, err := os.Stat(filepath.Join(dirPath, "Dockerfile")); err == nil {
			projects = append(projects, subProjectInfo{Name: dirName, Type: "Docker"})
		}
	}
	return projects
}
