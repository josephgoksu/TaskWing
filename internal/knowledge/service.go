package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// Repository abstracts all storage operations for the knowledge service.
// This is the single source of truth for repository interface requirements.
type Repository interface {
	// Read operations
	ListNodes(typeFilter string) ([]memory.Node, error)
	GetNode(id string) (*memory.Node, error)

	// Write operations
	CreateNode(n *memory.Node) error
	UpsertNodeBySummary(n memory.Node) error
	DeleteNodesByAgent(agent string) error
	DeleteNodesByFiles(agent string, filePaths []string) error
	GetNodesByFiles(agent string, filePaths []string) ([]memory.Node, error)

	// Feature/Decision/Pattern operations
	CreateFeature(f memory.Feature) error
	CreatePattern(p memory.Pattern) error
	AddDecision(featureID string, d memory.Decision) error
	ListFeatures() ([]memory.Feature, error)
	GetDecisions(featureID string) ([]memory.Decision, error)

	// Graph edge operations
	LinkNodes(from, to, relation string, confidence float64, properties map[string]any) error
	GetNodeEdges(nodeID string) ([]memory.NodeEdge, error)

	// FTS5 Hybrid Search (new)
	ListNodesWithEmbeddings() ([]memory.Node, error)
	SearchFTS(query string, limit int) ([]memory.FTSResult, error)

	// Embedding stats for dimension consistency checks
	GetEmbeddingStats() (*memory.EmbeddingStats, error)

	// Project Overview
	GetProjectOverview() (*memory.ProjectOverview, error)
}

// Service provides high-level knowledge operations
type Service struct {
	repo              Repository
	llmCfg            llm.Config
	retrievalCfg      RetrievalConfig // Dynamic search configuration
	basePath          string          // Project base path for verification
	chatModelFactory  func(ctx context.Context, cfg llm.Config) (*llm.CloseableChatModel, error)
	reranker          Reranker        // Optional reranker for two-stage retrieval
	rerankerFactory   RerankerFactory // Factory for creating reranker
	rerankerInitError error           // Cached error from reranker initialization
}

type NodeInput struct {
	Content     string
	Type        string // Optional manual override
	Summary     string // Optional
	SourceAgent string // Agent that produced this node
	Timestamp   time.Time
}

// NewService creates a new knowledge service with default retrieval config.
func NewService(repo Repository, cfg llm.Config) *Service {
	retrievalCfg := LoadRetrievalConfig()
	return &Service{
		repo:             repo,
		llmCfg:           cfg,
		retrievalCfg:     retrievalCfg,
		chatModelFactory: llm.NewCloseableChatModel,
		rerankerFactory:  DefaultRerankerFactory,
	}
}

// NewServiceWithConfig creates a new knowledge service with custom retrieval config.
func NewServiceWithConfig(repo Repository, llmCfg llm.Config, retrievalCfg RetrievalConfig) *Service {
	return &Service{
		repo:             repo,
		llmCfg:           llmCfg,
		retrievalCfg:     retrievalCfg,
		chatModelFactory: llm.NewCloseableChatModel,
		rerankerFactory:  DefaultRerankerFactory,
	}
}

// GetRetrievalConfig returns the current retrieval configuration.
func (s *Service) GetRetrievalConfig() RetrievalConfig {
	return s.retrievalCfg
}

// getReranker lazily initializes and returns the reranker.
// Returns nil if reranking is disabled or initialization failed.
func (s *Service) getReranker(ctx context.Context) Reranker {
	if s.reranker != nil {
		return s.reranker
	}
	if s.rerankerInitError != nil {
		return nil // Already tried and failed
	}
	if s.rerankerFactory == nil {
		return nil
	}

	reranker, err := s.rerankerFactory(ctx, s.retrievalCfg)
	if err != nil {
		s.rerankerInitError = err
		slog.Warn("failed to initialize reranker", "error", err)
		return nil
	}
	s.reranker = reranker
	return reranker
}

// SetBasePath sets the project base path for verification.
// This should be called before IngestFindings if verification is desired.
func (s *Service) SetBasePath(basePath string) {
	s.basePath = basePath
}

// ScoredNode represents a search result with visual relevance score
type ScoredNode struct {
	Node         *memory.Node `json:"node"`
	Score        float32      `json:"score"`
	ExpandedFrom string       `json:"expanded_from,omitempty"` // Parent node ID if this came from graph expansion
}

// RewriteQuery uses LLM to improve a user query for better search results.
// It fixes typos, clarifies intent, and makes queries more concise.
// Returns the original query if rewriting fails or is disabled.
func (s *Service) RewriteQuery(ctx context.Context, query string) (string, error) {
	if !s.retrievalCfg.QueryRewriteEnabled {
		return query, nil
	}

	// Use a domain-aware prompt for query rewriting
	// Include context about TaskWing to help fix domain-specific typos
	prompt := fmt.Sprintf(`You are improving a search query for TaskWing, a developer tool that gives AI coding assistants permanent memory about codebases.

Rules:
1. Fix typos (e.g., "TaskWink" → "TaskWing", "teh" → "the")
2. Keep all important context and terms - do NOT over-simplify
3. Remove only true redundancy (repeated words)
4. Preserve the user's intent completely
5. Return ONLY the improved query, nothing else

Original query: %s

Improved query:`, query)

	chatModel, err := s.chatModelFactory(ctx, s.llmCfg)
	if err != nil {
		slog.Debug("query rewrite: failed to get chat model", "error", err)
		return query, nil // Fallback to original on error
	}
	defer func() { _ = chatModel.Close() }()

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	if err != nil {
		slog.Debug("query rewrite: LLM call failed", "error", err)
		return query, nil // Fallback to original on error
	}

	rewritten := strings.TrimSpace(resp.Content)

	// Strip common LLM preambles that ignore our "ONLY the query" instruction
	preambles := []string{
		"Improved query:",
		"Here's the improved query:",
		"Here is the improved query:",
		"The improved query is:",
		"Rewritten query:",
	}
	for _, prefix := range preambles {
		if strings.HasPrefix(strings.ToLower(rewritten), strings.ToLower(prefix)) {
			rewritten = strings.TrimSpace(rewritten[len(prefix):])
			break
		}
	}

	// Remove quotes if LLM wrapped the query in them
	rewritten = strings.Trim(rewritten, `"'`)
	rewritten = strings.TrimSpace(rewritten)

	if rewritten == "" || len(rewritten) > len(query)*3 {
		// If response is empty or suspiciously long, use original
		return query, nil
	}

	slog.Debug("query rewrite", "original", query, "rewritten", rewritten)
	return rewritten, nil
}

// Search performs a hybrid search combining FTS5 keyword matching and vector similarity.
// This fixes the N+1 query pattern and provides keyword fallback when embeddings fail.
// Weights and thresholds are defined in config.go for centralized tuning.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]ScoredNode, error) {
	return s.searchInternal(ctx, query, "", limit)
}

// SearchByType performs a semantic search restricted to a specific node type.
// This allows for surgical retrieval of "workflows" or "constraints".
func (s *Service) SearchByType(ctx context.Context, query string, nodeType string, limit int) ([]ScoredNode, error) {
	return s.searchInternal(ctx, query, nodeType, limit)
}

// SearchWithFilter performs a workspace-aware search with the given filter options.
// This is the preferred method for monorepo-aware searches.
// It uses the standard search pipeline, then filters results by workspace.
func (s *Service) SearchWithFilter(ctx context.Context, query string, limit int, filter memory.NodeFilter) ([]ScoredNode, error) {
	// If no workspace filter, use regular search
	if filter.Workspace == "" {
		return s.searchInternal(ctx, query, filter.Type, limit)
	}

	// Fetch more candidates to account for post-filtering
	// Multiply by 3 to ensure we have enough after workspace filtering
	candidateLimit := limit * 3
	if candidateLimit < 15 {
		candidateLimit = 15
	}

	// Use standard search to get candidates
	results, err := s.searchInternal(ctx, query, filter.Type, candidateLimit)
	if err != nil {
		return nil, err
	}

	// Filter by workspace
	var filtered []ScoredNode
	for _, sn := range results {
		// Check if node matches workspace filter
		if matchesWorkspaceFilter(sn.Node.Workspace, filter) {
			filtered = append(filtered, sn)
			if len(filtered) >= limit {
				break
			}
		}
	}

	return filtered, nil
}

// matchesWorkspaceFilter checks if a node's workspace matches the filter criteria.
func matchesWorkspaceFilter(nodeWorkspace string, filter memory.NodeFilter) bool {
	// Empty workspace in node is treated as "root" for filtering purposes
	if nodeWorkspace == "" {
		nodeWorkspace = "root"
	}

	// Check exact workspace match
	if nodeWorkspace == filter.Workspace {
		return true
	}

	// If IncludeRoot is true, also include root/global nodes
	if filter.IncludeRoot && (nodeWorkspace == "root" || nodeWorkspace == "") {
		return true
	}

	return false
}

// ListNodesByType retrieves all nodes of a specific type.
// This allows for retrieving ALL mandatory constraints without semantic filtering.
func (s *Service) ListNodesByType(ctx context.Context, nodeType string) ([]memory.Node, error) {
	return s.repo.ListNodes(nodeType)
}

// GetNodesByFiles retrieves nodes relevant to specific files.
func (s *Service) GetNodesByFiles(agentName string, filePaths []string) ([]memory.Node, error) {
	return s.repo.GetNodesByFiles(agentName, filePaths)
}

func (s *Service) searchInternal(ctx context.Context, query string, typeFilter string, limit int) ([]ScoredNode, error) {
	if limit <= 0 {
		limit = 5
	}

	// Use dynamic configuration values
	cfg := s.retrievalCfg
	ftsWeight := float32(cfg.FTSWeight)
	vectorWeight := float32(cfg.VectorWeight)
	vectorThreshold := float32(cfg.VectorScoreThreshold)
	minResultThreshold := float32(cfg.MinResultScoreThreshold)

	// Two-stage retrieval: fetch more candidates for reranking
	// Stage 1 (Recall): Fetch Top-25 candidates using hybrid search
	recallLimit := cfg.RerankTopK
	if recallLimit <= 0 {
		recallLimit = 25 // Default recall candidates
	}
	if !cfg.RerankingEnabled {
		// If reranking disabled, just fetch what we need
		recallLimit = limit * 2 // Fetch 2x for graph expansion buffer
	}

	// Collect results from both search methods
	scoreByID := make(map[string]float32)
	nodeByID := make(map[string]*memory.Node)

	// 1. FTS5 keyword search (fast, no API call, always works)
	// Note: FTS currently searches all types. We filter later.
	ftsResults, err := s.repo.SearchFTS(query, recallLimit)
	if err != nil {
		// FTS5 errors are logged but don't fail the search
		// FTS5 may be unavailable on some systems (missing extension)
		slog.Debug("FTS search error", "error", err)
	}
	for _, r := range ftsResults {
		// Filter by type if requested
		if typeFilter != "" && r.Node.Type != typeFilter {
			// Check metadata for type override (e.g. workflow stored as pattern)
			if r.Node.Type == "pattern" && typeFilter == "workflow" {
				// Allow patterns tagged as workflow
				// This requires deserializing metadata which isn't available on FTSResult Node yet
				// We'll rely on vector search for deep metadata filtering or handle it when hydrating
			} else {
				continue
			}
		}

		// Convert BM25 rank to score (BM25 is negative, more negative = better)
		// Normalize to 0-1 range where 1 is best match
		// BM25 typical range: -1 (weak) to -15 (very strong match)
		// Formula: clamp(-rank / 10, 0, 1) - scales -10 rank to 1.0
		ftsScore := float32(-r.Rank / 10.0)
		if ftsScore > 1.0 {
			ftsScore = 1.0
		}
		if ftsScore < 0.1 {
			ftsScore = 0.1 // Minimum score for any FTS match
		}
		node := r.Node // Copy to avoid pointer issues
		nodeByID[r.Node.ID] = &node
		scoreByID[r.Node.ID] = ftsScore * ftsWeight
	}

	// 2. Vector similarity search (single query, not N+1)
	if vectorWeight > 0 {
		queryEmbedding, embErr := GenerateEmbedding(ctx, query, s.llmCfg)
		if embErr == nil && len(queryEmbedding) > 0 {
			// Use the optimized single-query method
			nodes, err := s.repo.ListNodesWithEmbeddings()
			if err == nil {
				for i := range nodes {
					n := &nodes[i]
					if len(n.Embedding) == 0 {
						continue
					}

					// TYPE FILTERING
					if typeFilter != "" {
						match := false
						if n.Type == typeFilter {
							match = true
						} else if typeFilter == "workflow" && n.Type == "pattern" {
							// Check metadata for workflow tag
							// We do a quick string check on the content for the "Steps:" marker
							if strings.Contains(n.Content, "Steps:") {
								match = true
							}
						}
						if !match {
							continue
						}
					}

					vectorScore := CosineSimilarity(queryEmbedding, n.Embedding)
					if vectorScore < vectorThreshold {
						continue // Skip low-relevance results
					}

					if _, exists := nodeByID[n.ID]; !exists {
						nodeByID[n.ID] = n
						scoreByID[n.ID] = 0
					}
					scoreByID[n.ID] += vectorScore * vectorWeight
				}
			}
		}
	}

	// 3. Merge, filter low-confidence, and sort by combined score
	var scored []ScoredNode
	for id, score := range scoreByID {
		// Filter out noise: only include results above minimum threshold
		if score < minResultThreshold {
			continue
		}
		if node, ok := nodeByID[id]; ok {
			scored = append(scored, ScoredNode{Node: node, Score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Limit to recall candidates before reranking
	if len(scored) > recallLimit {
		scored = scored[:recallLimit]
	}

	// 4. Stage 2 (Precision): Rerank using TEI if enabled
	if cfg.RerankingEnabled && len(scored) > 0 {
		reranker := s.getReranker(ctx)
		if reranker != nil {
			// Apply reranking with 5s timeout and fallback
			scored = rerankResults(ctx, reranker, query, scored, 5*time.Second)
		}
	}

	// 5. Graph Expansion: Add connected nodes via knowledge graph edges
	if cfg.GraphExpansionEnabled && len(scored) > 0 {
		scored = s.expandViaGraph(scored, cfg)
	}

	// 6. Limit results with reserved slots for graph-expanded nodes
	if cfg.GraphExpansionEnabled && cfg.GraphExpansionReservedSlots > 0 {
		// Separate expanded and non-expanded nodes
		var nonExpanded, expanded []ScoredNode
		for _, sn := range scored {
			if sn.ExpandedFrom != "" {
				expanded = append(expanded, sn)
			} else {
				nonExpanded = append(nonExpanded, sn)
			}
		}

		// Calculate slots: reserve up to GraphExpansionReservedSlots for expanded
		reservedSlots := min(len(expanded), cfg.GraphExpansionReservedSlots)
		primarySlots := max(0, limit-reservedSlots)

		// Take top primarySlots from non-expanded
		if len(nonExpanded) > primarySlots {
			nonExpanded = nonExpanded[:primarySlots]
		}
		// Take top reservedSlots from expanded
		if len(expanded) > reservedSlots {
			expanded = expanded[:reservedSlots]
		}

		// Combine and re-sort
		scored = append(nonExpanded, expanded...)
		sort.Slice(scored, func(i, j int) bool {
			return scored[i].Score > scored[j].Score
		})
	} else {
		// Simple limit without reserved slots
		if len(scored) > limit {
			scored = scored[:limit]
		}
	}

	return scored, nil
}

// expandViaGraph traverses knowledge graph edges to include related nodes.
// Connected nodes receive a discounted score based on parent score and edge confidence.
func (s *Service) expandViaGraph(initial []ScoredNode, cfg RetrievalConfig) []ScoredNode {
	// Use config values
	minEdgeConfidence := cfg.GraphExpansionMinEdgeConfidence
	discount := float32(cfg.GraphExpansionDiscount)
	minResultThreshold := float32(cfg.MinResultScoreThreshold)

	// Track which nodes we've already included
	includedIDs := make(map[string]bool)
	for _, sn := range initial {
		includedIDs[sn.Node.ID] = true
	}

	// Collect new nodes to add
	var expanded []ScoredNode
	expanded = append(expanded, initial...)

	// Only expand from top results to avoid noise
	topN := min(len(initial), 5)

	addedCount := 0
	for i := range topN {
		parentNode := initial[i]
		parentScore := parentNode.Score

		// Fetch edges for this node
		edges, err := s.repo.GetNodeEdges(parentNode.Node.ID)
		if err != nil {
			slog.Debug("graph expansion: GetNodeEdges error", "nodeID", parentNode.Node.ID, "error", err)
			continue
		}
		if len(edges) == 0 {
			slog.Debug("graph expansion: no edges for node", "nodeID", parentNode.Node.ID)
			continue
		}
		slog.Debug("graph expansion: found edges", "nodeID", parentNode.Node.ID, "edgeCount", len(edges))

		for _, edge := range edges {
			// Filter weak edges
			if edge.Confidence < minEdgeConfidence {
				slog.Debug("graph expansion: edge below confidence threshold", "confidence", edge.Confidence, "threshold", minEdgeConfidence)
				continue
			}

			// Determine connected node ID (could be from_node or to_node)
			connectedID := edge.ToNode
			if edge.ToNode == parentNode.Node.ID {
				connectedID = edge.FromNode
			}

			// Skip if already included
			if includedIDs[connectedID] {
				continue
			}

			// Fetch the connected node
			connectedNode, err := s.repo.GetNode(connectedID)
			if err != nil {
				slog.Debug("graph expansion: GetNode error", "connectedID", connectedID, "error", err)
				continue
			}

			// Calculate discounted score: parent_score * edge_confidence * discount
			connectedScore := parentScore * float32(edge.Confidence) * discount

			// Only include if score is above minimum threshold
			if connectedScore < minResultThreshold {
				slog.Debug("graph expansion: score below threshold", "score", connectedScore, "threshold", minResultThreshold)
				continue
			}

			includedIDs[connectedID] = true
			expanded = append(expanded, ScoredNode{
				Node:         connectedNode,
				Score:        connectedScore,
				ExpandedFrom: parentNode.Node.ID, // Track that this came from graph expansion
			})
			addedCount++
		}
	}

	slog.Debug("graph expansion complete", "initialCount", len(initial), "addedCount", addedCount, "totalCount", len(expanded))

	// Re-sort by score
	sort.Slice(expanded, func(i, j int) bool {
		return expanded[i].Score > expanded[j].Score
	})

	return expanded
}

// Ask generates an answer based on the search results
func (s *Service) Ask(ctx context.Context, query string, contextNodes []ScoredNode) (string, error) {
	if len(contextNodes) == 0 {
		return "I found no relevant information to answer your question.", nil
	}

	var contextParts []string
	for _, sn := range contextNodes {
		nodeContext := fmt.Sprintf("[%s] %s\n%s", sn.Node.Type, sn.Node.Summary, sn.Node.Content)
		contextParts = append(contextParts, nodeContext)
	}
	retrievedContext := strings.Join(contextParts, "\n\n---\n\n")

	prompt := fmt.Sprintf(`You are an expert on this codebase. Answer the user's question using ONLY the context below.
If the context doesn't contain enough information to answer, say so.
Be concise and direct.

## Retrieved Context:
%s

## Question:
%s

## Answer:`, retrievedContext, query)

	chatModel, err := s.chatModelFactory(ctx, s.llmCfg)
	if err != nil {
		return "", fmt.Errorf("create chat model: %w", err)
	}
	defer func() { _ = chatModel.Close() }()

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate answer: %w", err)
	}

	return resp.Content, nil
}

// AddNode process content (classifies, embeds) and saves it
func (s *Service) AddNode(ctx context.Context, input NodeInput) (*memory.Node, error) {
	node := &memory.Node{
		Content:     input.Content,
		Type:        input.Type,
		Summary:     input.Summary,
		SourceAgent: input.SourceAgent,
		CreatedAt:   input.Timestamp,
	}

	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now().UTC()
	}

	// 1. Classify if type/summary missing
	if node.Type == "" || node.Summary == "" {
		if s.llmCfg.APIKey != "" {
			classified, err := Classify(ctx, input.Content, s.llmCfg)
			if err == nil {
				if node.Type == "" {
					node.Type = classified.Type
				}
				if node.Summary == "" {
					node.Summary = classified.Summary
				}
			}
		}
		// Fallback defaults
		if node.Type == "" {
			node.Type = memory.NodeTypeUnknown
		}
		if node.Summary == "" {
			if len(input.Content) > 50 {
				node.Summary = input.Content[:47] + "..."
			} else {
				node.Summary = input.Content
			}
		}
	}

	// 2. Generate Embedding
	if s.llmCfg.APIKey != "" {
		emb, err := GenerateEmbedding(ctx, input.Content, s.llmCfg)
		if err == nil {
			node.Embedding = emb
		}
	}

	// 3. Save to Repo
	if err := s.repo.CreateNode(node); err != nil {
		return nil, fmt.Errorf("save node: %w", err)
	}

	return node, nil
}

// EmbeddingConsistencyCheck represents the result of an embedding consistency check.
type EmbeddingConsistencyCheck struct {
	TotalNodes             int
	NodesWithEmbeddings    int
	NodesWithoutEmbeddings int
	EmbeddingDimension     int
	MixedDimensions        bool
	NeedsAttention         bool   // True if issues were found
	Message                string // Human-readable summary
}

// CheckEmbeddingConsistency verifies embedding dimension consistency.
// This should be called on application startup to detect issues early.
// Returns nil if no issues are found, otherwise returns a check result.
func (s *Service) CheckEmbeddingConsistency() (*EmbeddingConsistencyCheck, error) {
	stats, err := s.repo.GetEmbeddingStats()
	if err != nil {
		return nil, fmt.Errorf("get embedding stats: %w", err)
	}
	if stats == nil || stats.TotalNodes == 0 {
		// Empty database - nothing to check
		return nil, nil
	}

	check := &EmbeddingConsistencyCheck{
		TotalNodes:             stats.TotalNodes,
		NodesWithEmbeddings:    stats.NodesWithEmbeddings,
		NodesWithoutEmbeddings: stats.NodesWithoutEmbeddings,
		EmbeddingDimension:     stats.EmbeddingDimension,
		MixedDimensions:        stats.MixedDimensions,
	}

	// Build message and determine if attention is needed
	var issues []string

	if stats.MixedDimensions {
		issues = append(issues, fmt.Sprintf("mixed embedding dimensions detected (found %d-dim, but others exist)", stats.EmbeddingDimension))
		check.NeedsAttention = true
	}

	if stats.NodesWithoutEmbeddings > 0 {
		issues = append(issues, fmt.Sprintf("%d nodes missing embeddings", stats.NodesWithoutEmbeddings))
		check.NeedsAttention = true
	}

	if check.NeedsAttention {
		fixHint := "Run 'taskwing memory rebuild-embeddings' to fix."
		if stats.MixedDimensions && stats.NodesWithoutEmbeddings > 0 {
			fixHint = "Run 'taskwing memory rebuild-embeddings' to fix mixed dimensions and regenerate missing embeddings."
		} else if !stats.MixedDimensions && stats.NodesWithoutEmbeddings > 0 {
			fixHint = "Run 'taskwing memory generate-embeddings' to backfill."
		}
		check.Message = fmt.Sprintf("Embedding issues: %s. %s", strings.Join(issues, "; "), fixHint)
		slog.Warn("embedding consistency check failed",
			"total_nodes", stats.TotalNodes,
			"nodes_with_embeddings", stats.NodesWithEmbeddings,
			"nodes_without_embeddings", stats.NodesWithoutEmbeddings,
			"mixed_dimensions", stats.MixedDimensions,
		)
	}

	if !check.NeedsAttention {
		return nil, nil // No issues
	}

	return check, nil
}

// SuggestContextQueries runs a lightweight LLM call to strategize what knowledge is needed.
func (s *Service) SuggestContextQueries(ctx context.Context, goal string) ([]string, error) {
	prompt := fmt.Sprintf(`You are a Research Specialist.
Your goal is to retrieve the most relevant architectural context to help an agent achieve: "%s".

Generate a JSON list of 3-5 short, natural language search phrases.
DO NOT use boolean operators like AND, OR, NOT.
DO NOT key-value pairs or file paths.
Just simple concepts.

Focus on:
1. Technology Stack (e.g. "Technology Stack", "Framework Decision")
2. Relevant Architectural Patterns (e.g. "Error Handling", "Auth Pattern")
3. Domain Knowledge (e.g. "User Model", "Pricing Logic")

Return JSON ONLY: ["concept 1", "concept 2"]`, goal)

	chatModel, err := s.chatModelFactory(ctx, s.llmCfg)
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}
	defer func() { _ = chatModel.Close() }()

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	// We expect a small JSON list
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("generate queries: %w", err)
	}

	// Simple cleaning of markdown code blocks if present
	content := resp.Content
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var queries []string
	if err := json.Unmarshal([]byte(content), &queries); err != nil {
		// Fallback: just return the goal + tech stack if parsing fails
		return []string{goal, "Technology Stack and Architecture"}, nil
	}

	return queries, nil
}

// SearchDebug performs a search with full debug information.
// It returns raw retrieval data including individual scores from each stage.
// This is used by the `memory inspect` command for transparency.
func (s *Service) SearchDebug(ctx context.Context, query string, limit int) (*DebugRetrievalResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	response := &DebugRetrievalResponse{
		Query:   query,
		Timings: make(map[string]int64),
	}
	pipeline := []string{}

	cfg := s.retrievalCfg
	ftsWeight := float32(cfg.FTSWeight)
	vectorWeight := float32(cfg.VectorWeight)
	vectorThreshold := float32(cfg.VectorScoreThreshold)

	recallLimit := cfg.RerankTopK
	if recallLimit <= 0 {
		recallLimit = 25
	}

	// Track individual scores per node
	type nodeScores struct {
		node         *memory.Node
		ftsScore     float32
		vectorScore  float32
		combined     float32
		rerankScore  float32
		isExact      bool
		isExpanded   bool
		expandedFrom string
	}
	scoreMap := make(map[string]*nodeScores)

	// 1. Check for exact ID match first (Task ID priority)
	startExact := time.Now()
	if strings.HasPrefix(query, "task-") || strings.HasPrefix(query, "n-") || strings.HasPrefix(query, "plan-") {
		node, err := s.repo.GetNode(query)
		if err == nil && node != nil {
			scoreMap[node.ID] = &nodeScores{
				node:     node,
				combined: 1.0, // Max score for exact match
				isExact:  true,
			}
			pipeline = append(pipeline, "ExactMatch")
		}
	}
	response.Timings["exact_match"] = time.Since(startExact).Milliseconds()

	// 2. FTS5 keyword search
	startFTS := time.Now()
	ftsResults, err := s.repo.SearchFTS(query, recallLimit)
	if err == nil && len(ftsResults) > 0 {
		pipeline = append(pipeline, "FTS")
		for _, r := range ftsResults {
			ftsScore := float32(-r.Rank / 10.0)
			if ftsScore > 1.0 {
				ftsScore = 1.0
			}
			if ftsScore < 0.1 {
				ftsScore = 0.1
			}

			node := r.Node
			if existing, ok := scoreMap[node.ID]; ok {
				existing.ftsScore = ftsScore
				existing.combined += ftsScore * ftsWeight
			} else {
				scoreMap[node.ID] = &nodeScores{
					node:     &node,
					ftsScore: ftsScore,
					combined: ftsScore * ftsWeight,
				}
			}
		}
	}
	response.Timings["fts"] = time.Since(startFTS).Milliseconds()

	// 3. Vector similarity search
	startVector := time.Now()
	if vectorWeight > 0 {
		queryEmbedding, embErr := GenerateEmbedding(ctx, query, s.llmCfg)
		if embErr == nil && len(queryEmbedding) > 0 {
			pipeline = append(pipeline, "Vector")
			nodes, err := s.repo.ListNodesWithEmbeddings()
			if err == nil {
				for i := range nodes {
					n := &nodes[i]
					if len(n.Embedding) == 0 {
						continue
					}

					vectorScore := CosineSimilarity(queryEmbedding, n.Embedding)
					if vectorScore < vectorThreshold {
						continue
					}

					if existing, ok := scoreMap[n.ID]; ok {
						existing.vectorScore = vectorScore
						existing.combined += vectorScore * vectorWeight
					} else {
						scoreMap[n.ID] = &nodeScores{
							node:        n,
							vectorScore: vectorScore,
							combined:    vectorScore * vectorWeight,
						}
					}
				}
			}
		}
	}
	response.Timings["vector"] = time.Since(startVector).Milliseconds()

	// 4. Convert to sorted slice
	var scored []ScoredNode
	for _, ns := range scoreMap {
		scored = append(scored, ScoredNode{Node: ns.node, Score: ns.combined})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > recallLimit {
		scored = scored[:recallLimit]
	}
	response.TotalCandidates = len(scored)

	// 5. Reranking (if enabled)
	startRerank := time.Now()
	if cfg.RerankingEnabled && len(scored) > 0 {
		reranker := s.getReranker(ctx)
		if reranker != nil {
			pipeline = append(pipeline, "Rerank")
			scored = rerankResults(ctx, reranker, query, scored, 5*time.Second)
			// Update rerank scores
			for i, sn := range scored {
				if ns, ok := scoreMap[sn.Node.ID]; ok {
					ns.rerankScore = sn.Score
					// Recalculate rank-based score
					ns.combined = sn.Score
					scored[i] = ScoredNode{Node: sn.Node, Score: sn.Score}
				}
			}
		}
	}
	response.Timings["rerank"] = time.Since(startRerank).Milliseconds()

	// 6. Graph Expansion
	startGraph := time.Now()
	if cfg.GraphExpansionEnabled && len(scored) > 0 {
		beforeCount := len(scored)
		scored = s.expandViaGraph(scored, cfg)
		if len(scored) > beforeCount {
			pipeline = append(pipeline, "Graph")
			// Mark expanded nodes
			for _, sn := range scored[beforeCount:] {
				if ns, ok := scoreMap[sn.Node.ID]; ok {
					ns.isExpanded = true
				} else {
					scoreMap[sn.Node.ID] = &nodeScores{
						node:         sn.Node,
						combined:     sn.Score,
						isExpanded:   true,
						expandedFrom: sn.ExpandedFrom,
					}
				}
			}
		}
	}
	response.Timings["graph"] = time.Since(startGraph).Milliseconds()

	// 7. Final limit
	if len(scored) > limit {
		scored = scored[:limit]
	}

	// 8. Build debug results
	response.Pipeline = pipeline
	for _, sn := range scored {
		ns := scoreMap[sn.Node.ID]
		if ns == nil {
			continue
		}

		result := DebugRetrievalResult{
			ID:              sn.Node.ID,
			ChunkID:         sn.Node.ID, // Same for now, could differ for sub-chunks
			NodeType:        sn.Node.Type,
			SourceAgent:     sn.Node.SourceAgent,
			Summary:         sn.Node.Summary,
			Content:         sn.Node.Content,
			FTSScore:        ns.ftsScore,
			VectorScore:     ns.vectorScore,
			CombinedScore:   ns.combined,
			RerankScore:     ns.rerankScore,
			IsExactMatch:    ns.isExact,
			IsGraphExpanded: ns.isExpanded,
			Evidence:        parseEvidence(sn.Node.Evidence),
		}

		// Extract source file path from evidence if available
		if len(result.Evidence) > 0 {
			result.SourceFilePath = result.Evidence[0].File
		}

		// Add embedding dimension if present
		if len(sn.Node.Embedding) > 0 {
			result.EmbeddingDimension = len(sn.Node.Embedding)
		}

		response.Results = append(response.Results, result)
	}

	return response, nil
}

// SearchByID retrieves a node by exact ID match.
// This prioritizes Task IDs and Plan IDs for direct lookup.
func (s *Service) SearchByID(ctx context.Context, id string) (*memory.Node, error) {
	return s.repo.GetNode(id)
}
