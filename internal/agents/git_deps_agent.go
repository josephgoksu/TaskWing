/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// GitAgent analyzes git history to understand project evolution and key milestones
type GitAgent struct {
	llmConfig llm.Config
}

// NewGitAgent creates a new git history analysis agent
func NewGitAgent(cfg llm.Config) *GitAgent {
	return &GitAgent{llmConfig: cfg}
}

func (a *GitAgent) Name() string { return "git" }
func (a *GitAgent) Description() string {
	return "Analyzes git history for project evolution and key milestones"
}

func (a *GitAgent) Run(ctx context.Context, input Input) (Output, error) {
	var output Output

	// Gather git history
	gitInfo := a.gatherGitInfo(input.BasePath)
	if gitInfo == "" {
		output.Error = fmt.Errorf("no git history available")
		return output, nil
	}

	// Build prompt
	prompt := a.buildPrompt(input.ProjectName, gitInfo)

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

func (a *GitAgent) gatherGitInfo(basePath string) string {
	var sb strings.Builder

	// Get recent commits (last 50)
	cmd := exec.Command("git", "log", "--oneline", "--no-decorate", "-50")
	cmd.Dir = basePath
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		sb.WriteString("## Recent Commits (last 50)\n```\n")
		sb.WriteString(string(out))
		sb.WriteString("```\n\n")
	}

	// Get commit stats by conventional commit type
	cmd = exec.Command("git", "log", "--format=%s", "-200")
	cmd.Dir = basePath
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		lines := strings.Split(string(out), "\n")
		typeCounts := make(map[string]int)
		scopeCounts := make(map[string]int)

		for _, line := range lines {
			if strings.HasPrefix(line, "feat") {
				typeCounts["feat"]++
			} else if strings.HasPrefix(line, "fix") {
				typeCounts["fix"]++
			} else if strings.HasPrefix(line, "refactor") {
				typeCounts["refactor"]++
			} else if strings.HasPrefix(line, "chore") {
				typeCounts["chore"]++
			} else if strings.HasPrefix(line, "docs") {
				typeCounts["docs"]++
			}

			// Extract scope if present (e.g., feat(web): ...)
			if idx := strings.Index(line, "("); idx != -1 {
				if end := strings.Index(line[idx:], ")"); end != -1 {
					scope := line[idx+1 : idx+end]
					scopeCounts[scope]++
				}
			}
		}

		if len(typeCounts) > 0 {
			sb.WriteString("## Commit Type Distribution\n")
			for t, c := range typeCounts {
				sb.WriteString(fmt.Sprintf("- %s: %d\n", t, c))
			}
			sb.WriteString("\n")
		}

		if len(scopeCounts) > 0 {
			sb.WriteString("## Most Active Scopes\n")
			for s, c := range scopeCounts {
				if c > 2 { // Only show scopes with 3+ commits
					sb.WriteString(fmt.Sprintf("- %s: %d commits\n", s, c))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Get contributors
	cmd = exec.Command("git", "shortlog", "-sn", "--all")
	cmd.Dir = basePath
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		lines := strings.Split(string(out), "\n")
		if len(lines) > 10 {
			lines = lines[:10]
		}
		sb.WriteString("## Top Contributors\n```\n")
		sb.WriteString(strings.Join(lines, "\n"))
		sb.WriteString("\n```\n\n")
	}

	// Get first commit date (project start)
	cmd = exec.Command("git", "log", "--reverse", "--format=%ai", "-1")
	cmd.Dir = basePath
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		sb.WriteString(fmt.Sprintf("## Project Started: %s\n\n", strings.TrimSpace(string(out))))
	}

	return sb.String()
}

func (a *GitAgent) buildPrompt(projectName, gitInfo string) string {
	return fmt.Sprintf(`You are a software historian. Analyze the git history for project "%s".

Identify KEY MILESTONES and EVOLUTION PATTERNS:
1. Major feature additions (from feat commits)
2. Significant refactors or architecture changes
3. Technology migrations or additions
4. Active development areas

For each finding, explain WHAT happened and WHY it matters.
IMPORTANT: Identify which component/feature each milestone relates to from commit scopes (e.g. "feat(auth):" → scope is "auth").

RESPOND IN JSON:
{
  "milestones": [
    {
      "title": "Milestone or decision title",
      "scope": "Component or feature this relates to (from commit scope, e.g. 'auth', 'api', 'ui')",
      "description": "What happened and why it matters",
      "evidence": "Commits or patterns that show this",
      "confidence": "high|medium|low"
    }
  ]
}

GIT HISTORY:
%s

Respond with JSON only:`, projectName, gitInfo)
}

func (a *GitAgent) parseResponse(response string) ([]Finding, error) {
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed struct {
		Milestones []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Evidence    string `json:"evidence"`
			Confidence  string `json:"confidence"`
			Scope       string `json:"scope"` // Component/feature this applies to
		} `json:"milestones"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}

	var findings []Finding
	for _, m := range parsed.Milestones {
		// Infer component from scope if provided by LLM
		component := strings.TrimSpace(m.Scope)
		if component == "" {
			component = "Project Evolution" // Fallback
		}

		findings = append(findings, Finding{
			Type:        FindingTypeDecision,
			Title:       m.Title,
			Description: m.Description,
			Why:         m.Evidence,
			Confidence:  m.Confidence,
			SourceAgent: a.Name(),
			Metadata: map[string]any{
				"component": component,
			},
		})
	}

	return findings, nil
}

// DepsAgent analyzes dependencies to understand technology choices
type DepsAgent struct {
	llmConfig llm.Config
}

// NewDepsAgent creates a new dependency analysis agent
func NewDepsAgent(cfg llm.Config) *DepsAgent {
	return &DepsAgent{llmConfig: cfg}
}

func (a *DepsAgent) Name() string { return "deps" }
func (a *DepsAgent) Description() string {
	return "Analyzes dependencies to understand technology choices"
}

func (a *DepsAgent) Run(ctx context.Context, input Input) (Output, error) {
	var output Output

	// Gather dependency info
	depsInfo := a.gatherDeps(input.BasePath)
	if depsInfo == "" {
		output.Error = fmt.Errorf("no dependency files found")
		return output, nil
	}

	// Build prompt
	prompt := a.buildPrompt(input.ProjectName, depsInfo)

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

func (a *DepsAgent) gatherDeps(basePath string) string {
	var sb strings.Builder

	// Find and read package.json files
	cmd := exec.Command("find", ".", "-name", "package.json", "-not", "-path", "*/node_modules/*", "-type", "f")
	cmd.Dir = basePath
	out, _ := cmd.Output()

	if len(out) > 0 {
		files := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, file := range files {
			if file == "" {
				continue
			}
			catCmd := exec.Command("cat", file)
			catCmd.Dir = basePath
			content, err := catCmd.Output()
			if err == nil {
				// Truncate large files
				if len(content) > 3000 {
					content = content[:3000]
				}
				sb.WriteString(fmt.Sprintf("## %s\n```json\n%s\n```\n\n", file, string(content)))
			}
		}
	}

	// Find and read go.mod files
	cmd = exec.Command("find", ".", "-name", "go.mod", "-type", "f")
	cmd.Dir = basePath
	out, _ = cmd.Output()

	if len(out) > 0 {
		files := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, file := range files {
			if file == "" {
				continue
			}
			catCmd := exec.Command("cat", file)
			catCmd.Dir = basePath
			content, err := catCmd.Output()
			if err == nil {
				if len(content) > 2000 {
					content = content[:2000]
				}
				sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", file, string(content)))
			}
		}
	}

	return sb.String()
}

func (a *DepsAgent) buildPrompt(projectName, depsInfo string) string {
	return fmt.Sprintf(`You are a technology analyst. Analyze the dependencies for project "%s".

Identify KEY TECHNOLOGY DECISIONS from the dependencies:
1. Framework choices (React, Vue, Express, Chi, etc.)
2. Database drivers (what databases are used)
3. Authentication libraries
4. Testing frameworks
5. Notable patterns (e.g., uses OpenTelemetry for observability)

For each finding, explain WHAT was chosen and WHY it matters.
IMPORTANT: Categorize each decision into a layer (e.g., "CLI Layer", "Storage Layer", "UI Layer", "API Layer", "Testing").

RESPOND IN JSON:
{
  "tech_decisions": [
    {
      "title": "Technology decision title",
      "category": "Which layer this belongs to (CLI Layer, Storage Layer, UI Layer, etc.)",
      "what": "What technology/framework/library",
      "why": "Why this choice matters or was likely made",
      "confidence": "high|medium|low"
    }
  ]
}

DEPENDENCIES:
%s

Respond with JSON only:`, projectName, depsInfo)
}

func (a *DepsAgent) parseResponse(response string) ([]Finding, error) {
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed struct {
		TechDecisions []struct {
			Title      string `json:"title"`
			Category   string `json:"category"`
			What       string `json:"what"`
			Why        string `json:"why"`
			Confidence string `json:"confidence"`
		} `json:"tech_decisions"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}

	var findings []Finding
	for _, d := range parsed.TechDecisions {
		// Infer component from category if provided by LLM
		component := strings.TrimSpace(d.Category)
		if component == "" {
			component = "Technology Stack" // Fallback
		}

		findings = append(findings, Finding{
			Type:        FindingTypeDecision,
			Title:       d.Title,
			Description: d.What,
			Why:         d.Why,
			Confidence:  d.Confidence,
			SourceAgent: a.Name(),
			Metadata: map[string]any{
				"component": component,
			},
		})
	}

	return findings, nil
}

// Silence unused import warning
var _ = bytes.Buffer{}
