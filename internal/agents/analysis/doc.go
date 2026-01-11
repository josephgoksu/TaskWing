/*
Package analysis provides agents for analyzing documentation.
*/
package analysis

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/tools"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// DocAgent analyzes documentation files to extract product features.
// Call Close() when done to release resources.
type DocAgent struct {
	core.BaseAgent
	chain       *core.DeterministicChain[docAnalysisResponse]
	modelCloser io.Closer
}

// NewDocAgent creates a new documentation analysis agent.
func NewDocAgent(cfg llm.Config) *DocAgent {
	return &DocAgent{
		BaseAgent: core.NewBaseAgent("doc", "Analyzes documentation to extract product features", cfg),
	}
}

// Close releases LLM resources. Safe to call multiple times.
func (a *DocAgent) Close() error {
	if a.modelCloser != nil {
		return a.modelCloser.Close()
	}
	return nil
}

// Run executes the agent using Eino DeterministicChain.
func (a *DocAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	// Initialize chain if not ready (lazy init to support config updates if needed)
	if a.chain == nil {
		chatModel, err := a.CreateCloseableChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		a.modelCloser = chatModel
		chain, err := core.NewDeterministicChain[docAnalysisResponse](
			ctx,
			a.Name(),
			chatModel.BaseChatModel,
			config.PromptTemplateDocAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	limit := llm.GetMaxInputTokens(a.LLMConfig().Model)
	budget := tools.NewContextBudget(int(float64(limit) * 0.7))

	gatherer := tools.NewContextGatherer(input.BasePath)
	gatherer.SetBudget(budget)

	// Split work into parallel tracks if not in watch mode
	// Watch mode usually only has small changes, so we keep it simple
	if input.Mode == core.ModeWatch && len(input.ChangedFiles) > 0 {
		docContent := gatherer.GatherSpecificFiles(filterMarkdown(input.ChangedFiles))
		if docContent == "" {
			return core.Output{AgentName: a.Name()}, nil
		}
		chainInput := map[string]any{
			"ProjectName": input.ProjectName,
			"DocContent":  docContent,
		}
		parsed, _, duration, err := a.chain.Invoke(ctx, chainInput)
		if err != nil {
			return core.Output{AgentName: a.Name(), Error: err, Duration: duration}, nil
		}
		findings, relationships := a.parseFindings(parsed)
		output := core.BuildOutputWithRelationships(a.Name(), findings, relationships, "JSON output (watch)", duration)
		output.Coverage = convertToolsCoverage(gatherer.GetCoverage())
		return output, nil
	}

	// Full Bootstrap: Split into Parallel Execution
	// Track 1: General Documentation (Features & High-level Architecture)
	// Track 2: Rules, Workflows, CI, Configs (Prescriptive Constraints)

	type result struct {
		parsed   docAnalysisResponse
		duration time.Duration
		err      error
	}

	results := make(chan result, 2)

	// 1. General Docs
	go func() {
		content := gatherer.GatherMarkdownDocs()
		if content == "" {
			results <- result{}
			return
		}
		start := time.Now()
		// Add instruction hint to focus on features
		content = "FOCUS: PRODUCT FEATURES & ARCHITECTURE\n\n" + content
		p, _, _, err := a.chain.Invoke(ctx, map[string]any{"ProjectName": input.ProjectName, "DocContent": content})
		results <- result{parsed: p, duration: time.Since(start), err: err}
	}()

	// 2. Rules & Workflows
	go func() {
		// KeyFiles (Rules, Makefiles) + CI Configs
		content := gatherer.GatherKeyFiles()
		ci := gatherer.GatherCIConfigs()
		if ci != "" {
			content += "\n## CI/CD Configuration\n" + ci
		}
		if content == "" {
			results <- result{}
			return
		}
		start := time.Now()
		// Add instruction hint to focus on workflows
		content = "FOCUS: WORKFLOWS, RULES & CONSTRAINTS\n\n" + content
		p, _, _, err := a.chain.Invoke(ctx, map[string]any{"ProjectName": input.ProjectName, "DocContent": content})
		results <- result{parsed: p, duration: time.Since(start), err: err}
	}()

	// Wait for both
	var combinedParsed docAnalysisResponse
	var maxDuration time.Duration
	var errs []string

	for i := 0; i < 2; i++ {
		res := <-results
		if res.err != nil {
			errs = append(errs, res.err.Error())
			continue
		}
		if res.duration > maxDuration {
			maxDuration = res.duration
		}

		// Merge results
		combinedParsed.Features = append(combinedParsed.Features, res.parsed.Features...)
		combinedParsed.Constraints = append(combinedParsed.Constraints, res.parsed.Constraints...)
		combinedParsed.Workflows = append(combinedParsed.Workflows, res.parsed.Workflows...)
		combinedParsed.Relationships = append(combinedParsed.Relationships, res.parsed.Relationships...)
	}

	if len(errs) > 0 {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("partial failures: %s", strings.Join(errs, "; ")),
			Duration:  maxDuration,
		}, nil
	}

	findings, relationships := a.parseFindings(combinedParsed)

	// Warn if no findings were produced - helps diagnose silent failures
	if len(findings) == 0 {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("no findings extracted from documentation - check if markdown files exist and are readable"),
			Duration:  maxDuration,
		}, nil
	}

	output := core.BuildOutputWithRelationships(a.Name(), findings, relationships, "Joint analysis (Docs+Rules)", maxDuration)

	// Add coverage stats from context gathering
	toolsCoverage := gatherer.GetCoverage()
	output.Coverage = convertToolsCoverage(toolsCoverage)

	return output, nil
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
	Workflows []struct {
		Name       string              `json:"name"`
		Steps      string              `json:"steps"`
		Trigger    string              `json:"trigger"`
		Confidence any                 `json:"confidence"`
		Evidence   []core.EvidenceJSON `json:"evidence"`
		SourceFile string              `json:"source_file"`
	} `json:"workflows"`
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

	for _, w := range parsed.Workflows {
		evidence := core.ConvertEvidence(w.Evidence)
		if len(evidence) == 0 && w.SourceFile != "" {
			evidence = []core.Evidence{{FilePath: w.SourceFile}}
		}
		confidenceScore, confidenceLabel := core.ParseConfidence(w.Confidence)
		description := fmt.Sprintf("Trigger: %s\nSteps:\n%s", w.Trigger, w.Steps)
		findings = append(findings, core.Finding{
			Type:               core.FindingTypePattern, // Use pattern as base type
			Title:              w.Name,
			Description:        description,
			ConfidenceScore:    confidenceScore,
			Confidence:         confidenceLabel,
			Evidence:           evidence,
			VerificationStatus: core.VerificationStatusPending,
			SourceAgent:        a.Name(),
			Metadata:           map[string]any{"type": "workflow", "trigger": w.Trigger, "steps": w.Steps},
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
	core.RegisterAgent("doc", func(cfg llm.Config, basePath string) core.Agent {
		return NewDocAgent(cfg)
	}, "Documentation", "Extracts knowledge from README and documentation files")
}
