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
	CreateNode(n memory.Node) error
	UpsertNodeBySummary(n memory.Node) error
	DeleteNodesByAgent(agent string) error

	// Feature/Decision/Pattern operations
	CreateFeature(f memory.Feature) error
	CreatePattern(p memory.Pattern) error
	AddDecision(featureID string, d memory.Decision) error
	ListFeatures() ([]memory.Feature, error)
	GetDecisions(featureID string) ([]memory.Decision, error)
	Link(fromID, toID string, relType string) error

	// Graph edge operations
	LinkNodes(from, to, relation string, confidence float64, properties map[string]any) error
	GetNodeEdges(nodeID string) ([]memory.NodeEdge, error)

	// FTS5 Hybrid Search (new)
	ListNodesWithEmbeddings() ([]memory.Node, error)
	SearchFTS(query string, limit int) ([]memory.FTSResult, error)
}

// Service provides high-level knowledge operations
type Service struct {
	repo             Repository
	llmCfg           llm.Config
	basePath         string // Project base path for verification
	chatModelFactory func(ctx context.Context, cfg llm.Config) (*llm.CloseableChatModel, error)
}

type NodeInput struct {
	Content     string
	Type        string // Optional manual override
	Summary     string // Optional
	SourceAgent string // Agent that produced this node
	Timestamp   time.Time
}

// NewService creates a new knowledge service
func NewService(repo Repository, cfg llm.Config) *Service {
	return &Service{
		repo:             repo,
		llmCfg:           cfg,
		chatModelFactory: llm.NewCloseableChatModel,
	}
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

// Search performs a hybrid search combining FTS5 keyword matching and vector similarity.
// This fixes the N+1 query pattern and provides keyword fallback when embeddings fail.
// Weights and thresholds are defined in config.go for centralized tuning.
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

// ListNodesByType retrieves all nodes of a specific type.
// This allows for retrieving ALL mandatory constraints without semantic filtering.
func (s *Service) ListNodesByType(ctx context.Context, nodeType string) ([]memory.Node, error) {
	return s.repo.ListNodes(nodeType)
}

func (s *Service) searchInternal(ctx context.Context, query string, typeFilter string, limit int) ([]ScoredNode, error) {
	if limit <= 0 {
		limit = 5
	}

	// Collect results from both search methods
	scoreByID := make(map[string]float32)
	nodeByID := make(map[string]*memory.Node)

	// 1. FTS5 keyword search (fast, no API call, always works)
	// Note: FTS currently searches all types. We filter later.
	ftsResults, err := s.repo.SearchFTS(query, limit*2)
	if err != nil {
		// FTS5 errors are logged but don't fail the search
		// FTS5 may be unavailable on some systems (missing extension)
		_ = err
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
		ftsScore := float32(1.0 / (1.0 - r.Rank)) // Convert negative rank to positive score
		if ftsScore > 1.0 {
			ftsScore = 1.0
		}
		node := r.Node // Copy to avoid pointer issues
		nodeByID[r.Node.ID] = &node
		scoreByID[r.Node.ID] = ftsScore * FTSWeight
	}

	// 2. Vector similarity search (single query, not N+1)
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
				if vectorScore < VectorScoreThreshold {
					continue // Skip low-relevance results
				}

				if _, exists := nodeByID[n.ID]; !exists {
					nodeByID[n.ID] = n
					scoreByID[n.ID] = 0
				}
				scoreByID[n.ID] += vectorScore * VectorWeight
			}
		}
	}

	// 3. Merge, filter low-confidence, and sort by combined score
	var scored []ScoredNode
	for id, score := range scoreByID {
		// Filter out noise: only include results above minimum threshold
		if score < MinResultScoreThreshold {
			continue
		}
		if node, ok := nodeByID[id]; ok {
			scored = append(scored, ScoredNode{Node: node, Score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// 4. Graph Expansion: Add connected nodes via knowledge graph edges
	if GraphExpansionEnabled && len(scored) > 0 {
		scored = s.expandViaGraph(scored, scoreByID, limit)
	}

	// 5. Limit results with reserved slots for graph-expanded nodes
	if GraphExpansionEnabled && GraphExpansionReservedSlots > 0 {
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
		reservedSlots := GraphExpansionReservedSlots
		if len(expanded) < reservedSlots {
			reservedSlots = len(expanded)
		}
		primarySlots := limit - reservedSlots
		if primarySlots < 0 {
			primarySlots = 0
		}

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
func (s *Service) expandViaGraph(initial []ScoredNode, existingScores map[string]float32, limit int) []ScoredNode {
	// Track which nodes we've already included
	includedIDs := make(map[string]bool)
	for _, sn := range initial {
		includedIDs[sn.Node.ID] = true
	}

	// Collect new nodes to add
	var expanded []ScoredNode
	expanded = append(expanded, initial...)

	// Only expand from top results to avoid noise
	topN := len(initial)
	if topN > 5 {
		topN = 5 // Limit expansion to top 5 matches
	}

	addedCount := 0
	for i := 0; i < topN; i++ {
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
			if edge.Confidence < GraphExpansionMinEdgeConfidence {
				slog.Debug("graph expansion: edge below confidence threshold", "confidence", edge.Confidence, "threshold", GraphExpansionMinEdgeConfidence)
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
			connectedScore := parentScore * float32(edge.Confidence) * GraphExpansionDiscount

			// Only include if score is above minimum threshold
			if connectedScore < MinResultScoreThreshold {
				slog.Debug("graph expansion: score below threshold", "score", connectedScore, "threshold", MinResultScoreThreshold)
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
	if err := s.repo.CreateNode(*node); err != nil {
		return nil, fmt.Errorf("save node: %w", err)
	}

	return node, nil
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
