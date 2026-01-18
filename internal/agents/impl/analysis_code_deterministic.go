/*
Package analysis provides the deterministic code agent for bootstrap.
*/
package impl

import (
	"context"
	"fmt"
	"io"

	"strings"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/tools"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// CodeAgent analyzes source code using a single LLM call (deterministic).
// This is used for bootstrap. For interactive exploration, use ReactAgent.
// Call Close() when done to release resources.
type CodeAgent struct {
	core.BaseAgent
	basePath    string
	chain       *core.DeterministicChain[codeAnalysisResponse]
	modelCloser io.Closer // For releasing LLM resources
}

// NewCodeAgent creates a new deterministic code analysis agent.
func NewCodeAgent(cfg llm.Config, basePath string) *CodeAgent {
	return &CodeAgent{
		BaseAgent: core.NewBaseAgent("code", "Analyzes source code to identify architectural patterns", cfg),
		basePath:  basePath,
	}
}

// Close releases LLM resources. Safe to call multiple times.
func (a *CodeAgent) Close() error {
	if a.modelCloser != nil {
		return a.modelCloser.Close()
	}
	return nil
}

// Run executes the agent using a single LLM call with pre-gathered context.
func (a *CodeAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	// Initialize chain (lazy)
	if a.chain == nil {
		chatModel, err := a.CreateCloseableChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		a.modelCloser = chatModel // Store for cleanup
		chain, err := core.NewDeterministicChain[codeAnalysisResponse](
			ctx,
			a.Name(),
			chatModel.BaseChatModel,
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

	// Gather context - prefer symbol index, fallback to raw files
	var sourceCode string
	var dirTree string
	var gatherer *tools.ContextGatherer // Track for coverage stats
	isIncremental := input.Mode == core.ModeWatch && len(input.ChangedFiles) > 0

	if isIncremental {
		// INCR ANALYSIS: Always use raw files for changed files
		gatherer = tools.NewContextGatherer(basePath)
		limit := llm.GetMaxInputTokens(a.LLMConfig().Model)
		budget := tools.NewContextBudget(int(float64(limit) * 0.7))
		gatherer.SetBudget(budget)
		dirTree = gatherer.ListDirectoryTree(5)
		sourceCode = gatherer.GatherSpecificFiles(input.ChangedFiles)
	} else {
		// FULL ANALYSIS: Try symbol index first
		symbolCtx, err := tools.NewSymbolContext(basePath, a.LLMConfig())
		if err == nil {
			// Use symbol index (compact, scalable)
			defer func() { _ = symbolCtx.Close() }()
			symbolCtx.SetConfig(tools.SymbolContextConfig{
				MaxTokens:    50000, // ~50k tokens for symbols
				PreferPublic: true,
			})
			sourceCode, err = symbolCtx.GatherArchitecturalContext(ctx)
			if err != nil {
				sourceCode = "" // Fall through to raw files
			}
		}

		// Fallback to raw files if index not available or failed
		if sourceCode == "" {
			gatherer = tools.NewContextGatherer(basePath)
			limit := llm.GetMaxInputTokens(a.LLMConfig().Model)
			budget := tools.NewContextBudget(int(float64(limit) * 0.5)) // Stricter for raw files
			gatherer.SetBudget(budget)
			dirTree = gatherer.ListDirectoryTree(5)
			sourceCode = gatherer.GatherSourceCode()
		} else {
			// Still need dir tree for context
			gatherer = tools.NewContextGatherer(basePath)
			dirTree = gatherer.ListDirectoryTree(5)
		}
	}

	if sourceCode == "" || sourceCode == "No source code files found." {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("no source code found to analyze"),
		}, nil
	}

	// Format existing knowledge context
	var existingKnowledgeStr string
	if nodesObj, ok := input.ExistingContext["existing_nodes"]; ok {
		if nodes, ok := nodesObj.([]memory.Node); ok && len(nodes) > 0 {
			var sb strings.Builder
			for _, n := range nodes {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", n.Type, n.ID, n.Summary))
			}
			existingKnowledgeStr = sb.String()
		}
	}

	// Execute Chain with single LLM call
	chainInput := map[string]any{
		"ProjectName":       input.ProjectName,
		"DirTree":           dirTree,
		"SourceCode":        sourceCode,
		"IsIncremental":     isIncremental,
		"ExistingKnowledge": existingKnowledgeStr,
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
		Title        string              `json:"title"`
		Component    string              `json:"component"`
		What         string              `json:"what"`
		Why          string              `json:"why"`
		Tradeoffs    string              `json:"tradeoffs"`
		Confidence   any                 `json:"confidence"`
		Evidence     []core.EvidenceJSON `json:"evidence"`
		DebtScore    any                 `json:"debt_score"`    // Debt classification
		DebtReason   string              `json:"debt_reason"`   // Why this is considered debt
		RefactorHint string              `json:"refactor_hint"` // How to eliminate the debt
	} `json:"decisions"`
	Patterns []struct {
		Name         string              `json:"name"`
		Context      string              `json:"context"`
		Solution     string              `json:"solution"`
		Consequences string              `json:"consequences"`
		Confidence   any                 `json:"confidence"`
		Evidence     []core.EvidenceJSON `json:"evidence"`
		DebtScore    any                 `json:"debt_score"`    // Debt classification
		DebtReason   string              `json:"debt_reason"`   // Why this is considered debt
		RefactorHint string              `json:"refactor_hint"` // How to eliminate the debt
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
		findings = append(findings, core.NewFindingWithDebt(
			core.FindingTypeDecision,
			d.Title, d.What, d.Why, d.Tradeoffs,
			d.Confidence, d.Evidence, a.Name(),
			map[string]any{"component": d.Component},
			core.DebtInfo{DebtScore: d.DebtScore, DebtReason: d.DebtReason, RefactorHint: d.RefactorHint},
		))
	}

	for _, p := range parsed.Patterns {
		findings = append(findings, core.NewFindingWithDebt(
			core.FindingTypePattern,
			p.Name, p.Context, "", p.Consequences,
			p.Confidence, p.Evidence, a.Name(),
			map[string]any{"context": p.Context, "solution": p.Solution, "consequences": p.Consequences},
			core.DebtInfo{DebtScore: p.DebtScore, DebtReason: p.DebtReason, RefactorHint: p.RefactorHint},
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
	core.RegisterAgent("code", func(cfg llm.Config, basePath string) core.Agent {
		return NewCodeAgent(cfg, basePath)
	}, "Code Analysis", "Analyzes source code structure, patterns, and architecture")
}
