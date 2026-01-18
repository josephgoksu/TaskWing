package memory

import (
	"github.com/josephgoksu/TaskWing/internal/task"
)

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

// SearchPlans returns plans matching query and status.
func (r *Repository) SearchPlans(query string, status task.PlanStatus) ([]task.Plan, error) {
	return r.db.SearchPlans(query, status)
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

// SetGitBaseline records the git state when a task was claimed.
func (r *Repository) SetGitBaseline(taskID string, baseline []string) error {
	return r.db.SetGitBaseline(taskID, baseline)
}

// CompleteTask marks a task as completed with summary and files modified.
func (r *Repository) CompleteTask(taskID, summary string, filesModified []string) error {
	return r.db.CompleteTask(taskID, summary, filesModified)
}

// GetActivePlan returns the currently active plan.
func (r *Repository) GetActivePlan() (*task.Plan, error) {
	return r.db.GetActivePlan()
}

// SetActivePlan sets the active plan.
func (r *Repository) SetActivePlan(id string) error {
	return r.db.SetActivePlan(id)
}

// UpdatePlanAuditReport updates the audit report and status for a plan.
func (r *Repository) UpdatePlanAuditReport(id string, status task.PlanStatus, auditReportJSON string) error {
	return r.db.UpdatePlanAuditReport(id, status, auditReportJSON)
}
