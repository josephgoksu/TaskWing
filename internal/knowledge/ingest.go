package knowledge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// IngestFindings processes a list of agent findings and saves them to the repository
func (s *Service) IngestFindings(ctx context.Context, findings []agents.Finding, verbose bool) error {
	if len(findings) == 0 {
		return nil
	}

	// 1. Generate Embeddings for all findings
	if verbose {
		fmt.Print("  Generating embeddings")
	}

	// Helper for progress
	progress := func() {
		if verbose {
			fmt.Print(".")
		}
	}

	// s.repo already implements all needed methods via the Repository interface

	// 0. Purge stale nodes for involved agents (Replace Strategy)
	// This prevents infinite growth due to non-deterministic LLM outputs (different summaries)
	seenAgents := make(map[string]bool)
	for _, f := range findings {
		if f.SourceAgent != "" && !seenAgents[f.SourceAgent] {
			if verbose {
				fmt.Printf("  ♻️  Purging stale nodes for agent: %s\n", f.SourceAgent)
			}
			_ = s.repo.DeleteNodesByAgent(f.SourceAgent)
			seenAgents[f.SourceAgent] = true
		}
	}

	nodesCreated := 0

	for _, f := range findings {
		content := f.Title + "\n" + f.Description
		if f.Why != "" {
			content += "\n\nWhy: " + f.Why
		}
		if f.Tradeoffs != "" {
			content += "\nTradeoffs: " + f.Tradeoffs
		}

		node := memory.Node{
			ID:          uuid.New().String(),
			Type:        string(f.Type),
			Summary:     f.Title,
			Content:     content, // Use already-built content for DRY
			SourceAgent: f.SourceAgent,
			CreatedAt:   time.Now().UTC(),
		}

		// Generate embedding
		if s.llmCfg.APIKey != "" {
			if embedding, err := GenerateEmbedding(ctx, content, s.llmCfg); err == nil {
				node.Embedding = embedding
				progress()
			}
		}

		// Use Upsert to prevent duplication
		if err := s.repo.UpsertNodeBySummary(node); err == nil {
			nodesCreated++
		}
	}

	if verbose {
		fmt.Println(" done")
		fmt.Print("  Saving structured data")
	}

	// 2. Structured Data Ingestion (Features, Decisions, Patterns)
	featuresCreated := 0
	decisionsCreated := 0
	patternsCreated := 0

	featureIDByName := make(map[string]string)

	// Load existing features
	if existing, err := s.repo.ListFeatures(); err == nil {
		for _, f := range existing {
			featureIDByName[strings.ToLower(f.Name)] = f.ID
		}
	}

	// Process Features
	for _, f := range findings {
		if f.Type != agents.FindingTypeFeature {
			continue
		}
		name := strings.TrimSpace(f.Title)
		if name == "" {
			continue
		}

		key := strings.ToLower(name)
		if _, exists := featureIDByName[key]; exists {
			continue
		}

		newID := "f-" + uuid.New().String()[:8]
		err := s.repo.CreateFeature(memory.Feature{
			ID:        newID,
			Name:      name,
			OneLiner:  f.Description, // Truncate if needed in repo
			Status:    memory.FeatureStatusActive,
			CreatedAt: time.Now(),
		})
		if err == nil {
			featureIDByName[key] = newID
			featuresCreated++
		}
	}

	// Process Patterns
	for _, f := range findings {
		if f.Type != agents.FindingTypePattern {
			continue
		}
		name := strings.TrimSpace(f.Title)
		if name == "" {
			continue
		}

		context, _ := f.Metadata["context"].(string)
		solution, _ := f.Metadata["solution"].(string)
		consequences, _ := f.Metadata["consequences"].(string)

		err := s.repo.CreatePattern(memory.Pattern{
			Name:         name,
			Context:      context,
			Solution:     solution,
			Consequences: consequences,
		})
		if err == nil {
			patternsCreated++
		}
	}

	// Process Decisions
	for _, f := range findings {
		if f.Type != agents.FindingTypeDecision {
			continue
		}
		title := strings.TrimSpace(f.Title)
		if title == "" {
			continue
		}

		// Component inference
		compName, _ := f.Metadata["component"].(string)
		if compName == "" {
			switch f.SourceAgent {
			case "git":
				compName = "Project Evolution"
			case "deps":
				compName = "Technology Stack"
			default:
				compName = "Core Architecture"
			}
		}

		// Ensure feature exists
		featKey := strings.ToLower(compName)
		featID := featureIDByName[featKey]
		if featID == "" {
			featID = "f-" + uuid.New().String()[:8]
			if err := s.repo.CreateFeature(memory.Feature{
				ID:       featID,
				Name:     compName,
				OneLiner: "Auto-detected component",
				Status:   memory.FeatureStatusActive,
			}); err == nil {
				featureIDByName[featKey] = featID
				featuresCreated++
			}
		}

		if featID != "" {
			if err := s.repo.AddDecision(featID, memory.Decision{
				Title:     title,
				Summary:   f.Description,
				Reasoning: f.Why,
				Tradeoffs: f.Tradeoffs,
			}); err == nil {
				decisionsCreated++
			}
		}
	}

	// 3. Create Knowledge Graph Edges
	// Link nodes that share the same component/agent (Phase 1: Co-occurrence)
	edgesCreated := 0
	nodesByAgent := make(map[string][]string) // agent -> node IDs

	// Build index of nodes by source agent
	allNodes, _ := s.repo.ListNodes("")
	for _, n := range allNodes {
		if n.SourceAgent != "" {
			nodesByAgent[n.SourceAgent] = append(nodesByAgent[n.SourceAgent], n.ID)
		}
	}

	// Create relates_to edges between nodes from the same agent
	for _, nodeIDs := range nodesByAgent {
		if len(nodeIDs) < 2 {
			continue
		}
		// Link each pair (limit to avoid N^2 explosion)
		maxEdges := 10
		for i := 0; i < len(nodeIDs) && edgesCreated < maxEdges*len(nodesByAgent); i++ {
			for j := i + 1; j < len(nodeIDs) && j < i+3; j++ { // Connect up to 2 neighbors
				if err := s.repo.LinkNodes(nodeIDs[i], nodeIDs[j], memory.NodeRelationRelatesTo, 0.7, nil); err == nil {
					edgesCreated++
				}
			}
		}
	}

	// 4. Semantic Similarity Edges (Phase 2)
	// Compare embeddings across ALL nodes and link if similarity > threshold
	semanticEdges := 0
	const similarityThreshold = 0.75

	// Get full nodes with embeddings
	nodesWithEmbeddings := make([]memory.Node, 0)
	for _, n := range allNodes {
		fullNode, err := s.repo.GetNode(n.ID)
		if err == nil && len(fullNode.Embedding) > 0 {
			nodesWithEmbeddings = append(nodesWithEmbeddings, *fullNode)
		}
	}

	// Compare all pairs (O(n^2) but limited by embedding count)
	// TODO: Use LLM inference for higher quality relationships (future enhancement)
	for i := 0; i < len(nodesWithEmbeddings); i++ {
		for j := i + 1; j < len(nodesWithEmbeddings); j++ {
			nodeA := nodesWithEmbeddings[i]
			nodeB := nodesWithEmbeddings[j]

			// Skip if same agent (already linked by co-occurrence)
			if nodeA.SourceAgent == nodeB.SourceAgent {
				continue
			}

			similarity := CosineSimilarity(nodeA.Embedding, nodeB.Embedding)
			if similarity >= similarityThreshold {
				props := map[string]any{"similarity": similarity}
				if err := s.repo.LinkNodes(nodeA.ID, nodeB.ID, memory.NodeRelationSemanticallySimilar, float64(similarity), props); err == nil {
					semanticEdges++
				}
			}
		}
	}

	totalEdges := edgesCreated + semanticEdges

	if verbose {
		fmt.Println(" done")
		fmt.Printf("\n✅ Saved %d knowledge nodes, %d features, %d patterns, %d decisions, %d edges (%d semantic) to memory.\n",
			nodesCreated, featuresCreated, patternsCreated, decisionsCreated, totalEdges, semanticEdges)
	}

	return nil
}
