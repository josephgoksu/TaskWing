/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"github.com/josephgoksu/taskwing.app/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// Archive represents a collection of archived tasks
type Archive struct {
	Version     string           `json:"version"`
	ArchivedAt  time.Time        `json:"archived_at"`
	ArchivedBy  string           `json:"archived_by"`
	Project     ProjectInfo      `json:"project"`
	Tasks       []ArchivedTask   `json:"tasks"`
	Retrospective Retrospective  `json:"retrospective,omitempty"`
	Patterns    []Pattern        `json:"patterns_identified,omitempty"`
	Metrics     ArchiveMetrics   `json:"metrics"`
}

// ProjectInfo contains project metadata
type ProjectInfo struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	DurationDays float64 `json:"duration_days"`
	Health       string  `json:"health"`
}

// ArchivedTask extends task with outcome information
type ArchivedTask struct {
	models.Task
	Outcome        TaskOutcome      `json:"outcome,omitempty"`
	LessonsLearned []string        `json:"lessons_learned,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
}

// TaskOutcome describes the result of a task
type TaskOutcome struct {
	Success bool                   `json:"success"`
	Notes   string                 `json:"notes,omitempty"`
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

// Retrospective captures project insights
type Retrospective struct {
	WhatWentWell  []string      `json:"what_went_well,omitempty"`
	WhatWentWrong []string      `json:"what_went_wrong,omitempty"`
	ActionItems   []string      `json:"action_items,omitempty"`
	KeyDecisions  []Decision    `json:"key_decisions,omitempty"`
}

// Decision represents a key decision made
type Decision struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
	Outcome   string `json:"outcome"`
}

// Pattern represents a reusable task pattern
type Pattern struct {
	Pattern        string   `json:"pattern"`
	Trigger        string   `json:"trigger"`
	Approach       string   `json:"approach"`
	SuccessFactors []string `json:"success_factors"`
}

// ArchiveMetrics contains project metrics
type ArchiveMetrics struct {
	TotalTasks       int     `json:"total_tasks"`
	CompletionRate   float64 `json:"completion_rate"`
	DurationHours    float64 `json:"duration_hours"`
	BlockersFound    int     `json:"blockers_encountered"`
	ReworkRequired   int     `json:"rework_required"`
}

// RetrospectiveMetrics contains calculated project metrics for retrospectives
type RetrospectiveMetrics struct {
	Duration          time.Duration  `json:"duration"`
	TaskCount         int            `json:"task_count"`
	CompletionRate    float64        `json:"completion_rate"`
	AvgTaskDuration   time.Duration  `json:"avg_task_duration"`
	PriorityBreakdown map[string]int `json:"priority_breakdown"`
	StatusBreakdown   map[string]int `json:"status_breakdown"`
}

// RetrospectiveData contains all data for generating a retrospective
type RetrospectiveData struct {
	Project     ProjectInfo   `json:"project"`
	Tasks       []models.Task `json:"tasks"`
	Metrics     RetrospectiveMetrics `json:"metrics"`
	Insights    UserInsights  `json:"insights"`
	GeneratedAt time.Time     `json:"generated_at"`
}

// UserInsights contains user-provided retrospective insights
type UserInsights struct {
	WhatWentWell    []string   `json:"what_went_well"`
	Challenges      []string   `json:"challenges"`
	KeyDecisions    []Decision `json:"key_decisions"`
	LessonsLearned  []string   `json:"lessons_learned"`
	ActionItems     []string   `json:"action_items"`
	PatternObserved string     `json:"pattern_observed"`
	SuccessFactors  []string   `json:"success_factors"`
}

// ArchiveIndex maintains a searchable index of archives
type ArchiveIndex struct {
	Version    string         `json:"version"`
	Archives   []ArchiveEntry `json:"archives"`
	Statistics IndexStats     `json:"statistics"`
	LastUpdated time.Time     `json:"last_updated"`
}

// ArchiveEntry is a summary of an archive
type ArchiveEntry struct {
	ID             string    `json:"id"`
	Date           string    `json:"date"`
	ProjectName    string    `json:"project_name"`
	FilePath       string    `json:"file_path"`
	TaskCount      int       `json:"task_count"`
	Tags           []string  `json:"tags"`
	Summary        string    `json:"summary"`
	CompletionRate float64   `json:"completion_rate"`
	DurationHours  float64   `json:"duration_hours"`
}

// IndexStats contains archive statistics
type IndexStats struct {
	TotalArchives     int      `json:"total_archives"`
	TotalTasksArchived int      `json:"total_tasks_archived"`
	MostCommonPatterns []string `json:"most_common_patterns"`
	AvgProjectDays    float64  `json:"average_project_duration_days"`
	AvgCompletionRate float64  `json:"average_completion_rate"`
}

var (
	projectName    string
	retrospective  bool
	extractPatterns bool
)

// archiveCmd represents the archive command
var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive completed tasks with metadata and lessons learned",
	Long: `Archive completed tasks to preserve project history, capture lessons learned,
and build a knowledge base for improving future task management.

Examples:
  taskwing archive                        # Interactive archive of completed tasks
  taskwing archive --project "Sprint 1"   # Archive with specific project name
  taskwing archive --retrospective        # Include retrospective prompts
  taskwing archive list                   # View archive history
  taskwing archive show <archive-id>      # View specific archive`,
	Run: runArchive,
}

// archiveListCmd lists all archives
var archiveListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all archives",
	Run:   runArchiveList,
}

// archiveShowCmd shows a specific archive
var archiveShowCmd = &cobra.Command{
	Use:   "show [archive-id]",
	Short: "Show details of a specific archive",
	Args:  cobra.ExactArgs(1),
	Run:   runArchiveShow,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
	archiveCmd.AddCommand(archiveListCmd)
	archiveCmd.AddCommand(archiveShowCmd)

	archiveCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name for the archive")
	archiveCmd.Flags().BoolVarP(&retrospective, "retrospective", "r", false, "Include retrospective prompts")
	archiveCmd.Flags().BoolVar(&extractPatterns, "extract-patterns", false, "Extract and save task patterns")
}

func runArchive(cmd *cobra.Command, args []string) {
	// Initialize task store
	cfg := GetConfig()
	taskStore, err := GetStore()
	if err != nil {
		HandleError("Error: Could not initialize the task store.", err)
		return
	}
	defer taskStore.Close()

	// Get completed tasks
	completedTasks, err := taskStore.ListTasks(
		func(t models.Task) bool {
			return t.Status == "completed"
		},
		nil,
	)
	if err != nil {
		fmt.Printf("Error fetching completed tasks: %v\n", err)
		return
	}

	if len(completedTasks) == 0 {
		fmt.Println("No completed tasks to archive.")
		return
	}

	// Show tasks to be archived
	fmt.Printf("\n📦 Tasks to Archive (%d tasks):\n", len(completedTasks))
	for _, task := range completedTasks {
		fmt.Printf("  • %s (%s)\n", task.Title, task.Priority)
	}

	// Get project name if not provided
	if projectName == "" {
		prompt := promptui.Prompt{
			Label:   "Project Name",
			Default: fmt.Sprintf("Project-%s", time.Now().Format("2006-01-02")),
		}
		projectName, err = prompt.Run()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	}

	// Get project description
	descPrompt := promptui.Prompt{
		Label: "Project Description (optional)",
	}
	projectDesc, _ := descPrompt.Run()

	// Create archive
	archive := Archive{
		Version:    "1.0",
		ArchivedAt: time.Now(),
		ArchivedBy: getArchiveUser(),
		Project: ProjectInfo{
			Name:         projectName,
			Description:  projectDesc,
			DurationDays: calculateProjectDuration(completedTasks),
			Health:       "completed",
		},
		Tasks:   make([]ArchivedTask, 0, len(completedTasks)),
		Metrics: calculateMetrics(completedTasks),
	}

	// Convert tasks to archived tasks
	for _, task := range completedTasks {
		archivedTask := ArchivedTask{
			Task: task,
		}

		// Optionally collect outcome for each task
		if retrospective {
			fmt.Printf("\n📝 Task: %s\n", task.Title)
			
			outcomePrompt := promptui.Prompt{
				Label: "Outcome notes (optional)",
			}
			outcome, _ := outcomePrompt.Run()
			if outcome != "" {
				archivedTask.Outcome = TaskOutcome{
					Success: true,
					Notes:   outcome,
				}
			}

			lessonPrompt := promptui.Prompt{
				Label: "Lessons learned (optional)",
			}
			lesson, _ := lessonPrompt.Run()
			if lesson != "" {
				archivedTask.LessonsLearned = []string{lesson}
			}
		}

		archive.Tasks = append(archive.Tasks, archivedTask)
	}

	// Collect retrospective if requested
	if retrospective {
		archive.Retrospective = collectRetrospective()
		
		// Also generate detailed retrospective document
		fmt.Println("\n📝 Generating detailed retrospective document...")
		retroData := RetrospectiveData{
			Project: ProjectInfo{
				Name:         projectName,
				Description:  projectDesc,
				DurationDays: calculateProjectDuration(completedTasks),
				Health:       "completed",
			},
			Tasks:       completedTasks,
			Metrics:     calculateProjectMetrics(completedTasks),
			Insights:    convertRetrospectiveToInsights(archive.Retrospective),
			GeneratedAt: time.Now(),
		}
		
		if err := saveRetrospective(retroData, cfg); err != nil {
			fmt.Printf("Warning: Could not save retrospective document: %v\n", err)
		} else {
			fmt.Printf("📄 Retrospective document saved to knowledge base\n")
		}
	}

	// Extract patterns if requested
	if extractPatterns {
		archive.Patterns = extractTaskPatterns(completedTasks)
	}

	// Save archive
	if err := saveArchive(archive, cfg); err != nil {
		fmt.Printf("Error saving archive: %v\n", err)
		return
	}

	// Update archive index
	if err := updateArchiveIndex(archive, cfg); err != nil {
		fmt.Printf("Error updating archive index: %v\n", err)
		return
	}

	// Delete archived tasks
	fmt.Print("\n🗑️  Clear completed tasks from active board? (y/N): ")
	var response string
	fmt.Scanln(&response)
	if response == "y" || response == "Y" {
		for _, task := range completedTasks {
			if err := taskStore.DeleteTask(task.ID); err != nil {
				fmt.Printf("Error deleting task %s: %v\n", task.ID, err)
			}
		}
		fmt.Printf("✅ Cleared %d completed tasks from active board\n", len(completedTasks))
	}

	fmt.Printf("\n✅ Archive created successfully!\n")
	fmt.Printf("📁 Location: %s\n", getArchiveFilePath(archive, cfg))
	fmt.Printf("📊 Tasks archived: %d\n", len(completedTasks))
	fmt.Printf("🎯 Completion rate: %.0f%%\n", archive.Metrics.CompletionRate)
}

func runArchiveList(cmd *cobra.Command, args []string) {
	cfg := GetConfig()
	index, err := loadArchiveIndex(cfg)
	if err != nil {
		fmt.Printf("Error loading archive index: %v\n", err)
		return
	}

	if len(index.Archives) == 0 {
		fmt.Println("No archives found.")
		return
	}

	fmt.Printf("\n📚 Archive History (%d archives):\n\n", len(index.Archives))
	for _, entry := range index.Archives {
		fmt.Printf("📦 %s - %s\n", entry.Date, entry.ProjectName)
		fmt.Printf("   Tasks: %d | Completion: %.0f%% | Duration: %.1fh\n", 
			entry.TaskCount, entry.CompletionRate, entry.DurationHours)
		fmt.Printf("   Summary: %s\n", entry.Summary)
		fmt.Printf("   ID: %s\n\n", entry.ID)
	}

	fmt.Printf("📊 Statistics:\n")
	fmt.Printf("   Total archives: %d\n", index.Statistics.TotalArchives)
	fmt.Printf("   Total tasks archived: %d\n", index.Statistics.TotalTasksArchived)
	fmt.Printf("   Average completion rate: %.0f%%\n", index.Statistics.AvgCompletionRate)
}

func runArchiveShow(cmd *cobra.Command, args []string) {
	cfg := GetConfig()
	archiveID := args[0]

	// Load archive index to find file path
	index, err := loadArchiveIndex(cfg)
	if err != nil {
		fmt.Printf("Error loading archive index: %v\n", err)
		return
	}

	var archivePath string
	for _, entry := range index.Archives {
		if entry.ID == archiveID || entry.Date == archiveID {
			archivePath = filepath.Join(cfg.Project.RootDir, "archive", entry.FilePath)
			break
		}
	}

	if archivePath == "" {
		fmt.Printf("Archive not found: %s\n", archiveID)
		return
	}

	// Load archive
	data, err := os.ReadFile(archivePath)
	if err != nil {
		fmt.Printf("Error reading archive: %v\n", err)
		return
	}

	var archive Archive
	if err := json.Unmarshal(data, &archive); err != nil {
		fmt.Printf("Error parsing archive: %v\n", err)
		return
	}

	// Display archive details
	fmt.Printf("\n📦 Archive: %s\n", archive.Project.Name)
	fmt.Printf("📅 Date: %s\n", archive.ArchivedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("📝 Description: %s\n", archive.Project.Description)
	fmt.Printf("\n📋 Tasks (%d):\n", len(archive.Tasks))
	
	for _, task := range archive.Tasks {
		fmt.Printf("\n  • %s [%s]\n", task.Title, task.Priority)
		if task.Outcome.Notes != "" {
			fmt.Printf("    Outcome: %s\n", task.Outcome.Notes)
		}
		if len(task.LessonsLearned) > 0 {
			fmt.Printf("    Lessons: %s\n", task.LessonsLearned[0])
		}
	}

	if len(archive.Retrospective.WhatWentWell) > 0 {
		fmt.Printf("\n✅ What Went Well:\n")
		for _, item := range archive.Retrospective.WhatWentWell {
			fmt.Printf("  • %s\n", item)
		}
	}

	if len(archive.Retrospective.WhatWentWrong) > 0 {
		fmt.Printf("\n❌ Challenges:\n")
		for _, item := range archive.Retrospective.WhatWentWrong {
			fmt.Printf("  • %s\n", item)
		}
	}

	fmt.Printf("\n📊 Metrics:\n")
	fmt.Printf("  • Completion Rate: %.0f%%\n", archive.Metrics.CompletionRate)
	fmt.Printf("  • Duration: %.1f hours\n", archive.Metrics.DurationHours)
	fmt.Printf("  • Blockers: %d\n", archive.Metrics.BlockersFound)
}

// Helper functions

func getArchiveUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "TaskWing User"
}

func calculateProjectDuration(tasks []models.Task) float64 {
	if len(tasks) == 0 {
		return 0
	}

	var earliest, latest time.Time
	for i, task := range tasks {
		if i == 0 || task.CreatedAt.Before(earliest) {
			earliest = task.CreatedAt
		}
		if task.CompletedAt != nil && (latest.IsZero() || task.CompletedAt.After(latest)) {
			latest = *task.CompletedAt
		}
	}

	if latest.IsZero() {
		latest = time.Now()
	}

	return latest.Sub(earliest).Hours() / 24.0
}

func calculateMetrics(tasks []models.Task) ArchiveMetrics {
	completed := 0
	for _, task := range tasks {
		if task.Status == "completed" {
			completed++
		}
	}

	completionRate := 100.0
	if len(tasks) > 0 {
		completionRate = float64(completed) / float64(len(tasks)) * 100
	}

	return ArchiveMetrics{
		TotalTasks:     len(tasks),
		CompletionRate: completionRate,
		DurationHours:  calculateProjectDuration(tasks) * 24,
		BlockersFound:  0, // TODO: Track blockers
		ReworkRequired: 0, // TODO: Track rework
	}
}

func collectRetrospective() Retrospective {
	retro := Retrospective{}

	fmt.Println("\n🔄 Project Retrospective")

	// What went well
	prompt := promptui.Prompt{
		Label: "What went well? (comma-separated)",
	}
	if wellStr, err := prompt.Run(); err == nil && wellStr != "" {
		retro.WhatWentWell = splitAndTrim(wellStr)
	}

	// What went wrong
	prompt = promptui.Prompt{
		Label: "What challenges did you face? (comma-separated)",
	}
	if wrongStr, err := prompt.Run(); err == nil && wrongStr != "" {
		retro.WhatWentWrong = splitAndTrim(wrongStr)
	}

	// Action items
	prompt = promptui.Prompt{
		Label: "Action items for next time? (comma-separated)",
	}
	if actionStr, err := prompt.Run(); err == nil && actionStr != "" {
		retro.ActionItems = splitAndTrim(actionStr)
	}

	return retro
}

func extractTaskPatterns(tasks []models.Task) []Pattern {
	// Simple pattern extraction - could be enhanced with ML
	patterns := []Pattern{}
	
	// Group tasks by similar titles/descriptions to find patterns
	// This is a placeholder implementation
	if len(tasks) > 3 {
		patterns = append(patterns, Pattern{
			Pattern:  "Task Group Pattern",
			Trigger:  "Multiple related tasks",
			Approach: "Sequential task completion",
			SuccessFactors: []string{
				"Clear task dependencies",
				"Consistent priority levels",
			},
		})
	}

	return patterns
}

func saveArchive(archive Archive, cfg *types.AppConfig) error {
	// Create archive directory structure
	archiveDir := filepath.Join(cfg.Project.RootDir, "archive", 
		time.Now().Format("2006"), time.Now().Format("01"))
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return err
	}

	// Generate filename
	filename := fmt.Sprintf("%s_%s.json", 
		time.Now().Format("2006-01-02"),
		sanitizeFilename(archive.Project.Name))
	archivePath := filepath.Join(archiveDir, filename)

	// Marshal archive to JSON
	data, err := json.MarshalIndent(archive, "", "  ")
	if err != nil {
		return err
	}

	// Write archive file
	return os.WriteFile(archivePath, data, 0644)
}

func updateArchiveIndex(archive Archive, cfg *types.AppConfig) error {
	// Load existing index or create new one
	index, _ := loadArchiveIndex(cfg)
	if index == nil {
		index = &ArchiveIndex{
			Version:  "1.0",
			Archives: []ArchiveEntry{},
		}
	}

	// Create archive entry
	entry := ArchiveEntry{
		ID:          fmt.Sprintf("%s-%s", archive.Project.Name, time.Now().Format("2006-01-02")),
		Date:        time.Now().Format("2006-01-02"),
		ProjectName: archive.Project.Name,
		FilePath: fmt.Sprintf("%s/%s/%s_%s.json",
			time.Now().Format("2006"),
			time.Now().Format("01"),
			time.Now().Format("2006-01-02"),
			sanitizeFilename(archive.Project.Name)),
		TaskCount:      len(archive.Tasks),
		Tags:           extractTags(archive.Tasks),
		Summary:        archive.Project.Description,
		CompletionRate: archive.Metrics.CompletionRate,
		DurationHours:  archive.Metrics.DurationHours,
	}

	// Add to index
	index.Archives = append(index.Archives, entry)
	index.Statistics.TotalArchives++
	index.Statistics.TotalTasksArchived += len(archive.Tasks)
	index.LastUpdated = time.Now()

	// Recalculate averages
	var totalCompletion float64
	var totalDuration float64
	for _, e := range index.Archives {
		totalCompletion += e.CompletionRate
		totalDuration += e.DurationHours
	}
	index.Statistics.AvgCompletionRate = totalCompletion / float64(len(index.Archives))
	index.Statistics.AvgProjectDays = (totalDuration / float64(len(index.Archives))) / 24.0

	// Save index
	indexPath := filepath.Join(cfg.Project.RootDir, "archive", "index.json")
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}

func loadArchiveIndex(cfg *types.AppConfig) (*ArchiveIndex, error) {
	indexPath := filepath.Join(cfg.Project.RootDir, "archive", "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ArchiveIndex{
				Version:  "1.0",
				Archives: []ArchiveEntry{},
			}, nil
		}
		return nil, err
	}

	var index ArchiveIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

func getArchiveFilePath(archive Archive, cfg *types.AppConfig) string {
	return filepath.Join(cfg.Project.RootDir, "archive",
		time.Now().Format("2006"),
		time.Now().Format("01"),
		fmt.Sprintf("%s_%s.json",
			time.Now().Format("2006-01-02"),
			sanitizeFilename(archive.Project.Name)))
}

func sanitizeFilename(name string) string {
	// Replace spaces and special characters with underscores
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
		   (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result += string(r)
		} else if r == ' ' {
			result += "_"
		}
	}
	return result
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := []string{}
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func extractTags(tasks []ArchivedTask) []string {
	tagMap := make(map[string]bool)
	for _, task := range tasks {
		for _, tag := range task.Tags {
			tagMap[tag] = true
		}
	}
	
	tags := []string{}
	for tag := range tagMap {
		tags = append(tags, tag)
	}
	return tags
}

// Helper function to convert Retrospective to UserInsights format
func convertRetrospectiveToInsights(retro Retrospective) UserInsights {
	insights := UserInsights{
		WhatWentWell:   retro.WhatWentWell,
		Challenges:     retro.WhatWentWrong,
		ActionItems:    retro.ActionItems,
		KeyDecisions:   retro.KeyDecisions,
	}
	
	// Convert key decisions to lessons learned
	for _, decision := range retro.KeyDecisions {
		lesson := fmt.Sprintf("%s - %s", decision.Decision, decision.Outcome)
		insights.LessonsLearned = append(insights.LessonsLearned, lesson)
	}
	
	return insights
}

// ProjectMetrics calculation for retrospectives
func calculateProjectMetrics(tasks []models.Task) RetrospectiveMetrics {
	var totalDuration time.Duration
	completed := 0
	priorityBreakdown := make(map[string]int)
	statusBreakdown := make(map[string]int)
	
	var projectStart, projectEnd time.Time
	
	for i, task := range tasks {
		// Track project timeline
		if i == 0 || task.CreatedAt.Before(projectStart) {
			projectStart = task.CreatedAt
		}
		if task.CompletedAt != nil && (projectEnd.IsZero() || task.CompletedAt.After(projectEnd)) {
			projectEnd = *task.CompletedAt
		}
		
		// Count completions
		if task.Status == "completed" {
			completed++
			if task.CompletedAt != nil {
				totalDuration += task.CompletedAt.Sub(task.CreatedAt)
			}
		}
		
		// Track breakdowns
		priorityBreakdown[string(task.Priority)]++
		statusBreakdown[string(task.Status)]++
	}
	
	completionRate := float64(completed) / float64(len(tasks)) * 100
	
	var avgTaskDuration time.Duration
	if completed > 0 {
		avgTaskDuration = totalDuration / time.Duration(completed)
	}
	
	var projectDuration time.Duration
	if !projectEnd.IsZero() {
		projectDuration = projectEnd.Sub(projectStart)
	}
	
	return RetrospectiveMetrics{
		Duration:          projectDuration,
		TaskCount:         len(tasks),
		CompletionRate:    completionRate,
		AvgTaskDuration:   avgTaskDuration,
		PriorityBreakdown: priorityBreakdown,
		StatusBreakdown:   statusBreakdown,
	}
}