/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package agents provides graph-based orchestration for running multiple agents
and synthesizing their findings into a coherent analysis.
*/
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// AgentGraph orchestrates multiple agents and synthesizes their outputs
type AgentGraph struct {
	llmConfig llm.Config
	basePath  string
	agents    []Agent
	verbose   bool
}

// NewAgentGraph creates a new agent orchestration graph
func NewAgentGraph(cfg llm.Config, basePath string) *AgentGraph {
	return &AgentGraph{
		llmConfig: cfg,
		basePath:  basePath,
		agents:    make([]Agent, 0),
	}
}

// AddAgent adds an agent to the graph
func (g *AgentGraph) AddAgent(agent Agent) {
	g.agents = append(g.agents, agent)
}

// SetVerbose enables detailed logging
func (g *AgentGraph) SetVerbose(v bool) {
	g.verbose = v
}

// RunResult contains the complete output from the agent graph
type RunResult struct {
	AgentOutputs      map[string]Output // Individual agent outputs
	SynthesizedReport *SynthesizedReport
	TotalDuration     time.Duration
	TotalFindings     int
}

// SynthesizedReport is the final cross-referenced output
type SynthesizedReport struct {
	Summary          string            `json:"summary"`
	KeyDecisions     []Finding         `json:"key_decisions"`
	Conflicts        []Conflict        `json:"conflicts"`
	ConfidenceScores map[string]string `json:"confidence_scores"`
	Recommendations  []string          `json:"recommendations"`
}

// Conflict represents a contradiction between agent findings
type Conflict struct {
	Topic       string   `json:"topic"`
	Description string   `json:"description"`
	Sources     []string `json:"sources"` // Agent names that conflict
	Resolution  string   `json:"resolution"`
}

// Run executes all agents in parallel and synthesizes their outputs
func (g *AgentGraph) Run(ctx context.Context, input Input) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{
		AgentOutputs: make(map[string]Output),
	}

	if len(g.agents) == 0 {
		return nil, fmt.Errorf("no agents configured in graph")
	}

	// Run all agents in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	outputs := make([]Output, len(g.agents))
	errors := make([]error, len(g.agents))

	if g.verbose {
		fmt.Printf("  🔄 Running %d agents in parallel...\n", len(g.agents))
	}

	for i, agent := range g.agents {
		wg.Add(1)
		go func(idx int, a Agent) {
			defer wg.Done()

			agentStart := time.Now()
			if g.verbose {
				fmt.Printf("    → Starting %s agent\n", a.Name())
			}

			out, err := a.Run(ctx, input)
			out.AgentName = a.Name()
			out.Duration = time.Since(agentStart)

			mu.Lock()
			outputs[idx] = out
			if err != nil {
				errors[idx] = err
				out.Error = err
			}
			result.AgentOutputs[a.Name()] = out
			mu.Unlock()

			if g.verbose {
				fmt.Printf("    ✓ %s completed in %.1fs (%d findings)\n",
					a.Name(), out.Duration.Seconds(), len(out.Findings))
			}
		}(i, agent)
	}

	wg.Wait()

	// Aggregate all findings
	allFindings := make([]Finding, 0)
	for _, out := range outputs {
		for _, f := range out.Findings {
			f.SourceAgent = out.AgentName
			allFindings = append(allFindings, f)
		}
	}
	result.TotalFindings = len(allFindings)

	if g.verbose {
		fmt.Printf("  ✓ All agents completed: %d findings total\n", result.TotalFindings)
		fmt.Printf("  🧠 Running synthesizer...\n")
	}

	// Run synthesizer to cross-reference and deduplicate
	synthesized, err := g.synthesize(ctx, allFindings)
	if err != nil {
		// Synthesizer failure is not fatal - return raw findings
		if g.verbose {
			fmt.Printf("  ⚠ Synthesizer warning: %v\n", err)
		}
		// Create basic report from raw findings
		synthesized = &SynthesizedReport{
			Summary:      fmt.Sprintf("Found %d architectural decisions across %d agents", len(allFindings), len(g.agents)),
			KeyDecisions: allFindings,
		}
	}

	result.SynthesizedReport = synthesized
	result.TotalDuration = time.Since(start)

	if g.verbose {
		fmt.Printf("  ✓ Graph completed in %.1fs\n", result.TotalDuration.Seconds())
	}

	return result, nil
}

// synthesize uses an LLM to cross-reference, deduplicate, and resolve conflicts
func (g *AgentGraph) synthesize(ctx context.Context, findings []Finding) (*SynthesizedReport, error) {
	if len(findings) == 0 {
		return &SynthesizedReport{
			Summary: "No findings to synthesize",
		}, nil
	}

	// Create chat model for synthesis
	chatModel, err := llm.NewChatModel(ctx, g.llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create synthesizer model: %w", err)
	}

	// Build prompt with all findings
	prompt := g.buildSynthesisPrompt(findings)

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("synthesizer generate: %w", err)
	}

	// Parse response
	return g.parseSynthesisResponse(resp.Content, findings)
}

// buildSynthesisPrompt creates the prompt for the synthesizer
func (g *AgentGraph) buildSynthesisPrompt(findings []Finding) string {
	var sb strings.Builder

	sb.WriteString(`You are an expert technical analyst synthesizing findings from multiple analysis agents.

## Task
1. Remove duplicate findings (same decision described differently)
2. Identify conflicts (contradictory claims about the same topic)
3. Rank findings by importance and confidence
4. Provide a summary and recommendations

## Findings from Agents
`)

	for i, f := range findings {
		sb.WriteString(fmt.Sprintf("\n### Finding %d (from %s agent)\n", i+1, f.SourceAgent))
		sb.WriteString(fmt.Sprintf("**%s**\n", f.Title))
		sb.WriteString(fmt.Sprintf("- What: %s\n", f.Description))
		if f.Why != "" {
			sb.WriteString(fmt.Sprintf("- Why: %s\n", f.Why))
		}
		if f.Tradeoffs != "" {
			sb.WriteString(fmt.Sprintf("- Tradeoffs: %s\n", f.Tradeoffs))
		}
		sb.WriteString(fmt.Sprintf("- Confidence: %s\n", f.Confidence))
	}

	sb.WriteString(`

## Output Format
Respond with JSON:

` + "```json" + `
{
  "summary": "2-3 sentence summary of the codebase architecture",
  "key_decisions": [
    {
      "title": "Decision title",
      "description": "What was decided",
      "why": "Why this matters",
      "confidence": "high|medium|low"
    }
  ],
  "conflicts": [
    {
      "topic": "Conflicting topic",
      "description": "What the conflict is",
      "sources": ["agent1", "agent2"],
      "resolution": "Which is correct and why"
    }
  ],
  "recommendations": ["List of actionable recommendations based on findings"]
}
` + "```" + `

Respond with JSON only:`)

	return sb.String()
}

// parseSynthesisResponse extracts the synthesized report from LLM response
func (g *AgentGraph) parseSynthesisResponse(response string, originalFindings []Finding) (*SynthesizedReport, error) {
	// Extract JSON from response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		// Fall back to raw findings
		return &SynthesizedReport{
			Summary:      "Unable to synthesize - returning raw findings",
			KeyDecisions: originalFindings,
		}, nil
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var parsed struct {
		Summary      string `json:"summary"`
		KeyDecisions []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Why         string `json:"why"`
			Confidence  string `json:"confidence"`
		} `json:"key_decisions"`
		Conflicts []struct {
			Topic       string   `json:"topic"`
			Description string   `json:"description"`
			Sources     []string `json:"sources"`
			Resolution  string   `json:"resolution"`
		} `json:"conflicts"`
		Recommendations []string `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return &SynthesizedReport{
			Summary:      "Parse error - returning raw findings",
			KeyDecisions: originalFindings,
		}, nil
	}

	// Convert to report
	report := &SynthesizedReport{
		Summary:          parsed.Summary,
		KeyDecisions:     make([]Finding, 0, len(parsed.KeyDecisions)),
		Conflicts:        make([]Conflict, 0, len(parsed.Conflicts)),
		Recommendations:  parsed.Recommendations,
		ConfidenceScores: make(map[string]string),
	}

	for _, kd := range parsed.KeyDecisions {
		report.KeyDecisions = append(report.KeyDecisions, Finding{
			Type:        FindingTypeDecision,
			Title:       kd.Title,
			Description: kd.Description,
			Why:         kd.Why,
			Confidence:  kd.Confidence,
			SourceAgent: "synthesizer",
		})
	}

	for _, c := range parsed.Conflicts {
		report.Conflicts = append(report.Conflicts, Conflict{
			Topic:       c.Topic,
			Description: c.Description,
			Sources:     c.Sources,
			Resolution:  c.Resolution,
		})
	}

	return report, nil
}

// CreateDefaultAgentGraph creates a graph with the standard set of agents
func CreateDefaultAgentGraph(cfg llm.Config, basePath string) *AgentGraph {
	graph := NewAgentGraph(cfg, basePath)

	// Add the ReAct code agent (dynamic exploration)
	graph.AddAgent(NewReactCodeAgent(cfg, basePath))

	// Add the traditional doc agent for documentation analysis
	graph.AddAgent(NewDocAgent(cfg))

	// Add deps agent for dependency analysis
	graph.AddAgent(NewDepsAgent(cfg))

	return graph
}
