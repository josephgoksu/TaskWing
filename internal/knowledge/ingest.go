package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/verification"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// IngestFindings processes a list of agent findings and saves them to the repository.
// For incremental updates, provide filePaths to selectively purge/update nodes.
// If filePaths is nil or empty, it assumes a full update for the agent(s) involved.
func (s *Service) IngestFindings(ctx context.Context, findings []core.Finding, filePaths []string, verbose bool) error {
	return s.IngestFindingsWithRelationships(ctx, findings, nil, filePaths, verbose)
}

// IngestFindingsWithRelationships processes findings and LLM-extracted relationships
func (s *Service) IngestFindingsWithRelationships(ctx context.Context, findings []core.Finding, relationships []core.Relationship, filePaths []string, verbose bool) error {
	if len(findings) == 0 {
		return nil
	}

	if verbose {
		fmt.Println("  Ingesting findings...")
	}

	// 0. Verify Findings (if basePath is set)
	verifiedCount, rejectedCount := 0, 0
	if s.basePath != "" {
		if verbose {
			fmt.Print("  Verifying evidence")
		}
		findings, verifiedCount, rejectedCount = s.verifyFindings(ctx, findings, verbose)
		if verbose {
			fmt.Printf(" done (%d verified, %d rejected)\n", verifiedCount, rejectedCount)
		}
	}

	// 1. Purge Stale Data
	if err := s.purgeStaleData(findings, filePaths, verbose); err != nil {
		return err
	}

	// 2. Ingest Nodes (Documents)
	nodesCreated, skippedDuplicates, nodesByTitle, err := s.ingestNodesWithIndex(ctx, findings, verbose)
	if err != nil {
		return err
	}

	// 3. Ingest Structured Data (Features, Decisions, Patterns, Constraints)
	featuresCreated, decisionsCreated, patternsCreated, constraintsFound, err := s.ingestStructuredData(findings)
	if err != nil {
		return err
	}

	// 4. Link Knowledge Graph (evidence-based + semantic)
	evidenceEdges, semanticEdges, err := s.linkKnowledgeGraph(verbose)
	if err != nil {
		return err
	}

	// 5. Process LLM-extracted relationships
	llmEdges := s.linkByLLMRelationships(relationships, nodesByTitle)

	totalEdges := evidenceEdges + semanticEdges + llmEdges

	if verbose {
		fmt.Println(" done")
		if rejectedCount > 0 {
			fmt.Printf("\n⚠️  Rejected %d findings with unverifiable evidence.\n", rejectedCount)
		}
		fmt.Printf("\n✅ Saved %d knowledge nodes (%d duplicates skipped), %d features, %d patterns, %d decisions, %d constraints, %d edges (%d evidence, %d semantic, %d llm) to memory.\n",
			nodesCreated, skippedDuplicates, featuresCreated, patternsCreated, decisionsCreated, constraintsFound, totalEdges, evidenceEdges, semanticEdges, llmEdges)
	}

	return nil
}

// verifyFindings runs the VerificationAgent on findings and filters out rejected ones.
// Returns the filtered findings and counts of verified/rejected.
func (s *Service) verifyFindings(ctx context.Context, findings []core.Finding, verbose bool) ([]core.Finding, int, int) {
	verifier := verification.NewAgent(s.basePath)

	verified := verifier.VerifyFindings(ctx, findings)

	// Filter out rejected findings
	filtered := verification.FilterVerifiedFindings(verified)

	// Count results
	verifiedCount := 0
	rejectedCount := 0
	for _, f := range verified {
		switch f.VerificationStatus {
		case core.VerificationStatusVerified:
			verifiedCount++
			if verbose {
				fmt.Print("✓")
			}
		case core.VerificationStatusPartial:
			verifiedCount++ // Partial counts as verified (kept)
			if verbose {
				fmt.Print("~")
			}
		case core.VerificationStatusRejected:
			rejectedCount++
			if verbose {
				fmt.Print("✗")
			}
		default:
			if verbose {
				fmt.Print(".")
			}
		}
	}

	return filtered, verifiedCount, rejectedCount
}

// purgeStaleData removes nodes from agents involved in the current finding set.
// If filePaths is provided, only nodes referencing those files are purged (incremental).
// Otherwise, all nodes for the agent are purged (full update).
func (s *Service) purgeStaleData(findings []core.Finding, filePaths []string, verbose bool) error {
	seenAgents := make(map[string]bool)
	for _, f := range findings {
		if f.SourceAgent != "" && !seenAgents[f.SourceAgent] {
			seenAgents[f.SourceAgent] = true

			// Incremental Purge
			if len(filePaths) > 0 {
				if verbose {
					fmt.Printf("  ♻️  Purging stale nodes for agent %s (files: %d)\n", f.SourceAgent, len(filePaths))
				}
				if err := s.repo.DeleteNodesByFiles(f.SourceAgent, filePaths); err != nil {
					return fmt.Errorf("purge files for agent %s: %w", f.SourceAgent, err)
				}
				continue
			}

			// Full Purge
			if verbose {
				fmt.Printf("  ♻️  Purging all stale nodes for agent: %s\n", f.SourceAgent)
			}
			if err := s.repo.DeleteNodesByAgent(f.SourceAgent); err != nil {
				return fmt.Errorf("purge agent %s: %w", f.SourceAgent, err)
			}
		}
	}
	return nil
}

// ingestNodesWithIndex creates document nodes and returns a title->nodeID index for LLM relationship linking
func (s *Service) ingestNodesWithIndex(ctx context.Context, findings []core.Finding, verbose bool) (int, int, map[string]string, error) {
	if verbose {
		fmt.Print("  Generating embeddings")
	}

	nodesCreated := 0
	skippedDuplicates := 0
	nodesByTitle := make(map[string]string) // title -> nodeID

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
		// Also index existing nodes by title for relationship linking
		nodesByTitle[strings.ToLower(n.Summary)] = n.ID
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

		nodeID := uuid.New().String()
		node := memory.Node{
			ID:          nodeID,
			Type:        string(f.Type),
			Summary:     f.Title,
			Content:     content,
			SourceAgent: f.SourceAgent,
			CreatedAt:   time.Now().UTC(),
		}

		// Store verification status
		if f.VerificationStatus != "" {
			node.VerificationStatus = string(f.VerificationStatus)
		} else {
			node.VerificationStatus = string(core.VerificationStatusPending)
		}

		// Store confidence score
		node.ConfidenceScore = f.ConfidenceScore
		if node.ConfidenceScore == 0 {
			node.ConfidenceScore = 0.5 // Default
		}

		// Serialize evidence to JSON
		if len(f.Evidence) > 0 {
			if evidenceJSON, err := json.Marshal(f.Evidence); err == nil {
				node.Evidence = string(evidenceJSON)
			}
		}

		// Serialize verification result to JSON
		if f.VerificationResult != nil {
			if resultJSON, err := json.Marshal(f.VerificationResult); err == nil {
				node.VerificationResult = string(resultJSON)
			}
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
			nodesByTitle[strings.ToLower(f.Title)] = nodeID
		}
	}
	return nodesCreated, skippedDuplicates, nodesByTitle, nil
}

// ingestStructuredData processes Features, Decisions, Patterns, and Constraints
func (s *Service) ingestStructuredData(findings []core.Finding) (int, int, int, int, error) {
	featuresCreated := 0
	decisionsCreated := 0
	patternsCreated := 0
	constraintsFound := 0
	featureIDByName := make(map[string]string)

	// Load existing features
	if existing, err := s.repo.ListFeatures(); err == nil {
		for _, f := range existing {
			featureIDByName[strings.ToLower(f.Name)] = f.ID
		}
	}

	for _, f := range findings {
		switch f.Type {
		case core.FindingTypeFeature:
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

		case core.FindingTypePattern:
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

		case core.FindingTypeDecision:
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

		case core.FindingTypeConstraint:
			// Constraints are stored as nodes (already done in ingestNodesWithIndex)
			// We just count them here for reporting
			if strings.TrimSpace(f.Title) != "" {
				constraintsFound++
			}
		}
	}

	return featuresCreated, decisionsCreated, patternsCreated, constraintsFound, nil
}

// linkKnowledgeGraph creates meaningful edges based on:
// 1. Shared evidence (nodes referencing the same files)
// 2. Semantic similarity (embedding-based)
// Returns (evidenceEdges, semanticEdges, error)
func (s *Service) linkKnowledgeGraph(verbose bool) (int, int, error) {
	if verbose {
		fmt.Print("  Linking knowledge graph")
	}

	allNodes, err := s.repo.ListNodes("")
	if err != nil {
		return 0, 0, err
	}

	// Phase 1: Evidence-based linking (shared file references)
	evidenceEdges := s.linkByEvidence(allNodes)

	// Phase 2: Semantic similarity-based edges
	semanticEdges := s.linkSemantic(allNodes)

	return evidenceEdges, semanticEdges, nil
}

// linkByEvidence creates edges between nodes that reference the same files.
// This creates meaningful relationships based on actual code context.
// Note: allNodes must include Evidence fields (use ListNodes which now includes them).
func (s *Service) linkByEvidence(allNodes []memory.Node) int {
	count := 0

	// Build map: file path -> list of node IDs that reference it
	fileToNodes := make(map[string][]string)
	nodeEvidence := make(map[string][]string) // nodeID -> file paths

	// Nodes already have Evidence populated from ListNodes() - no N+1 refetch needed
	for _, n := range allNodes {
		if n.Evidence == "" {
			continue
		}

		// Parse evidence JSON
		var evidenceList []struct {
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal([]byte(n.Evidence), &evidenceList); err != nil {
			continue
		}

		// Track which files this node references
		seenFiles := make(map[string]bool)
		for _, ev := range evidenceList {
			if ev.FilePath == "" || seenFiles[ev.FilePath] {
				continue
			}
			seenFiles[ev.FilePath] = true
			fileToNodes[ev.FilePath] = append(fileToNodes[ev.FilePath], n.ID)
			nodeEvidence[n.ID] = append(nodeEvidence[n.ID], ev.FilePath)
		}
	}

	// Create edges between nodes that share file references
	// Only link if they share at least one file
	linkedPairs := make(map[string]bool) // "nodeA:nodeB" to avoid duplicates

	for filePath, nodeIDs := range fileToNodes {
		if len(nodeIDs) < 2 {
			continue
		}

		// Link all pairs of nodes that share this file
		for i := 0; i < len(nodeIDs); i++ {
			for j := i + 1; j < len(nodeIDs); j++ {
				nodeA, nodeB := nodeIDs[i], nodeIDs[j]
				pairKey := nodeA + ":" + nodeB
				if nodeA > nodeB {
					pairKey = nodeB + ":" + nodeA
				}

				if linkedPairs[pairKey] {
					continue
				}
				linkedPairs[pairKey] = true

				// Calculate weight based on number of shared files
				sharedFiles := countSharedFiles(nodeEvidence[nodeA], nodeEvidence[nodeB])
				weight := EdgeWeightRelatesTo
				if sharedFiles >= 2 {
					weight = EdgeWeightDependsOn
				}

				props := map[string]any{
					"shared_file":  filePath,
					"shared_count": sharedFiles,
				}
				if err := s.repo.LinkNodes(nodeA, nodeB, memory.NodeRelationSharesEvidence, weight, props); err == nil {
					count++
				}
			}
		}
	}

	return count
}

// countSharedFiles returns how many files two nodes share in common.
func countSharedFiles(filesA, filesB []string) int {
	setA := make(map[string]bool)
	for _, f := range filesA {
		setA[f] = true
	}
	count := 0
	for _, f := range filesB {
		if setA[f] {
			count++
		}
	}
	return count
}

func (s *Service) linkSemantic(allNodes []memory.Node) int {
	count := 0
	threshold := SemanticSimilarityThreshold

	// Fetch all nodes with embeddings in a single query (no N+1)
	nodesWithEmbeddings, err := s.repo.ListNodesWithEmbeddings()
	if err != nil {
		return 0
	}

	// Compare pairs
	// Note: O(N^2) - suitable for < 2000 nodes. For larger sets, use vector index.
	for i := 0; i < len(nodesWithEmbeddings); i++ {
		for j := i + 1; j < len(nodesWithEmbeddings); j++ {
			nodeA := nodesWithEmbeddings[i]
			nodeB := nodesWithEmbeddings[j]

			// Allow same-agent comparisons for semantic similarity
			// (nodes from same agent can still be semantically related)

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

// linkByLLMRelationships creates edges from LLM-extracted relationships.
// The LLM explicitly identifies relationships during analysis (Phase 3).
func (s *Service) linkByLLMRelationships(relationships []core.Relationship, nodesByTitle map[string]string) int {
	if len(relationships) == 0 {
		return 0
	}

	count := 0
	for _, rel := range relationships {
		// Look up node IDs by title (case-insensitive)
		fromID := nodesByTitle[strings.ToLower(rel.From)]
		toID := nodesByTitle[strings.ToLower(rel.To)]

		if fromID == "" || toID == "" {
			// Try partial matching if exact match fails
			fromID = findNodeByPartialTitle(nodesByTitle, rel.From)
			toID = findNodeByPartialTitle(nodesByTitle, rel.To)
		}

		if fromID == "" || toID == "" || fromID == toID {
			continue
		}

		// Map relation type
		relationType := memory.NodeRelationRelatesTo
		weight := EdgeWeightRelatesTo
		switch rel.Relation {
		case "depends_on":
			relationType = memory.NodeRelationDependsOn
			weight = EdgeWeightDependsOn
		case "affects":
			relationType = memory.NodeRelationAffects
			weight = EdgeWeightDependsOn
		case "extends":
			relationType = memory.NodeRelationExtends
			weight = EdgeWeightDependsOn
		}

		props := map[string]any{
			"llm_extracted": true,
			"reason":        rel.Reason,
		}

		if err := s.repo.LinkNodes(fromID, toID, relationType, weight, props); err == nil {
			count++
		}
	}

	return count
}

// findNodeByPartialTitle finds a node ID using multiple matching strategies:
// 1. Exact substring match
// 2. Word-based similarity (Jaccard on word tokens)
func findNodeByPartialTitle(nodesByTitle map[string]string, search string) string {
	searchLower := strings.ToLower(search)

	// Strategy 1: Substring matching (original behavior)
	for title, id := range nodesByTitle {
		if strings.Contains(title, searchLower) || strings.Contains(searchLower, title) {
			return id
		}
	}

	// Strategy 2: Word-based similarity matching
	// This catches cases where LLM uses different phrasing (e.g., "JWT Auth" vs "JWT Authentication")
	searchWords := wordTokens(searchLower)
	if len(searchWords) == 0 {
		return ""
	}

	bestMatch := ""
	bestScore := 0.0
	threshold := 0.4 // Require at least 40% word overlap

	for title, id := range nodesByTitle {
		titleWords := wordTokens(title)
		if len(titleWords) == 0 {
			continue
		}

		// Calculate Jaccard similarity on word tokens
		intersection := 0
		for w := range searchWords {
			if titleWords[w] {
				intersection++
			}
		}
		union := len(searchWords) + len(titleWords) - intersection
		if union == 0 {
			continue
		}
		similarity := float64(intersection) / float64(union)

		if similarity >= threshold && similarity > bestScore {
			bestScore = similarity
			bestMatch = id
		}
	}

	return bestMatch
}

// stopWordsIngest is a package-level set for efficient word filtering
var stopWordsIngest = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
	"use": true, "using": true, "based": true, "via": true,
}

// wordTokens extracts significant words from a string for matching
func wordTokens(s string) map[string]bool {
	tokens := make(map[string]bool)
	// Replace common separators with spaces
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "/", " ")

	words := strings.Fields(s)
	for _, w := range words {
		w = strings.ToLower(w)
		if len(w) > 2 && !stopWordsIngest[w] {
			tokens[w] = true
		}
	}
	return tokens
}
