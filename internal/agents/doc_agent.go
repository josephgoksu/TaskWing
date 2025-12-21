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

// DocAgent analyzes documentation files (README, docs/, ARCHITECTURE.md, etc.)
// to extract product-level features and user-facing functionality
type DocAgent struct {
	llmConfig llm.Config
}

// NewDocAgent creates a new documentation analysis agent
func NewDocAgent(cfg llm.Config) *DocAgent {
	return &DocAgent{llmConfig: cfg}
}

func (a *DocAgent) Name() string        { return "doc" }
func (a *DocAgent) Description() string { return "Analyzes documentation to extract product features" }

func (a *DocAgent) Run(ctx context.Context, input Input) (Output, error) {
	var output Output

	// Gather documentation content based on mode
	var docContent string
	if input.Mode == ModeWatch && len(input.ChangedFiles) > 0 {
		// Watch mode: only read changed files
		docContent = a.gatherChangedDocs(input.BasePath, input.ChangedFiles)
	} else {
		// Bootstrap mode: full scan
		docContent = a.gatherDocs(input.BasePath)
	}

	if docContent == "" {
		// No docs to analyze - this is OK in watch mode for non-doc changes
		return output, nil
	}

	// Build prompt
	prompt := a.buildPrompt(input.ProjectName, docContent)

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

// gatherChangedDocs reads only the specified changed files
func (a *DocAgent) gatherChangedDocs(basePath string, changedFiles []string) string {
	var sb strings.Builder

	for _, relPath := range changedFiles {
		// Only process .md files
		if !strings.HasSuffix(strings.ToLower(relPath), ".md") {
			continue
		}

		fullPath := filepath.Join(basePath, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		// Limit size
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

	// Scan root directory for all .md files
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

			// Limit size
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
			// Skip if already read from root
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

func (a *DocAgent) parseResponse(response string) ([]Finding, error) {
	// Clean response
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed struct {
		Features []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			SourceFile  string `json:"source_file"`
			Confidence  string `json:"confidence"`
		} `json:"features"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
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
		})
	}

	return findings, nil
}
