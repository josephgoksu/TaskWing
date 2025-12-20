/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// DuplicateAgent checks if a requested feature already exists in the knowledge graph
// This is the critical agent that prevents redundant implementation
type DuplicateAgent struct {
	llmConfig llm.Config
	store     *memory.SQLiteStore
}

// NewDuplicateAgent creates a new duplicate detection agent
func NewDuplicateAgent(cfg llm.Config, store *memory.SQLiteStore) *DuplicateAgent {
	return &DuplicateAgent{
		llmConfig: cfg,
		store:     store,
	}
}

func (a *DuplicateAgent) Name() string { return "duplicate" }
func (a *DuplicateAgent) Description() string {
	return "Checks if a requested feature already exists in the project"
}

// CheckDuplicate checks if a feature request matches existing knowledge
func (a *DuplicateAgent) CheckDuplicate(ctx context.Context, query string) (*DuplicateCheckResult, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no memory store configured")
	}

	// Get all existing features from knowledge graph
	nodes, err := a.store.ListNodes("feature")
	if err != nil {
		return nil, fmt.Errorf("list features: %w", err)
	}

	if len(nodes) == 0 {
		return &DuplicateCheckResult{
			IsDuplicate: false,
			Message:     "No existing features in knowledge graph. Bootstrap required.",
		}, nil
	}

	// Build context from existing features
	var featureContext strings.Builder
	for _, node := range nodes {
		featureContext.WriteString(fmt.Sprintf("- %s: %s\n", node.Summary, node.Content))
	}

	// Use LLM to check semantic similarity
	prompt := a.buildCheckPrompt(query, featureContext.String())

	chatModel, err := llm.NewChatModel(ctx, a.llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	return a.parseCheckResponse(resp.Content)
}

func (a *DuplicateAgent) buildCheckPrompt(query, existingFeatures string) string {
	return fmt.Sprintf(`You are a product analyst checking for duplicate features.

USER REQUEST: "%s"

EXISTING FEATURES IN THIS PROJECT:
%s

TASK: Determine if the user's request overlaps with any existing feature.

Consider:
1. Exact match: User is asking for something that already exists
2. Partial overlap: User wants to extend something that exists
3. No overlap: This is genuinely new

RESPOND IN JSON:
{
  "is_duplicate": true|false,
  "confidence": "high|medium|low",
  "matching_feature": "Name of matching feature (if any)",
  "overlap_type": "exact|partial|none",
  "explanation": "Brief explanation of why this is/isn't a duplicate",
  "recommendation": "What the user should do instead"
}

Respond with JSON only:`, query, existingFeatures)
}

func (a *DuplicateAgent) parseCheckResponse(response string) (*DuplicateCheckResult, error) {
	// Clean response
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed struct {
		IsDuplicate     bool   `json:"is_duplicate"`
		Confidence      string `json:"confidence"`
		MatchingFeature string `json:"matching_feature"`
		OverlapType     string `json:"overlap_type"`
		Explanation     string `json:"explanation"`
		Recommendation  string `json:"recommendation"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}

	return &DuplicateCheckResult{
		IsDuplicate:     parsed.IsDuplicate,
		Confidence:      parsed.Confidence,
		MatchingFeature: parsed.MatchingFeature,
		OverlapType:     parsed.OverlapType,
		Message:         parsed.Explanation,
		Recommendation:  parsed.Recommendation,
	}, nil
}

// Run implements Agent interface but DuplicateAgent is typically used via CheckDuplicate
func (a *DuplicateAgent) Run(ctx context.Context, input Input) (Output, error) {
	var output Output

	// Use project name as a generic check query
	query := fmt.Sprintf("What features exist in %s?", input.ProjectName)
	result, err := a.CheckDuplicate(ctx, query)
	if err != nil {
		return output, err
	}

	output.RawOutput = fmt.Sprintf("Duplicate check result: %+v", result)
	return output, nil
}

// DuplicateCheckResult holds the result of a duplicate check
type DuplicateCheckResult struct {
	IsDuplicate     bool
	Confidence      string
	MatchingFeature string
	OverlapType     string // "exact", "partial", "none"
	Message         string
	Recommendation  string
}

// FormatWarning returns a user-friendly warning message if duplicate detected
func (r *DuplicateCheckResult) FormatWarning() string {
	if !r.IsDuplicate {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("⚠️  DUPLICATE DETECTED\n")
	sb.WriteString(fmt.Sprintf("   Feature: %s\n", r.MatchingFeature))
	sb.WriteString(fmt.Sprintf("   Overlap: %s (%s confidence)\n", r.OverlapType, r.Confidence))
	sb.WriteString(fmt.Sprintf("   Reason: %s\n", r.Message))
	sb.WriteString(fmt.Sprintf("   Recommendation: %s\n", r.Recommendation))
	return sb.String()
}
