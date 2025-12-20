/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// CodeAgent analyzes code structure to identify architectural patterns and tech decisions
type CodeAgent struct {
	llmConfig llm.Config
}

// NewCodeAgent creates a new code structure analysis agent
func NewCodeAgent(cfg llm.Config) *CodeAgent {
	return &CodeAgent{llmConfig: cfg}
}

func (a *CodeAgent) Name() string        { return "code" }
func (a *CodeAgent) Description() string { return "Analyzes code structure for architectural patterns" }

func (a *CodeAgent) Run(ctx context.Context, input Input) (Output, error) {
	var output Output

	// Gather code structure info
	structure := a.analyzeStructure(input.BasePath)
	if structure == "" {
		return output, fmt.Errorf("no code structure found")
	}

	// Build prompt
	prompt := a.buildPrompt(input.ProjectName, structure)

	// Call LLM
	chatModel, err := llm.NewChatModel(ctx, a.llmConfig)
	if err != nil {
		return output, fmt.Errorf("create model: %w", err)
	}

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return output, fmt.Errorf("llm generate: %w", err)
	}

	output.RawOutput = resp.Content

	// Parse response
	findings, err := a.parseResponse(resp.Content)
	if err != nil {
		return output, fmt.Errorf("parse response: %w", err)
	}

	output.Findings = findings
	return output, nil
}

func (a *CodeAgent) analyzeStructure(basePath string) string {
	var sb strings.Builder

	// Get top-level directory structure
	sb.WriteString("## Directory Structure\n```\n")
	entries, err := os.ReadDir(basePath)
	if err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if entry.IsDir() {
				sb.WriteString(fmt.Sprintf("%s/\n", entry.Name()))
				// List first-level subdirectories
				subPath := filepath.Join(basePath, entry.Name())
				subEntries, err := os.ReadDir(subPath)
				if err == nil {
					for _, sub := range subEntries {
						if strings.HasPrefix(sub.Name(), ".") {
							continue
						}
						if sub.IsDir() {
							sb.WriteString(fmt.Sprintf("  %s/\n", sub.Name()))
						} else {
							sb.WriteString(fmt.Sprintf("  %s\n", sub.Name()))
						}
					}
				}
			} else {
				sb.WriteString(fmt.Sprintf("%s\n", entry.Name()))
			}
		}
	}
	sb.WriteString("```\n\n")

	// Read key entry points
	entryPoints := []string{
		"main.go",
		"cmd/main.go",
		"src/index.ts",
		"src/main.ts",
		"src/App.tsx",
		"pages/_app.tsx",
		"app/layout.tsx",
		"internal/app.go",
	}

	for _, ep := range entryPoints {
		path := filepath.Join(basePath, ep)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(content) > 1500 {
			content = content[:1500]
		}
		sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", ep, string(content)))
	}

	// Read package files
	packageFiles := []string{"package.json", "go.mod", "Cargo.toml", "pyproject.toml"}
	for _, pf := range packageFiles {
		path := filepath.Join(basePath, pf)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(content) > 2000 {
			content = content[:2000]
		}
		sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", pf, string(content)))
	}

	return sb.String()
}

func (a *CodeAgent) buildPrompt(projectName, structure string) string {
	return fmt.Sprintf(`You are a software architect. Analyze the following code structure for project "%s".

Identify KEY ARCHITECTURAL DECISIONS - choices made about:
1. Framework/library choices
2. Code organization patterns
3. Packaging/module structure
4. Technology choices

For each decision, explain:
- WHAT was chosen
- WHY it was likely chosen (infer from context)
- TRADEOFFS of this choice

RESPOND IN JSON:
{
  "decisions": [
    {
      "title": "Decision title",
      "what": "What was chosen",
      "why": "Why this was likely chosen",
      "tradeoffs": "What tradeoffs this implies",
      "confidence": "high|medium|low"
    }
  ]
}

CODE STRUCTURE:
%s

Respond with JSON only:`, projectName, structure)
}

func (a *CodeAgent) parseResponse(response string) ([]Finding, error) {
	// Clean response
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed struct {
		Decisions []struct {
			Title      string `json:"title"`
			What       string `json:"what"`
			Why        string `json:"why"`
			Tradeoffs  string `json:"tradeoffs"`
			Confidence string `json:"confidence"`
		} `json:"decisions"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}

	var findings []Finding
	for _, d := range parsed.Decisions {
		findings = append(findings, Finding{
			Type:        FindingTypeDecision,
			Title:       d.Title,
			Description: d.What,
			Why:         d.Why,
			Tradeoffs:   d.Tradeoffs,
			Confidence:  d.Confidence,
		})
	}

	return findings, nil
}
