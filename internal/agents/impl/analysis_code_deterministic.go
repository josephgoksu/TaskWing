/*
Package analysis provides the deterministic code agent for bootstrap.
*/
package impl

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

// Run executes the agent using chunked processing with deduplication.
// For large codebases, splits files into chunks, analyzes each, then merges results.
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

	isIncremental := input.Mode == core.ModeWatch && len(input.ChangedFiles) > 0

	// Format existing knowledge context (used by all analysis paths)
	existingKnowledgeStr := a.formatExistingKnowledge(input.ExistingContext)

	// Get directory tree (used by all analysis paths)
	gatherer := tools.NewContextGatherer(basePath)
	dirTree := gatherer.ListDirectoryTree(5)

	// Truncate dirTree if it's too large (max ~5k tokens = ~20k chars)
	const maxDirTreeChars = 20000
	if len(dirTree) > maxDirTreeChars {
		dirTree = dirTree[:maxDirTreeChars] + "\n... (truncated)"
	}

	// Truncate existingKnowledge if too large (max ~2k tokens = ~8k chars)
	const maxExistingKnowledgeChars = 8000
	if len(existingKnowledgeStr) > maxExistingKnowledgeChars {
		existingKnowledgeStr = existingKnowledgeStr[:maxExistingKnowledgeChars] + "\n... (truncated)"
	}

	// Route to appropriate analysis strategy
	if isIncremental {
		return a.runIncrementalAnalysis(ctx, input, basePath, dirTree, existingKnowledgeStr)
	}

	// FULL ANALYSIS: Try symbol index first (compact, scalable)
	symbolCtx, err := tools.NewSymbolContext(basePath, a.LLMConfig())
	if err == nil {
		defer func() { _ = symbolCtx.Close() }()
		symbolCtx.SetConfig(tools.SymbolContextConfig{
			MaxTokens:    50000, // ~50k tokens for symbols
			PreferPublic: true,
		})
		sourceCode, err := symbolCtx.GatherArchitecturalContext(ctx)
		if err == nil && sourceCode != "" {
			// C3 Fix: Get coverage from symbol index stats, not empty gatherer
			coverage := tools.CoverageStats{} // Default empty
			if stats, statsErr := symbolCtx.GetStats(ctx); statsErr == nil && stats != nil {
				coverage = tools.CoverageStats{
					FilesRead: make([]tools.FileRecord, 0),
					FilesSkipped: []tools.SkipRecord{{
						Path:   "(symbol index)",
						Reason: fmt.Sprintf("analyzed %d symbols from %d indexed files", stats.SymbolsFound, stats.FilesIndexed),
					}},
				}
			}
			// Symbol index available - single LLM call
			return a.runSingleAnalysis(ctx, input, dirTree, sourceCode, existingKnowledgeStr, coverage)
		}
	}

	// Fallback: Chunked processing for raw files (handles large codebases)
	return a.runChunkedAnalysis(ctx, input, basePath, dirTree, existingKnowledgeStr)
}

// runIncrementalAnalysis handles watch mode with specific changed files.
func (a *CodeAgent) runIncrementalAnalysis(ctx context.Context, input core.Input, basePath, dirTree, existingKnowledge string) (core.Output, error) {
	gatherer := tools.NewContextGatherer(basePath)
	limit := llm.GetMaxInputTokens(a.LLMConfig().Model)
	budget := tools.NewSafeContextBudget(int(float64(limit) * 0.7))
	gatherer.SetBudget(budget)

	sourceCode := gatherer.GatherSpecificFiles(input.ChangedFiles)
	if sourceCode == "" {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("no source code found in changed files"),
		}, nil
	}

	return a.runSingleAnalysis(ctx, input, dirTree, sourceCode, existingKnowledge, gatherer.GetCoverage())
}

// runSingleAnalysis executes a single LLM call (used for symbol index and incremental).
func (a *CodeAgent) runSingleAnalysis(ctx context.Context, input core.Input, dirTree, sourceCode, existingKnowledge string, coverage tools.CoverageStats) (core.Output, error) {
	chainInput := map[string]any{
		"ProjectName":       input.ProjectName,
		"DirTree":           dirTree,
		"SourceCode":        sourceCode,
		"IsIncremental":     input.Mode == core.ModeWatch,
		"ExistingKnowledge": existingKnowledge,
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
	output.Coverage = convertToolsCoverage(coverage)

	return output, nil
}

// runChunkedAnalysis processes large codebases in chunks, then deduplicates.
func (a *CodeAgent) runChunkedAnalysis(ctx context.Context, input core.Input, basePath, dirTree, existingKnowledge string) (core.Output, error) {
	chunker := tools.NewCodeChunker(basePath)
	chunker.SetConfig(tools.ChunkConfig{
		MaxTokensPerChunk:  30000, // 30k tokens per chunk - safe margin
		MaxFilesPerChunk:   40,    // Limit files per chunk
		IncludeLineNumbers: true,
	})

	chunks, err := chunker.ChunkSourceCode()
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chunking failed: %w", err),
		}, nil
	}

	if len(chunks) == 0 {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("no source code found to analyze"),
		}, nil
	}

	// Calculate max chunks based on model's token limit
	// Reserve overhead for system prompt (~5k), dirTree (~3k), and safety margin (~2k)
	const overheadTokens = 10000
	const tokensPerChunk = 30000
	modelLimit := llm.GetMaxInputTokens(a.LLMConfig().Model)

	// Cap at MaxSafeContextBudget to stay within practical API limits
	effectiveLimit := min(modelLimit, tools.MaxSafeContextBudget)

	maxChunks := max(1, (effectiveLimit-overheadTokens)/tokensPerChunk)

	// Limit chunks to prevent token overflow
	if len(chunks) > maxChunks {
		chunks = chunks[:maxChunks]
	}

	// Process each chunk and collect findings
	var allFindings []core.Finding
	var allRelationships []core.Relationship
	var totalDuration time.Duration
	var chunkErrors []string
	successfulChunks := 0

	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return core.Output{
				AgentName: a.Name(),
				Error:     ctx.Err(),
			}, nil
		default:
		}

		// Add chunk context to help LLM understand partial view
		chunkContext := fmt.Sprintf("(Chunk %d/%d: %s)", i+1, len(chunks), chunk.Description)

		chainInput := map[string]any{
			"ProjectName":       input.ProjectName + " " + chunkContext,
			"DirTree":           dirTree,
			"SourceCode":        chunk.Content,
			"IsIncremental":     false,
			"ExistingKnowledge": existingKnowledge,
		}

		parsed, _, duration, err := a.chain.Invoke(ctx, chainInput)
		totalDuration += duration

		if err != nil {
			// Track failed chunks for reporting
			chunkErrors = append(chunkErrors, fmt.Sprintf("chunk %d (%s): %v", i+1, chunk.Description, err))
			continue
		}

		successfulChunks++
		findings, relationships := a.parseFindings(parsed)
		allFindings = append(allFindings, findings...)
		allRelationships = append(allRelationships, relationships...)
	}

	// C1 Fix: If ALL chunks failed, return an error instead of silent empty result
	if successfulChunks == 0 {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("all %d chunks failed: %s", len(chunks), strings.Join(chunkErrors, "; ")),
			Duration:  totalDuration,
		}, nil
	}

	// Deduplicate findings from all chunks
	deduplicator := tools.NewFindingDeduplicator()
	dedupedFindings := deduplicator.DeduplicateFindings(allFindings)
	dedupedRelationships := deduplicator.DeduplicateRelationships(allRelationships)

	// C2 Fix: Include chunk success/failure stats in output
	rawOutput := fmt.Sprintf("Chunked analysis: %d/%d chunks succeeded, %d findings deduplicated to %d",
		successfulChunks, len(chunks), len(allFindings), len(dedupedFindings))
	if len(chunkErrors) > 0 {
		rawOutput += fmt.Sprintf(" (failures: %s)", strings.Join(chunkErrors, "; "))
	}

	output := core.BuildOutputWithRelationships(
		a.Name(),
		dedupedFindings,
		dedupedRelationships,
		rawOutput,
		totalDuration,
	)
	output.Coverage = convertToolsCoverage(chunker.GetCoverage())

	return output, nil
}

// formatExistingKnowledge formats existing knowledge nodes for the prompt.
func (a *CodeAgent) formatExistingKnowledge(existingContext map[string]any) string {
	if existingContext == nil {
		return ""
	}

	nodesObj, ok := existingContext["existing_nodes"]
	if !ok {
		return ""
	}

	nodes, ok := nodesObj.([]memory.Node)
	if !ok || len(nodes) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, n := range nodes {
		fmt.Fprintf(&sb, "- [%s] %s: %s\n", n.Type, n.ID, n.Summary)
	}
	return sb.String()
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
