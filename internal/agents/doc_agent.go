/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// DocAgent analyzes documentation files (README, docs/, ARCHITECTURE.md, etc.)
// to extract product-level features and user-facing functionality.
type DocAgent struct {
	BaseAgent // Embed BaseAgent for shared LLM functionality
}

// NewDocAgent creates a new documentation analysis agent.
func NewDocAgent(cfg llm.Config) *DocAgent {
	return &DocAgent{
		BaseAgent: NewBaseAgent("doc", "Analyzes documentation to extract product features", cfg),
	}
}

func (a *DocAgent) Run(ctx context.Context, input Input) (Output, error) {
	start := time.Now()

	// Gather documentation content based on mode
	var docContent string
	if input.Mode == ModeWatch && len(input.ChangedFiles) > 0 {
		docContent = a.gatherChangedDocs(input.BasePath, input.ChangedFiles)
	} else {
		docContent = a.gatherDocs(input.BasePath)
	}

	if docContent == "" {
		// No docs to analyze - this is OK in watch mode for non-doc changes
		return Output{}, nil
	}

	// Build prompt and call LLM using BaseAgent.Generate()
	prompt := a.buildPrompt(input.ProjectName, docContent)
	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	rawOutput, err := a.Generate(ctx, messages)
	if err != nil {
		return Output{}, err
	}

	// Parse response using shared JSON parser
	findings, err := a.parseResponse(rawOutput)
	if err != nil {
		return Output{}, fmt.Errorf("parse response: %w", err)
	}

	return BuildOutput(a.Name(), findings, rawOutput, time.Since(start)), nil
}

// docFeaturesResponse is the expected JSON structure from LLM.
type docFeaturesResponse struct {
	Features []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		SourceFile  string `json:"source_file"`
		Confidence  string `json:"confidence"`
	} `json:"features"`
}

func (a *DocAgent) parseResponse(response string) ([]Finding, error) {
	parsed, err := ParseJSONResponse[docFeaturesResponse](response)
	if err != nil {
		return nil, err
	}

	var findings []Finding
	for _, f := range parsed.Features {
		findings = append(findings, Finding{
			Type:        FindingTypeFeature,
			Title:       f.Name,
			Description: f.Description,
			Confidence:  f.Confidence,
			SourceFiles: []string{f.SourceFile},
			SourceAgent: a.Name(),
		})
	}

	return findings, nil
}

// gatherChangedDocs reads only the specified changed files.
func (a *DocAgent) gatherChangedDocs(basePath string, changedFiles []string) string {
	var sb strings.Builder

	for _, relPath := range changedFiles {
		if !strings.HasSuffix(strings.ToLower(relPath), ".md") {
			continue
		}

		fullPath := filepath.Join(basePath, relPath)
		content, err := os.ReadFile(fullPath)
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

func (a *DocAgent) gatherDocs(basePath string) string {
	var sb strings.Builder
	seen := make(map[string]bool)

	entries, err := os.ReadDir(basePath)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".md") {
				continue
			}

			path := filepath.Join(basePath, name)
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			if len(content) > 4000 {
				content = content[:4000]
			}

			sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", name, string(content)))
			seen[strings.ToLower(name)] = true
		}
	}

	// Also check docs/ directory
	docsDir := filepath.Join(basePath, "docs")
	entries, err = os.ReadDir(docsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".md") {
				continue
			}
			if seen[strings.ToLower(name)] {
				continue
			}

			path := filepath.Join(docsDir, name)
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if len(content) > 3000 {
				content = content[:3000]
			}
			sb.WriteString(fmt.Sprintf("## docs/%s\n```\n%s\n```\n\n", name, string(content)))
		}
	}

	return sb.String()
}

func (a *DocAgent) buildPrompt(projectName, docContent string) string {
	return fmt.Sprintf(`You are a product analyst. Analyze the following documentation for project "%s".

Extract PRODUCT FEATURES - things the product does for users, not technical implementation details.

For each feature, identify:
1. Name - concise feature name
2. Description - what it does for users
3. Evidence - where in the docs this is mentioned

RESPOND IN JSON:
{
  "features": [
    {
      "name": "Feature Name",
      "description": "What it does for users",
      "source_file": "README.md",
      "confidence": "high|medium|low"
    }
  ]
}

DOCUMENTATION:
%s

Respond with JSON only:`, projectName, docContent)
}
