/*
Package analysis provides agents for analyzing documentation.
*/
package analysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/tools"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// DocAgent analyzes documentation files to extract product features.
type DocAgent struct {
	core.BaseAgent
	chain *core.DeterministicChain[docAnalysisResponse]
}

// NewDocAgent creates a new documentation analysis agent.
func NewDocAgent(cfg llm.Config) *DocAgent {
	return &DocAgent{
		BaseAgent: core.NewBaseAgent("doc", "Analyzes documentation to extract product features", cfg),
	}
}

// Run executes the agent using Eino DeterministicChain.
func (a *DocAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	// Initialize chain if not ready (lazy init to support config updates if needed)
	if a.chain == nil {
		chatModel, err := a.CreateChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		chain, err := core.NewDeterministicChain[docAnalysisResponse](
			ctx,
			a.Name(),
			chatModel,
			config.PromptTemplateDocAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	gatherer := tools.NewContextGatherer(input.BasePath)
	var docContent string
	if input.Mode == core.ModeWatch && len(input.ChangedFiles) > 0 {
		docContent = gatherer.GatherSpecificFiles(filterMarkdown(input.ChangedFiles))
	} else {
		docContent = gatherer.GatherMarkdownDocs()
		// Also include CI/CD configs - they often contain architectural decisions
		ciConfigs := gatherer.GatherCIConfigs()
		if ciConfigs != "" {
			docContent += "\n## CI/CD Configuration\n" + ciConfigs
		}
	}

	if docContent == "" {
		return core.Output{AgentName: a.Name()}, nil
	}

	// Execute Chain
	chainInput := map[string]any{
		"ProjectName": input.ProjectName,
		"DocContent":  docContent,
	}

	parsed, _, duration, err := a.chain.Invoke(ctx, chainInput)
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chain execution failed: %w", err),
			Duration:  duration,
		}, nil
	}

	findings, relationships := a.parseFindings(parsed)
	return core.BuildOutputWithRelationships(a.Name(), findings, relationships, "JSON output handled by Eino", duration), nil
}

type docAnalysisResponse struct {
	Features []struct {
		Name        string              `json:"name"`
		Description string              `json:"description"`
		Confidence  any                 `json:"confidence"`
		Evidence    []core.EvidenceJSON `json:"evidence"`
		SourceFile  string              `json:"source_file"`
	} `json:"features"`
	Constraints []struct {
		Rule       string              `json:"rule"`
		Reason     string              `json:"reason"`
		Severity   string              `json:"severity"`
		Confidence any                 `json:"confidence"`
		Evidence   []core.EvidenceJSON `json:"evidence"`
		SourceFile string              `json:"source_file"`
	} `json:"constraints"`
	Relationships []struct {
		From     string `json:"from"`
		To       string `json:"to"`
		Relation string `json:"relation"`
		Reason   string `json:"reason"`
	} `json:"relationships"`
}

func (a *DocAgent) parseFindings(parsed docAnalysisResponse) ([]core.Finding, []core.Relationship) {
	var findings []core.Finding

	for _, f := range parsed.Features {
		evidence := core.ConvertEvidence(f.Evidence)
		if len(evidence) == 0 && f.SourceFile != "" {
			evidence = []core.Evidence{{FilePath: f.SourceFile}}
		}
		confidenceScore, confidenceLabel := core.ParseConfidence(f.Confidence)
		findings = append(findings, core.Finding{
			Type:               core.FindingTypeFeature,
			Title:              f.Name,
			Description:        f.Description,
			ConfidenceScore:    confidenceScore,
			Confidence:         confidenceLabel,
			Evidence:           evidence,
			VerificationStatus: core.VerificationStatusPending,
			SourceAgent:        a.Name(),
		})
	}

	for _, c := range parsed.Constraints {
		evidence := core.ConvertEvidence(c.Evidence)
		if len(evidence) == 0 && c.SourceFile != "" {
			evidence = []core.Evidence{{FilePath: c.SourceFile}}
		}
		confidenceScore, _ := core.ParseConfidence(c.Confidence)
		if confidenceScore == 0.5 && c.Severity != "" {
			switch c.Severity {
			case "critical":
				confidenceScore = 0.95
			case "high":
				confidenceScore = 0.85
			case "medium":
				confidenceScore = 0.7
			}
		}
		findings = append(findings, core.Finding{
			Type:               core.FindingTypeConstraint,
			Title:              c.Rule,
			Description:        c.Reason,
			ConfidenceScore:    confidenceScore,
			Confidence:         core.ConfidenceLabelFromScore(confidenceScore),
			Evidence:           evidence,
			VerificationStatus: core.VerificationStatusPending,
			SourceAgent:        a.Name(),
			Metadata:           map[string]any{"severity": c.Severity},
		})
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

func filterMarkdown(files []string) []string {
	var filtered []string
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".md") {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func init() {
	core.RegisterAgentFactory("doc", func(cfg llm.Config, basePath string) core.Agent {
		return NewDocAgent(cfg)
	})
}
