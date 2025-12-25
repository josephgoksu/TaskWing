/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package agents

import (
	"context"
	"fmt"
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
	gatherer := NewContextGatherer(input.BasePath)
	var docContent string
	if input.Mode == ModeWatch && len(input.ChangedFiles) > 0 {
		docContent = gatherer.GatherSpecificFiles(filterMarkdown(input.ChangedFiles))
	} else {
		docContent = gatherer.GatherMarkdownDocs()
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

// docAnalysisResponse is the expected JSON structure from LLM.
// It captures both product features and architectural constraints.
type docAnalysisResponse struct {
	Features []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		SourceFile  string `json:"source_file"`
		Confidence  string `json:"confidence"`
	} `json:"features"`
	Constraints []struct {
		Rule       string `json:"rule"`
		Reason     string `json:"reason"`
		SourceFile string `json:"source_file"`
		Severity   string `json:"severity"` // critical, high, medium
	} `json:"constraints"`
}

func (a *DocAgent) parseResponse(response string) ([]Finding, error) {
	parsed, err := ParseJSONResponse[docAnalysisResponse](response)
	if err != nil {
		return nil, err
	}

	var findings []Finding

	// Extract features
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

	// Extract constraints (architectural rules)
	for _, c := range parsed.Constraints {
		findings = append(findings, Finding{
			Type:        FindingTypeConstraint,
			Title:       c.Rule,
			Description: c.Reason,
			Confidence:  c.Severity, // Map severity to confidence field for consistency
			SourceFiles: []string{c.SourceFile},
			SourceAgent: a.Name(),
			Metadata: map[string]any{
				"severity": c.Severity,
			},
		})
	}

	return findings, nil
}

func filterMarkdown(files []string) []string {
	var filtered []string
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".md") {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func (a *DocAgent) buildPrompt(projectName, docContent string) string {
	return fmt.Sprintf(`You are a technical analyst. Analyze the following documentation for project "%s".

Extract TWO types of information:

## 1. PRODUCT FEATURES
Things the product does for users (not technical implementation).
- Name: concise feature name
- Description: what it does for users
- Source: where in the docs this is mentioned

## 2. ARCHITECTURAL CONSTRAINTS
Mandatory rules developers MUST follow. Look for:
- Words like: CRITICAL, MUST, REQUIRED, mandatory, always, never
- Database access rules (replicas, connection pools)
- Caching requirements
- Security requirements
- Performance mandates

For each constraint:
- Rule: the exact requirement
- Reason: why it's important
- Severity: critical, high, or medium

RESPOND IN JSON:
{
  "features": [
    {
      "name": "Feature Name",
      "description": "What it does for users",
      "source_file": "README.md",
      "confidence": "high|medium|low"
    }
  ],
  "constraints": [
    {
      "rule": "Use ReadReplica for high-volume reads",
      "reason": "Prevents primary DB overload",
      "source_file": "docs/architecture.md",
      "severity": "critical|high|medium"
    }
  ]
}

DOCUMENTATION:
%s

Respond with JSON only:`, projectName, docContent)
}
