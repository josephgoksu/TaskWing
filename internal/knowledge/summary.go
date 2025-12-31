package knowledge

import (
	"context"
)

// GetProjectSummary returns a high-level overview of the project memory.
// This centralizes summary logic so CLI and MCP usage remains consistent.
func (s *Service) GetProjectSummary(ctx context.Context) (ProjectSummary, error) {
	// Node-based system only
	nodes, err := s.repo.ListNodes("")
	if err != nil {
		return ProjectSummary{}, err
	}

	byType := make(map[string][]string) // type -> summaries
	for _, n := range nodes {
		t := n.Type
		if t == "" {
			t = "unknown"
		}
		byType[t] = append(byType[t], n.Summary)
	}

	// Build compact summary with top 3 examples per type
	typeSummaries := make(map[string]TypeSummary)
	for t, summaries := range byType {
		examples := summaries
		if len(examples) > 3 {
			examples = examples[:3]
		}
		var nonEmpty []string
		for _, e := range examples {
			if e != "" {
				nonEmpty = append(nonEmpty, e)
			}
		}
		typeSummaries[t] = TypeSummary{
			Count:    len(summaries),
			Examples: nonEmpty,
		}
	}

	return ProjectSummary{
		Total: len(nodes),
		Types: typeSummaries,
	}, nil
}
