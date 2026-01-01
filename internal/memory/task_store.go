package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// === Plan CRUD ===

// CreatePlan creates a new plan in the database.
func (s *SQLiteStore) CreatePlan(p *task.Plan) error {
	if p.ID == "" {
		p.ID = "plan-" + uuid.New().String()[:8]
	}
	if p.Status == "" {
		p.Status = task.PlanStatusDraft
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	_, err := s.db.Exec(`
		INSERT INTO plans (id, goal, enriched_goal, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, p.ID, p.Goal, p.EnrichedGoal, p.Status, p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("insert plan: %w", err)
	}

	return nil
}

// GetPlan retrieves a plan by ID, including its tasks.
func (s *SQLiteStore) GetPlan(id string) (*task.Plan, error) {
	var p task.Plan
	var createdAt, updatedAt string

	err := s.db.QueryRow(`
		SELECT id, goal, enriched_goal, status, created_at, updated_at
		FROM plans WHERE id = ?
	`, id).Scan(&p.ID, &p.Goal, &p.EnrichedGoal, &p.Status, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("plan not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query plan: %w", err)
	}

	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Fetch tasks
	tasks, err := s.ListTasks(id)
	if err != nil {
		return nil, fmt.Errorf("list tasks for plan: %w", err)
	}
	p.Tasks = tasks

	return &p, nil
}

// ListPlans returns all plans.
func (s *SQLiteStore) ListPlans() ([]task.Plan, error) {
	rows, err := s.db.Query(`
		SELECT id, goal, enriched_goal, status, created_at, updated_at FROM plans ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query plans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var plans []task.Plan
	for rows.Next() {
		var p task.Plan
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Goal, &p.EnrichedGoal, &p.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		plans = append(plans, p)
	}
	// Note: We don't fetch tasks here to keep it lightweight. Use GetPlan for details.

	return plans, nil
}

// UpdatePlan updates mutable plan fields.
func (s *SQLiteStore) UpdatePlan(id string, goal, enrichedGoal string, status task.PlanStatus) error {
	if id == "" {
		return fmt.Errorf("plan id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)

	// Build dynamic update based on provided fields
	query := "UPDATE plans SET "
	args := []any{}
	sets := []string{}

	if goal != "" {
		sets = append(sets, "goal = ?")
		args = append(args, goal)
	}
	if enrichedGoal != "" {
		sets = append(sets, "enriched_goal = ?")
		args = append(args, enrichedGoal)
	}
	if status != "" {
		sets = append(sets, "status = ?")
		args = append(args, status)
	}
	// Always update updated_at when any field changes
	if len(sets) == 0 {
		return fmt.Errorf("no fields to update")
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, now)

	query += strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)

	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update plan: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update plan rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("plan not found: %s", id)
	}
	return nil
}

// DeletePlan removes a plan and its tasks (via FK cascade).
func (s *SQLiteStore) DeletePlan(id string) error {
	res, err := s.db.Exec(`DELETE FROM plans WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete plan rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("plan not found: %s", id)
	}
	return nil
}

// === Task CRUD ===

// CreateTask adds a new task to a plan.
func (s *SQLiteStore) CreateTask(t *task.Task) error {
	if t.ID == "" {
		t.ID = "task-" + uuid.New().String()[:8]
	}
	if t.Status == "" {
		t.Status = task.StatusPending
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now

	acJSON, err := json.Marshal(t.AcceptanceCriteria)
	if err != nil {
		return fmt.Errorf("marshal acceptance criteria: %w", err)
	}
	vsJSON, err := json.Marshal(t.ValidationSteps)
	if err != nil {
		return fmt.Errorf("marshal validation steps: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var parentID interface{}
	if t.ParentTaskID != "" {
		parentID = t.ParentTaskID
	} else {
		parentID = nil
	}

	_, err = tx.Exec(`
		INSERT INTO tasks (
			id, plan_id, title, description,
			acceptance_criteria, validation_steps,
			status, priority, assigned_agent, parent_task_id, context_summary,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.PlanID, t.Title, t.Description,
		string(acJSON), string(vsJSON),
		t.Status, t.Priority, t.AssignedAgent, parentID, t.ContextSummary,
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	// Insert Dependencies
	for _, depID := range t.Dependencies {
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO task_dependencies (task_id, depends_on) VALUES (?, ?)
		`, t.ID, depID)
		if err != nil {
			return fmt.Errorf("insert dependency %s: %w", depID, err)
		}
	}

	// Insert Context Nodes
	for _, nodeID := range t.ContextNodes {
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO task_node_links (task_id, node_id, link_type) VALUES (?, ?, 'context')
		`, t.ID, nodeID)
		if err != nil {
			return fmt.Errorf("insert node link %s: %w", nodeID, err)
		}
	}

	return tx.Commit()
}

// taskRowScanner abstracts row scanning for reuse between QueryRow and rows.Next()
type taskRowScanner interface {
	Scan(dest ...any) error
}

// scanTaskRow scans a task row into a Task struct (DRY helper).
func scanTaskRow(row taskRowScanner) (task.Task, error) {
	var t task.Task
	var desc, acJSON, vsJSON sql.NullString
	var parentID sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&t.ID, &t.PlanID, &t.Title, &desc, &acJSON, &vsJSON,
		&t.Status, &t.Priority, &t.AssignedAgent, &parentID, &t.ContextSummary,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return t, err
	}

	t.Description = desc.String
	t.ParentTaskID = parentID.String
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if acJSON.Valid && acJSON.String != "" {
		_ = json.Unmarshal([]byte(acJSON.String), &t.AcceptanceCriteria)
	}
	if vsJSON.Valid && vsJSON.String != "" {
		_ = json.Unmarshal([]byte(vsJSON.String), &t.ValidationSteps)
	}

	return t, nil
}

const taskSelectColumns = `id, plan_id, title, description, acceptance_criteria, validation_steps,
       status, priority, assigned_agent, parent_task_id, context_summary,
       created_at, updated_at`

// GetTask retrieves a task by ID.
func (s *SQLiteStore) GetTask(id string) (*task.Task, error) {
	row := s.db.QueryRow(`SELECT `+taskSelectColumns+` FROM tasks WHERE id = ?`, id)

	t, err := scanTaskRow(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query task: %w", err)
	}

	// Fetch dependencies
	deps, err := s.GetTaskDependencies(id)
	if err != nil {
		return nil, err
	}
	t.Dependencies = deps

	// Fetch context nodes
	nodes, err := s.GetTaskContextNodes(id)
	if err != nil {
		return nil, err
	}
	t.ContextNodes = nodes

	return &t, nil
}

// ListTasks returns all tasks for a plan.
func (s *SQLiteStore) ListTasks(planID string) ([]task.Task, error) {
	rows, err := s.db.Query(`SELECT `+taskSelectColumns+` FROM tasks WHERE plan_id = ? ORDER BY created_at`, planID)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tasks []task.Task
	var taskIDs []string
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
		taskIDs = append(taskIDs, t.ID)
	}

	// Batch fetch all dependencies in a single query (fixes N+1)
	if len(taskIDs) > 0 {
		depsMap, err := s.batchGetTaskDependencies(taskIDs)
		if err == nil {
			for i := range tasks {
				tasks[i].Dependencies = depsMap[tasks[i].ID]
			}
		}
	}

	return tasks, nil
}

// UpdateTaskStatus updates a task's status and updated_at timestamp.
func (s *SQLiteStore) UpdateTaskStatus(id string, status task.TaskStatus) error {
	if id == "" {
		return fmt.Errorf("task id is required")
	}
	if status == "" {
		return fmt.Errorf("status is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task status rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	return nil
}

// DeleteTask removes a task and its links.
func (s *SQLiteStore) DeleteTask(id string) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete task rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	return nil
}

// === Task Helpers ===

func (s *SQLiteStore) GetTaskDependencies(taskID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT depends_on FROM task_dependencies WHERE task_id = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query deps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var deps []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, nil
}

// batchGetTaskDependencies fetches dependencies for multiple tasks in a single query.
// Returns a map of task_id -> []depends_on. Fixes the N+1 query pattern in ListTasks.
func (s *SQLiteStore) batchGetTaskDependencies(taskIDs []string) (map[string][]string, error) {
	if len(taskIDs) == 0 {
		return make(map[string][]string), nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(taskIDs))
	args := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `SELECT task_id, depends_on FROM task_dependencies WHERE task_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch query deps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]string)
	for rows.Next() {
		var taskID, dependsOn string
		if err := rows.Scan(&taskID, &dependsOn); err != nil {
			return nil, err
		}
		result[taskID] = append(result[taskID], dependsOn)
	}
	return result, nil
}

func (s *SQLiteStore) GetTaskContextNodes(taskID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT node_id FROM task_node_links WHERE task_id = ? AND link_type='context'`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query context nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var nodes []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (s *SQLiteStore) LinkTaskToNode(taskID, nodeID, linkType string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO task_node_links (task_id, node_id, link_type) VALUES (?, ?, ?)
	`, taskID, nodeID, linkType)
	return err
}
