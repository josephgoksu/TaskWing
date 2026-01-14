package task

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Repository defines the data access methods required by the Task Service.
// This interface allows the service to be decoupled from the concrete memory implementation.
type Repository interface {
	GetPlan(id string) (*Plan, error)
	ListPlans() ([]Plan, error)
	CreatePlan(p *Plan) error
	UpdatePlan(id, goal, enrichedGoal string, status PlanStatus) error
	DeletePlan(id string) error

	ListTasks(planID string) ([]Task, error)
	GetTask(id string) (*Task, error)
	UpdateTaskStatus(id string, status TaskStatus) error
	AddDependency(taskID, dependsOn string) error
	RemoveDependency(taskID, dependsOn string) error

	// Active Plan Management (DB-based)
	SetActivePlan(id string) error
	GetActivePlan() (*Plan, error)

	// Search
	SearchPlans(query string, status PlanStatus) ([]Plan, error)

	// Audit
	UpdatePlanAuditReport(id string, status PlanStatus, auditReportJSON string) error
}

// Service encapsulates all business logic for managing Plans and Tasks.
// It handles higher-level operations, state management (active plan), and data formatting.
type Service struct {
	repo     Repository
	stateDir string
}

// NewService creates a new Task Service.
func NewService(repo Repository, stateDir string) *Service {
	return &Service{
		repo:     repo,
		stateDir: stateDir,
	}
}

// ResolveLatestPlanID finds the ID of the most recently created plan.
func (s *Service) ResolveLatestPlanID() (string, error) {
	plans, err := s.repo.ListPlans()
	if err != nil {
		return "", fmt.Errorf("list plans: %w", err)
	}
	if len(plans) == 0 {
		return "", fmt.Errorf("no plans found")
	}

	// Sort by CreatedAt descending
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].CreatedAt.After(plans[j].CreatedAt)
	})

	return plans[0].ID, nil
}

// ResolvePlanID resolves "latest" to the actual ID, or returns the given ID.
func (s *Service) ResolvePlanID(id string) (string, error) {
	if id == "latest" {
		return s.ResolveLatestPlanID()
	}
	return id, nil
}

// ListPlans returns all plans.
func (s *Service) ListPlans() ([]Plan, error) {
	return s.repo.ListPlans()
}

// SearchPlans filters plans by query and status.
func (s *Service) SearchPlans(query string, status PlanStatus) ([]Plan, error) {
	return s.repo.SearchPlans(query, status)
}

// GetPlan retrieves a plan by ID.
func (s *Service) GetPlan(id string) (*Plan, error) {
	realID, err := s.ResolvePlanID(id)
	if err != nil {
		return nil, err
	}
	return s.repo.GetPlan(realID)
}

// GetPlanWithTasks retrieves a plan and ensures its Tasks slice is populated.
func (s *Service) GetPlanWithTasks(id string) (*Plan, error) {
	plan, err := s.GetPlan(id)
	if err != nil {
		return nil, err
	}

	// Ensure tasks are loaded (Repository might not load them by default in GetPlan)
	// But based on current repo implementation, GetPlan loads tasks?
	// Let's explicitly load to be safe/future-proof, or reuse what's there.
	// Current memory.SqliteStore.GetPlan DOES join tasks.
	// But let's verify if we need to do anything else.
	// For now, assume repo returns fully hydrated plan.

	// However, if we want to be sure, we could call ListTasks and assign.
	if len(plan.Tasks) == 0 {
		tasks, err := s.repo.ListTasks(plan.ID)
		if err == nil {
			plan.Tasks = tasks
		}
	}

	return plan, nil
}

// DeletePlan deletes a plan and associated tasks.
func (s *Service) DeletePlan(id string) error {
	realID, err := s.ResolvePlanID(id)
	if err != nil {
		return err
	}

	// Logic check: prevent deleting the active plan?
	// Or maybe clear active plan if it is deleted.
	activeID, _ := s.GetActivePlanID()
	if activeID == realID {
		_ = s.ClearActivePlan()
	}

	return s.repo.DeletePlan(realID)
}

// UpdatePlan updates a plan's details.
func (s *Service) UpdatePlan(id, goal, enrichedGoal string, status PlanStatus) error {
	realID, err := s.ResolvePlanID(id)
	if err != nil {
		return err
	}
	return s.repo.UpdatePlan(realID, goal, enrichedGoal, status)
}

// RenamePlan is a convenience method for updating just the goal.
func (s *Service) RenamePlan(id, newGoal string) error {
	return s.UpdatePlan(id, newGoal, "", "")
}

// ArchivePlan sets the status to Archived.
func (s *Service) ArchivePlan(id string) error {
	return s.UpdatePlan(id, "", "", PlanStatusArchived)
}

// UnarchivePlan sets the status to Active.
func (s *Service) UnarchivePlan(id string) error {
	return s.UpdatePlan(id, "", "", PlanStatusActive)
}

// --- Active Plan State Management ---

// SetActivePlan delegates to repository for DB-level switch.
func (s *Service) SetActivePlan(id string) error {
	realID, err := s.ResolvePlanID(id)
	if err != nil {
		return err
	}
	return s.repo.SetActivePlan(realID)
}

// GetActivePlanID retrieves the ID of the currently active plan from DB.
func (s *Service) GetActivePlanID() (string, error) {
	plan, err := s.repo.GetActivePlan()
	if err != nil {
		return "", err
	}
	if plan == nil {
		return "", nil // No active plan
	}
	return plan.ID, nil
}

// ClearActivePlan deactivates the current plan (sets to draft).
func (s *Service) ClearActivePlan() error {
	plan, err := s.repo.GetActivePlan()
	if err != nil {
		return err
	}
	if plan == nil {
		return nil
	}
	// Demote to draft
	return s.repo.UpdatePlan(plan.ID, "", "", PlanStatusDraft)
}

// --- Export Logic ---

// FormatPlanMarkdown returns the plan as a markdown string.
func (s *Service) FormatPlanMarkdown(plan *Plan) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("# Plan: %s\n\n", plan.Goal))
	if plan.EnrichedGoal != "" {
		buf.WriteString(fmt.Sprintf("**Refined Goal**: %s\n\n", plan.EnrichedGoal))
	}

	for _, t := range plan.Tasks {
		buf.WriteString(fmt.Sprintf("## Task: %s\n", t.Title))
		buf.WriteString(fmt.Sprintf("**Priority**: %d | **Agent**: %s | **Status**: %s\n\n", t.Priority, t.AssignedAgent, t.Status))
		buf.WriteString(fmt.Sprintf("%s\n\n", t.Description))

		if len(t.AcceptanceCriteria) > 0 {
			buf.WriteString("### Acceptance Criteria\n")
			for _, ac := range t.AcceptanceCriteria {
				buf.WriteString(fmt.Sprintf("- [ ] %s\n", ac))
			}
			buf.WriteString("\n")
		}

		if len(t.ValidationSteps) > 0 {
			buf.WriteString("### Validation\n")
			buf.WriteString("```bash\n")
			for _, vs := range t.ValidationSteps {
				buf.WriteString(fmt.Sprintf("%s\n", vs))
			}
			buf.WriteString("```\n\n")
		}
	}

	return buf.String()
}

// ExportPlanToFile writes the plan markdown to a file.
// If validPath is empty, it generates a default filename in .taskwing/plans/.
func (s *Service) ExportPlanToFile(plan *Plan, customPath string) (string, error) {
	content := s.FormatPlanMarkdown(plan)

	var finalPath string
	if customPath != "" {
		finalPath = customPath
	} else {
		// Generate slug from goal
		slug := strings.ToLower(plan.Goal)
		slug = strings.ReplaceAll(slug, " ", "-")
		reg, _ := regexp.Compile("[^a-z0-9-]")
		slug = reg.ReplaceAllString(slug, "")
		if len(slug) > 50 {
			slug = slug[:50]
		}

		finalPath = filepath.Join(s.stateDir, "plans", fmt.Sprintf("%s-%s.md", plan.ID, slug))
	}

	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return "", err
	}

	if err := os.WriteFile(finalPath, []byte(content), 0644); err != nil {
		return "", err
	}

	return finalPath, nil
}
