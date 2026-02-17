package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// AddResult contains the result of adding knowledge to the memory.
// This is the canonical response type used by both CLI and MCP.
type AddResult struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Summary      string `json:"summary"`
	HasEmbedding bool   `json:"has_embedding"`
}

// AddOptions configures the behavior of an add operation.
type AddOptions struct {
	Type   string // Optional manual type override (decision, feature, plan, note)
	SkipAI bool   // Skip AI classification, store as-is
}

// MemoryApp provides knowledge CRUD operations.
// This is THE implementation - CLI and MCP both call these methods.
type MemoryApp struct {
	ctx *Context
}

// NewMemoryApp creates a new memory application service.
func NewMemoryApp(ctx *Context) *MemoryApp {
	return &MemoryApp{ctx: ctx}
}

// Add ingests knowledge with AI classification and embedding.
// This method encapsulates the entire ingestion pipeline:
// 1. AI classification (type and summary extraction)
// 2. Embedding generation
// 3. Storage in SQLite
// 4. Relationship detection (optional)
func (a *MemoryApp) Add(ctx context.Context, content string, opts AddOptions) (*AddResult, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// Quality gate: minimum content length
	trimmed := strings.TrimSpace(content)
	if len(trimmed) < 20 {
		return nil, fmt.Errorf("content too short (minimum 20 characters, got %d)", len(trimmed))
	}

	// Quality gate: reject noise content
	if isNoiseContent(trimmed) {
		return nil, fmt.Errorf("content rejected: appears to be placeholder or noise")
	}

	ks := knowledge.NewService(a.ctx.Repo, a.ctx.LLMCfg)

	// Prepare input
	input := knowledge.NodeInput{
		Content: content,
		Type:    opts.Type,
	}

	// If skipping AI, provide fallback values
	if opts.SkipAI {
		if input.Type == "" {
			input.Type = memory.NodeTypeUnknown
		}
		input.Summary = utils.Truncate(content, 100)
	}

	// Execute add operation
	node, err := ks.AddNode(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("add node: %w", err)
	}

	return &AddResult{
		ID:           node.ID,
		Type:         node.Type,
		Summary:      node.Summary,
		HasEmbedding: len(node.Embedding) > 0,
	}, nil
}

// List returns all knowledge nodes, optionally filtered by type.
func (a *MemoryApp) List(ctx context.Context, typeFilter string) ([]knowledge.NodeResponse, error) {
	nodes, err := a.ctx.Repo.ListNodes(typeFilter)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	results := make([]knowledge.NodeResponse, 0, len(nodes))
	for _, n := range nodes {
		// Score of 0 since these aren't search results
		results = append(results, knowledge.NodeToResponse(n, 0))
	}
	return results, nil
}

// Delete removes a knowledge node by ID.
func (a *MemoryApp) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("node ID is required")
	}
	return a.ctx.Repo.DeleteNode(id)
}

// Get retrieves a single knowledge node by ID.
func (a *MemoryApp) Get(ctx context.Context, id string) (*knowledge.NodeResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("node ID is required")
	}

	node, err := a.ctx.Repo.GetNode(id)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	if node == nil {
		return nil, fmt.Errorf("node not found: %s", id)
	}

	// Score of 0 since this isn't a search result
	resp := knowledge.NodeToResponse(*node, 0)
	return &resp, nil
}

// isNoiseContent detects content that shouldn't be stored as knowledge:
// single words, URL-only content, or common placeholder text.
func isNoiseContent(content string) bool {
	// Single word (no spaces)
	if !strings.Contains(content, " ") {
		return true
	}

	// URL-only content
	lower := strings.ToLower(content)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if !strings.Contains(content, " ") {
			return true
		}
	}

	// Common placeholder/test strings
	placeholders := []string{"test", "hello world", "foo bar", "lorem ipsum", "asdf", "todo", "fixme"}
	for _, p := range placeholders {
		if lower == p {
			return true
		}
	}

	return false
}
