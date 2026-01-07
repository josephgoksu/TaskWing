package memory

import (
	"fmt"
)

// CreateFeature saves a feature to both DB and File.
func (r *Repository) CreateFeature(f Feature) error {
	// 1. Save to DB (Primary)
	if err := r.db.CreateFeature(f); err != nil {
		return fmt.Errorf("db create: %w", err)
	}

	// 2. Fetch decisions for file generation
	decisions, _ := r.db.GetDecisions(f.ID)

	// 3. Save to File (Secondary)
	if err := r.files.WriteFeature(f, decisions); err != nil {
		// Compensating transaction: undo DB change
		_ = r.db.DeleteFeature(f.ID)
		return fmt.Errorf("file create: %w", err)
	}

	return nil
}

// GetFeature retrieves a feature by ID.
func (r *Repository) GetFeature(id string) (*Feature, error) {
	return r.db.GetFeature(id)
}

// UpdateFeature updates a feature in both DB and File.
func (r *Repository) UpdateFeature(f Feature) error {
	if err := r.db.UpdateFeature(f); err != nil {
		return fmt.Errorf("db update: %w", err)
	}

	// Reload to get latest (including file path if needed) and decisions
	updatedF, err := r.db.GetFeature(f.ID)
	if err != nil {
		return fmt.Errorf("reload feature: %w", err)
	}

	decisions, _ := r.db.GetDecisions(f.ID)

	if err := r.files.WriteFeature(*updatedF, decisions); err != nil {
		return fmt.Errorf("file update: %w", err)
	}

	return nil
}

// DeleteFeature removes a feature from both DB and File.
func (r *Repository) DeleteFeature(id string) error {
	// Get feature first to know file path
	f, err := r.db.GetFeature(id)
	if err != nil {
		return fmt.Errorf("get feature: %w", err)
	}

	// Delete from DB
	if err := r.db.DeleteFeature(id); err != nil {
		return fmt.Errorf("db delete: %w", err)
	}

	// Delete from File
	if f.FilePath != "" {
		_ = r.files.RemoveFile(f.FilePath)
	}

	return nil
}

// ListFeatures returns all features from the DB.
func (r *Repository) ListFeatures() ([]Feature, error) {
	summaries, err := r.db.ListFeatures()
	if err != nil {
		return nil, err
	}
	features := make([]Feature, len(summaries))
	for i, s := range summaries {
		features[i] = Feature{
			ID:   s.ID,
			Name: s.Name,
		}
	}
	return features, nil
}

// RebuildFiles regenerates all markdown files from the database.
func (r *Repository) RebuildFiles() error {
	features, err := r.db.ListFeatures()
	if err != nil {
		return err
	}

	for _, fs := range features {
		f, err := r.db.GetFeature(fs.ID)
		if err != nil {
			continue
		}
		decisions, _ := r.db.GetDecisions(fs.ID)
		if err := r.files.WriteFeature(*f, decisions); err != nil {
			return fmt.Errorf("write feature %s: %w", f.Name, err)
		}
	}
	return nil
}

// AddDecision adds a decision and updates the feature file.
func (r *Repository) AddDecision(featureID string, d Decision) error {
	if err := r.db.AddDecision(featureID, d); err != nil {
		return err
	}

	// Refresh feature file
	f, err := r.db.GetFeature(featureID)
	if err != nil {
		return err
	}
	decisions, _ := r.db.GetDecisions(featureID)
	return r.files.WriteFeature(*f, decisions)
}

// UpdateDecision updates a decision and refreshes the feature file.
func (r *Repository) UpdateDecision(d Decision) error {
	if d.FeatureID == "" {
		existing, err := r.db.GetDecision(d.ID)
		if err != nil {
			return err
		}
		d.FeatureID = existing.FeatureID
	}

	if err := r.db.UpdateDecision(d); err != nil {
		return err
	}

	// Refresh feature file
	f, err := r.db.GetFeature(d.FeatureID)
	if err != nil {
		return err
	}
	decisions, _ := r.db.GetDecisions(d.FeatureID)

	return r.files.WriteFeature(*f, decisions)
}

// DeleteDecision deletes a decision and refreshes the feature file.
func (r *Repository) DeleteDecision(id string) error {
	// Get decision first to know FeatureID
	d, err := r.db.GetDecision(id)
	if err != nil {
		return err
	}

	if err := r.db.DeleteDecision(id); err != nil {
		return err
	}

	// Refresh feature file
	f, err := r.db.GetFeature(d.FeatureID)
	if err != nil {
		return err
	}
	decisions, _ := r.db.GetDecisions(d.FeatureID)

	if err := r.files.WriteFeature(*f, decisions); err != nil {
		return fmt.Errorf("write feature: %w", err)
	}

	return nil
}

// GetDecisions returns all decisions for a feature.
func (r *Repository) GetDecisions(featureID string) ([]Decision, error) {
	return r.db.GetDecisions(featureID)
}

// GetIndex returns the feature index.
func (r *Repository) GetIndex() (*FeatureIndex, error) {
	return r.db.GetIndex()
}
