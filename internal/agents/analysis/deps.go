/*
Package analysis provides agents for analyzing dependencies.
*/
package analysis

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// DepsAgent analyzes dependencies to understand technology choices.
// Call Close() when done to release resources.
type DepsAgent struct {
	core.BaseAgent
	chain       *core.DeterministicChain[depsTechDecisionsResponse]
	modelCloser io.Closer
}

// NewDepsAgent creates a new dependency analysis agent.
func NewDepsAgent(cfg llm.Config) *DepsAgent {
	return &DepsAgent{
		BaseAgent: core.NewBaseAgent("deps", "Analyzes dependencies to understand technology choices", cfg),
	}
}

// Close releases LLM resources. Safe to call multiple times.
func (a *DepsAgent) Close() error {
	if a.modelCloser != nil {
		return a.modelCloser.Close()
	}
	return nil
}

// Run executes the agent using Eino DeterministicChain.
func (a *DepsAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	// Initialize chain (lazy)
	if a.chain == nil {
		chatModel, err := a.CreateCloseableChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		a.modelCloser = chatModel
		chain, err := core.NewDeterministicChain[depsTechDecisionsResponse](
			ctx,
			a.Name(),
			chatModel.BaseChatModel,
			config.PromptTemplateDepsAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	depsInfo := gatherDeps(input.BasePath)
	if depsInfo == "" {
		return core.Output{Error: fmt.Errorf("no dependency files found")}, nil
	}

	// Execute Chain
	chainInput := map[string]any{
		"ProjectName": input.ProjectName,
		"DepsInfo":    depsInfo,
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

type depsTechDecisionsResponse struct {
	TechDecisions []struct {
		Title      string              `json:"title"`
		Category   string              `json:"category"`
		What       string              `json:"what"`
		Why        string              `json:"why"`
		Confidence any                 `json:"confidence"`
		Evidence   []core.EvidenceJSON `json:"evidence"`
	} `json:"tech_decisions"`
}

func (a *DepsAgent) parseFindings(parsed depsTechDecisionsResponse) []core.Finding {
	var findings []core.Finding
	for _, d := range parsed.TechDecisions {
		component := d.Category
		if component == "" {
			component = "Technology Stack"
		}
		findings = append(findings, core.NewFindingWithEvidence(
			core.FindingTypeDecision,
			d.Title,
			d.What,
			d.Why,
			"",
			d.Confidence,
			d.Evidence,
			a.Name(),
			map[string]any{"component": component},
		))
	}
	return findings
}

func gatherDeps(basePath string) string {
	var sb strings.Builder

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
				if len(content) > 3000 {
					content = content[:3000]
				}
				sb.WriteString(fmt.Sprintf("## %s\n```json\n%s\n```\n\n", file, string(content)))
			}
		}
	}

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

func init() {
	core.RegisterAgent("deps", func(cfg llm.Config, basePath string) core.Agent {
		return NewDepsAgent(cfg)
	}, "Dependencies", "Analyzes project dependencies and their purposes")
}
