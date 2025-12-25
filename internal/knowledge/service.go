package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
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

	// FTS5 Hybrid Search (new)
	ListNodesWithEmbeddings() ([]memory.Node, error)
	SearchFTS(query string, limit int) ([]memory.FTSResult, error)
}

// Service provides high-level knowledge operations
type Service struct {
	repo             Repository
	llmCfg           llm.Config
	chatModelFactory func(ctx context.Context, cfg llm.Config) (model.BaseChatModel, error)
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
		chatModelFactory: llm.NewChatModel,
	}
}

// ScoredNode represents a search result with visual relevance score
type ScoredNode struct {
	Node  *memory.Node `json:"node"`
	Score float32      `json:"score"`
}

// Search performs a hybrid search combining FTS5 keyword matching and vector similarity.
// This fixes the N+1 query pattern and provides keyword fallback when embeddings fail.
// Weights and thresholds are defined in config.go for centralized tuning.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]ScoredNode, error) {
	if limit <= 0 {
		limit = 5
	}

	// Collect results from both search methods
	scoreByID := make(map[string]float32)
	nodeByID := make(map[string]*memory.Node)

	// 1. FTS5 keyword search (fast, no API call, always works)
	ftsResults, err := s.repo.SearchFTS(query, limit*2)
	if err != nil {
		// FTS5 errors are logged but don't fail the search
		// FTS5 may be unavailable on some systems (missing extension)
		_ = err
	}
	for _, r := range ftsResults {
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

	// 4. Limit results
	if len(scored) > limit {
		scored = scored[:limit]
	}

	return scored, nil
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
