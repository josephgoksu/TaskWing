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

// NodeSource abstracts the data source requirement for the knowledge service
type NodeSource interface {
	ListNodes(typeFilter string) ([]memory.Node, error)
	GetNode(id string) (*memory.Node, error)
}

// Service provides high-level knowledge operations
type Service struct {
	source           NodeSource
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
func NewService(source NodeSource, cfg llm.Config) *Service {
	return &Service{
		source:           source,
		llmCfg:           cfg,
		chatModelFactory: llm.NewChatModel,
	}
}

// ScoredNode represents a search result with visual relevance score
type ScoredNode struct {
	Node  *memory.Node `json:"node"`
	Score float32      `json:"score"`
}

// Search performs a semantic search over the knowledge base
func (s *Service) Search(ctx context.Context, query string, limit int) ([]ScoredNode, error) {
	// 1. Generate embedding for query
	queryEmbedding, err := GenerateEmbedding(ctx, query, s.llmCfg)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	// 2. Fetch all nodes (Note: Optimization needed here later - Vector Store)
	nodes, err := s.source.ListNodes("")
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	// 3. Compute similarity
	var scored []ScoredNode
	for _, n := range nodes {
		// N+1 query to get full node with embedding
		// TODO: Optimize this in Phase 2
		fullNode, err := s.source.GetNode(n.ID)
		if err != nil {
			continue // Skip missing nodes
		}
		if len(fullNode.Embedding) == 0 {
			continue // Skip nodes without embeddings
		}

		score := CosineSimilarity(queryEmbedding, fullNode.Embedding)
		scored = append(scored, ScoredNode{Node: fullNode, Score: score})
	}

	// 4. Sort by score
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// 5. Limit results
	if limit > 0 && len(scored) > limit {
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
	// We need to access CreateNode, but NodeSource interface is read-only (List/Get).
	// We need to type assert or expand the interface.
	// For now, let's assume the source is a Repository (which it is in implementation).

	// Better architecture: KnowledgeService should depend on Repository interface, not just NodeSource.
	// But to avoid breaking changes to `NewService` signature right now, we can check.
	type NodeCreator interface {
		CreateNode(n memory.Node) error
	}

	if creator, ok := s.source.(NodeCreator); ok {
		if err := creator.CreateNode(*node); err != nil {
			return nil, fmt.Errorf("save node: %w", err)
		}
	} else {
		return nil, fmt.Errorf("storage source does not support creating nodes")
	}

	return node, nil
}
