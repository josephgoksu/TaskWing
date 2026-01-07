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

// NewDefaultRepository creates a Repository with standard SQLite and Markdown stores.
func NewDefaultRepository(basePath string) (*Repository, error) {
	db, err := NewSQLiteStore(basePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}
	files := NewMarkdownStore(basePath)
	return NewRepository(db, files), nil
}

// GetDB returns the underlying SQLiteStore (temporary helper during refactor)
func (r *Repository) GetDB() *SQLiteStore {
	return r.db
}

// Check performs integrity checks on the repository.
func (r *Repository) Check() ([]Issue, error) {
	return r.db.Check()
}

// Repair attempts to fix integrity issues in the repository.
func (r *Repository) Repair() error {
	// 1. Repair DB issues
	if err := r.db.Repair(); err != nil {
		return err
	}
	// 2. Ensuring files match DB
	return r.RebuildFiles()
}

// Close closes the underlying database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}
