/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// These types are defined in archive.go to avoid duplication

// retrospectiveCmd represents the retrospective command
var retrospectiveCmd = &cobra.Command{
	Use:   "retrospective",
	Short: "Generate project retrospective from completed tasks",
	Long: `Generate a comprehensive project retrospective by analyzing completed tasks
and collecting insights through interactive prompts.

This command creates structured retrospective documentation that captures:
- Project metrics and outcomes
- What went well and what could be improved
- Key decisions made and their impacts
- Lessons learned for future projects
- Patterns and success factors identified

The retrospective is saved to the knowledge base for future reference.`,
	Run: runRetrospective,
}

func init() {
	rootCmd.AddCommand(retrospectiveCmd)
}

func runRetrospective(cmd *cobra.Command, args []string) {
	cfg := GetConfig()
	taskStore, err := GetStore()
	if err != nil {
		HandleError("Error: Could not initialize the task store.", err)
		return
	}
	defer taskStore.Close()

	// Get completed tasks for retrospective
	completedTasks, err := taskStore.ListTasks(
		func(t models.Task) bool {
			return t.Status == "completed"
		},
		func(tasks []models.Task) []models.Task {
			// Sort by completion date
			for i := 0; i < len(tasks)-1; i++ {
				for j := i + 1; j < len(tasks); j++ {
					if tasks[i].CompletedAt != nil && tasks[j].CompletedAt != nil &&
						tasks[i].CompletedAt.Before(*tasks[j].CompletedAt) {
						tasks[i], tasks[j] = tasks[j], tasks[i]
					}
				}
			}
			return tasks
		},
	)
	if err != nil {
		fmt.Printf("Error fetching completed tasks: %v\n", err)
		return
	}

	if len(completedTasks) == 0 {
		fmt.Println("No completed tasks found for retrospective.")
		return
	}

	fmt.Printf("\n📋 Retrospective Analysis\n")
	fmt.Printf("Found %d completed tasks for analysis\n\n", len(completedTasks))

	// Get project information
	projectInfo := collectProjectInfo(completedTasks)
	
	// Calculate metrics
	metrics := calculateProjectMetrics(completedTasks)
	
	// Display metrics summary
	displayMetricsSummary(metrics, projectInfo)
	
	// Collect user insights
	insights := collectUserInsights()
	
	// Generate retrospective
	retroData := RetrospectiveData{
		Project:     projectInfo,
		Tasks:       completedTasks,
		Metrics:     metrics,
		Insights:    insights,
		GeneratedAt: time.Now(),
	}
	
	// Save retrospective
	if err := saveRetrospective(retroData, cfg); err != nil {
		fmt.Printf("Error saving retrospective: %v\n", err)
		return
	}
	
	// Update knowledge base
	if err := updateKnowledgeBase(retroData, cfg); err != nil {
		fmt.Printf("Warning: Could not update knowledge base: %v\n", err)
	}
	
	fmt.Printf("\n✅ Retrospective generated successfully!\n")
	fmt.Printf("📁 Location: %s\n", getRetrospectiveFilePath(retroData, cfg))
	fmt.Printf("📚 Knowledge base updated with insights\n")
}

func collectProjectInfo(tasks []models.Task) ProjectInfo {
	// Try to get project name from user
	prompt := promptui.Prompt{
		Label:   "Project Name",
		Default: fmt.Sprintf("Project-%s", time.Now().Format("2006-01-02")),
	}
	
	name, err := prompt.Run()
	if err != nil {
		name = fmt.Sprintf("Project-%s", time.Now().Format("2006-01-02"))
	}
	
	// Get project description
	descPrompt := promptui.Prompt{
		Label: "Project Description",
	}
	description, _ := descPrompt.Run()
	
	// Calculate duration
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
	
	duration := latest.Sub(earliest).Hours() / 24.0
	
	return ProjectInfo{
		Name:         name,
		Description:  description,
		DurationDays: duration,
		Health:       "completed",
	}
}

// calculateProjectMetrics is defined in archive.go

func displayMetricsSummary(metrics RetrospectiveMetrics, project ProjectInfo) {
	fmt.Printf("📊 Project Metrics Summary\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Project: %s\n", project.Name)
	fmt.Printf("Duration: %.1f days (%.1f hours)\n", project.DurationDays, metrics.Duration.Hours())
	fmt.Printf("Tasks: %d total\n", metrics.TaskCount)
	fmt.Printf("Completion Rate: %.1f%%\n", metrics.CompletionRate)
	if metrics.AvgTaskDuration > 0 {
		fmt.Printf("Average Task Duration: %.1f hours\n", metrics.AvgTaskDuration.Hours())
	}
	
	fmt.Printf("\nPriority Breakdown:\n")
	for priority, count := range metrics.PriorityBreakdown {
		percentage := float64(count) / float64(metrics.TaskCount) * 100
		fmt.Printf("  %s: %d tasks (%.1f%%)\n", priority, count, percentage)
	}
	
	fmt.Printf("\nStatus Breakdown:\n")
	for status, count := range metrics.StatusBreakdown {
		percentage := float64(count) / float64(metrics.TaskCount) * 100
		fmt.Printf("  %s: %d tasks (%.1f%%)\n", status, count, percentage)
	}
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

func collectUserInsights() UserInsights {
	insights := UserInsights{}
	
	fmt.Printf("🔄 Project Retrospective - Please share your insights:\n\n")
	
	// What went well
	wellPrompt := promptui.Prompt{
		Label: "What went well? (comma-separated)",
	}
	if wellStr, err := wellPrompt.Run(); err == nil && wellStr != "" {
		insights.WhatWentWell = splitAndTrimItems(wellStr)
	}
	
	// Challenges
	challengePrompt := promptui.Prompt{
		Label: "What challenges did you face? (comma-separated)",
	}
	if challengeStr, err := challengePrompt.Run(); err == nil && challengeStr != "" {
		insights.Challenges = splitAndTrimItems(challengeStr)
	}
	
	// Lessons learned
	lessonPrompt := promptui.Prompt{
		Label: "Key lessons learned? (comma-separated)",
	}
	if lessonStr, err := lessonPrompt.Run(); err == nil && lessonStr != "" {
		insights.LessonsLearned = splitAndTrimItems(lessonStr)
	}
	
	// Success factors
	successPrompt := promptui.Prompt{
		Label: "What were the key success factors? (comma-separated)",
	}
	if successStr, err := successPrompt.Run(); err == nil && successStr != "" {
		insights.SuccessFactors = splitAndTrimItems(successStr)
	}
	
	// Action items
	actionPrompt := promptui.Prompt{
		Label: "Action items for future projects? (comma-separated)",
	}
	if actionStr, err := actionPrompt.Run(); err == nil && actionStr != "" {
		insights.ActionItems = splitAndTrimItems(actionStr)
	}
	
	// Pattern observed
	patternPrompt := promptui.Prompt{
		Label: "Did you follow a specific pattern or approach?",
	}
	if patternStr, err := patternPrompt.Run(); err == nil && patternStr != "" {
		insights.PatternObserved = patternStr
	}
	
	// Key decisions (optional detailed collection)
	fmt.Printf("\n💡 Key Decisions (optional - press Enter to skip)\n")
	decisionPrompt := promptui.Prompt{
		Label: "Describe a key decision made",
	}
	if decisionStr, err := decisionPrompt.Run(); err == nil && decisionStr != "" {
		rationalePrompt := promptui.Prompt{
			Label: "What was the rationale?",
		}
		rationale, _ := rationalePrompt.Run()
		
		outcomePrompt := promptui.Prompt{
			Label: "What was the outcome?",
		}
		outcome, _ := outcomePrompt.Run()
		
		insights.KeyDecisions = append(insights.KeyDecisions, Decision{
			Decision:  decisionStr,
			Rationale: rationale,
			Outcome:   outcome,
		})
	}
	
	return insights
}

func saveRetrospective(data RetrospectiveData, cfg *AppConfig) error {
	// Create retrospectives directory
	retroDir := filepath.Join(cfg.Project.RootDir, "knowledge", "retrospectives")
	if err := os.MkdirAll(retroDir, 0755); err != nil {
		return err
	}
	
	// Generate filename
	filename := fmt.Sprintf("%s_%s.md", 
		data.GeneratedAt.Format("2006-01-02"),
		sanitizeFilename(data.Project.Name))
	retroPath := filepath.Join(retroDir, filename)
	
	// Generate retrospective content
	content := generateRetrospectiveMarkdown(data)
	
	// Write retrospective file
	return os.WriteFile(retroPath, []byte(content), 0644)
}

func generateRetrospectiveMarkdown(data RetrospectiveData) string {
	var content strings.Builder
	
	// Header
	content.WriteString(fmt.Sprintf("# Project Retrospective: %s\n\n", data.Project.Name))
	content.WriteString(fmt.Sprintf("**Project**: %s  \n", data.Project.Name))
	content.WriteString(fmt.Sprintf("**Date**: %s  \n", data.GeneratedAt.Format("2006-01-02")))
	content.WriteString(fmt.Sprintf("**Duration**: %.1f days (%.1f hours)  \n", data.Project.DurationDays, data.Metrics.Duration.Hours()))
	content.WriteString(fmt.Sprintf("**Tasks Completed**: %d/%d (%.1f%%)  \n", 
		len(data.Tasks), data.Metrics.TaskCount, data.Metrics.CompletionRate))
	
	if data.Insights.PatternObserved != "" {
		content.WriteString(fmt.Sprintf("**Pattern Used**: %s\n\n", data.Insights.PatternObserved))
	} else {
		content.WriteString("\n")
	}
	
	// Project Summary
	content.WriteString("## Project Summary\n\n")
	if data.Project.Description != "" {
		content.WriteString(fmt.Sprintf("%s\n\n", data.Project.Description))
	} else {
		content.WriteString("Project completed successfully with all objectives met.\n\n")
	}
	
	// What Went Well
	if len(data.Insights.WhatWentWell) > 0 {
		content.WriteString("## What Went Well ✅\n\n")
		for _, item := range data.Insights.WhatWentWell {
			content.WriteString(fmt.Sprintf("- %s\n", item))
		}
		content.WriteString("\n")
	}
	
	// Challenges
	if len(data.Insights.Challenges) > 0 {
		content.WriteString("## Challenges Faced ❌\n\n")
		for _, item := range data.Insights.Challenges {
			content.WriteString(fmt.Sprintf("- %s\n", item))
		}
		content.WriteString("\n")
	}
	
	// Key Decisions
	if len(data.Insights.KeyDecisions) > 0 {
		content.WriteString("## Key Decisions Made 🎯\n\n")
		for i, decision := range data.Insights.KeyDecisions {
			content.WriteString(fmt.Sprintf("### %d. %s\n", i+1, decision.Decision))
			if decision.Rationale != "" {
				content.WriteString(fmt.Sprintf("- **Rationale**: %s\n", decision.Rationale))
			}
			if decision.Outcome != "" {
				content.WriteString(fmt.Sprintf("- **Outcome**: %s\n", decision.Outcome))
			}
			content.WriteString("\n")
		}
	}
	
	// Lessons Learned
	if len(data.Insights.LessonsLearned) > 0 {
		content.WriteString("## Lessons Learned 🧠\n\n")
		for _, lesson := range data.Insights.LessonsLearned {
			content.WriteString(fmt.Sprintf("- %s\n", lesson))
		}
		content.WriteString("\n")
	}
	
	// Success Factors
	if len(data.Insights.SuccessFactors) > 0 {
		content.WriteString("## Success Factors 🚀\n\n")
		for _, factor := range data.Insights.SuccessFactors {
			content.WriteString(fmt.Sprintf("- %s\n", factor))
		}
		content.WriteString("\n")
	}
	
	// Action Items
	if len(data.Insights.ActionItems) > 0 {
		content.WriteString("## Action Items for Future Projects 📋\n\n")
		for _, item := range data.Insights.ActionItems {
			content.WriteString(fmt.Sprintf("- [ ] %s\n", item))
		}
		content.WriteString("\n")
	}
	
	// Metrics
	content.WriteString("## Project Metrics 📊\n\n")
	content.WriteString(fmt.Sprintf("- **Total Tasks**: %d\n", data.Metrics.TaskCount))
	content.WriteString(fmt.Sprintf("- **Completion Rate**: %.1f%%\n", data.Metrics.CompletionRate))
	content.WriteString(fmt.Sprintf("- **Project Duration**: %.1f days\n", data.Project.DurationDays))
	if data.Metrics.AvgTaskDuration > 0 {
		content.WriteString(fmt.Sprintf("- **Average Task Duration**: %.1f hours\n", data.Metrics.AvgTaskDuration.Hours()))
	}
	
	content.WriteString("\n### Priority Distribution\n")
	for priority, count := range data.Metrics.PriorityBreakdown {
		percentage := float64(count) / float64(data.Metrics.TaskCount) * 100
		content.WriteString(fmt.Sprintf("- **%s**: %d tasks (%.1f%%)\n", 
			strings.Title(priority), count, percentage))
	}
	
	// Task Details
	content.WriteString("\n## Task Details\n\n")
	for _, task := range data.Tasks {
		content.WriteString(fmt.Sprintf("### %s\n", task.Title))
		content.WriteString(fmt.Sprintf("- **Priority**: %s\n", task.Priority))
		content.WriteString(fmt.Sprintf("- **Status**: %s\n", task.Status))
		if task.CompletedAt != nil {
			duration := task.CompletedAt.Sub(task.CreatedAt)
			content.WriteString(fmt.Sprintf("- **Duration**: %.1f hours\n", duration.Hours()))
		}
		if task.Description != "" {
			content.WriteString(fmt.Sprintf("- **Description**: %s\n", task.Description))
		}
		content.WriteString("\n")
	}
	
	// Footer
	content.WriteString("---\n\n")
	content.WriteString(fmt.Sprintf("*Retrospective generated on %s*\n", 
		data.GeneratedAt.Format("2006-01-02 15:04:05")))
	
	return content.String()
}

func updateKnowledgeBase(data RetrospectiveData, cfg *AppConfig) error {
	// This would update the main KNOWLEDGE.md file with insights from this retrospective
	// For now, we'll create a simple update - in a full implementation,
	// this would parse and update the existing knowledge base
	
	knowledgePath := filepath.Join(cfg.Project.RootDir, "..", "KNOWLEDGE.md")
	
	// Read existing knowledge base
	existingContent, err := os.ReadFile(knowledgePath)
	if err != nil {
		return err // Knowledge base doesn't exist or can't be read
	}
	
	// For now, just append a note about the new retrospective
	// In a full implementation, this would intelligently merge insights
	updateNote := fmt.Sprintf("\n\n<!-- Auto-updated: %s -->\n", time.Now().Format("2006-01-02 15:04:05"))
	updateNote += fmt.Sprintf("**New Retrospective Added**: %s - %s\n", 
		data.GeneratedAt.Format("2006-01-02"), data.Project.Name)
	
	updatedContent := string(existingContent) + updateNote
	
	return os.WriteFile(knowledgePath, []byte(updatedContent), 0644)
}

func getRetrospectiveFilePath(data RetrospectiveData, cfg *AppConfig) string {
	return filepath.Join(cfg.Project.RootDir, "knowledge", "retrospectives",
		fmt.Sprintf("%s_%s.md", 
			data.GeneratedAt.Format("2006-01-02"),
			sanitizeFilename(data.Project.Name)))
}

func splitAndTrimItems(s string) []string {
	parts := strings.Split(s, ",")
	result := []string{}
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}