/*
Package analysis provides agents for analyzing git history.
*/
package analysis

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// GitAgent analyzes git history to understand project evolution.
type GitAgent struct {
	core.BaseAgent
	chain *core.DeterministicChain[gitMilestonesResponse]
}

// NewGitAgent creates a new git history analysis agent.
func NewGitAgent(cfg llm.Config) *GitAgent {
	return &GitAgent{
		BaseAgent: core.NewBaseAgent("git", "Analyzes git history for project evolution and key milestones", cfg),
	}
}

// Run executes the agent using Eino DeterministicChain.
func (a *GitAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	// Initialize chain (lazy)
	if a.chain == nil {
		chatModel, err := a.CreateChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		chain, err := core.NewDeterministicChain[gitMilestonesResponse](
			ctx,
			a.Name(),
			chatModel,
			config.PromptTemplateGitAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	gitInfo := gatherGitInfo(input.BasePath)
	if gitInfo == "" {
		return core.Output{Error: fmt.Errorf("no git history available")}, nil
	}

	// Execute Chain
	chainInput := map[string]any{
		"ProjectName": input.ProjectName,
		"GitInfo":     gitInfo,
	}

	parsed, _, duration, err := a.chain.Invoke(ctx, chainInput)
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chain execution failed: %w", err),
			Duration:  duration,
		}, nil
	}

	findings := a.parseFindings(parsed)
	return core.BuildOutput(a.Name(), findings, "JSON output handled by Eino", duration), nil
}

type gitMilestonesResponse struct {
	Milestones []struct {
		Title       string              `json:"title"`
		Scope       string              `json:"scope"`
		Description string              `json:"description"`
		Confidence  any                 `json:"confidence"`
		Evidence    []core.EvidenceJSON `json:"evidence"`
		EvidenceOld string              `json:"evidence_old"`
	} `json:"milestones"`
}

func (a *GitAgent) parseFindings(parsed gitMilestonesResponse) []core.Finding {
	var findings []core.Finding
	for _, m := range parsed.Milestones {
		component := m.Scope
		if component == "" {
			component = "Project Evolution"
		}
		evidence := m.Evidence
		if len(evidence) == 0 && m.EvidenceOld != "" {
			evidence = []core.EvidenceJSON{{FilePath: ".git/logs/HEAD", Snippet: m.EvidenceOld}}
		}
		findings = append(findings, core.NewFindingWithEvidence(
			core.FindingTypeDecision,
			m.Title,
			m.Description,
			m.EvidenceOld,
			"",
			m.Confidence,
			evidence,
			a.Name(),
			map[string]any{"component": component},
		))
	}
	return findings
}

func gatherGitInfo(basePath string) string {
	var sb strings.Builder

	cmd := exec.Command("git", "log", "--oneline", "--no-decorate", "-50")
	cmd.Dir = basePath
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		sb.WriteString("## Recent Commits (last 50)\n```\n")
		sb.WriteString(string(out))
		sb.WriteString("```\n\n")
	}

	cmd = exec.Command("git", "log", "--format=%s", "-200")
	cmd.Dir = basePath
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		lines := strings.Split(string(out), "\n")
		typeCounts := make(map[string]int)
		scopeCounts := make(map[string]int)

		for _, line := range lines {
			switch {
			case strings.HasPrefix(line, "feat"):
				typeCounts["feat"]++
			case strings.HasPrefix(line, "fix"):
				typeCounts["fix"]++
			case strings.HasPrefix(line, "refactor"):
				typeCounts["refactor"]++
			case strings.HasPrefix(line, "chore"):
				typeCounts["chore"]++
			case strings.HasPrefix(line, "docs"):
				typeCounts["docs"]++
			}

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
				if c > 2 {
					sb.WriteString(fmt.Sprintf("- %s: %d commits\n", s, c))
				}
			}
			sb.WriteString("\n")
		}
	}

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

	cmd = exec.Command("git", "log", "--reverse", "--format=%ai", "-1")
	cmd.Dir = basePath
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		sb.WriteString(fmt.Sprintf("## Project Started: %s\n\n", strings.TrimSpace(string(out))))
	}

	return sb.String()
}

func init() {
	core.RegisterAgentFactory("git", func(cfg llm.Config, basePath string) core.Agent {
		return NewGitAgent(cfg)
	})
}
