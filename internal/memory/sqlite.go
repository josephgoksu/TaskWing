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
	var dbPath string
	if basePath == ":memory:" {
		dbPath = ":memory:"
	} else {
		dbPath = filepath.Join(basePath, "memory.db")

		// Ensure directory exists
		if err := os.MkdirAll(basePath, 0755); err != nil {
			return nil, fmt.Errorf("create memory directory: %w", err)
		}
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	store := &SQLiteStore{
		db:       db,
		basePath: basePath,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		_ = db.Close()
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

	-- Knowledge graph tables
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		content TEXT NOT NULL,              -- Original text input
		type TEXT,                          -- AI-inferred: decision, feature, plan, note
		summary TEXT,                       -- AI-extracted title/summary
		source_agent TEXT DEFAULT '',       -- Agent that created this node (doc, code, git, deps)
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

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_decisions_feature ON decisions(feature_id);
	CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
	CREATE INDEX IF NOT EXISTS idx_nodes_source_agent ON nodes(source_agent);
	CREATE INDEX IF NOT EXISTS idx_nodes_summary_agent ON nodes(summary, source_agent);
	CREATE INDEX IF NOT EXISTS idx_node_edges_from ON node_edges(from_node);
	CREATE INDEX IF NOT EXISTS idx_node_edges_to ON node_edges(to_node);
	CREATE INDEX IF NOT EXISTS idx_node_edges_relation ON node_edges(relation);

	-- Patterns table (V2 First-Class)
	CREATE TABLE IF NOT EXISTS patterns (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		context TEXT NOT NULL,
		solution TEXT NOT NULL,
		consequences TEXT,
		created_at TEXT NOT NULL
	);

	-- Plans table (High-level goals)
	CREATE TABLE IF NOT EXISTS plans (
		id TEXT PRIMARY KEY,
		goal TEXT NOT NULL,                -- Original user intent
		enriched_goal TEXT,                -- Refined after clarification
		status TEXT DEFAULT 'draft',       -- draft, active, completed, archived
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);

	-- Tasks table (atomic work units)
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		plan_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		acceptance_criteria TEXT,          -- JSON array
		validation_steps TEXT,             -- JSON array
		status TEXT DEFAULT 'pending',     -- pending, in_progress, verifying, completed, failed
		priority INTEGER DEFAULT 50,
		assigned_agent TEXT,
		parent_task_id TEXT,
		context_summary TEXT,              -- Summary of linked knowledge nodes
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE,
		FOREIGN KEY (parent_task_id) REFERENCES tasks(id) ON DELETE SET NULL
	);

	-- Task dependencies (DAG structure)
	CREATE TABLE IF NOT EXISTS task_dependencies (
		task_id TEXT NOT NULL,
		depends_on TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
		FOREIGN KEY (depends_on) REFERENCES tasks(id) ON DELETE CASCADE,
		PRIMARY KEY (task_id, depends_on)
	);

	-- Link tasks to Knowledge Graph nodes
	CREATE TABLE IF NOT EXISTS task_node_links (
		task_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		link_type TEXT NOT NULL,           -- context, modifies, references
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
		-- Note: We don't enforce FK on node_id rigidly because nodes might be in FTS or different tables,
		-- but conceptually it refers to 'nodes.id' or 'features.id'.
		-- For strictness, we'd reference nodes(id), but legacy features are in a different table.
		-- So we keep it soft for now, or we'll ensure everything is a node in v2 migration.
		PRIMARY KEY (task_id, node_id, link_type)
	);

	-- Clarification history (Audit trail for the agent)
	CREATE TABLE IF NOT EXISTS plan_clarifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id TEXT NOT NULL,
		question TEXT NOT NULL,
		answer TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE
	);

	-- Project overview (high-level project description for AI context)
	CREATE TABLE IF NOT EXISTS project_overview (
		id INTEGER PRIMARY KEY CHECK (id = 1),  -- Singleton: only one row allowed
		short_description TEXT NOT NULL,        -- One-sentence summary
		long_description TEXT NOT NULL,         -- Detailed description (2-3 paragraphs)
		generated_at TEXT NOT NULL,             -- When auto-generated by bootstrap
		last_edited_at TEXT                     -- When manually edited (NULL if never)
	);

	-- FTS5 for keyword search (hybrid with vector similarity)
	CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts USING fts5(
		id UNINDEXED,
		summary,
		content,
		content='nodes',
		content_rowid='rowid'
	);

	-- === Code Intelligence Tables ===
	-- These tables store symbol-level code intelligence data.
	-- Unlike architectural knowledge (nodes), symbol data is NOT mirrored to Markdown.

	-- Symbols table (code-level entities: functions, types, etc.)
	CREATE TABLE IF NOT EXISTS symbols (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,              -- function, method, struct, interface, type, variable, constant, field, package
		file_path TEXT NOT NULL,
		start_line INTEGER NOT NULL,
		end_line INTEGER NOT NULL,
		signature TEXT,                  -- e.g., "func(ctx context.Context) error"
		doc_comment TEXT,
		module_path TEXT,                -- e.g., "internal/memory"
		visibility TEXT DEFAULT 'public', -- public, private
		language TEXT NOT NULL,          -- go, typescript, python, etc.
		file_hash TEXT,                  -- SHA256 for incremental updates
		embedding BLOB,                  -- Vector for semantic search
		last_modified TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
	CREATE INDEX IF NOT EXISTS idx_symbols_file ON symbols(file_path);
	CREATE INDEX IF NOT EXISTS idx_symbols_kind ON symbols(kind);
	CREATE INDEX IF NOT EXISTS idx_symbols_language ON symbols(language);
	CREATE INDEX IF NOT EXISTS idx_symbols_module ON symbols(module_path);
	CREATE INDEX IF NOT EXISTS idx_symbols_file_hash ON symbols(file_hash);

	-- Symbol relationships (call graphs, inheritance, etc.)
	-- Enables recursive queries for impact analysis
	CREATE TABLE IF NOT EXISTS symbol_relations (
		from_symbol_id INTEGER NOT NULL,
		to_symbol_id INTEGER NOT NULL,
		relation_type TEXT NOT NULL,     -- calls, implements, extends, uses, defines, references
		call_site_line INTEGER,          -- For calls: line where the call occurs
		metadata TEXT,                   -- JSON for additional context
		PRIMARY KEY (from_symbol_id, to_symbol_id, relation_type),
		FOREIGN KEY (from_symbol_id) REFERENCES symbols(id) ON DELETE CASCADE,
		FOREIGN KEY (to_symbol_id) REFERENCES symbols(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_symbol_relations_from ON symbol_relations(from_symbol_id);
	CREATE INDEX IF NOT EXISTS idx_symbol_relations_to ON symbol_relations(to_symbol_id);
	CREATE INDEX IF NOT EXISTS idx_symbol_relations_type ON symbol_relations(relation_type);

	-- Dependencies from lockfiles (package.json, Cargo.lock, poetry.lock, etc.)
	-- Enables dependency analysis, security scanning, and upgrade planning
	CREATE TABLE IF NOT EXISTS dependencies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,              -- Package name
		version TEXT NOT NULL,           -- Installed version
		ecosystem TEXT NOT NULL,         -- npm, pypi, crates.io
		lockfile_ref TEXT NOT NULL,      -- Path to the lockfile
		resolved TEXT,                   -- URL/path where package was resolved from
		integrity TEXT,                  -- Hash for verification
		is_dev INTEGER DEFAULT 0,        -- Whether this is a dev dependency
		source TEXT,                     -- Source type (registry, git, path, etc.)
		extras TEXT,                     -- JSON for additional metadata
		last_modified TEXT NOT NULL,
		UNIQUE(name, version, lockfile_ref)
	);

	CREATE INDEX IF NOT EXISTS idx_dependencies_name ON dependencies(name);
	CREATE INDEX IF NOT EXISTS idx_dependencies_ecosystem ON dependencies(ecosystem);
	CREATE INDEX IF NOT EXISTS idx_dependencies_lockfile ON dependencies(lockfile_ref);

	-- FTS5 for dependency search (name only for now)
	CREATE VIRTUAL TABLE IF NOT EXISTS dependencies_fts USING fts5(
		name, ecosystem,
		content='dependencies',
		content_rowid='id'
	);

	-- FTS5 for symbol search (name, signature, doc_comment)
	CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(
		name, signature, doc_comment, module_path,
		content='symbols',
		content_rowid='id'
	);
	`

	// Execute main schema
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Add triggers separately (SQLite doesn't support IF NOT EXISTS for triggers)
	// We use INSERT OR REPLACE pattern by checking if trigger exists first
	triggers := []struct {
		name string
		sql  string
	}{
		{
			name: "nodes_fts_ai",
			sql: `CREATE TRIGGER nodes_fts_ai AFTER INSERT ON nodes BEGIN
				INSERT INTO nodes_fts(rowid, id, summary, content)
				VALUES (NEW.rowid, NEW.id, COALESCE(NEW.summary, ''), NEW.content);
			END`,
		},
		{
			name: "nodes_fts_ad",
			sql: `CREATE TRIGGER nodes_fts_ad AFTER DELETE ON nodes BEGIN
				INSERT INTO nodes_fts(nodes_fts, rowid, id, summary, content)
				VALUES('delete', OLD.rowid, OLD.id, COALESCE(OLD.summary, ''), OLD.content);
			END`,
		},
		{
			name: "nodes_fts_au",
			sql: `CREATE TRIGGER nodes_fts_au AFTER UPDATE ON nodes BEGIN
				INSERT INTO nodes_fts(nodes_fts, rowid, id, summary, content)
				VALUES('delete', OLD.rowid, OLD.id, COALESCE(OLD.summary, ''), OLD.content);
				INSERT INTO nodes_fts(rowid, id, summary, content)
				VALUES (NEW.rowid, NEW.id, COALESCE(NEW.summary, ''), NEW.content);
			END`,
		},
		// Symbols FTS triggers for code intelligence
		{
			name: "symbols_fts_ai",
			sql: `CREATE TRIGGER symbols_fts_ai AFTER INSERT ON symbols BEGIN
				INSERT INTO symbols_fts(rowid, name, signature, doc_comment, module_path)
				VALUES (NEW.id, NEW.name, COALESCE(NEW.signature, ''), COALESCE(NEW.doc_comment, ''), COALESCE(NEW.module_path, ''));
			END`,
		},
		{
			name: "symbols_fts_ad",
			sql: `CREATE TRIGGER symbols_fts_ad AFTER DELETE ON symbols BEGIN
				INSERT INTO symbols_fts(symbols_fts, rowid, name, signature, doc_comment, module_path)
				VALUES('delete', OLD.id, OLD.name, COALESCE(OLD.signature, ''), COALESCE(OLD.doc_comment, ''), COALESCE(OLD.module_path, ''));
			END`,
		},
		{
			name: "symbols_fts_au",
			sql: `CREATE TRIGGER symbols_fts_au AFTER UPDATE ON symbols BEGIN
				INSERT INTO symbols_fts(symbols_fts, rowid, name, signature, doc_comment, module_path)
				VALUES('delete', OLD.id, OLD.name, COALESCE(OLD.signature, ''), COALESCE(OLD.doc_comment, ''), COALESCE(OLD.module_path, ''));
				INSERT INTO symbols_fts(rowid, name, signature, doc_comment, module_path)
				VALUES (NEW.id, NEW.name, COALESCE(NEW.signature, ''), COALESCE(NEW.doc_comment, ''), COALESCE(NEW.module_path, ''));
			END`,
		},
		// Dependencies FTS triggers
		{
			name: "dependencies_fts_ai",
			sql: `CREATE TRIGGER dependencies_fts_ai AFTER INSERT ON dependencies BEGIN
				INSERT INTO dependencies_fts(rowid, name, ecosystem)
				VALUES (NEW.id, NEW.name, NEW.ecosystem);
			END`,
		},
		{
			name: "dependencies_fts_ad",
			sql: `CREATE TRIGGER dependencies_fts_ad AFTER DELETE ON dependencies BEGIN
				INSERT INTO dependencies_fts(dependencies_fts, rowid, name, ecosystem)
				VALUES('delete', OLD.id, OLD.name, OLD.ecosystem);
			END`,
		},
		{
			name: "dependencies_fts_au",
			sql: `CREATE TRIGGER dependencies_fts_au AFTER UPDATE ON dependencies BEGIN
				INSERT INTO dependencies_fts(dependencies_fts, rowid, name, ecosystem)
				VALUES('delete', OLD.id, OLD.name, OLD.ecosystem);
				INSERT INTO dependencies_fts(rowid, name, ecosystem)
				VALUES (NEW.id, NEW.name, NEW.ecosystem);
			END`,
		},
	}

	for _, t := range triggers {
		var count int
		err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?", t.name).Scan(&count)
		if err != nil {
			return fmt.Errorf("check trigger %s: %w", t.name, err)
		}
		if count == 0 {
			if _, err := s.db.Exec(t.sql); err != nil {
				return fmt.Errorf("create trigger %s: %w", t.name, err)
			}
		}
	}

	// Migration: Add verification columns to nodes table for Evidence-Based Findings
	// These columns support the verification pipeline that validates agent findings
	migrations := []struct {
		column string
		ddl    string
	}{
		{"verification_status", "ALTER TABLE nodes ADD COLUMN verification_status TEXT DEFAULT 'pending_verification'"},
		{"evidence", "ALTER TABLE nodes ADD COLUMN evidence TEXT"},                       // JSON blob of []Evidence
		{"verification_result", "ALTER TABLE nodes ADD COLUMN verification_result TEXT"}, // JSON blob of VerificationResult
		{"confidence_score", "ALTER TABLE nodes ADD COLUMN confidence_score REAL DEFAULT 0.5"},
	}

	for _, m := range migrations {
		// Check if column exists by querying table info
		var exists bool
		rows, err := s.db.Query("PRAGMA table_info(nodes)")
		if err == nil {
			for rows.Next() {
				var cid int
				var name, ctype string
				var notnull, pk int
				var dflt any
				if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
					if name == m.column {
						exists = true
						break
					}
				}
			}
			_ = rows.Close()
		}

		if !exists {
			if _, err := s.db.Exec(m.ddl); err != nil {
				// Only ignore "duplicate column" errors (happens in rare race conditions)
				// Other errors (disk full, corrupted DB) should propagate
				errMsg := err.Error()
				if !strings.Contains(errMsg, "duplicate column") {
					return fmt.Errorf("migration %s failed: %w", m.column, err)
				}
			}
		}
	}

	// Add index for verification status queries (enables efficient filtering)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_nodes_verification_status ON nodes(verification_status)`)

	// Migration: Add AI integration columns to tasks table for MCP tool support
	// These columns enable task lifecycle management via slash commands
	taskMigrations := []struct {
		column string
		ddl    string
	}{
		{"scope", "ALTER TABLE tasks ADD COLUMN scope TEXT"},                                       // e.g., "auth", "api", "vectorsearch"
		{"keywords", "ALTER TABLE tasks ADD COLUMN keywords TEXT"},                                 // JSON array of extracted keywords
		{"suggested_recall_queries", "ALTER TABLE tasks ADD COLUMN suggested_recall_queries TEXT"}, // JSON array of pre-computed queries
		{"claimed_by", "ALTER TABLE tasks ADD COLUMN claimed_by TEXT"},                             // Session ID that claimed this task
		{"claimed_at", "ALTER TABLE tasks ADD COLUMN claimed_at TEXT"},                             // Timestamp when claimed
		{"completed_at", "ALTER TABLE tasks ADD COLUMN completed_at TEXT"},                         // Timestamp when completed
		{"completion_summary", "ALTER TABLE tasks ADD COLUMN completion_summary TEXT"},             // AI-generated summary on completion
		{"files_modified", "ALTER TABLE tasks ADD COLUMN files_modified TEXT"},                     // JSON array of modified files
		{"block_reason", "ALTER TABLE tasks ADD COLUMN block_reason TEXT"},                         // Reason if task is blocked
	}

	for _, m := range taskMigrations {
		// Check if column exists
		var exists bool
		rows, err := s.db.Query("PRAGMA table_info(tasks)")
		if err == nil {
			for rows.Next() {
				var cid int
				var name, ctype string
				var notnull, pk int
				var dflt any
				if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
					if name == m.column {
						exists = true
						break
					}
				}
			}
			_ = rows.Close()
		}

		if !exists {
			if _, err := s.db.Exec(m.ddl); err != nil {
				errMsg := err.Error()
				if !strings.Contains(errMsg, "duplicate column") {
					return fmt.Errorf("task migration %s failed: %w", m.column, err)
				}
			}
		}
	}

	// Add index for finding next available task efficiently
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_status_priority ON tasks(status, priority DESC)`)

	// Migration: Add audit report column to plans table for audit agent
	planMigrations := []struct {
		column string
		ddl    string
	}{
		{"last_audit_report", "ALTER TABLE plans ADD COLUMN last_audit_report TEXT"}, // JSON-serialized AuditReport
	}

	for _, m := range planMigrations {
		var exists bool
		rows, err := s.db.Query("PRAGMA table_info(plans)")
		if err == nil {
			for rows.Next() {
				var cid int
				var name, ctype string
				var notnull, pk int
				var dflt any
				if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
					if name == m.column {
						exists = true
						break
					}
				}
			}
			_ = rows.Close()
		}

		if !exists {
			if _, err := s.db.Exec(m.ddl); err != nil {
				errMsg := err.Error()
				if !strings.Contains(errMsg, "duplicate column") {
					return fmt.Errorf("plan migration %s failed: %w", m.column, err)
				}
			}
		}
	}

	return nil
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
	defer func() { _ = tx.Rollback() }()

	// Insert into SQLite
	_, err = tx.Exec(`
		INSERT INTO features (id, name, one_liner, status, created_at, updated_at, tags, file_path, decision_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, f.ID, f.Name, f.OneLiner, f.Status, f.CreatedAt.Format(time.RFC3339),
		f.UpdatedAt.Format(time.RFC3339), string(tagsJSON), f.FilePath, 0)
	if err != nil {
		return fmt.Errorf("insert feature: %w", err)
	}

	if err := tx.Commit(); err != nil {
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

	s.indexCache = nil
	return nil
}

func (s *SQLiteStore) DeleteFeature(id string) error {
	// Get file path before deletion
	var filePath string
	err := s.db.QueryRow("SELECT file_path FROM features WHERE id = ?", id).Scan(&filePath)
	if err == sql.ErrNoRows {
		return fmt.Errorf("feature not found: %s", id)
	}
	if err != nil {
		return fmt.Errorf("get feature: %w", err)
	}

	// Delete from SQLite (cascades to decisions)
	_, err = s.db.Exec("DELETE FROM features WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete feature: %w", err)
	}

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
	if err := json.Unmarshal([]byte(tagsJSON), &f.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}

	return &f, nil
}

func (s *SQLiteStore) ListFeatures() ([]FeatureSummary, error) {
	rows, err := s.db.Query(`
		SELECT id, name, one_liner, status, decision_count FROM features ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query features: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = tx.Rollback() }()

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
	defer func() { _ = tx.Rollback() }()

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
	defer func() { _ = rows.Close() }()

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

func (s *SQLiteStore) GetDecision(id string) (*Decision, error) {
	var d Decision
	var createdAt string
	var reasoning, tradeoffs sql.NullString

	err := s.db.QueryRow(`
		SELECT id, feature_id, title, summary, reasoning, tradeoffs, created_at
		FROM decisions WHERE id = ?
	`, id).Scan(&d.ID, &d.FeatureID, &d.Title, &d.Summary, &reasoning, &tradeoffs, &createdAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("decision not found: %s", id)
		}
		return nil, fmt.Errorf("get decision: %w", err)
	}

	d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	d.Reasoning = reasoning.String
	d.Tradeoffs = tradeoffs.String

	return &d, nil
}

// === Pattern CRUD ===

type Pattern struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Context      string    `json:"context"`
	Solution     string    `json:"solution"`
	Consequences string    `json:"consequences"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *SQLiteStore) CreatePattern(p Pattern) error {
	if p.ID == "" {
		p.ID = "p-" + uuid.New().String()[:8]
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.Exec(`
		INSERT INTO patterns (id, name, context, solution, consequences, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, p.ID, p.Name, p.Context, p.Solution, p.Consequences, p.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("insert pattern: %w", err)
	}

	s.indexCache = nil
	return nil
}

func (s *SQLiteStore) ListPatterns() ([]Pattern, error) {
	rows, err := s.db.Query(`
		SELECT id, name, context, solution, consequences, created_at FROM patterns ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query patterns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var patterns []Pattern
	for rows.Next() {
		var p Pattern
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Context, &p.Solution, &p.Consequences, &createdAt); err != nil {
			return nil, fmt.Errorf("scan pattern: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		patterns = append(patterns, p)
	}
	return patterns, nil
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
	defer func() { _ = rows.Close() }()

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

	return issues, nil
}

func (s *SQLiteStore) Repair() error {
	// Rebuild index
	return s.RebuildIndex()
}

// === Lifecycle ===

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying database handle.
// This allows other packages (like codeintel) to share the same database connection.
func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}

// === Helpers ===

// === Node Helpers ===

// populateNodeFromScan populates a Node struct from scanned nullable fields.
// This centralizes the repetitive null-handling and type conversion logic.
func populateNodeFromScan(n *Node, nodeType, summary, sourceAgent sql.NullString, createdAt string, embeddingBytes []byte) {
	n.Type = nodeType.String
	n.Summary = summary.String
	n.SourceAgent = sourceAgent.String
	n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if len(embeddingBytes) > 0 {
		n.Embedding = bytesToFloat32Slice(embeddingBytes)
	}
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
		INSERT INTO nodes (id, content, type, summary, source_agent, embedding, created_at,
		                   evidence, verification_status, verification_result, confidence_score)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, n.ID, n.Content, n.Type, n.Summary, n.SourceAgent, embeddingBytes, n.CreatedAt.Format(time.RFC3339),
		n.Evidence, n.VerificationStatus, n.VerificationResult, n.ConfidenceScore)

	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}

	return nil
}

// GetNode retrieves a node by ID including evidence and verification fields.
func (s *SQLiteStore) GetNode(id string) (*Node, error) {
	var n Node
	var createdAt string
	var nodeType, summary, sourceAgent sql.NullString
	var evidence, verificationStatus, verificationResult sql.NullString
	var confidenceScore sql.NullFloat64
	var embeddingBytes []byte

	err := s.db.QueryRow(`
		SELECT id, content, type, summary, source_agent, embedding, created_at,
		       evidence, verification_status, verification_result, confidence_score
		FROM nodes WHERE id = ?
	`, id).Scan(&n.ID, &n.Content, &nodeType, &summary, &sourceAgent, &embeddingBytes, &createdAt,
		&evidence, &verificationStatus, &verificationResult, &confidenceScore)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query node: %w", err)
	}

	populateNodeFromScan(&n, nodeType, summary, sourceAgent, createdAt, embeddingBytes)

	// Populate evidence and verification fields
	if evidence.Valid {
		n.Evidence = evidence.String
	}
	if verificationStatus.Valid {
		n.VerificationStatus = verificationStatus.String
	}
	if verificationResult.Valid {
		n.VerificationResult = verificationResult.String
	}
	if confidenceScore.Valid {
		n.ConfidenceScore = confidenceScore.Float64
	}

	return &n, nil
}

// ListNodes returns all nodes, optionally filtered by type.
func (s *SQLiteStore) ListNodes(nodeType string) ([]Node, error) {
	var rows *sql.Rows
	var err error

	if nodeType != "" {
		rows, err = s.db.Query(`
			SELECT id, content, type, summary, source_agent, created_at,
			       evidence, verification_status, verification_result, confidence_score
			FROM nodes WHERE type = ? ORDER BY created_at DESC
		`, nodeType)
	} else {
		rows, err = s.db.Query(`
			SELECT id, content, type, summary, source_agent, created_at,
			       evidence, verification_status, verification_result, confidence_score
			FROM nodes ORDER BY created_at DESC
		`)
	}

	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var nodes []Node
	for rows.Next() {
		var n Node
		var createdAt string
		var nodeTypeStr, summary, sourceAgent sql.NullString
		var evidence, verificationStatus, verificationResult sql.NullString
		var confidenceScore sql.NullFloat64

		if err := rows.Scan(&n.ID, &n.Content, &nodeTypeStr, &summary, &sourceAgent, &createdAt,
			&evidence, &verificationStatus, &verificationResult, &confidenceScore); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		populateNodeFromScan(&n, nodeTypeStr, summary, sourceAgent, createdAt, nil)

		// Populate evidence fields
		if evidence.Valid {
			n.Evidence = evidence.String
		}
		if verificationStatus.Valid {
			n.VerificationStatus = verificationStatus.String
		}
		if verificationResult.Valid {
			n.VerificationResult = verificationResult.String
		}
		if confidenceScore.Valid {
			n.ConfidenceScore = confidenceScore.Float64
		}

		nodes = append(nodes, n)
	}

	return nodes, nil
}

// UpdateNode updates mutable node fields.
func (s *SQLiteStore) UpdateNode(id, content, nodeType, summary string) error {
	if id == "" {
		return fmt.Errorf("node id is required")
	}
	sets := []string{}
	args := []any{}

	if content != "" {
		sets = append(sets, "content = ?")
		args = append(args, content)
	}
	if nodeType != "" {
		sets = append(sets, "type = ?")
		args = append(args, nodeType)
	}
	if summary != "" {
		sets = append(sets, "summary = ?")
		args = append(args, summary)
	}
	if len(sets) == 0 {
		return fmt.Errorf("no fields to update")
	}

	query := "UPDATE nodes SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update node rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("node not found: %s", id)
	}
	return nil
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

// DeleteNodesByType removes all nodes of a specific type.
func (s *SQLiteStore) DeleteNodesByType(nodeType string) (int64, error) {
	if nodeType == "" {
		return 0, fmt.Errorf("node type is required")
	}
	result, err := s.db.Exec("DELETE FROM nodes WHERE type = ?", nodeType)
	if err != nil {
		return 0, fmt.Errorf("delete nodes by type: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete nodes by type rows affected: %w", err)
	}
	return rows, nil
}

// DeleteNodesByAgent removes all nodes created by a specific agent.
// This is used for agent-level replace strategy during selective re-bootstrapping.
func (s *SQLiteStore) DeleteNodesByAgent(agentName string) error {
	_, err := s.db.Exec("DELETE FROM nodes WHERE source_agent = ?", agentName)
	if err != nil {
		return fmt.Errorf("delete nodes by agent: %w", err)
	}
	return nil
}

// DeleteNodesByFiles removes nodes from a specific agent that reference any of the given files.
// Used for incremental updates to avoid full agent purge.
func (s *SQLiteStore) DeleteNodesByFiles(agentName string, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}

	// 1. Get all nodes for this agent with their evidence
	rows, err := s.db.Query(`SELECT id, evidence FROM nodes WHERE source_agent = ?`, agentName)
	if err != nil {
		return fmt.Errorf("query nodes for purge: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var idsToDelete []string
	targetFiles := make(map[string]bool)
	for _, f := range filePaths {
		targetFiles[f] = true
	}

	for rows.Next() {
		var id string
		var evidenceJSON sql.NullString
		if err := rows.Scan(&id, &evidenceJSON); err != nil {
			continue
		}

		if !evidenceJSON.Valid || evidenceJSON.String == "" {
			continue
		}

		// Simple heuristics to avoid heavy JSON parsing if possible
		if !strings.Contains(evidenceJSON.String, "file_path") {
			continue
		}

		var evList []struct {
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal([]byte(evidenceJSON.String), &evList); err != nil {
			continue
		}

		for _, ev := range evList {
			if targetFiles[ev.FilePath] {
				idsToDelete = append(idsToDelete, id)
				break
			}
		}
	}
	_ = rows.Close()

	if len(idsToDelete) == 0 {
		return nil
	}

	// 2. Delete the identified nodes in batches
	const batchSize = 500
	for i := 0; i < len(idsToDelete); i += batchSize {
		end := i + batchSize
		if end > len(idsToDelete) {
			end = len(idsToDelete)
		}
		batch := idsToDelete[i:end]

		query := `DELETE FROM nodes WHERE id IN (?` + strings.Repeat(",?", len(batch)-1) + `)`
		args := make([]any, len(batch))
		for j, id := range batch {
			args[j] = id
		}

		if _, err := s.db.Exec(query, args...); err != nil {
			return fmt.Errorf("delete batch: %w", err)
		}
	}

	return nil
}

// GetNodesByFiles returns nodes from a specific agent that reference any of the given files.
// Used for fetching context during incremental analysis.
func (s *SQLiteStore) GetNodesByFiles(agentName string, filePaths []string) ([]Node, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}

	// 1. Get all nodes for this agent
	// Note: We select * to reconstruct the Node
	rows, err := s.db.Query(`SELECT id, content, type, summary, source_agent, embedding, created_at, evidence FROM nodes WHERE source_agent = ?`, agentName)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var nodes []Node
	targetFiles := make(map[string]bool)
	for _, f := range filePaths {
		targetFiles[f] = true
	}

	for rows.Next() {
		var n Node
		var evidenceJSON sql.NullString
		var embeddingBytes []byte // temp for blob

		// Scan matching the SELECT columns
		if err := rows.Scan(&n.ID, &n.Content, &n.Type, &n.Summary, &n.SourceAgent, &embeddingBytes, &n.CreatedAt, &evidenceJSON); err != nil {
			continue // skip errors
		}

		if !evidenceJSON.Valid || evidenceJSON.String == "" {
			continue
		}
		n.Evidence = evidenceJSON.String

		// Simple heuristics to filter
		if !strings.Contains(n.Evidence, "file_path") {
			continue
		}

		// Parse evidence to check file path matches
		var evList []struct {
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal([]byte(n.Evidence), &evList); err != nil {
			continue
		}

		match := false
		for _, ev := range evList {
			if targetFiles[ev.FilePath] {
				match = true
				break
			}
		}

		if match {
			// Rehydrate embedding if needed (skipping for now as we don't use it in prompt)
			nodes = append(nodes, n)
		}
	}

	return nodes, nil
}

// ClearAllKnowledge removes all nodes, features, decisions, and patterns.
// Used for clean-slate re-bootstrapping when the user wants to start fresh.
func (s *SQLiteStore) ClearAllKnowledge() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Clear in order respecting foreign key constraints
	tables := []string{"node_edges", "nodes", "decisions", "patterns", "features"}
	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.indexCache = nil
	return nil
}

// UpsertNodeBySummary inserts a new node or updates existing one matched by summary and agent.
// This is used for incremental watch mode - findings with same title from same agent are updated.
// If no exact match is found, it checks for semantically similar summaries and updates those instead
// to prevent duplicate nodes from accumulating.
// Uses a transaction to prevent race conditions in concurrent watch mode.
func (s *SQLiteStore) UpsertNodeBySummary(n Node) error {
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

	// Use IMMEDIATE transaction to prevent race conditions in concurrent watch mode
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// First check if node with exact summary+agent exists
	var existingID string
	err = tx.QueryRow(`
		SELECT id FROM nodes WHERE summary = ? AND source_agent = ?
	`, n.Summary, n.SourceAgent).Scan(&existingID)

	if err == nil && existingID != "" {
		// Update existing node with exact match (including evidence columns)
		_, err = tx.Exec(`
			UPDATE nodes SET content = ?, type = ?, embedding = ?,
			       evidence = ?, verification_status = ?, verification_result = ?, confidence_score = ?
			WHERE id = ?
		`, n.Content, n.Type, embeddingBytes,
			n.Evidence, n.VerificationStatus, n.VerificationResult, n.ConfidenceScore, existingID)
		if err != nil {
			return fmt.Errorf("update existing node: %w", err)
		}
		return tx.Commit()
	}

	// No exact match - check for semantically similar summaries from same agent
	// This prevents duplicate nodes when LLM generates slightly different titles
	rows, err := tx.Query(`
		SELECT id, summary, content FROM nodes WHERE source_agent = ?
	`, n.SourceAgent)
	if err != nil {
		return fmt.Errorf("query similar nodes: %w", err)
	}

	var similarID string
	var similarContent string
	for rows.Next() {
		var existingSummary string
		if err := rows.Scan(&similarID, &existingSummary, &similarContent); err != nil {
			continue
		}
		sim := textSimilarity(n.Summary, existingSummary)
		if sim >= textSimilarityThreshold {
			_ = rows.Close() // Close before executing update
			// Found a similar node - update it instead of inserting new (including evidence columns)
			if n.Content != similarContent {
				_, err = tx.Exec(`
					UPDATE nodes SET content = ?, type = ?, embedding = ?, summary = ?,
					       evidence = ?, verification_status = ?, verification_result = ?, confidence_score = ?
					WHERE id = ?
				`, n.Content, n.Type, embeddingBytes, n.Summary,
					n.Evidence, n.VerificationStatus, n.VerificationResult, n.ConfidenceScore, similarID)
			} else {
				_, err = tx.Exec(`
					UPDATE nodes SET type = ?, embedding = ?, summary = ?,
					       evidence = ?, verification_status = ?, verification_result = ?, confidence_score = ?
					WHERE id = ?
				`, n.Type, embeddingBytes, n.Summary,
					n.Evidence, n.VerificationStatus, n.VerificationResult, n.ConfidenceScore, similarID)
			}
			if err != nil {
				return fmt.Errorf("update similar node: %w", err)
			}
			return tx.Commit()
		}
	}
	_ = rows.Close()

	// No similar node found - insert new node (including evidence columns)
	_, err = tx.Exec(`
		INSERT INTO nodes (id, content, type, summary, source_agent, embedding, created_at,
		                   evidence, verification_status, verification_result, confidence_score)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, n.ID, n.Content, n.Type, n.Summary, n.SourceAgent, embeddingBytes, n.CreatedAt.Format(time.RFC3339),
		n.Evidence, n.VerificationStatus, n.VerificationResult, n.ConfidenceScore)

	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}

	return tx.Commit()
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
	defer func() { _ = rows.Close() }()

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
			_ = json.Unmarshal([]byte(propsJSON.String), &e.Properties)
		}
		edges = append(edges, e)
	}

	return edges, nil
}

// GetAllNodeEdges returns all edges in the knowledge graph.
func (s *SQLiteStore) GetAllNodeEdges() ([]NodeEdge, error) {
	rows, err := s.db.Query(`
		SELECT id, from_node, to_node, relation, properties, confidence, created_at
		FROM node_edges ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query all edges: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
			_ = json.Unmarshal([]byte(propsJSON.String), &e.Properties)
		}
		edges = append(edges, e)
	}

	return edges, nil
}

// === FTS5 Search Methods ===

// FTSResult represents a full-text search result with relevance rank
type FTSResult struct {
	Node Node
	Rank float64 // BM25 rank (lower is more relevant)
}

// ListNodesWithEmbeddings returns all nodes with embeddings in a single query.
// This fixes the N+1 query pattern in search - one query instead of 1+N.
func (s *SQLiteStore) ListNodesWithEmbeddings() ([]Node, error) {
	rows, err := s.db.Query(`
		SELECT id, content, type, summary, source_agent, embedding, created_at,
		       evidence, verification_status, verification_result, confidence_score
		FROM nodes WHERE embedding IS NOT NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query nodes with embeddings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var nodes []Node
	for rows.Next() {
		var n Node
		var createdAt string
		var nodeType, summary, sourceAgent sql.NullString
		var embeddingBytes []byte
		var evidence, verificationStatus, verificationResult sql.NullString
		var confidenceScore sql.NullFloat64

		if err := rows.Scan(&n.ID, &n.Content, &nodeType, &summary, &sourceAgent, &embeddingBytes, &createdAt,
			&evidence, &verificationStatus, &verificationResult, &confidenceScore); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		populateNodeFromScan(&n, nodeType, summary, sourceAgent, createdAt, embeddingBytes)

		// Populate evidence fields
		if evidence.Valid {
			n.Evidence = evidence.String
		}
		if verificationStatus.Valid {
			n.VerificationStatus = verificationStatus.String
		}
		if verificationResult.Valid {
			n.VerificationResult = verificationResult.String
		}
		if confidenceScore.Valid {
			n.ConfidenceScore = confidenceScore.Float64
		}

		nodes = append(nodes, n)
	}

	return nodes, nil
}

// sanitizeFTSQueryForNodes sanitizes a query for FTS5 knowledge node search.
// It uses OR logic for multi-word queries to improve recall when exact matches fail.
// Stop words are filtered to focus on content words.
func sanitizeFTSQueryForNodes(query string) string {
	if query == "" {
		return ""
	}

	// Common stop words that rarely help search
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"what": true, "which": true, "who": true, "whom": true, "this": true,
		"that": true, "these": true, "those": true, "it": true, "its": true,
		"of": true, "for": true, "with": true, "about": true, "against": true,
		"between": true, "into": true, "through": true, "during": true,
		"before": true, "after": true, "above": true, "below": true, "to": true,
		"from": true, "up": true, "down": true, "in": true, "out": true,
		"on": true, "off": true, "over": true, "under": true, "again": true,
		"how": true, "why": true, "when": true, "where": true, "use": true,
		"using": true, "used": true, "type": true, "types": true,
	}

	// FTS5 special characters to replace
	replacer := strings.NewReplacer(
		`"`, " ", `^`, " ", `:`, " ", `(`, " ", `)`, " ",
		`{`, " ", `}`, " ", `[`, " ", `]`, " ", `-`, " ", `+`, " ",
		`?`, " ", `!`, " ", `.`, " ", `,`, " ", `;`, " ",
	)
	sanitized := replacer.Replace(strings.ToLower(query))

	// Split into words and filter
	words := strings.Fields(sanitized)
	var filtered []string
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) < 2 {
			continue
		}
		// Skip stop words
		if stopWords[word] {
			continue
		}
		// Skip FTS5 operators
		upper := strings.ToUpper(word)
		if upper == "OR" || upper == "AND" || upper == "NOT" || upper == "NEAR" {
			continue
		}
		// Remove any remaining * characters
		word = strings.ReplaceAll(word, "*", "")
		if word != "" {
			filtered = append(filtered, word)
		}
	}

	if len(filtered) == 0 {
		return ""
	}

	// Use OR logic for better recall - finding any matching term is better than nothing
	// Quote each word for safety, join with OR
	var quoted []string
	for _, w := range filtered {
		quoted = append(quoted, `"`+w+`"`)
	}
	return strings.Join(quoted, " OR ")
}

// SearchFTS performs full-text search using FTS5 with BM25 ranking.
// Returns nodes matching the query, ordered by relevance.
func (s *SQLiteStore) SearchFTS(query string, limit int) ([]FTSResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Sanitize query for FTS5 to prevent syntax errors and improve matching
	sanitizedQuery := sanitizeFTSQueryForNodes(query)
	if sanitizedQuery == "" {
		return nil, nil // Empty query returns no results
	}

	rows, err := s.db.Query(`
		SELECT n.id, n.content, n.type, n.summary, n.source_agent, n.embedding, n.created_at,
		       bm25(nodes_fts) as rank
		FROM nodes_fts f
		JOIN nodes n ON f.id = n.id
		WHERE nodes_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, sanitizedQuery, limit)
	if err != nil {
		// Return empty results with error so caller can decide how to handle
		// Common case: FTS table doesn't exist yet (returns "no such table")
		return nil, fmt.Errorf("FTS search failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []FTSResult
	for rows.Next() {
		var n Node
		var createdAt string
		var nodeType, summary, sourceAgent sql.NullString
		var embeddingBytes []byte
		var rank float64

		if err := rows.Scan(&n.ID, &n.Content, &nodeType, &summary, &sourceAgent, &embeddingBytes, &createdAt, &rank); err != nil {
			continue
		}
		populateNodeFromScan(&n, nodeType, summary, sourceAgent, createdAt, embeddingBytes)
		results = append(results, FTSResult{Node: n, Rank: rank})
	}

	return results, nil
}

// RebuildFTS rebuilds the FTS5 index from existing nodes.
// Call this after schema migration to populate FTS for existing data.
func (s *SQLiteStore) RebuildFTS() error {
	// First, clear the FTS index
	if _, err := s.db.Exec("DELETE FROM nodes_fts"); err != nil {
		return fmt.Errorf("clear fts index: %w", err)
	}

	// Repopulate from nodes table
	_, err := s.db.Exec(`
		INSERT INTO nodes_fts(rowid, id, summary, content)
		SELECT rowid, id, COALESCE(summary, ''), content FROM nodes
	`)
	if err != nil {
		return fmt.Errorf("rebuild fts index: %w", err)
	}

	return nil
}

// === Project Overview ===

// GetProjectOverview retrieves the project overview from the database.
// Returns nil if no overview exists yet.
func (s *SQLiteStore) GetProjectOverview() (*ProjectOverview, error) {
	row := s.db.QueryRow(`
		SELECT short_description, long_description, generated_at, last_edited_at
		FROM project_overview
		WHERE id = 1
	`)

	var overview ProjectOverview
	var generatedAt string
	var lastEditedAt sql.NullString

	err := row.Scan(&overview.ShortDescription, &overview.LongDescription, &generatedAt, &lastEditedAt)
	if err == sql.ErrNoRows {
		return nil, nil // No overview exists yet
	}
	if err != nil {
		return nil, fmt.Errorf("scan project overview: %w", err)
	}

	// Parse timestamps
	overview.GeneratedAt, _ = time.Parse(time.RFC3339, generatedAt)
	if lastEditedAt.Valid {
		overview.LastEditedAt, _ = time.Parse(time.RFC3339, lastEditedAt.String)
	}

	return &overview, nil
}

// SaveProjectOverview creates or updates the project overview.
// Uses INSERT OR REPLACE for upsert behavior on the singleton row.
func (s *SQLiteStore) SaveProjectOverview(overview *ProjectOverview) error {
	if overview == nil {
		return fmt.Errorf("overview cannot be nil")
	}

	// Validate required fields
	if strings.TrimSpace(overview.ShortDescription) == "" {
		return fmt.Errorf("short description cannot be empty")
	}
	if strings.TrimSpace(overview.LongDescription) == "" {
		return fmt.Errorf("long description cannot be empty")
	}

	// Set generated_at if not set
	if overview.GeneratedAt.IsZero() {
		overview.GeneratedAt = time.Now().UTC()
	}

	var lastEditedAt *string
	if !overview.LastEditedAt.IsZero() {
		edited := overview.LastEditedAt.Format(time.RFC3339)
		lastEditedAt = &edited
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO project_overview (id, short_description, long_description, generated_at, last_edited_at)
		VALUES (1, ?, ?, ?, ?)
	`, overview.ShortDescription, overview.LongDescription, overview.GeneratedAt.Format(time.RFC3339), lastEditedAt)

	if err != nil {
		return fmt.Errorf("save project overview: %w", err)
	}

	return nil
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

const (
	textSimilarityThreshold = 0.35
)

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
	"should": true, "may": true, "might": true, "must": true, "shall": true,
	"can": true, "need": true, "dare": true, "ought": true, "used": true,
	"it": true, "its": true, "this": true, "that": true, "these": true, "those": true,
	"which": true, "who": true, "whom": true, "where": true, "when": true, "why": true, "how": true,
	"all": true, "each": true, "every": true, "both": true, "few": true, "more": true,
	"most": true, "other": true, "some": true, "such": true, "no": true, "not": true,
	"only": true, "same": true, "so": true, "than": true, "too": true, "very": true,
	"just": true, "also": true, "now": true, "here": true, "there": true, "then": true,
}

func wordTokens(s string) map[string]bool {
	tokens := make(map[string]bool)
	s = strings.ToLower(s)
	// Replace hyphens and underscores with spaces before tokenizing
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for _, w := range words {
		if len(w) > 2 && !stopWords[w] {
			tokens[w] = true
		}
	}
	return tokens
}

func jaccardSimilarity(a, b string) float64 {
	tokensA := wordTokens(a)
	tokensB := wordTokens(b)

	if len(tokensA) == 0 && len(tokensB) == 0 {
		return 1.0
	}
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0.0
	}

	intersection := 0
	for token := range tokensA {
		if tokensB[token] {
			intersection++
		}
	}

	union := len(tokensA) + len(tokensB) - intersection
	return float64(intersection) / float64(union)
}

func textSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	return jaccardSimilarity(a, b)
}

// EmbeddingStats holds statistics about embeddings in the database.
type EmbeddingStats struct {
	TotalNodes             int  // Total number of nodes
	NodesWithEmbeddings    int  // Nodes that have embeddings
	NodesWithoutEmbeddings int  // Nodes missing embeddings
	EmbeddingDimension     int  // Dimension of embeddings (0 if none exist)
	MixedDimensions        bool // True if embeddings have different dimensions
}

// GetEmbeddingStats returns statistics about embeddings in the database.
// This is useful for validating embedding consistency and detecting dimension mismatches.
func (s *SQLiteStore) GetEmbeddingStats() (*EmbeddingStats, error) {
	stats := &EmbeddingStats{}

	// Count total nodes
	var totalCount int
	err := s.db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("count nodes: %w", err)
	}
	stats.TotalNodes = totalCount

	// Count nodes with embeddings
	var withEmbeddings int
	err = s.db.QueryRow("SELECT COUNT(*) FROM nodes WHERE embedding IS NOT NULL AND length(embedding) > 0").Scan(&withEmbeddings)
	if err != nil {
		return nil, fmt.Errorf("count nodes with embeddings: %w", err)
	}
	stats.NodesWithEmbeddings = withEmbeddings
	stats.NodesWithoutEmbeddings = totalCount - withEmbeddings

	if withEmbeddings == 0 {
		return stats, nil
	}

	// Get embedding dimensions by sampling a few embeddings
	// Embedding is stored as binary: 4 bytes per float32
	rows, err := s.db.Query(`
		SELECT length(embedding) / 4 as dim
		FROM nodes
		WHERE embedding IS NOT NULL AND length(embedding) > 0
		LIMIT 100
	`)
	if err != nil {
		return nil, fmt.Errorf("query embedding dimensions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	dimensions := make(map[int]bool)
	for rows.Next() {
		var dim int
		if err := rows.Scan(&dim); err != nil {
			return nil, fmt.Errorf("scan dimension: %w", err)
		}
		dimensions[dim] = true
	}

	// Check for mixed dimensions
	if len(dimensions) > 1 {
		stats.MixedDimensions = true
		// Return the first dimension found
		for dim := range dimensions {
			stats.EmbeddingDimension = dim
			break
		}
	} else if len(dimensions) == 1 {
		for dim := range dimensions {
			stats.EmbeddingDimension = dim
		}
	}

	return stats, nil
}
