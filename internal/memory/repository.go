package memory

import (
	"fmt"
)

// Repository orchestrates access to both the database and the filesystem.
// It ensures that data is synchronized between the two stores.
type Repository struct {
	db    *SQLiteStore
	files *MarkdownStore
}

// NewRepository creates a new repository backed by SQLite and the filesystem.
func NewRepository(db *SQLiteStore, files *MarkdownStore) *Repository {
	return &Repository{
		db:    db,
		files: files,
	}
}

// GetDB returns the underlying SQLiteStore (temporary helper during refactor)
func (r *Repository) GetDB() *SQLiteStore {
	return r.db
}

// CreateFeature saves a feature to both DB and File.
func (r *Repository) CreateFeature(f Feature) error {
	// 1. Save to DB (Primary)
	// Note: We should ideally wrap this in a transaction that we can rollback
	// if the file write fails, or use an outbox pattern.
	// For now, we mimic the original partial-transaction safety by deleting on fail.

	// Create in DB
	if err := r.db.CreateFeature(f); err != nil {
		return fmt.Errorf("db create: %w", err)
	}

	// 2. Fetch decisions for file generation (usually empty on create, but good practice)
	decisions, _ := r.db.GetDecisions(f.ID)

	// 3. Save to File (Secondary)
	if err := r.files.WriteFeature(f, decisions); err != nil {
		// Compensating transaction: undo DB change
		// This is a "Best Effort" rollback
		_ = r.db.DeleteFeature(f.ID)
		return fmt.Errorf("file create: %w", err)
	}

	return nil
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
	// If FeatureID is missing in input, try to fetch it first?
	// The caller should ideally provide it, but if not:
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
		return err // Should not happen if DB integrity holds
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

// === Relationships ===

func (r *Repository) Link(from, to, relationType string) error {
	return r.db.Link(from, to, relationType)
}

func (r *Repository) Unlink(from, to, relationType string) error {
	return r.db.Unlink(from, to, relationType)
}

func (r *Repository) GetDependencies(featureID string) ([]string, error) {
	return r.db.GetDependencies(featureID)
}

func (r *Repository) GetDependents(featureID string) ([]string, error) {
	return r.db.GetDependents(featureID)
}

func (r *Repository) GetRelated(featureID string, maxDepth int) ([]string, error) {
	return r.db.GetRelated(featureID, maxDepth)
}

// === Index ===

func (r *Repository) GetIndex() (*FeatureIndex, error) {
	return r.db.GetIndex()
}

// === Integrity ===

func (r *Repository) Check() ([]Issue, error) {
	return r.db.Check()
}

func (r *Repository) Repair() error {
	// 1. Repair DB issues
	if err := r.db.Repair(); err != nil {
		return err
	}
	// 2. Ensuring files match DB
	return r.RebuildFiles()
}

// === Node Access (Delegated to DB) ===

func (r *Repository) ListNodes(filter string) ([]Node, error) {
	return r.db.ListNodes(filter)
}

func (r *Repository) GetNode(id string) (*Node, error) {
	return r.db.GetNode(id)
}

func (r *Repository) CreateNode(n Node) error {
	return r.db.CreateNode(n)
}

func (r *Repository) UpdateNodeEmbedding(id string, embedding []float32) error {
	return r.db.UpdateNodeEmbedding(id, embedding)
}

func (r *Repository) DeleteNode(id string) error {
	return r.db.DeleteNode(id)
}

func (r *Repository) DeleteNodesByAgent(agent string) error {
	return r.db.DeleteNodesByAgent(agent)
}

// CreatePattern stores a new pattern in the DB.
func (r *Repository) CreatePattern(p Pattern) error {
	return r.db.CreatePattern(p)
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

// GetDecisions returns all decisions for a feature.
func (r *Repository) GetDecisions(featureID string) ([]Decision, error) {
	return r.db.GetDecisions(featureID)
}

// Close closes the underlying database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}
