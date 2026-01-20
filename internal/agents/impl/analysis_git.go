/*
Package analysis provides agents for analyzing git history.
*/
package impl

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/project"
)

const (
	// Git analysis configuration
	gitChunkSize      = 50  // Commits per chunk
	gitMaxCommits     = 300 // Total commits to analyze
	gitMaxChunks      = 6   // Maximum chunks to process
	gitRecentMaxItems = 8   // Max findings for newest chunk
	gitDecayFactor    = 0.6 // Each older chunk gets this fraction of previous max
	gitMinItems       = 2   // Minimum findings per chunk
)

// GitAgent analyzes git history to understand project evolution.
// Call Close() when done to release resources.
type GitAgent struct {
	core.BaseAgent
	chain       *core.DeterministicChain[gitMilestonesResponse]
	modelCloser io.Closer
}

// NewGitAgent creates a new git history analysis agent.
func NewGitAgent(cfg llm.Config) *GitAgent {
	return &GitAgent{
		BaseAgent: core.NewBaseAgent("git", "Analyzes git history for project evolution and key milestones", cfg),
	}
}

// Close releases LLM resources. Safe to call multiple times.
func (a *GitAgent) Close() error {
	if a.modelCloser != nil {
		return a.modelCloser.Close()
	}
	return nil
}

// Run executes the agent using chunked analysis with recency weighting.
func (a *GitAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	start := time.Now()

	// Initialize chain (lazy)
	if a.chain == nil {
		chatModel, err := a.CreateCloseableChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		a.modelCloser = chatModel
		chain, err := core.NewDeterministicChain[gitMilestonesResponse](
			ctx,
			a.Name(),
			chatModel.BaseChatModel,
			config.PromptTemplateGitAgentChunked,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	// Gather commits and split into chunks
	chunks, projectMeta := gatherGitChunks(input.BasePath)
	if len(chunks) == 0 {
		return core.Output{AgentName: a.Name(), Error: fmt.Errorf("no git history available")}, nil
	}

	// Process chunks with recency weighting (newest first)
	var allFindings []core.Finding
	seenTitles := make(map[string]bool)
	chunksProcessed := 0
	chunksFailed := 0
	totalCommits := 0

	chunksToProcess := min(len(chunks), gitMaxChunks)
	for i := 0; i < chunksToProcess; i++ {
		chunk := chunks[i]
		commitCount := strings.Count(chunk, "\n") + 1
		totalCommits += commitCount

		// Calculate max findings for this chunk (decay with age)
		maxFindings := calculateMaxFindings(i)

		chainInput := map[string]any{
			"ProjectName": input.ProjectName,
			"ChunkNumber": i + 1,
			"TotalChunks": chunksToProcess,
			"MaxFindings": maxFindings,
			"IsRecent":    i == 0,
			"ProjectMeta": projectMeta,
			"CommitChunk": chunk,
		}

		parsed, _, _, err := a.chain.Invoke(ctx, chainInput)
		if err != nil {
			// Log the error for debugging (Task 1: Error logging for chunk failures)
			log.Printf("[git] chunk %d/%d parse failed: %v", i+1, chunksToProcess, err)
			chunksFailed++
			continue // Skip failed chunks, don't abort entirely
		}
		chunksProcessed++

		// Parse and deduplicate findings
		chunkFindings := a.parseFindings(parsed)
		for _, f := range chunkFindings {
			titleKey := strings.ToLower(f.Title)
			if !seenTitles[titleKey] {
				seenTitles[titleKey] = true
				allFindings = append(allFindings, f)
			}
		}
	}

	duration := time.Since(start)

	// If all chunks failed, return error
	if chunksProcessed == 0 && chunksFailed > 0 {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("all %d chunks failed to process", chunksFailed),
			Duration:  duration,
		}, nil
	}

	// Task 4: Log processing summary for debugging
	log.Printf("[git] Processed %d/%d chunks (%d failed), found %d milestones from %d commits",
		chunksProcessed, chunksToProcess, chunksFailed, len(allFindings), totalCommits)

	// Task 2: Defensive check for empty results (similar to doc agent)
	// Warn if we processed chunks but found nothing
	if len(allFindings) == 0 && chunksProcessed > 0 {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("analyzed %d commits across %d chunks but found no significant milestones (commit messages may lack conventional format or architectural decisions)", totalCommits, chunksProcessed),
			Duration:  duration,
		}, nil
	}

	output := core.BuildOutput(a.Name(), allFindings, "Chunked analysis with recency weighting", duration)

	// Add coverage stats for consistency with other agents
	// Note: Git agent analyzes commits, not files. We report this as a single "file" (git history)
	// with metadata about chunks processed for transparency.
	output.Coverage = core.CoverageStats{
		FilesAnalyzed:   1, // Git history treated as single source
		TotalFiles:      1,
		CoveragePercent: float64(chunksProcessed) / float64(chunksToProcess) * 100,
		CharactersRead:  totalCommits * 80, // Approximate characters analyzed
		FilesRead: []core.FileRead{{
			Path:       fmt.Sprintf(".git/logs/HEAD (%d/%d chunks, %d commits)", chunksProcessed, chunksToProcess, totalCommits),
			Characters: totalCommits * 80,
			Lines:      totalCommits,
			Truncated:  len(chunks) > gitMaxChunks,
		}},
	}

	return output, nil
}

// calculateMaxFindings returns max findings for a chunk based on its age (0 = newest)
func calculateMaxFindings(chunkIndex int) int {
	maxItems := float64(gitRecentMaxItems)
	for i := 0; i < chunkIndex; i++ {
		maxItems *= gitDecayFactor
	}
	result := int(maxItems)
	if result < gitMinItems {
		return gitMinItems
	}
	return result
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
			"", // Git milestones don't have separate "why" - context is in description
			"", // No tradeoffs for git history findings
			m.Confidence,
			evidence,
			a.Name(),
			map[string]any{"component": component},
		))
	}
	return findings
}

// gatherGitChunks returns commit chunks (newest first) and project metadata.
// When running in a monorepo (ProjectRoot != GitRoot), it scopes git analysis
// to only include commits affecting the project subdirectory.
// Requires project context to be set via config.SetProjectContext() - no fallbacks.
func gatherGitChunks(basePath string) ([]string, string) {
	// DETERMINISTIC: Use project context from CLI init - no fallback detection
	projectCtx := config.GetProjectContext()
	// Note: projectCtx may be nil if running outside CLI context (e.g., tests)
	// In that case, git commands run without path scoping (full repo analysis)
	scopePath := getGitScopePath(projectCtx)

	// Task 6: Log input parameters for debugging
	if projectCtx != nil {
		log.Printf("[git] gatherGitChunks called: basePath=%q, projectCtx.RootPath=%q, projectCtx.GitRoot=%q, projectCtx.IsMonorepo=%v, scopePath=%q",
			basePath, projectCtx.RootPath, projectCtx.GitRoot, projectCtx.IsMonorepo, scopePath)
	} else {
		log.Printf("[git] gatherGitChunks called: basePath=%q, projectCtx=nil, scopePath=%q",
			basePath, scopePath)
	}

	// Build git log command with optional path scoping
	args := []string{"log", "--format=%h %ad %s", "--date=short", fmt.Sprintf("-%d", gitMaxCommits)}
	if scopePath != "" {
		// Scope to project subdirectory: git log ... -- <path>
		args = append(args, "--", scopePath)
	}

	cmd := exec.Command("git", args...)
	workDir := getGitWorkDir(projectCtx, basePath)
	cmd.Dir = workDir

	// Task 5: Add error logging when git command fails
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[git] git log command failed: %v (dir=%s, args=%v)", err, workDir, args)
		return nil, ""
	}
	if len(out) == 0 {
		log.Printf("[git] git log returned empty output (dir=%s, args=%v)", workDir, args)
		return nil, ""
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, ""
	}
	lines := strings.Split(trimmed, "\n")

	// Split into chunks
	var chunks []string
	for i := 0; i < len(lines); i += gitChunkSize {
		end := i + gitChunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunk := strings.Join(lines[i:end], "\n")
		chunks = append(chunks, chunk)
	}

	// Gather project metadata (once, not per-chunk)
	meta := gatherProjectMeta(basePath, lines, projectCtx)

	return chunks, meta
}

// getGitScopePath returns the relative path to scope git operations to,
// or empty string if no scoping is needed.
func getGitScopePath(ctx *project.Context) string {
	if ctx == nil {
		return ""
	}
	// If GitRoot differs from RootPath, we're in a monorepo
	if ctx.IsMonorepo && ctx.GitRoot != "" && ctx.RootPath != ctx.GitRoot {
		// Use the relative path from git root to project root
		rel := ctx.RelativeGitPath()
		if rel != "." && rel != "" {
			// DEFENSIVE: A path starting with ".." indicates GitRoot is BELOW RootPath,
			// which is impossible for a valid git repository. This means the project
			// context is corrupted. Fall back to full repo analysis.
			if strings.HasPrefix(rel, "..") {
				log.Printf("[git] WARNING: Invalid scopePath %q (GitRoot=%q, RootPath=%q) - context appears corrupted, falling back to full repo analysis",
					rel, ctx.GitRoot, ctx.RootPath)
				return ""
			}
			return rel
		}
	}
	return ""
}

// getGitWorkDir returns the directory where git commands should be executed.
// For monorepos, this is the GitRoot; otherwise, it's the basePath.
func getGitWorkDir(ctx *project.Context, basePath string) string {
	if ctx != nil && ctx.GitRoot != "" {
		return ctx.GitRoot
	}
	return basePath
}

// gatherProjectMeta collects project-level git statistics.
// When in a monorepo, scopes contributor and age stats to the project subdirectory.
func gatherProjectMeta(basePath string, allCommits []string, projectCtx *project.Context) string {
	var sb strings.Builder

	// Commit type distribution
	typeCounts := make(map[string]int)
	scopeCounts := make(map[string]int)

	for _, line := range allCommits {
		// Skip hash and date to get message
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		msg := parts[2]

		switch {
		case strings.HasPrefix(msg, "feat"):
			typeCounts["feat"]++
		case strings.HasPrefix(msg, "fix"):
			typeCounts["fix"]++
		case strings.HasPrefix(msg, "refactor"):
			typeCounts["refactor"]++
		case strings.HasPrefix(msg, "chore"):
			typeCounts["chore"]++
		case strings.HasPrefix(msg, "docs"):
			typeCounts["docs"]++
		case strings.HasPrefix(msg, "test"):
			typeCounts["test"]++
		case strings.HasPrefix(msg, "perf"):
			typeCounts["perf"]++
		}

		if idx := strings.Index(msg, "("); idx != -1 {
			if end := strings.Index(msg[idx:], ")"); end != -1 {
				scope := msg[idx+1 : idx+end]
				scopeCounts[scope]++
			}
		}
	}

	// Add monorepo context note if applicable
	scopePath := getGitScopePath(projectCtx)
	if scopePath != "" {
		sb.WriteString(fmt.Sprintf("Scoped to: %s (monorepo subdirectory)\n\n", scopePath))
	}

	sb.WriteString(fmt.Sprintf("Total commits analyzed: %d\n\n", len(allCommits)))

	if len(typeCounts) > 0 {
		sb.WriteString("Commit Type Distribution:\n")
		for t, c := range typeCounts {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", t, c))
		}
		sb.WriteString("\n")
	}

	if len(scopeCounts) > 0 {
		sb.WriteString("Active Scopes:\n")
		for s, c := range scopeCounts {
			if c >= 3 {
				sb.WriteString(fmt.Sprintf("- %s: %d commits\n", s, c))
			}
		}
		sb.WriteString("\n")
	}

	// Top contributors - scope to project subdirectory if in monorepo
	gitDir := getGitWorkDir(projectCtx, basePath)
	shortlogArgs := []string{"shortlog", "-sn", "--all", "-5"}
	if scopePath != "" {
		shortlogArgs = append(shortlogArgs, "--", scopePath)
	}
	cmd := exec.Command("git", shortlogArgs...)
	cmd.Dir = gitDir
	out, _ := cmd.Output()
	if len(out) > 0 {
		sb.WriteString("Top Contributors:\n")
		sb.WriteString(strings.TrimSpace(string(out)))
		sb.WriteString("\n\n")
	}

	// Project age - scope to project subdirectory if in monorepo
	logArgs := []string{"log", "--reverse", "--format=%ai", "-1"}
	if scopePath != "" {
		logArgs = append(logArgs, "--", scopePath)
	}
	cmd = exec.Command("git", logArgs...)
	cmd.Dir = gitDir
	out, _ = cmd.Output()
	if len(out) > 0 {
		sb.WriteString(fmt.Sprintf("Project Started: %s\n", strings.TrimSpace(string(out))))
	}

	return sb.String()
}

func init() {
	core.RegisterAgent("git", func(cfg llm.Config, basePath string) core.Agent {
		return NewGitAgent(cfg)
	}, "Git History", "Extracts decisions and patterns from git commit history")
}
