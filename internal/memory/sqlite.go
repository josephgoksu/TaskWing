package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements MemoryStore using SQLite for persistence.
type SQLiteStore struct {
	db         *sql.DB
	basePath   string // Path to .taskwing/memory directory
	indexCache *FeatureIndex
}

// NewSQLiteStore creates a new SQLite-backed memory store.
func NewSQLiteStore(basePath string) (*SQLiteStore, error) {
	dbPath := filepath.Join(basePath, "memory.db")

	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create memory directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	store := &SQLiteStore{
		db:       db,
		basePath: basePath,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database tables if they don't exist.
func (s *SQLiteStore) initSchema() error {
	schema := `
	-- Legacy tables (kept for dual-write migration)
	CREATE TABLE IF NOT EXISTS features (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		one_liner TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		tags TEXT,
		file_path TEXT NOT NULL,
		decision_count INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS decisions (
		id TEXT PRIMARY KEY,
		feature_id TEXT NOT NULL,
		title TEXT NOT NULL,
		summary TEXT NOT NULL,
		reasoning TEXT,
		tradeoffs TEXT,
		created_at TEXT NOT NULL,
		FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_feature TEXT NOT NULL,
		to_feature TEXT NOT NULL,
		edge_type TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (from_feature) REFERENCES features(id) ON DELETE CASCADE,
		FOREIGN KEY (to_feature) REFERENCES features(id) ON DELETE CASCADE,
		UNIQUE(from_feature, to_feature, edge_type)
	);

	-- New knowledge graph tables (v2 pivot)
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		content TEXT NOT NULL,              -- Original text input
		type TEXT,                          -- AI-inferred: decision, feature, plan, note
		summary TEXT,                       -- AI-extracted title/summary
		embedding BLOB,                     -- Vector for similarity search
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS node_edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_node TEXT NOT NULL,
		to_node TEXT NOT NULL,
		relation TEXT NOT NULL,             -- relates_to, depends_on, affects, etc.
		properties TEXT,                    -- JSON for arbitrary metadata (adopted from simple-graph)
		confidence REAL DEFAULT 1.0,        -- AI confidence score
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (from_node) REFERENCES nodes(id) ON DELETE CASCADE,
		FOREIGN KEY (to_node) REFERENCES nodes(id) ON DELETE CASCADE,
		UNIQUE(from_node, to_node, relation)
	);

	-- Indexes for legacy tables
	CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_feature);
	CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_feature);
	CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(edge_type);
	CREATE INDEX IF NOT EXISTS idx_decisions_feature ON decisions(feature_id);

	-- Indexes for new tables
	CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
	CREATE INDEX IF NOT EXISTS idx_node_edges_from ON node_edges(from_node);
	CREATE INDEX IF NOT EXISTS idx_node_edges_to ON node_edges(to_node);
	CREATE INDEX IF NOT EXISTS idx_node_edges_relation ON node_edges(relation);
	`

	_, err := s.db.Exec(schema)
	return err
}

// === Feature CRUD ===

func (s *SQLiteStore) CreateFeature(f Feature) error {
	if f.ID == "" {
		f.ID = "f-" + uuid.New().String()[:8]
	}
	if f.Status == "" {
		f.Status = FeatureStatusActive
	}
	now := time.Now().UTC()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	f.UpdatedAt = now

	// Generate file path
	safeName := strings.ToLower(strings.ReplaceAll(f.Name, " ", "-"))
	f.FilePath = filepath.Join(s.basePath, "features", safeName+".md")

	// Marshal tags to JSON
	tagsJSON, err := json.Marshal(f.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert into SQLite
	_, err = tx.Exec(`
		INSERT INTO features (id, name, one_liner, status, created_at, updated_at, tags, file_path, decision_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, f.ID, f.Name, f.OneLiner, f.Status, f.CreatedAt.Format(time.RFC3339),
		f.UpdatedAt.Format(time.RFC3339), string(tagsJSON), f.FilePath, 0)
	if err != nil {
		return fmt.Errorf("insert feature: %w", err)
	}

	// Create markdown file
	if err := s.writeMarkdownFile(f); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}

	if err := tx.Commit(); err != nil {
		os.Remove(f.FilePath)
		return fmt.Errorf("commit: %w", err)
	}

	// Invalidate cache
	s.indexCache = nil

	return nil
}

func (s *SQLiteStore) UpdateFeature(f Feature) error {
	f.UpdatedAt = time.Now().UTC()

	// Fetch existing file_path from database if not provided
	if f.FilePath == "" {
		err := s.db.QueryRow("SELECT file_path FROM features WHERE id = ?", f.ID).Scan(&f.FilePath)
		if err == sql.ErrNoRows {
			return fmt.Errorf("feature not found: %s", f.ID)
		}
		if err != nil {
			return fmt.Errorf("get file_path: %w", err)
		}
	}

	tagsJSON, err := json.Marshal(f.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE features
		SET name = ?, one_liner = ?, status = ?, updated_at = ?, tags = ?
		WHERE id = ?
	`, f.Name, f.OneLiner, f.Status, f.UpdatedAt.Format(time.RFC3339), string(tagsJSON), f.ID)
	if err != nil {
		return fmt.Errorf("update feature: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("feature not found: %s", f.ID)
	}

	// Update markdown file
	if err := s.writeMarkdownFile(f); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}

	s.indexCache = nil
	return nil
}

func (s *SQLiteStore) DeleteFeature(id string) error {
	// Check for dependents
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM edges WHERE to_feature = ? AND edge_type = 'depends_on'
	`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("check dependents: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete feature with %d dependents", count)
	}

	// Get file path before deletion
	var filePath string
	err = s.db.QueryRow("SELECT file_path FROM features WHERE id = ?", id).Scan(&filePath)
	if err == sql.ErrNoRows {
		return fmt.Errorf("feature not found: %s", id)
	}
	if err != nil {
		return fmt.Errorf("get feature: %w", err)
	}

	// Delete from SQLite (cascades to decisions and edges)
	_, err = s.db.Exec("DELETE FROM features WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete feature: %w", err)
	}

	// Delete markdown file
	os.Remove(filePath)

	s.indexCache = nil
	return nil
}

func (s *SQLiteStore) GetFeature(id string) (*Feature, error) {
	var f Feature
	var tagsJSON string
	var createdAt, updatedAt string

	err := s.db.QueryRow(`
		SELECT id, name, one_liner, status, created_at, updated_at, tags, file_path, decision_count
		FROM features WHERE id = ?
	`, id).Scan(&f.ID, &f.Name, &f.OneLiner, &f.Status, &createdAt, &updatedAt,
		&tagsJSON, &f.FilePath, &f.DecisionCount)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("feature not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query feature: %w", err)
	}

	f.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	f.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	json.Unmarshal([]byte(tagsJSON), &f.Tags)

	return &f, nil
}

func (s *SQLiteStore) ListFeatures() ([]FeatureSummary, error) {
	rows, err := s.db.Query(`
		SELECT id, name, one_liner, status, decision_count FROM features ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query features: %w", err)
	}
	defer rows.Close()

	var features []FeatureSummary
	for rows.Next() {
		var f FeatureSummary
		if err := rows.Scan(&f.ID, &f.Name, &f.OneLiner, &f.Status, &f.DecisionCount); err != nil {
			return nil, fmt.Errorf("scan feature: %w", err)
		}
		features = append(features, f)
	}

	return features, nil
}

// === Decision CRUD ===

func (s *SQLiteStore) AddDecision(featureID string, d Decision) error {
	if d.ID == "" {
		d.ID = "d-" + uuid.New().String()[:8]
	}
	d.FeatureID = featureID
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO decisions (id, feature_id, title, summary, reasoning, tradeoffs, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, d.ID, d.FeatureID, d.Title, d.Summary, d.Reasoning, d.Tradeoffs,
		d.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert decision: %w", err)
	}

	// Update decision count
	_, err = tx.Exec(`
		UPDATE features SET decision_count = (
			SELECT COUNT(*) FROM decisions WHERE feature_id = ?
		) WHERE id = ?
	`, featureID, featureID)
	if err != nil {
		return fmt.Errorf("update decision count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.indexCache = nil
	return nil
}

func (s *SQLiteStore) UpdateDecision(d Decision) error {
	result, err := s.db.Exec(`
		UPDATE decisions
		SET title = ?, summary = ?, reasoning = ?, tradeoffs = ?
		WHERE id = ?
	`, d.Title, d.Summary, d.Reasoning, d.Tradeoffs, d.ID)
	if err != nil {
		return fmt.Errorf("update decision: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("decision not found: %s", d.ID)
	}

	return nil
}

func (s *SQLiteStore) DeleteDecision(id string) error {
	// Get feature ID for cache invalidation
	var featureID string
	err := s.db.QueryRow("SELECT feature_id FROM decisions WHERE id = ?", id).Scan(&featureID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("decision not found: %s", id)
	}
	if err != nil {
		return fmt.Errorf("get decision: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM decisions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete decision: %w", err)
	}

	// Update decision count
	_, err = tx.Exec(`
		UPDATE features SET decision_count = (
			SELECT COUNT(*) FROM decisions WHERE feature_id = ?
		) WHERE id = ?
	`, featureID, featureID)
	if err != nil {
		return fmt.Errorf("update decision count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.indexCache = nil
	return nil
}

func (s *SQLiteStore) GetDecisions(featureID string) ([]Decision, error) {
	rows, err := s.db.Query(`
		SELECT id, feature_id, title, summary, reasoning, tradeoffs, created_at
		FROM decisions WHERE feature_id = ? ORDER BY created_at
	`, featureID)
	if err != nil {
		return nil, fmt.Errorf("query decisions: %w", err)
	}
	defer rows.Close()

	var decisions []Decision
	for rows.Next() {
		var d Decision
		var createdAt string
		var reasoning, tradeoffs sql.NullString

		if err := rows.Scan(&d.ID, &d.FeatureID, &d.Title, &d.Summary,
			&reasoning, &tradeoffs, &createdAt); err != nil {
			return nil, fmt.Errorf("scan decision: %w", err)
		}

		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		d.Reasoning = reasoning.String
		d.Tradeoffs = tradeoffs.String
		decisions = append(decisions, d)
	}

	return decisions, nil
}

// === Relationships ===

func (s *SQLiteStore) Link(from, to, relationType string) error {
	// Validate relation type
	validTypes := map[string]bool{
		EdgeTypeDependsOn: true,
		EdgeTypeExtends:   true,
		EdgeTypeReplaces:  true,
		EdgeTypeRelated:   true,
	}
	if !validTypes[relationType] {
		return fmt.Errorf("invalid relation type: %s", relationType)
	}

	// Check features exist
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM features WHERE id IN (?, ?)", from, to).Scan(&count)
	if err != nil {
		return fmt.Errorf("check features: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("one or both features not found")
	}

	// Check for circular dependency (for depends_on)
	if relationType == EdgeTypeDependsOn {
		deps, err := s.GetDependencies(to)
		if err != nil {
			return fmt.Errorf("check circular: %w", err)
		}
		for _, dep := range deps {
			if dep == from {
				return fmt.Errorf("circular dependency detected")
			}
		}
	}

	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO edges (from_feature, to_feature, edge_type, created_at)
		VALUES (?, ?, ?, ?)
	`, from, to, relationType, time.Now().UTC().Format(time.RFC3339))

	return err
}

func (s *SQLiteStore) Unlink(from, to, relationType string) error {
	result, err := s.db.Exec(`
		DELETE FROM edges WHERE from_feature = ? AND to_feature = ? AND edge_type = ?
	`, from, to, relationType)
	if err != nil {
		return fmt.Errorf("delete edge: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("relationship not found")
	}

	return nil
}

func (s *SQLiteStore) GetDependencies(featureID string) ([]string, error) {
	// Recursive CTE to get all dependencies
	rows, err := s.db.Query(`
		WITH RECURSIVE deps AS (
			SELECT to_feature, 1 as depth
			FROM edges
			WHERE from_feature = ? AND edge_type = 'depends_on'
			UNION ALL
			SELECT e.to_feature, d.depth + 1
			FROM edges e
			JOIN deps d ON e.from_feature = d.to_feature
			WHERE e.edge_type = 'depends_on' AND d.depth < 10
		)
		SELECT DISTINCT to_feature FROM deps
	`, featureID)
	if err != nil {
		return nil, fmt.Errorf("query dependencies: %w", err)
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, dep)
	}

	return deps, nil
}

func (s *SQLiteStore) GetDependents(featureID string) ([]string, error) {
	rows, err := s.db.Query(`
		WITH RECURSIVE dependents AS (
			SELECT from_feature, 1 as depth
			FROM edges
			WHERE to_feature = ? AND edge_type = 'depends_on'
			UNION ALL
			SELECT e.from_feature, d.depth + 1
			FROM edges e
			JOIN dependents d ON e.to_feature = d.from_feature
			WHERE e.edge_type = 'depends_on' AND d.depth < 10
		)
		SELECT DISTINCT from_feature FROM dependents
	`, featureID)
	if err != nil {
		return nil, fmt.Errorf("query dependents: %w", err)
	}
	defer rows.Close()

	var dependents []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dependent: %w", err)
		}
		dependents = append(dependents, dep)
	}

	return dependents, nil
}

func (s *SQLiteStore) GetRelated(featureID string, maxDepth int) ([]string, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}

	rows, err := s.db.Query(`
		WITH RECURSIVE related AS (
			SELECT to_feature as feature, 1 as depth
			FROM edges WHERE from_feature = ?
			UNION
			SELECT from_feature as feature, 1 as depth
			FROM edges WHERE to_feature = ?
			UNION ALL
			SELECT CASE WHEN e.from_feature = r.feature THEN e.to_feature ELSE e.from_feature END,
				   r.depth + 1
			FROM edges e
			JOIN related r ON e.from_feature = r.feature OR e.to_feature = r.feature
			WHERE r.depth < ?
		)
		SELECT DISTINCT feature FROM related WHERE feature != ?
	`, featureID, featureID, maxDepth, featureID)
	if err != nil {
		return nil, fmt.Errorf("query related: %w", err)
	}
	defer rows.Close()

	var related []string
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			return nil, fmt.Errorf("scan related: %w", err)
		}
		related = append(related, rel)
	}

	return related, nil
}

func (s *SQLiteStore) FindPath(from, to string) ([]string, error) {
	// BFS to find shortest path
	rows, err := s.db.Query(`
		WITH RECURSIVE path AS (
			SELECT from_feature, to_feature, from_feature || ',' || to_feature as route, 1 as depth
			FROM edges WHERE from_feature = ?
			UNION ALL
			SELECT e.from_feature, e.to_feature, p.route || ',' || e.to_feature, p.depth + 1
			FROM edges e
			JOIN path p ON e.from_feature = p.to_feature
			WHERE p.depth < 10 AND p.route NOT LIKE '%' || e.to_feature || '%'
		)
		SELECT route FROM path WHERE to_feature = ? ORDER BY depth LIMIT 1
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query path: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var route string
		if err := rows.Scan(&route); err != nil {
			return nil, fmt.Errorf("scan path: %w", err)
		}
		return strings.Split(route, ","), nil
	}

	return nil, nil // No path found
}

// === Cache Management ===

func (s *SQLiteStore) RebuildIndex() error {
	features, err := s.ListFeatures()
	if err != nil {
		return fmt.Errorf("list features: %w", err)
	}

	index := &FeatureIndex{
		Features:    features,
		LastUpdated: time.Now().UTC(),
	}

	indexPath := filepath.Join(s.basePath, "index.json")
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	s.indexCache = index
	return nil
}

func (s *SQLiteStore) GetIndex() (*FeatureIndex, error) {
	if s.indexCache != nil {
		return s.indexCache, nil
	}

	// Try to load from file
	indexPath := filepath.Join(s.basePath, "index.json")
	data, err := os.ReadFile(indexPath)
	if err == nil {
		var index FeatureIndex
		if json.Unmarshal(data, &index) == nil {
			s.indexCache = &index
			return &index, nil
		}
	}

	// Rebuild if not found or invalid
	if err := s.RebuildIndex(); err != nil {
		return nil, err
	}

	return s.indexCache, nil
}

// === Integrity ===

func (s *SQLiteStore) Check() ([]Issue, error) {
	var issues []Issue

	// Check: every feature has a markdown file
	rows, err := s.db.Query("SELECT id, file_path FROM features")
	if err != nil {
		return nil, fmt.Errorf("query features: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			return nil, fmt.Errorf("scan feature: %w", err)
		}
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			issues = append(issues, Issue{
				Type:      "missing_file",
				FeatureID: id,
				Message:   fmt.Sprintf("Markdown file missing: %s", filePath),
			})
		}
	}

	// Check: edges reference existing features
	edgeRows, err := s.db.Query(`
		SELECT e.from_feature, e.to_feature
		FROM edges e
		LEFT JOIN features f1 ON e.from_feature = f1.id
		LEFT JOIN features f2 ON e.to_feature = f2.id
		WHERE f1.id IS NULL OR f2.id IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("query orphan edges: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var from, to string
		if err := edgeRows.Scan(&from, &to); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		issues = append(issues, Issue{
			Type:    "orphan_edge",
			Message: fmt.Sprintf("Edge references non-existent feature: %s -> %s", from, to),
		})
	}

	return issues, nil
}

func (s *SQLiteStore) Repair() error {
	// Get all issues
	issues, err := s.Check()
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}

	for _, issue := range issues {
		switch issue.Type {
		case "missing_file":
			// Regenerate markdown file from SQLite data
			f, err := s.GetFeature(issue.FeatureID)
			if err != nil {
				continue
			}
			if err := s.writeMarkdownFile(*f); err != nil {
				continue
			}
		case "orphan_edge":
			// Delete orphan edges
			s.db.Exec(`
				DELETE FROM edges WHERE from_feature NOT IN (SELECT id FROM features)
				OR to_feature NOT IN (SELECT id FROM features)
			`)
		}
	}

	// Rebuild index
	return s.RebuildIndex()
}

// === Lifecycle ===

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// === Helpers ===

func (s *SQLiteStore) writeMarkdownFile(f Feature) error {
	featuresDir := filepath.Join(s.basePath, "features")
	if err := os.MkdirAll(featuresDir, 0755); err != nil {
		return fmt.Errorf("create features dir: %w", err)
	}

	// Get decisions for this feature
	decisions, _ := s.GetDecisions(f.ID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", f.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", f.OneLiner))

	if len(decisions) > 0 {
		sb.WriteString("## Decisions\n\n")
		for _, d := range decisions {
			sb.WriteString(fmt.Sprintf("### %s\n", d.Title))
			sb.WriteString(fmt.Sprintf("- **Summary:** %s\n", d.Summary))
			if d.Reasoning != "" {
				sb.WriteString(fmt.Sprintf("- **Why:** %s\n", d.Reasoning))
			}
			if d.Tradeoffs != "" {
				sb.WriteString(fmt.Sprintf("- **Trade-offs:** %s\n", d.Tradeoffs))
			}
			sb.WriteString(fmt.Sprintf("- **Date:** %s\n\n", d.CreatedAt.Format("2006-01-02")))
		}
	}

	sb.WriteString("## Notes\n\n")
	sb.WriteString("<!-- Add notes here -->\n")

	return os.WriteFile(f.FilePath, []byte(sb.String()), 0644)
}

// === Node CRUD (v2 Knowledge Graph) ===

// CreateNode stores a new node in the knowledge graph.
func (s *SQLiteStore) CreateNode(n Node) error {
	if n.ID == "" {
		n.ID = "n-" + uuid.New().String()[:8]
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}

	// Serialize embedding to bytes if present
	var embeddingBytes []byte
	if len(n.Embedding) > 0 {
		embeddingBytes = float32SliceToBytes(n.Embedding)
	}

	_, err := s.db.Exec(`
		INSERT INTO nodes (id, content, type, summary, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, n.ID, n.Content, n.Type, n.Summary, embeddingBytes, n.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}

	return nil
}

// GetNode retrieves a node by ID.
func (s *SQLiteStore) GetNode(id string) (*Node, error) {
	var n Node
	var createdAt string
	var nodeType, summary sql.NullString
	var embeddingBytes []byte

	err := s.db.QueryRow(`
		SELECT id, content, type, summary, embedding, created_at
		FROM nodes WHERE id = ?
	`, id).Scan(&n.ID, &n.Content, &nodeType, &summary, &embeddingBytes, &createdAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query node: %w", err)
	}

	n.Type = nodeType.String
	n.Summary = summary.String
	n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if len(embeddingBytes) > 0 {
		n.Embedding = bytesToFloat32Slice(embeddingBytes)
	}

	return &n, nil
}

// ListNodes returns all nodes, optionally filtered by type.
func (s *SQLiteStore) ListNodes(nodeType string) ([]Node, error) {
	var rows *sql.Rows
	var err error

	if nodeType != "" {
		rows, err = s.db.Query(`
			SELECT id, content, type, summary, created_at
			FROM nodes WHERE type = ? ORDER BY created_at DESC
		`, nodeType)
	} else {
		rows, err = s.db.Query(`
			SELECT id, content, type, summary, created_at
			FROM nodes ORDER BY created_at DESC
		`)
	}

	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var createdAt string
		var nodeTypeStr, summary sql.NullString

		if err := rows.Scan(&n.ID, &n.Content, &nodeTypeStr, &summary, &createdAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}

		n.Type = nodeTypeStr.String
		n.Summary = summary.String
		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		nodes = append(nodes, n)
	}

	return nodes, nil
}

// DeleteNode removes a node and its edges.
func (s *SQLiteStore) DeleteNode(id string) error {
	result, err := s.db.Exec("DELETE FROM nodes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("node not found: %s", id)
	}

	return nil
}

// UpdateNodeEmbedding updates the embedding for an existing node.
func (s *SQLiteStore) UpdateNodeEmbedding(id string, embedding []float32) error {
	embeddingBytes := float32SliceToBytes(embedding)

	result, err := s.db.Exec("UPDATE nodes SET embedding = ? WHERE id = ?", embeddingBytes, id)
	if err != nil {
		return fmt.Errorf("update embedding: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("node not found: %s", id)
	}

	return nil
}

// LinkNodes creates a relationship between two nodes.
func (s *SQLiteStore) LinkNodes(from, to, relation string, confidence float64, properties map[string]any) error {
	if confidence <= 0 {
		confidence = 1.0
	}

	var propsJSON []byte
	if len(properties) > 0 {
		var err error
		propsJSON, err = json.Marshal(properties)
		if err != nil {
			return fmt.Errorf("marshal properties: %w", err)
		}
	}

	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO node_edges (from_node, to_node, relation, properties, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, from, to, relation, propsJSON, confidence, time.Now().UTC().Format(time.RFC3339))

	return err
}

// GetNodeEdges returns all edges for a node.
func (s *SQLiteStore) GetNodeEdges(nodeID string) ([]NodeEdge, error) {
	rows, err := s.db.Query(`
		SELECT id, from_node, to_node, relation, properties, confidence, created_at
		FROM node_edges WHERE from_node = ? OR to_node = ?
	`, nodeID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}
	defer rows.Close()

	var edges []NodeEdge
	for rows.Next() {
		var e NodeEdge
		var createdAt string
		var propsJSON sql.NullString

		if err := rows.Scan(&e.ID, &e.FromNode, &e.ToNode, &e.Relation, &propsJSON, &e.Confidence, &createdAt); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}

		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if propsJSON.Valid && propsJSON.String != "" {
			json.Unmarshal([]byte(propsJSON.String), &e.Properties)
		}
		edges = append(edges, e)
	}

	return edges, nil
}

// === Embedding Helpers ===

func float32SliceToBytes(floats []float32) []byte {
	buf := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := *(*uint32)(unsafe.Pointer(&f))
		buf[i*4] = byte(bits)
		buf[i*4+1] = byte(bits >> 8)
		buf[i*4+2] = byte(bits >> 16)
		buf[i*4+3] = byte(bits >> 24)
	}
	return buf
}

func bytesToFloat32Slice(buf []byte) []float32 {
	floats := make([]float32, len(buf)/4)
	for i := range floats {
		bits := uint32(buf[i*4]) | uint32(buf[i*4+1])<<8 | uint32(buf[i*4+2])<<16 | uint32(buf[i*4+3])<<24
		floats[i] = *(*float32)(unsafe.Pointer(&bits))
	}
	return floats
}
