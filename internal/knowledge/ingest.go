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

	if verbose {
		fmt.Println("  Ingesting findings...")
	}

	// 1. Purge Stale Data
	if err := s.purgeStaleData(findings, verbose); err != nil {
		return err
	}

	// 2. Ingest Nodes (Documents)
	nodesCreated, skippedDuplicates, err := s.ingestNodes(ctx, findings, verbose)
	if err != nil {
		return err
	}

	// 3. Ingest Structured Data (Features, Decisions, Patterns)
	featuresCreated, decisionsCreated, patternsCreated, err := s.ingestStructuredData(findings)
	if err != nil {
		return err
	}

	// 4. Link Knowledge Graph
	edgesCreated, semanticEdges, err := s.linkKnowledgeGraph(verbose)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Println(" done")
		fmt.Printf("\n✅ Saved %d knowledge nodes (%d duplicates skipped), %d features, %d patterns, %d decisions, %d edges (%d semantic) to memory.\n",
			nodesCreated, skippedDuplicates, featuresCreated, patternsCreated, decisionsCreated, edgesCreated, semanticEdges)
	}

	return nil
}

// purgeStaleData removes nodes from agents involved in the current finding set
func (s *Service) purgeStaleData(findings []agents.Finding, verbose bool) error {
	seenAgents := make(map[string]bool)
	for _, f := range findings {
		if f.SourceAgent != "" && !seenAgents[f.SourceAgent] {
			if verbose {
				fmt.Printf("  ♻️  Purging stale nodes for agent: %s\n", f.SourceAgent)
			}
			if err := s.repo.DeleteNodesByAgent(f.SourceAgent); err != nil {
				return fmt.Errorf("purge agent %s: %w", f.SourceAgent, err)
			}
			seenAgents[f.SourceAgent] = true
		}
	}
	return nil
}

// ingestNodes creates document nodes from findings
func (s *Service) ingestNodes(ctx context.Context, findings []agents.Finding, verbose bool) (int, int, error) {
	if verbose {
		fmt.Print("  Generating embeddings")
	}

	nodesCreated := 0
	skippedDuplicates := 0

	// Deduplication index
	existingNodes, _ := s.repo.ListNodesWithEmbeddings()
	existingByContent := make(map[string]bool)
	dedupKey := func(content string) string {
		if len(content) > 200 {
			return content[:200]
		}
		return content
	}
	for _, n := range existingNodes {
		existingByContent[dedupKey(n.Content)] = true
	}

	for _, f := range findings {
		content := f.Title + "\n" + f.Description
		if f.Why != "" {
			content += "\n\nWhy: " + f.Why
		}
		if f.Tradeoffs != "" {
			content += "\nTradeoffs: " + f.Tradeoffs
		}

		// Deduplication
		key := dedupKey(content)
		if existingByContent[key] {
			skippedDuplicates++
			continue
		}
		existingByContent[key] = true

		node := memory.Node{
			ID:          uuid.New().String(),
			Type:        string(f.Type),
			Summary:     f.Title,
			Content:     content,
			SourceAgent: f.SourceAgent,
			CreatedAt:   time.Now().UTC(),
		}

		// Generate embedding
		if s.llmCfg.APIKey != "" {
			if embedding, err := GenerateEmbedding(ctx, content, s.llmCfg); err == nil {
				node.Embedding = embedding
				if verbose {
					fmt.Print(".")
				}
			}
		}

		if err := s.repo.UpsertNodeBySummary(node); err == nil {
			nodesCreated++
		}
	}
	return nodesCreated, skippedDuplicates, nil
}

// ingestStructuredData processes Features, Decisions, and Patterns
func (s *Service) ingestStructuredData(findings []agents.Finding) (int, int, int, error) {
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

	for _, f := range findings {
		switch f.Type {
		case agents.FindingTypeFeature:
			name := strings.TrimSpace(f.Title)
			if name == "" {
				continue
			}
			key := strings.ToLower(name)
			if _, exists := featureIDByName[key]; exists {
				continue
			}

			newID := "f-" + uuid.New().String()[:8]
			if err := s.repo.CreateFeature(memory.Feature{
				ID:        newID,
				Name:      name,
				OneLiner:  f.Description,
				Status:    memory.FeatureStatusActive,
				CreatedAt: time.Now(),
			}); err == nil {
				featureIDByName[key] = newID
				featuresCreated++
			}

		case agents.FindingTypePattern:
			name := strings.TrimSpace(f.Title)
			if name == "" {
				continue
			}
			context, _ := f.Metadata["context"].(string)
			solution, _ := f.Metadata["solution"].(string)
			consequences, _ := f.Metadata["consequences"].(string)

			if err := s.repo.CreatePattern(memory.Pattern{
				Name:         name,
				Context:      context,
				Solution:     solution,
				Consequences: consequences,
			}); err == nil {
				patternsCreated++
			}

		case agents.FindingTypeDecision:
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
	}

	return featuresCreated, decisionsCreated, patternsCreated, nil
}

// linkKnowledgeGraph orchestrates all edge creation phases
func (s *Service) linkKnowledgeGraph(verbose bool) (int, int, error) {
	if verbose {
		fmt.Print("  Linking knowledge graph")
	}

	allNodes, err := s.repo.ListNodes("")
	if err != nil {
		return 0, 0, err
	}

	// Build indexes
	nodesByAgent := make(map[string][]string)
	nodesByType := make(map[string][]string)
	for _, n := range allNodes {
		if n.SourceAgent != "" {
			nodesByAgent[n.SourceAgent] = append(nodesByAgent[n.SourceAgent], n.ID)
		}
		if n.Type != "" {
			nodesByType[n.Type] = append(nodesByType[n.Type], n.ID)
		}
	}

	// Phase 1: Co-occurrence
	edgesCreated := s.linkCoOccurrence(nodesByAgent)

	// Phase 2: Structural (Type-based)
	structuralEdges := s.linkStructural(nodesByType)
	edgesCreated += structuralEdges

	// Phase 3: Semantic Similarity
	semanticEdges := s.linkSemantic(allNodes)

	return edgesCreated + semanticEdges, semanticEdges, nil
}

func (s *Service) linkCoOccurrence(nodesByAgent map[string][]string) int {
	count := 0
	for _, nodeIDs := range nodesByAgent {
		if len(nodeIDs) < 2 {
			continue
		}
		maxEdges := 10
		limit := len(nodeIDs) * maxEdges // Safety cap
		if limit > 200 {
			limit = 200
		}

		created := 0
		for i := 0; i < len(nodeIDs) && created < limit; i++ {
			for j := i + 1; j < len(nodeIDs) && j < i+3; j++ {
				if err := s.repo.LinkNodes(nodeIDs[i], nodeIDs[j], memory.NodeRelationRelatesTo, EdgeWeightRelatesTo, nil); err == nil {
					count++
					created++
				}
			}
		}
	}
	return count
}

func (s *Service) linkStructural(nodesByType map[string][]string) int {
	count := 0
	features := nodesByType["feature"]
	decisions := nodesByType["decision"]
	patterns := nodesByType["pattern"]

	// Link decisions to features
	if len(features) > 0 && len(decisions) > 0 {
		for i, decisionID := range decisions {
			featureIdx := i % len(features)
			if err := s.repo.LinkNodes(decisionID, features[featureIdx], memory.NodeRelationAffects, EdgeWeightAffects, nil); err == nil {
				count++
			}
			// Secondary link
			if len(features) > 1 {
				idx2 := (i + 1) % len(features)
				if idx2 != featureIdx {
					s.repo.LinkNodes(decisionID, features[idx2], memory.NodeRelationRelatesTo, EdgeWeightRelatesTo, nil)
				}
			}
		}
	}

	// Link patterns to features and decisions
	if len(patterns) > 0 {
		for i, patternID := range patterns {
			if len(features) > 0 {
				idx := i % len(features)
				if err := s.repo.LinkNodes(patternID, features[idx], memory.NodeRelationExtends, EdgeWeightExtends, nil); err == nil {
					count++
				}
			}
			if len(decisions) > 0 {
				idx := i % len(decisions)
				if err := s.repo.LinkNodes(patternID, decisions[idx], memory.NodeRelationRelatesTo, EdgeWeightRelatesTo, nil); err == nil {
					count++
				}
			}
		}
	}
	return count
}

func (s *Service) linkSemantic(allNodes []memory.Node) int {
	count := 0
	threshold := SemanticSimilarityThreshold

	// Hydrate with embeddings
	nodesWithEmbeddings := make([]memory.Node, 0, len(allNodes))
	for _, n := range allNodes {
		if full, err := s.repo.GetNode(n.ID); err == nil && len(full.Embedding) > 0 {
			nodesWithEmbeddings = append(nodesWithEmbeddings, *full)
		}
	}

	// Compare pairs
	// Note: O(N^2) - suitable for < 2000 nodes. For larger sets, use vector index.
	for i := 0; i < len(nodesWithEmbeddings); i++ {
		for j := i + 1; j < len(nodesWithEmbeddings); j++ {
			nodeA := nodesWithEmbeddings[i]
			nodeB := nodesWithEmbeddings[j]

			// Skip if same agent (covered by co-occurrence)
			if nodeA.SourceAgent == nodeB.SourceAgent {
				continue
			}

			similarity := CosineSimilarity(nodeA.Embedding, nodeB.Embedding)
			if similarity >= float32(threshold) {
				props := map[string]any{"similarity": similarity}
				if err := s.repo.LinkNodes(nodeA.ID, nodeB.ID, memory.NodeRelationSemanticallySimilar, float64(similarity), props); err == nil {
					count++
				}
			}
		}
	}
	return count
}
