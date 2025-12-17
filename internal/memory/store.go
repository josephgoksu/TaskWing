package memory

// MemoryStore defines the interface for project memory persistence.
// It manages features, decisions, and their relationships.
type MemoryStore interface {
	// === Feature CRUD ===

	// CreateFeature adds a new feature to the store.
	// It atomically updates SQLite, creates the markdown file, and invalidates the index cache.
	CreateFeature(f Feature) error

	// UpdateFeature modifies an existing feature.
	// Updates both SQLite and the markdown file.
	UpdateFeature(f Feature) error

	// DeleteFeature removes a feature and its associated decisions.
	// Returns error if feature has dependents (other features depend on it).
	DeleteFeature(id string) error

	// GetFeature retrieves a feature by ID, including its prose content from markdown.
	GetFeature(id string) (*Feature, error)

	// ListFeatures returns summaries of all features.
	ListFeatures() ([]FeatureSummary, error)

	// === Decision CRUD ===

	// AddDecision creates a new decision for a feature.
	// Updates the decision_count in the parent feature.
	AddDecision(featureID string, d Decision) error

	// UpdateDecision modifies an existing decision.
	UpdateDecision(d Decision) error

	// DeleteDecision removes a decision by ID.
	DeleteDecision(id string) error

	// GetDecisions returns all decisions for a feature.
	GetDecisions(featureID string) ([]Decision, error)

	// === Relationships ===

	// Link creates a relationship between two features.
	// relationType must be one of: depends_on, extends, replaces, related
	Link(from, to, relationType string) error

	// Unlink removes a relationship between two features.
	Unlink(from, to, relationType string) error

	// GetDependencies returns all features that the given feature depends on (recursive).
	GetDependencies(featureID string) ([]string, error)

	// GetDependents returns all features that depend on the given feature (recursive).
	GetDependents(featureID string) ([]string, error)

	// GetRelated returns features related to the given feature up to maxDepth.
	GetRelated(featureID string, maxDepth int) ([]string, error)

	// FindPath finds the shortest path between two features in the graph.
	FindPath(from, to string) ([]string, error)

	// === Cache Management ===

	// RebuildIndex regenerates index.json from SQLite data.
	RebuildIndex() error

	// GetIndex returns the cached feature index, regenerating if stale.
	GetIndex() (*FeatureIndex, error)

	// === Integrity ===

	// Check validates the integrity of the store.
	// Returns issues found (missing files, orphan edges, etc.)
	Check() ([]Issue, error)

	// Repair attempts to fix integrity issues.
	Repair() error

	// === Lifecycle ===

	// Close releases resources held by the store.
	Close() error
}
