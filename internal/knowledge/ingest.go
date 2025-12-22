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

	type NodeCreator interface {
		CreateNode(n memory.Node) error
		UpsertNodeBySummary(n memory.Node) error
		DeleteNodesByAgent(agent string) error
		CreateFeature(f memory.Feature) error
		CreatePattern(p memory.Pattern) error
		AddDecision(featureID string, d memory.Decision) error
		ListFeatures() ([]memory.Feature, error)
		GetDecisions(featureID string) ([]memory.Decision, error)
		Link(fromID, toID string, relType string) error
	}

	repo, ok := s.source.(NodeCreator)
	if !ok {
		return fmt.Errorf("storage source does not support ingestion operations")
	}

	// 0. Purge stale nodes for involved agents (Replace Strategy)
	// This prevents infinite growth due to non-deterministic LLM outputs (different summaries)
	seenAgents := make(map[string]bool)
	for _, f := range findings {
		if f.SourceAgent != "" && !seenAgents[f.SourceAgent] {
			if verbose {
				fmt.Printf("  ♻️  Purging stale nodes for agent: %s\n", f.SourceAgent)
			}
			_ = repo.DeleteNodesByAgent(f.SourceAgent)
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
			Content:     f.Description,
			SourceAgent: f.SourceAgent,
			CreatedAt:   time.Now().UTC(),
		}

		// Append extra context
		if f.Why != "" {
			node.Content += "\n\nWhy: " + f.Why
		}
		if f.Tradeoffs != "" {
			node.Content += "\nTradeoffs: " + f.Tradeoffs
		}

		// Generate embedding
		if s.llmCfg.APIKey != "" {
			if embedding, err := GenerateEmbedding(ctx, content, s.llmCfg); err == nil {
				node.Embedding = embedding
				progress()
			}
		}

		// Use Upsert to prevent duplication
		if err := repo.UpsertNodeBySummary(node); err == nil {
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
	if existing, err := repo.ListFeatures(); err == nil {
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
		err := repo.CreateFeature(memory.Feature{
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

		err := repo.CreatePattern(memory.Pattern{
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
			if err := repo.CreateFeature(memory.Feature{
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
			if err := repo.AddDecision(featID, memory.Decision{
				Title:     title,
				Summary:   f.Description,
				Reasoning: f.Why,
				Tradeoffs: f.Tradeoffs,
			}); err == nil {
				decisionsCreated++
			}
		}
	}

	if verbose {
		fmt.Println(" done")
		fmt.Printf("\n✅ Saved %d knowledge nodes, %d features, %d patterns, %d decisions to memory.\n",
			nodesCreated, featuresCreated, patternsCreated, decisionsCreated)
	}

	return nil
}
