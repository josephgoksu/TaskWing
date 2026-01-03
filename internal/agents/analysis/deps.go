/*
Package analysis provides agents for analyzing dependencies.
*/
package analysis

import (
	"context"
	"fmt"
	"io"
	"os"
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

	depsInfo, filesRead := gatherDepsWithTracking(input.BasePath)
	if depsInfo == "" {
		return core.Output{AgentName: a.Name(), Error: fmt.Errorf("no dependency files found")}, nil
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
	output := core.BuildOutput(a.Name(), findings, "JSON output handled by Eino", duration)

	// Add coverage stats
	output.Coverage = core.CoverageStats{
		FilesAnalyzed:   len(filesRead),
		TotalFiles:      len(filesRead),
		CoveragePercent: 100.0,
		FilesRead:       filesRead,
	}

	return output, nil
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

// gatherDepsWithTracking collects dependency file contents and tracks which files were read.
// Uses os.ReadFile instead of shelling out to cat for better portability and error handling.
func gatherDepsWithTracking(basePath string) (string, []core.FileRead) {
	var sb strings.Builder
	var filesRead []core.FileRead

	// Find and read package.json files (excluding node_modules)
	cmd := exec.Command("find", ".", "-name", "package.json", "-not", "-path", "*/node_modules/*", "-type", "f")
	cmd.Dir = basePath
	out, err := cmd.Output()
	// Note: find command may fail on non-Unix systems; we continue with empty results
	if err == nil && len(out) > 0 {
		files := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, file := range files {
			if file == "" {
				continue
			}
			// Use os.ReadFile for better error handling and portability
			fullPath := file
			if !strings.HasPrefix(file, "/") {
				fullPath = basePath + "/" + strings.TrimPrefix(file, "./")
			}
			content, err := readFileWithLimit(fullPath, 3000)
			if err != nil {
				continue // Skip files we can't read
			}
			truncated := len(content) == 3000
			sb.WriteString(fmt.Sprintf("## %s\n```json\n%s\n```\n\n", file, string(content)))
			filesRead = append(filesRead, core.FileRead{
				Path:       file,
				Characters: len(content),
				Lines:      strings.Count(string(content), "\n") + 1,
				Truncated:  truncated,
			})
		}
	}

	// Find and read go.mod files
	cmd = exec.Command("find", ".", "-name", "go.mod", "-type", "f")
	cmd.Dir = basePath
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		files := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, file := range files {
			if file == "" {
				continue
			}
			fullPath := file
			if !strings.HasPrefix(file, "/") {
				fullPath = basePath + "/" + strings.TrimPrefix(file, "./")
			}
			content, err := readFileWithLimit(fullPath, 2000)
			if err != nil {
				continue // Skip files we can't read
			}
			truncated := len(content) == 2000
			sb.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", file, string(content)))
			filesRead = append(filesRead, core.FileRead{
				Path:       file,
				Characters: len(content),
				Lines:      strings.Count(string(content), "\n") + 1,
				Truncated:  truncated,
			})
		}
	}

	return sb.String(), filesRead
}

// readFileWithLimit reads a file up to maxBytes, returning the content read.
func readFileWithLimit(path string, maxBytes int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	content := make([]byte, maxBytes)
	n, err := f.Read(content)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return content[:n], nil
}

func init() {
	core.RegisterAgent("deps", func(cfg llm.Config, basePath string) core.Agent {
		return NewDepsAgent(cfg)
	}, "Dependencies", "Analyzes project dependencies and their purposes")
}
