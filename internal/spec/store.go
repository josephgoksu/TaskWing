package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Store manages spec and task persistence
type Store struct {
	basePath string // path to .taskwing/specs/
}

// NewStore creates a new spec store
func NewStore(projectPath string) (*Store, error) {
	basePath := filepath.Join(projectPath, ".taskwing", "specs")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create specs directory: %w", err)
	}
	return &Store{basePath: basePath}, nil
}

// CreateSpec creates a new spec and saves it
func (s *Store) CreateSpec(title, description string) (*Spec, error) {
	id := "spec-" + uuid.New().String()[:8]
	slug := slugify(title)

	spec := &Spec{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      StatusDraft,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	// Create spec directory
	specDir := filepath.Join(s.basePath, slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return nil, fmt.Errorf("create spec directory: %w", err)
	}

	// Save spec
	if err := s.saveSpec(spec, specDir); err != nil {
		return nil, err
	}

	return spec, nil
}

// SaveSpec saves a spec to disk
func (s *Store) SaveSpec(spec *Spec) error {
	slug := slugify(spec.Title)
	specDir := filepath.Join(s.basePath, slug)
	return s.saveSpec(spec, specDir)
}

func (s *Store) saveSpec(spec *Spec, specDir string) error {
	spec.UpdatedAt = time.Now().UTC()

	// Save JSON
	jsonPath := filepath.Join(specDir, "spec.json")
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return fmt.Errorf("write spec.json: %w", err)
	}

	// Save markdown
	mdPath := filepath.Join(specDir, "spec.md")
	md := s.specToMarkdown(spec)
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("write spec.md: %w", err)
	}

	// Save tasks if present
	if len(spec.Tasks) > 0 {
		tasksPath := filepath.Join(specDir, "tasks.json")
		tasksData, err := json.MarshalIndent(spec.Tasks, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal tasks: %w", err)
		}
		if err := os.WriteFile(tasksPath, tasksData, 0644); err != nil {
			return fmt.Errorf("write tasks.json: %w", err)
		}
	}

	return nil
}

// GetSpec loads a spec by slug or ID
func (s *Store) GetSpec(slugOrID string) (*Spec, error) {
	// Try direct slug first
	specDir := filepath.Join(s.basePath, slugOrID)
	jsonPath := filepath.Join(specDir, "spec.json")

	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		// Try to find by ID - scan all specs
		entries, dirErr := os.ReadDir(s.basePath)
		if dirErr != nil {
			return nil, fmt.Errorf("spec not found: %s", slugOrID)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			spec, loadErr := s.loadSpecFromDir(filepath.Join(s.basePath, entry.Name()))
			if loadErr != nil {
				continue
			}
			if spec.ID == slugOrID {
				return spec, nil
			}
		}
		return nil, fmt.Errorf("spec not found: %s", slugOrID)
	}

	return s.loadSpecFromDir(specDir)
}

func (s *Store) loadSpecFromDir(specDir string) (*Spec, error) {
	jsonPath := filepath.Join(specDir, "spec.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("read spec.json: %w", err)
	}

	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec: %w", err)
	}

	return &spec, nil
}

// ListSpecs returns summaries of all specs
func (s *Store) ListSpecs() ([]SpecSummary, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read specs directory: %w", err)
	}

	var specs []SpecSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		spec, err := s.loadSpecFromDir(filepath.Join(s.basePath, entry.Name()))
		if err != nil {
			continue // Skip invalid specs
		}

		specs = append(specs, SpecSummary{
			ID:        spec.ID,
			Title:     spec.Title,
			Status:    spec.Status,
			TaskCount: len(spec.Tasks),
			CreatedAt: spec.CreatedAt,
		})
	}

	// Sort by creation date (newest first)
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].CreatedAt.After(specs[j].CreatedAt)
	})

	return specs, nil
}

// ListTasks returns all tasks, optionally filtered by spec
func (s *Store) ListTasks(specSlug string) ([]TaskSummary, error) {
	var tasks []TaskSummary

	if specSlug != "" {
		// Load tasks from specific spec
		spec, err := s.GetSpec(specSlug)
		if err != nil {
			return nil, err
		}
		for _, t := range spec.Tasks {
			tasks = append(tasks, TaskSummary{
				ID:       t.ID,
				SpecID:   t.SpecID,
				Title:    t.Title,
				Status:   t.Status,
				Priority: t.Priority,
				Estimate: t.Estimate,
			})
		}
	} else {
		// Load tasks from all specs
		specs, err := s.ListSpecs()
		if err != nil {
			return nil, err
		}
		for _, summary := range specs {
			spec, err := s.GetSpec(slugify(summary.Title))
			if err != nil {
				continue
			}
			for _, t := range spec.Tasks {
				tasks = append(tasks, TaskSummary{
					ID:       t.ID,
					SpecID:   t.SpecID,
					Title:    t.Title,
					Status:   t.Status,
					Priority: t.Priority,
					Estimate: t.Estimate,
				})
			}
		}
	}

	// Sort by priority
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Priority < tasks[j].Priority
	})

	return tasks, nil
}

// GetTask loads a task by ID
func (s *Store) GetTask(taskID string) (*Task, *Spec, error) {
	specs, err := s.ListSpecs()
	if err != nil {
		return nil, nil, err
	}

	for _, summary := range specs {
		spec, err := s.GetSpec(slugify(summary.Title))
		if err != nil {
			continue
		}
		for i := range spec.Tasks {
			if spec.Tasks[i].ID == taskID {
				return &spec.Tasks[i], spec, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("task not found: %s", taskID)
}

// UpdateTaskStatus updates a task's status
func (s *Store) UpdateTaskStatus(taskID string, status Status) error {
	// We need to find and update the task within the spec's Tasks slice
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		spec, loadErr := s.loadSpecFromDir(filepath.Join(s.basePath, entry.Name()))
		if loadErr != nil {
			continue
		}
		for i := range spec.Tasks {
			if spec.Tasks[i].ID == taskID {
				spec.Tasks[i].Status = status // Update in place
				return s.SaveSpec(spec)
			}
		}
	}

	return fmt.Errorf("task not found: %s", taskID)
}

// GetTaskContext returns full context for a task (for AI tools)
func (s *Store) GetTaskContext(taskID string) (*TaskContext, error) {
	task, spec, err := s.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	return &TaskContext{
		Task:        *task,
		SpecTitle:   spec.Title,
		SpecContent: s.specToMarkdown(spec),
	}, nil
}

func (s *Store) specToMarkdown(spec *Spec) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", spec.Title))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", spec.Status))
	sb.WriteString(fmt.Sprintf("**Created:** %s\n\n", spec.CreatedAt.Format("2006-01-02")))

	if spec.Description != "" {
		sb.WriteString("## Description\n\n")
		sb.WriteString(spec.Description)
		sb.WriteString("\n\n")
	}

	if spec.PMAnalysis != "" {
		sb.WriteString("---\n\n# PM Analysis\n\n")
		sb.WriteString(spec.PMAnalysis)
		sb.WriteString("\n\n")
	}

	if spec.ArchitectAnalysis != "" {
		sb.WriteString("---\n\n# Architect Analysis\n\n")
		sb.WriteString(spec.ArchitectAnalysis)
		sb.WriteString("\n\n")
	}

	if spec.EngineerAnalysis != "" {
		sb.WriteString("---\n\n# Engineer Analysis\n\n")
		sb.WriteString(spec.EngineerAnalysis)
		sb.WriteString("\n\n")
	}

	if spec.QAAnalysis != "" {
		sb.WriteString("---\n\n# QA Analysis\n\n")
		sb.WriteString(spec.QAAnalysis)
		sb.WriteString("\n\n")
	}

	if len(spec.Tasks) > 0 {
		sb.WriteString("---\n\n# Tasks\n\n")
		for _, t := range spec.Tasks {
			status := "[ ]"
			if t.Status == StatusDone {
				status = "[x]"
			} else if t.Status == StatusInProgress {
				status = "[/]"
			}
			sb.WriteString(fmt.Sprintf("- %s **%s** (%s) - %s\n", status, t.Title, t.Estimate, t.Description))
		}
	}

	return sb.String()
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug := result.String()
	slug = strings.Trim(slug, "-")
	if len(slug) > 50 {
		slug = slug[:50]
	}
	return slug
}
