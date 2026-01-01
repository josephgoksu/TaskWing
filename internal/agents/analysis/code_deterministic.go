/*
Package analysis provides the deterministic code agent for bootstrap.
*/
package analysis

import (
	"context"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/tools"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// CodeAgent analyzes source code using a single LLM call (deterministic).
// This is used for bootstrap. For interactive exploration, use ReactAgent.
type CodeAgent struct {
	core.BaseAgent
	basePath string
	chain    *core.DeterministicChain[codeAnalysisResponse]
}

// NewCodeAgent creates a new deterministic code analysis agent.
func NewCodeAgent(cfg llm.Config, basePath string) *CodeAgent {
	return &CodeAgent{
		BaseAgent: core.NewBaseAgent("code", "Analyzes source code to identify architectural patterns", cfg),
		basePath:  basePath,
	}
}

// Run executes the agent using a single LLM call with pre-gathered context.
func (a *CodeAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	// Initialize chain (lazy)
	if a.chain == nil {
		chatModel, err := a.CreateChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		chain, err := core.NewDeterministicChain[codeAnalysisResponse](
			ctx,
			a.Name(),
			chatModel,
			config.PromptTemplateCodeAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	basePath := input.BasePath
	if basePath == "" {
		basePath = a.basePath
	}

	// Gather context upfront (no tool calls needed)
	gatherer := tools.NewContextGatherer(basePath)
	dirTree := gatherer.ListDirectoryTree(3)
	sourceCode := gatherer.GatherSourceCode()

	if sourceCode == "" || sourceCode == "No source code files found." {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("no source code found to analyze"),
		}, nil
	}

	// Execute Chain with single LLM call
	chainInput := map[string]any{
		"ProjectName": input.ProjectName,
		"DirTree":     dirTree,
		"SourceCode":  sourceCode,
	}

	parsed, raw, duration, err := a.chain.Invoke(ctx, chainInput)
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chain execution failed: %w", err),
			Duration:  duration,
			RawOutput: raw,
		}, nil
	}

	findings, relationships := a.parseFindings(parsed)
	output := core.BuildOutputWithRelationships(a.Name(), findings, relationships, "JSON output handled by Eino", duration)

	// Add coverage stats from context gathering
	toolsCoverage := gatherer.GetCoverage()
	output.Coverage = convertToolsCoverage(toolsCoverage)

	return output, nil
}

// convertToolsCoverage converts tools.CoverageStats to core.CoverageStats
func convertToolsCoverage(tc tools.CoverageStats) core.CoverageStats {
	var filesRead []core.FileRead
	for _, fr := range tc.FilesRead {
		filesRead = append(filesRead, core.FileRead{
			Path:       fr.Path,
			Characters: fr.Characters,
			Lines:      fr.Lines,
			Truncated:  fr.Truncated,
		})
	}

	var filesSkipped []core.SkippedFile
	for _, fs := range tc.FilesSkipped {
		filesSkipped = append(filesSkipped, core.SkippedFile{
			Path:   fs.Path,
			Reason: fs.Reason,
		})
	}

	total := len(filesRead) + len(filesSkipped)
	var coverage float64
	if total > 0 {
		coverage = float64(len(filesRead)) / float64(total) * 100
	}

	return core.CoverageStats{
		FilesAnalyzed:   len(filesRead),
		FilesSkipped:    len(filesSkipped),
		TotalFiles:      total,
		CoveragePercent: coverage,
		FilesRead:       filesRead,
		FilesSkippedLog: filesSkipped,
	}
}

type codeAnalysisResponse struct {
	Decisions []struct {
		Title      string              `json:"title"`
		Component  string              `json:"component"`
		What       string              `json:"what"`
		Why        string              `json:"why"`
		Tradeoffs  string              `json:"tradeoffs"`
		Confidence any                 `json:"confidence"`
		Evidence   []core.EvidenceJSON `json:"evidence"`
	} `json:"decisions"`
	Patterns []struct {
		Name         string              `json:"name"`
		Context      string              `json:"context"`
		Solution     string              `json:"solution"`
		Consequences string              `json:"consequences"`
		Confidence   any                 `json:"confidence"`
		Evidence     []core.EvidenceJSON `json:"evidence"`
	} `json:"patterns"`
	Relationships []struct {
		From     string `json:"from"`
		To       string `json:"to"`
		Relation string `json:"relation"`
		Reason   string `json:"reason"`
	} `json:"relationships"`
}

func (a *CodeAgent) parseFindings(parsed codeAnalysisResponse) ([]core.Finding, []core.Relationship) {
	var findings []core.Finding

	for _, d := range parsed.Decisions {
		findings = append(findings, core.NewFindingWithEvidence(
			core.FindingTypeDecision,
			d.Title, d.What, d.Why, d.Tradeoffs,
			d.Confidence, d.Evidence, a.Name(),
			map[string]any{"component": d.Component},
		))
	}

	for _, p := range parsed.Patterns {
		findings = append(findings, core.NewFindingWithEvidence(
			core.FindingTypePattern,
			p.Name, p.Context, "", p.Consequences,
			p.Confidence, p.Evidence, a.Name(),
			map[string]any{"context": p.Context, "solution": p.Solution, "consequences": p.Consequences},
		))
	}

	// Parse LLM-extracted relationships
	var relationships []core.Relationship
	for _, r := range parsed.Relationships {
		if r.From != "" && r.To != "" && r.Relation != "" {
			relationships = append(relationships, core.Relationship{
				From:     r.From,
				To:       r.To,
				Relation: r.Relation,
				Reason:   r.Reason,
			})
		}
	}

	return findings, relationships
}

func init() {
	core.RegisterAgentFactory("code", func(cfg llm.Config, basePath string) core.Agent {
		return NewCodeAgent(cfg, basePath)
	})
}
