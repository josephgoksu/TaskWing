package memory

import (
	"context"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/task"
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
	return r.db.Repair()
}

// Close closes the underlying database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}

// === Task Repository Methods (delegate to SQLiteStore) ===

// AddDependency adds a dependency relationship between two tasks.
func (r *Repository) AddDependency(taskID, dependsOn string) error {
	return r.db.AddDependency(taskID, dependsOn)
}

// RemoveDependency removes a dependency relationship between two tasks.
func (r *Repository) RemoveDependency(taskID, dependsOn string) error {
	return r.db.RemoveDependency(taskID, dependsOn)
}

// FindTaskIDsByPrefix returns all task IDs that start with the given prefix.
// The ctx parameter is accepted for interface compatibility but not currently used.
func (r *Repository) FindTaskIDsByPrefix(ctx context.Context, prefix string) ([]string, error) {
	return r.db.FindTaskIDsByPrefix(prefix)
}

// FindPlanIDsByPrefix returns all plan IDs that start with the given prefix.
// The ctx parameter is accepted for interface compatibility but not currently used.
func (r *Repository) FindPlanIDsByPrefix(ctx context.Context, prefix string) ([]string, error) {
	return r.db.FindPlanIDsByPrefix(prefix)
}

// === Phase Repository Methods (delegate to SQLiteStore) ===

// CreatePhase creates a new phase.
func (r *Repository) CreatePhase(p *task.Phase) error {
	return r.db.CreatePhase(p)
}

// GetPhase retrieves a phase by ID.
func (r *Repository) GetPhase(id string) (*task.Phase, error) {
	return r.db.GetPhase(id)
}

// ListPhases returns all phases for a plan.
func (r *Repository) ListPhases(planID string) ([]task.Phase, error) {
	return r.db.ListPhases(planID)
}

// UpdatePhase updates a phase.
func (r *Repository) UpdatePhase(p *task.Phase) error {
	return r.db.UpdatePhase(p)
}

// UpdatePhaseStatus updates the status of a phase.
func (r *Repository) UpdatePhaseStatus(id string, status task.PhaseStatus) error {
	return r.db.UpdatePhaseStatus(id, status)
}

// DeletePhase deletes a phase.
func (r *Repository) DeletePhase(id string) error {
	return r.db.DeletePhase(id)
}

// CreatePhasesForPlan creates multiple phases for a plan atomically.
func (r *Repository) CreatePhasesForPlan(planID string, phases []task.Phase) error {
	return r.db.CreatePhasesForPlan(planID, phases)
}

// ListTasksByPhase returns all tasks for a phase.
func (r *Repository) ListTasksByPhase(phaseID string) ([]task.Task, error) {
	return r.db.ListTasksByPhase(phaseID)
}

// GetPlanWithPhases retrieves a plan with its phases.
func (r *Repository) GetPlanWithPhases(id string) (*task.Plan, error) {
	return r.db.GetPlanWithPhases(id)
}

// UpdatePlanDraftState updates the draft state JSON for a plan.
func (r *Repository) UpdatePlanDraftState(planID string, draftStateJSON string) error {
	return r.db.UpdatePlanDraftState(planID, draftStateJSON)
}

// UpdatePlanGenerationMode updates the generation mode for a plan.
func (r *Repository) UpdatePlanGenerationMode(planID string, mode task.GenerationMode) error {
	return r.db.UpdatePlanGenerationMode(planID, mode)
}

// === Clarify Session Repository Methods (delegate to SQLiteStore) ===

// CreateClarifySession creates a persisted clarify session.
func (r *Repository) CreateClarifySession(session *task.ClarifySession) error {
	return r.db.CreateClarifySession(session)
}

// GetClarifySession retrieves a clarify session by ID.
func (r *Repository) GetClarifySession(id string) (*task.ClarifySession, error) {
	return r.db.GetClarifySession(id)
}

// UpdateClarifySession updates persisted clarify session state.
func (r *Repository) UpdateClarifySession(session *task.ClarifySession) error {
	return r.db.UpdateClarifySession(session)
}

// CreateClarifyTurn persists a single clarify round turn.
func (r *Repository) CreateClarifyTurn(turn *task.ClarifyTurn) error {
	return r.db.CreateClarifyTurn(turn)
}

// ListClarifyTurns returns all clarify turns for a session.
func (r *Repository) ListClarifyTurns(sessionID string) ([]task.ClarifyTurn, error) {
	return r.db.ListClarifyTurns(sessionID)
}
