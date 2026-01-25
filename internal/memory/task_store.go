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

// nullTimeString returns nil for zero time, RFC3339 string otherwise
func nullTimeString(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

// txExecutor abstracts sql.Tx for task insertion (enables DRY)
type txExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// insertTaskTx inserts a task and its relations within a transaction.
// This is the SINGLE source of truth for task insertion logic.
func insertTaskTx(tx txExecutor, t *task.Task) error {
	acJSON, _ := json.Marshal(t.AcceptanceCriteria)
	vsJSON, _ := json.Marshal(t.ValidationSteps)
	keywordsJSON, _ := json.Marshal(t.Keywords)
	queriesJSON, _ := json.Marshal(t.SuggestedRecallQueries)
	filesJSON, _ := json.Marshal(t.FilesModified)
	expectedFilesJSON, _ := json.Marshal(t.ExpectedFiles)

	var parentID interface{}
	if t.ParentTaskID != "" {
		parentID = t.ParentTaskID
	}

	_, err := tx.Exec(`
		INSERT INTO tasks (
			id, plan_id, title, description,
			acceptance_criteria, validation_steps,
			status, priority, complexity, assigned_agent, parent_task_id, context_summary,
			scope, keywords, suggested_recall_queries,
			claimed_by, claimed_at, completed_at, completion_summary, files_modified, expected_files,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.PlanID, t.Title, t.Description,
		string(acJSON), string(vsJSON),
		t.Status, t.Priority, t.Complexity, t.AssignedAgent, parentID, t.ContextSummary,
		t.Scope, string(keywordsJSON), string(queriesJSON),
		t.ClaimedBy, nullTimeString(t.ClaimedAt), nullTimeString(t.CompletedAt), t.CompletionSummary, string(filesJSON), string(expectedFilesJSON),
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert task %s: %w", t.Title, err)
	}

	for _, depID := range t.Dependencies {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO task_dependencies (task_id, depends_on) VALUES (?, ?)`, t.ID, depID); err != nil {
			return fmt.Errorf("insert dependency %s: %w", depID, err)
		}
	}

	for _, nodeID := range t.ContextNodes {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO task_node_links (task_id, node_id, link_type) VALUES (?, ?, 'context')`, t.ID, nodeID); err != nil {
			return fmt.Errorf("insert node link %s: %w", nodeID, err)
		}
	}

	return nil
}

// prepareTask sets default values for a task before insertion
func prepareTask(t *task.Task, planID string, now time.Time) {
	t.PlanID = planID
	if t.ID == "" {
		t.ID = "task-" + uuid.New().String()[:8]
	}
	if t.Status == "" {
		t.Status = task.StatusPending
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
}

// === Plan CRUD ===

// CreatePlan creates a new plan in the database along with its tasks (atomically).
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

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.Exec(`
		INSERT INTO plans (id, goal, enriched_goal, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, p.ID, p.Goal, p.EnrichedGoal, p.Status, p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("insert plan: %w", err)
	}

	for i := range p.Tasks {
		prepareTask(&p.Tasks[i], p.ID, now)
		if err := insertTaskTx(tx, &p.Tasks[i]); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetPlan retrieves a plan by ID, including its tasks.
func (s *SQLiteStore) GetPlan(id string) (*task.Plan, error) {
	var p task.Plan
	var createdAt, updatedAt string
	var lastAuditReport sql.NullString

	err := s.db.QueryRow(`
		SELECT id, goal, enriched_goal, status, created_at, updated_at, last_audit_report
		FROM plans WHERE id = ?
	`, id).Scan(&p.ID, &p.Goal, &p.EnrichedGoal, &p.Status, &createdAt, &updatedAt, &lastAuditReport)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("plan not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query plan: %w", err)
	}

	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastAuditReport.Valid {
		p.LastAuditReport = lastAuditReport.String
	}

	// Fetch tasks
	tasks, err := s.ListTasks(id)
	if err != nil {
		return nil, fmt.Errorf("list tasks for plan: %w", err)
	}
	p.Tasks = tasks

	return &p, nil
}

// ListPlans returns all plans with task counts (but not full task data).
func (s *SQLiteStore) ListPlans() ([]task.Plan, error) {
	// Use a subquery to get task counts efficiently without loading all task data
	rows, err := s.db.Query(`
		SELECT p.id, p.goal, p.enriched_goal, p.status, p.created_at, p.updated_at, p.last_audit_report,
		       (SELECT COUNT(*) FROM tasks t WHERE t.plan_id = p.id) as task_count
		FROM plans p
		ORDER BY p.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query plans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var plans []task.Plan
	for rows.Next() {
		var p task.Plan
		var createdAt, updatedAt string
		var lastAuditReport sql.NullString
		var taskCount int
		if err := rows.Scan(&p.ID, &p.Goal, &p.EnrichedGoal, &p.Status, &createdAt, &updatedAt, &lastAuditReport, &taskCount); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if lastAuditReport.Valid {
			p.LastAuditReport = lastAuditReport.String
		}
		// Store task count in a placeholder slice (just for count display)
		// This avoids loading all tasks but allows len(p.Tasks) to work
		p.Tasks = make([]task.Task, taskCount)
		plans = append(plans, p)
	}
	if err := checkRowsErr(rows); err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}

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

// UpdatePlanAuditReport updates the audit report and status for a plan.
// This is called by the audit agent after verification completes.
// UpdatePlanAuditReport updates the audit report and status for a plan.
// Also records the audit in history.
func (s *SQLiteStore) UpdatePlanAuditReport(id string, status task.PlanStatus, auditReportJSON string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)

	// 1. Update Plan
	res, err := tx.Exec(`
		UPDATE plans
		SET status = ?, last_audit_report = ?, updated_at = ?
		WHERE id = ?
	`, status, auditReportJSON, now, id)
	if err != nil {
		return fmt.Errorf("update plan audit: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("plan not found: %s", id)
	}

	// 2. Insert into History (store raw status string if needed, or mapped)
	// We store the plan status as the audit status for now.
	_, err = tx.Exec(`
		INSERT INTO plan_audit_histories (plan_id, status, report_json, created_at)
		VALUES (?, ?, ?, ?)
	`, id, status, auditReportJSON, now)
	if err != nil {
		return fmt.Errorf("insert audit history: %w", err)
	}

	return tx.Commit()
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
	prepareTask(t, t.PlanID, time.Now().UTC())

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertTaskTx(tx, t); err != nil {
		return err
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
	var scope, keywordsJSON, queriesJSON, complexity sql.NullString
	var claimedBy, claimedAt, completedAt, completionSummary, filesJSON, expectedFilesJSON, gitBaselineJSON sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&t.ID, &t.PlanID, &t.Title, &desc, &acJSON, &vsJSON,
		&t.Status, &t.Priority, &complexity, &t.AssignedAgent, &parentID, &t.ContextSummary,
		&scope, &keywordsJSON, &queriesJSON,
		&claimedBy, &claimedAt, &completedAt, &completionSummary, &filesJSON, &expectedFilesJSON, &gitBaselineJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return t, err
	}

	t.Description = desc.String
	t.Complexity = complexity.String
	t.ParentTaskID = parentID.String
	t.Scope = scope.String
	t.ClaimedBy = claimedBy.String
	t.CompletionSummary = completionSummary.String
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if claimedAt.Valid && claimedAt.String != "" {
		t.ClaimedAt, _ = time.Parse(time.RFC3339, claimedAt.String)
	}
	if completedAt.Valid && completedAt.String != "" {
		t.CompletedAt, _ = time.Parse(time.RFC3339, completedAt.String)
	}

	if acJSON.Valid && acJSON.String != "" {
		_ = json.Unmarshal([]byte(acJSON.String), &t.AcceptanceCriteria)
	}
	if vsJSON.Valid && vsJSON.String != "" {
		_ = json.Unmarshal([]byte(vsJSON.String), &t.ValidationSteps)
	}
	if keywordsJSON.Valid && keywordsJSON.String != "" {
		_ = json.Unmarshal([]byte(keywordsJSON.String), &t.Keywords)
	}
	if queriesJSON.Valid && queriesJSON.String != "" {
		_ = json.Unmarshal([]byte(queriesJSON.String), &t.SuggestedRecallQueries)
	}
	if filesJSON.Valid && filesJSON.String != "" {
		_ = json.Unmarshal([]byte(filesJSON.String), &t.FilesModified)
	}
	if expectedFilesJSON.Valid && expectedFilesJSON.String != "" {
		_ = json.Unmarshal([]byte(expectedFilesJSON.String), &t.ExpectedFiles)
	}
	if gitBaselineJSON.Valid && gitBaselineJSON.String != "" {
		_ = json.Unmarshal([]byte(gitBaselineJSON.String), &t.GitBaseline)
	}

	return t, nil
}

const taskSelectColumns = `id, plan_id, title, description, acceptance_criteria, validation_steps,
       status, priority, complexity, assigned_agent, parent_task_id, context_summary,
       scope, keywords, suggested_recall_queries,
       claimed_by, claimed_at, completed_at, completion_summary, files_modified, expected_files, git_baseline,
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
	if err := checkRowsErr(rows); err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
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

// AddDependency adds a dependency relationship between two tasks.
func (s *SQLiteStore) AddDependency(taskID, dependsOn string) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO task_dependencies (task_id, depends_on) VALUES (?, ?)`, taskID, dependsOn)
	return err
}

// RemoveDependency removes a dependency relationship between two tasks.
func (s *SQLiteStore) RemoveDependency(taskID, dependsOn string) error {
	_, err := s.db.Exec(`DELETE FROM task_dependencies WHERE task_id = ? AND depends_on = ?`, taskID, dependsOn)
	return err
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
	if err := checkRowsErr(rows); err != nil {
		return nil, fmt.Errorf("get task dependencies: %w", err)
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
	if err := checkRowsErr(rows); err != nil {
		return nil, fmt.Errorf("batch get task dependencies: %w", err)
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
	if err := checkRowsErr(rows); err != nil {
		return nil, fmt.Errorf("get task context nodes: %w", err)
	}
	return nodes, nil
}

func (s *SQLiteStore) LinkTaskToNode(taskID, nodeID, linkType string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO task_node_links (task_id, node_id, link_type) VALUES (?, ?, ?)
	`, taskID, nodeID, linkType)
	return err
}

// === Task Lifecycle Methods (for MCP tools) ===

// GetNextTask returns the highest priority pending task from a plan whose dependencies are all completed.
// Returns nil if no pending tasks exist or all pending tasks have incomplete dependencies.
func (s *SQLiteStore) GetNextTask(planID string) (*task.Task, error) {
	// Find pending tasks that have NO incomplete dependencies
	// A task is ready if:
	// 1. It has no dependencies, OR
	// 2. All its dependencies have status = 'completed'
	row := s.db.QueryRow(`
		SELECT `+taskSelectColumns+`
		FROM tasks t
		WHERE t.plan_id = ? AND t.status = ?
		AND NOT EXISTS (
			SELECT 1 FROM task_dependencies td
			JOIN tasks dep ON dep.id = td.depends_on
			WHERE td.task_id = t.id AND dep.status != ?
		)
		ORDER BY t.priority DESC, t.created_at ASC
		LIMIT 1
	`, planID, task.StatusPending, task.StatusCompleted)

	t, err := scanTaskRow(row)
	if err == sql.ErrNoRows {
		return nil, nil // No ready pending tasks
	}
	if err != nil {
		return nil, fmt.Errorf("query next task: %w", err)
	}

	// Fetch dependencies
	deps, err := s.GetTaskDependencies(t.ID)
	if err != nil {
		return nil, err
	}
	t.Dependencies = deps

	// Also fetch context nodes for consistency with GetCurrentTask
	nodes, err := s.GetTaskContextNodes(t.ID)
	if err != nil {
		return nil, err
	}
	t.ContextNodes = nodes

	return &t, nil
}

// GetCurrentTask returns the in_progress task claimed by a session.
// Returns nil if no task is currently claimed by this session.
func (s *SQLiteStore) GetCurrentTask(sessionID string) (*task.Task, error) {
	row := s.db.QueryRow(`
		SELECT `+taskSelectColumns+`
		FROM tasks
		WHERE claimed_by = ? AND status = ?
		LIMIT 1
	`, sessionID, task.StatusInProgress)

	t, err := scanTaskRow(row)
	if err == sql.ErrNoRows {
		return nil, nil // No current task
	}
	if err != nil {
		return nil, fmt.Errorf("query current task: %w", err)
	}

	// Fetch dependencies and context nodes
	deps, err := s.GetTaskDependencies(t.ID)
	if err != nil {
		return nil, err
	}
	t.Dependencies = deps

	nodes, err := s.GetTaskContextNodes(t.ID)
	if err != nil {
		return nil, err
	}
	t.ContextNodes = nodes

	return &t, nil
}

// GetAnyInProgressTask returns any in_progress task from a plan (regardless of session).
// Useful for resuming work or detecting stuck tasks.
func (s *SQLiteStore) GetAnyInProgressTask(planID string) (*task.Task, error) {
	row := s.db.QueryRow(`
		SELECT `+taskSelectColumns+`
		FROM tasks
		WHERE plan_id = ? AND status = ?
		ORDER BY claimed_at DESC
		LIMIT 1
	`, planID, task.StatusInProgress)

	t, err := scanTaskRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query in-progress task: %w", err)
	}

	deps, err := s.GetTaskDependencies(t.ID)
	if err != nil {
		return nil, err
	}
	t.Dependencies = deps

	return &t, nil
}

// ClaimTask marks a task as in_progress and assigns it to a session.
// Fails if task is not in pending status.
func (s *SQLiteStore) ClaimTask(taskID, sessionID string) error {
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// Only allow claiming pending tasks
	res, err := s.db.Exec(`
		UPDATE tasks
		SET status = ?, claimed_by = ?, claimed_at = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, task.StatusInProgress, sessionID, nowStr, nowStr, taskID, task.StatusPending)

	if err != nil {
		return fmt.Errorf("claim task: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("claim task rows affected: %w", err)
	}
	if affected == 0 {
		// Check if task exists
		var status task.TaskStatus
		err := s.db.QueryRow(`SELECT status FROM tasks WHERE id = ?`, taskID).Scan(&status)
		if err == sql.ErrNoRows {
			return fmt.Errorf("task not found: %s", taskID)
		}
		return fmt.Errorf("cannot claim task: current status is %s (must be pending)", status)
	}

	return nil
}

// SetGitBaseline records the git state when a task was claimed.
// This allows accurate comparison of what changed during task execution.
func (s *SQLiteStore) SetGitBaseline(taskID string, baseline []string) error {
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}

	baselineJSON, err := json.Marshal(baseline)
	if err != nil {
		return fmt.Errorf("marshal git baseline: %w", err)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		UPDATE tasks
		SET git_baseline = ?, updated_at = ?
		WHERE id = ?
	`, string(baselineJSON), nowStr, taskID)

	if err != nil {
		return fmt.Errorf("set git baseline: %w", err)
	}

	return nil
}

// CompleteTask marks a task as completed with summary and files modified.
func (s *SQLiteStore) CompleteTask(taskID, summary string, filesModified []string) error {
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	filesJSON, err := json.Marshal(filesModified)
	if err != nil {
		return fmt.Errorf("marshal files modified: %w", err)
	}

	// Only allow completing in_progress tasks
	res, err := s.db.Exec(`
		UPDATE tasks
		SET status = ?, completed_at = ?, completion_summary = ?, files_modified = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, task.StatusCompleted, nowStr, summary, string(filesJSON), nowStr, taskID, task.StatusInProgress)

	if err != nil {
		return fmt.Errorf("complete task: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("complete task rows affected: %w", err)
	}
	if affected == 0 {
		var status task.TaskStatus
		err := s.db.QueryRow(`SELECT status FROM tasks WHERE id = ?`, taskID).Scan(&status)
		if err == sql.ErrNoRows {
			return fmt.Errorf("task not found: %s", taskID)
		}
		return fmt.Errorf("cannot complete task: current status is %s (must be in_progress)", status)
	}

	return nil
}

// SearchPlans returns plans matching the query and status (with task counts).
// Query searches in goal and enriched_goal.
func (s *SQLiteStore) SearchPlans(query string, status task.PlanStatus) ([]task.Plan, error) {
	q := `SELECT p.id, p.goal, p.enriched_goal, p.status, p.created_at, p.updated_at,
	             (SELECT COUNT(*) FROM tasks t WHERE t.plan_id = p.id) as task_count
	      FROM plans p WHERE 1=1`
	args := []any{}

	if status != "" {
		q += " AND p.status = ?"
		args = append(args, status)
	}

	if query != "" {
		q += " AND (p.goal LIKE ? OR p.enriched_goal LIKE ?)"
		wildcard := "%" + query + "%"
		args = append(args, wildcard, wildcard)
	}

	q += " ORDER BY p.updated_at DESC"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search plans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var plans []task.Plan
	for rows.Next() {
		var p task.Plan
		var createdAt, updatedAt string
		var taskCount int
		if err := rows.Scan(&p.ID, &p.Goal, &p.EnrichedGoal, &p.Status, &createdAt, &updatedAt, &taskCount); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		// Store task count in placeholder slice for display
		p.Tasks = make([]task.Task, taskCount)
		plans = append(plans, p)
	}
	if err := checkRowsErr(rows); err != nil {
		return nil, fmt.Errorf("search plans: %w", err)
	}

	return plans, nil
}

// GetActivePlan returns the currently active plan (status = active).
// Returns nil if no active plan exists.
func (s *SQLiteStore) GetActivePlan() (*task.Plan, error) {
	var p task.Plan
	var createdAt, updatedAt string
	var lastAuditReport sql.NullString

	err := s.db.QueryRow(`
		SELECT id, goal, enriched_goal, status, created_at, updated_at, last_audit_report
		FROM plans WHERE status = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, task.PlanStatusActive).Scan(&p.ID, &p.Goal, &p.EnrichedGoal, &p.Status, &createdAt, &updatedAt, &lastAuditReport)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query active plan: %w", err)
	}

	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastAuditReport.Valid {
		p.LastAuditReport = lastAuditReport.String
	}

	// Fetch tasks
	tasks, err := s.ListTasks(p.ID)
	if err != nil {
		return nil, fmt.Errorf("list tasks for plan: %w", err)
	}
	p.Tasks = tasks

	return &p, nil
}

// SetActivePlan atomically sets the given plan as active and deactivates others.
func (s *SQLiteStore) SetActivePlan(id string) error {
	if id == "" {
		return fmt.Errorf("plan id is required")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)

	// 1. Demote any currently active plans to 'draft'
	// Note: Ideally we'd have a 'paused' state, but 'draft' works for now to ensure exclusivity.
	// We check if it's NOT the target plan to avoid unnecessary updates if re-setting same plan.
	_, err = tx.Exec(`
		UPDATE plans
		SET status = ?, updated_at = ?
		WHERE status = ? AND id != ?
	`, task.PlanStatusDraft, now, task.PlanStatusActive, id)
	if err != nil {
		return fmt.Errorf("demote active plans: %w", err)
	}

	// 2. Promote target plan to 'active'
	res, err := tx.Exec(`
		UPDATE plans
		SET status = ?, updated_at = ?
		WHERE id = ?
	`, task.PlanStatusActive, now, id)
	if err != nil {
		return fmt.Errorf("activate plan: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("activate plan rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("plan not found: %s", id)
	}

	return tx.Commit()
}

// FindTaskIDsByPrefix returns all task IDs that start with the given prefix.
// Results are ordered by ID for consistent output.
func (s *SQLiteStore) FindTaskIDsByPrefix(prefix string) ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM tasks WHERE id LIKE ? ORDER BY id`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("find task IDs by prefix: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan task ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := checkRowsErr(rows); err != nil {
		return nil, err
	}

	return ids, nil
}

// FindPlanIDsByPrefix returns all plan IDs that start with the given prefix.
// Results are ordered by ID for consistent output.
func (s *SQLiteStore) FindPlanIDsByPrefix(prefix string) ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM plans WHERE id LIKE ? ORDER BY id`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("find plan IDs by prefix: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan plan ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := checkRowsErr(rows); err != nil {
		return nil, err
	}

	return ids, nil
}
