package memory

import (
	"fmt"
)

// GenerateArchitectureMD creates a comprehensive ARCHITECTURE.md file
// that consolidates all project knowledge into a single document.
func (r *Repository) GenerateArchitectureMD(projectName string) error {
	// Gather all features
	featureSummaries, err := r.db.ListFeatures()
	if err != nil {
		return fmt.Errorf("list features: %w", err)
	}

	features := make([]Feature, 0, len(featureSummaries))
	decisions := make(map[string][]Decision)

	for _, fs := range featureSummaries {
		f, err := r.db.GetFeature(fs.ID)
		if err != nil {
			continue
		}
		features = append(features, *f)

		// Get decisions for this feature
		decs, err := r.db.GetDecisions(fs.ID)
		if err == nil && len(decs) > 0 {
			decisions[fs.ID] = decs
		}
	}

	// Gather all nodes by type
	allNodes, err := r.db.ListNodes("")
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	var patterns, constraints []Node
	for _, n := range allNodes {
		switch n.Type {
		case NodeTypeConstraint:
			constraints = append(constraints, n)
		case NodeTypePattern:
			patterns = append(patterns, n)
		}
	}

	data := ArchitectureData{
		Features:    features,
		Decisions:   decisions,
		Patterns:    patterns,
		Constraints: constraints,
		AllNodes:    allNodes,
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
