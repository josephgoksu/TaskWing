package memory

import (
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

// LinkNodes creates an edge between two nodes in the knowledge graph.
func (r *Repository) LinkNodes(from, to, relation string, confidence float64, properties map[string]any) error {
	return r.db.LinkNodes(from, to, relation, confidence, properties)
}

// GetAllNodeEdges returns all edges in the knowledge graph.
func (r *Repository) GetAllNodeEdges() ([]NodeEdge, error) {
	return r.db.GetAllNodeEdges()
}

// GetNodeEdges returns all edges connected to a specific node.
func (r *Repository) GetNodeEdges(nodeID string) ([]NodeEdge, error) {
	return r.db.GetNodeEdges(nodeID)
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

func (r *Repository) UpdateNode(id, content, nodeType, summary string) error {
	return r.db.UpdateNode(id, content, nodeType, summary)
}

func (r *Repository) UpdateNodeEmbedding(id string, embedding []float32) error {
	return r.db.UpdateNodeEmbedding(id, embedding)
}

func (r *Repository) DeleteNode(id string) error {
	return r.db.DeleteNode(id)
}

func (r *Repository) DeleteNodesByType(nodeType string) (int64, error) {
	return r.db.DeleteNodesByType(nodeType)
}

func (r *Repository) DeleteNodesByAgent(agent string) error {
	return r.db.DeleteNodesByAgent(agent)
}

func (r *Repository) DeleteNodesByFiles(agent string, filePaths []string) error {
	return r.db.DeleteNodesByFiles(agent, filePaths)
}

// GetNodesByFiles returns nodes from a specific agent that reference any of the given files.
func (r *Repository) GetNodesByFiles(agent string, filePaths []string) ([]Node, error) {
	return r.db.GetNodesByFiles(agent, filePaths)
}

func (r *Repository) UpsertNodeBySummary(n Node) error {
	return r.db.UpsertNodeBySummary(n)
}

// ClearAllKnowledge removes all nodes, edges, features, decisions, and patterns.
// Used for clean-slate re-bootstrapping.
func (r *Repository) ClearAllKnowledge() error {
	return r.db.ClearAllKnowledge()
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

// === Task & Plan Management ===

func (r *Repository) CreatePlan(p *task.Plan) error {
	return r.db.CreatePlan(p)
}

func (r *Repository) GetPlan(id string) (*task.Plan, error) {
	return r.db.GetPlan(id)
}

func (r *Repository) ListPlans() ([]task.Plan, error) {
	return r.db.ListPlans()
}

func (r *Repository) UpdatePlan(id string, goal, enrichedGoal string, status task.PlanStatus) error {
	return r.db.UpdatePlan(id, goal, enrichedGoal, status)
}

func (r *Repository) DeletePlan(id string) error {
	return r.db.DeletePlan(id)
}

func (r *Repository) CreateTask(t *task.Task) error {
	return r.db.CreateTask(t)
}

func (r *Repository) GetTask(id string) (*task.Task, error) {
	return r.db.GetTask(id)
}

func (r *Repository) ListTasks(planID string) ([]task.Task, error) {
	return r.db.ListTasks(planID)
}

func (r *Repository) UpdateTaskStatus(id string, status task.TaskStatus) error {
	return r.db.UpdateTaskStatus(id, status)
}

func (r *Repository) DeleteTask(id string) error {
	return r.db.DeleteTask(id)
}

// === Task Lifecycle (for MCP tools) ===

// GetNextTask returns the highest priority pending task from a plan.
func (r *Repository) GetNextTask(planID string) (*task.Task, error) {
	return r.db.GetNextTask(planID)
}

// GetCurrentTask returns the in-progress task claimed by a session.
func (r *Repository) GetCurrentTask(sessionID string) (*task.Task, error) {
	return r.db.GetCurrentTask(sessionID)
}

// GetAnyInProgressTask returns any in-progress task from a plan.
func (r *Repository) GetAnyInProgressTask(planID string) (*task.Task, error) {
	return r.db.GetAnyInProgressTask(planID)
}

// ClaimTask marks a task as in_progress and assigns it to a session.
func (r *Repository) ClaimTask(taskID, sessionID string) error {
	return r.db.ClaimTask(taskID, sessionID)
}

// CompleteTask marks a task as completed with summary and files modified.
func (r *Repository) CompleteTask(taskID, summary string, filesModified []string) error {
	return r.db.CompleteTask(taskID, summary, filesModified)
}

// GetActivePlan returns the currently active plan.
func (r *Repository) GetActivePlan() (*task.Plan, error) {
	return r.db.GetActivePlan()
}

// UpdatePlanAuditReport updates the audit report and status for a plan.
func (r *Repository) UpdatePlanAuditReport(id string, status task.PlanStatus, auditReportJSON string) error {
	return r.db.UpdatePlanAuditReport(id, status, auditReportJSON)
}

// === FTS5 Hybrid Search ===

// ListNodesWithEmbeddings returns all nodes with embeddings in a single query.
// This fixes the N+1 query pattern - one query instead of 1+N.
func (r *Repository) ListNodesWithEmbeddings() ([]Node, error) {
	return r.db.ListNodesWithEmbeddings()
}

// SearchFTS performs full-text search using FTS5 with BM25 ranking.
func (r *Repository) SearchFTS(query string, limit int) ([]FTSResult, error) {
	return r.db.SearchFTS(query, limit)
}

// RebuildFTS rebuilds the FTS5 index from existing nodes.
func (r *Repository) RebuildFTS() error {
	return r.db.RebuildFTS()
}

// Close closes the underlying database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}
