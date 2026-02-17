package memory

import (
	"fmt"
)

// GenerateArchitectureMD creates a comprehensive ARCHITECTURE.md file
// that consolidates all project knowledge into a single document.
// All data is sourced from the nodes table — the single source of truth.
func (r *Repository) GenerateArchitectureMD(projectName string) error {
	allNodes, err := r.db.ListNodes("")
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	var features, decisions, patterns, constraints []Node
	for _, n := range allNodes {
		switch n.Type {
		case NodeTypeFeature:
			features = append(features, n)
		case NodeTypeDecision:
			decisions = append(decisions, n)
		case NodeTypePattern:
			patterns = append(patterns, n)
		case NodeTypeConstraint:
			constraints = append(constraints, n)
		}
	}

	data := ArchitectureData{
		Features:    features,
		Decisions:   decisions,
		Patterns:    patterns,
		Constraints: constraints,
	}

	return r.files.GenerateArchitectureMD(data, projectName)
}

// GetProjectOverview retrieves the project overview from the database.
// Returns nil if no overview exists yet.
func (r *Repository) GetProjectOverview() (*ProjectOverview, error) {
	return r.db.GetProjectOverview()
}

// SaveProjectOverview creates or updates the project overview.
func (r *Repository) SaveProjectOverview(overview *ProjectOverview) error {
	return r.db.SaveProjectOverview(overview)
}
