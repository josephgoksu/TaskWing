package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/prompts"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/spf13/cobra"
)

// nextCmd suggests the next best task to work on based on
// dependency readiness, priority, and simple tie-breakers.
var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Suggest the next ready task to start",
	Long: `Finds tasks that are ready to start (all dependencies completed)
and prioritizes them by priority, dependency count, and creation time.

Provides context-aware AI-powered recommendations by default
based on project patterns, dependency relationships, and development flow.
Use --ai-suggestions=false to disable AI suggestions.

Displays rich info and suggested follow-up commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskStore, err := GetStore()
		if err != nil {
			return fmt.Errorf("failed to get task store: %w", err)
		}
		defer func() {
			_ = taskStore.Close()
		}()

		// Load all tasks; we’ll filter locally
		allTasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return fmt.Errorf("failed to list tasks: %w", err)
		}
		if len(allTasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}

		// Index by ID for quick lookups
		byID := map[string]models.Task{}
		for _, t := range allTasks {
			byID[t.ID] = t
		}

		// Ready = in todo or doing AND all dependencies done
		ready := make([]models.Task, 0)
		for _, t := range allTasks {
			if t.Status != models.StatusTodo && t.Status != models.StatusDoing {
				continue
			}
			depsDone := true
			for _, depID := range t.Dependencies {
				dep, ok := byID[depID]
				if !ok || dep.Status != models.StatusDone {
					depsDone = false
					break
				}
			}
			if depsDone {
				ready = append(ready, t)
			}
		}

		if len(ready) == 0 {
			fmt.Println("No ready tasks found (dependencies may be incomplete).")
			return nil
		}

		// Sort by priority (urgent > high > medium > low), then fewest deps, then createdAt asc
		prioRank := map[models.TaskPriority]int{
			models.PriorityUrgent: 0,
			models.PriorityHigh:   1,
			models.PriorityMedium: 2,
			models.PriorityLow:    3,
		}
		sort.SliceStable(ready, func(i, j int) bool {
			a, b := ready[i], ready[j]
			ra, rb := prioRank[a.Priority], prioRank[b.Priority]
			if ra != rb {
				return ra < rb
			}
			if len(a.Dependencies) != len(b.Dependencies) {
				return len(a.Dependencies) < len(b.Dependencies)
			}
			if !a.CreatedAt.Equal(b.CreatedAt) {
				return a.CreatedAt.Before(b.CreatedAt)
			}
			return strings.Compare(a.ID, b.ID) < 0
		})

		// Check if AI suggestions are requested
		aiSuggestions, _ := cmd.Flags().GetBool("ai-suggestions")

		// Pick top candidate (either from AI suggestions or traditional logic)
		var best models.Task
		var aiRecommendations []types.TaskSuggestion

		if aiSuggestions {
			// Try AI-powered suggestions
			config := GetConfig()
			if config.LLM.Provider != "" && config.LLM.APIKey != "" {
				aiRecommendations, err = getAISuggestions(ready, allTasks, config)
				if err != nil {
					fmt.Printf("⚠️  AI suggestions failed: %v\n", err)
					fmt.Println("Falling back to traditional prioritization...")
					fmt.Println()
				} else if len(aiRecommendations) > 0 {
					// Find the AI-recommended task in our ready list
					for _, task := range ready {
						if task.ID == aiRecommendations[0].TaskID {
							best = task
							break
						}
					}
					// If AI task not found in ready list, fall back to first ready task
					if best.ID == "" {
						best = ready[0]
					}
				}
			} else {
				fmt.Println("⚠️  AI suggestions require LLM configuration (provider and API key)")
				fmt.Println("Falling back to traditional prioritization...")
				fmt.Println()
			}
		}

		// If AI didn't provide a recommendation, use traditional logic
		if best.ID == "" {
			best = ready[0]
		}

		if len(aiRecommendations) > 0 {
			fmt.Println("— AI-Powered Task Recommendation —")
		} else {
			fmt.Println("— Next Suggested Task —")
		}
		fmt.Printf("ID:        %s\n", best.ID)
		fmt.Printf("Title:     %s\n", best.Title)
		fmt.Printf("Status:    %s\n", best.Status)
		fmt.Printf("Priority:  %s\n", best.Priority)
		if best.Description != "" {
			fmt.Printf("Desc:      %s\n", best.Description)
		}

		// Display AI reasoning if available
		if len(aiRecommendations) > 0 && aiRecommendations[0].TaskID == best.ID {
			rec := aiRecommendations[0]
			fmt.Printf("AI Reasoning: %s\n", rec.Reasoning)
			fmt.Printf("Confidence:   %.0f%% | Phase: %s | Effort: %s\n",
				rec.ConfidenceScore*100, rec.ProjectPhase, rec.EstimatedEffort)
		}
		if best.AcceptanceCriteria != "" {
			fmt.Println("Acceptance:")
			for _, line := range strings.Split(best.AcceptanceCriteria, "\n") {
				fmt.Printf("  %s\n", line)
			}
		}

		// Dependencies with status indicator
		if len(best.Dependencies) > 0 {
			fmt.Println("Dependencies:")
			for _, depID := range best.Dependencies {
				dep := byID[depID]
				mark := "⏱️"
				if dep.Status == models.StatusDone {
					mark = "✅"
				}
				fmt.Printf("  %s %s (%s)\n", mark, dep.Title, dep.ID)
			}
		} else {
			fmt.Println("Dependencies: none")
		}

		// Subtasks list
		if len(best.SubtaskIDs) > 0 {
			fmt.Println("Subtasks:")
			for _, sid := range best.SubtaskIDs {
				if sub, ok := byID[sid]; ok {
					fmt.Printf("  - %s (%s, %s)\n", sub.Title, sub.Status, sid)
				}
			}
		}

		fmt.Println()
		fmt.Println("Suggested actions:")
		fmt.Printf("  • Start:    taskwing start %s\n", best.ID)
		fmt.Printf("  • Show:     taskwing show %s\n", best.ID)
		fmt.Printf("  • Subtasks: taskwing list --parent %s\n", best.ID)
		fmt.Printf("  • Finish:   taskwing finish %s\n", best.ID)

		// Display AI-recommended actions if available
		if len(aiRecommendations) > 0 && aiRecommendations[0].TaskID == best.ID {
			rec := aiRecommendations[0]
			if len(rec.RecommendedActions) > 0 {
				fmt.Println("\nAI-Recommended Actions:")
				for i, action := range rec.RecommendedActions {
					fmt.Printf("  %d. %s\n", i+1, action)
				}
			}
		}

		return nil
	},
}

// getAISuggestions calls the AI provider to get context-aware task recommendations.
func getAISuggestions(readyTasks, allTasks []models.Task, config *types.AppConfig) ([]types.TaskSuggestion, error) {
	provider, err := createLLMProvider(&config.LLM)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Get the system prompt
	systemPrompt, err := prompts.GetPrompt(prompts.KeySuggestNextTask, config.Project.TemplatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
	}

	// Build context information for AI
	contextInfo := buildNextTaskContext(readyTasks, allTasks)

	// Call AI provider
	suggestions, err := provider.SuggestNextTask(
		context.Background(),
		systemPrompt,
		contextInfo,
		config.LLM.ModelName,
		config.LLM.APIKey,
		config.LLM.ProjectID,
		config.LLM.MaxOutputTokens,
		config.LLM.Temperature,
	)
	if err != nil {
		return nil, fmt.Errorf("AI provider call failed: %w", err)
	}

	return suggestions, nil
}

// buildNextTaskContext creates a rich context description for the AI to analyze.
func buildNextTaskContext(readyTasks, allTasks []models.Task) string {
	var context strings.Builder

	// Project overview
	context.WriteString("=== PROJECT OVERVIEW ===\n")
	todoCount := 0
	doingCount := 0
	reviewCount := 0
	doneCount := 0

	for _, task := range allTasks {
		switch task.Status {
		case models.StatusTodo:
			todoCount++
		case models.StatusDoing:
			doingCount++
		case models.StatusReview:
			reviewCount++
		case models.StatusDone:
			doneCount++
		}
	}

	context.WriteString(fmt.Sprintf("Total Tasks: %d (Todo: %d, Doing: %d, Review: %d, Done: %d)\n",
		len(allTasks), todoCount, doingCount, reviewCount, doneCount))
	context.WriteString(fmt.Sprintf("Ready to Start: %d tasks\n\n", len(readyTasks)))

	// Current work in progress
	if doingCount > 0 {
		context.WriteString("=== CURRENTLY IN PROGRESS ===\n")
		for _, task := range allTasks {
			if task.Status == models.StatusDoing {
				context.WriteString(fmt.Sprintf("- [%s] %s", task.ID[:8], task.Title))
				if task.Priority != "" {
					context.WriteString(fmt.Sprintf(" (Priority: %s)", task.Priority))
				}
				context.WriteString("\n")
			}
		}
		context.WriteString("\n")
	}

	// Ready tasks with details
	context.WriteString("=== READY TO START ===\n")
	for _, task := range readyTasks {
		context.WriteString(fmt.Sprintf("Task ID: %s\n", task.ID))
		context.WriteString(fmt.Sprintf("Title: %s\n", task.Title))
		context.WriteString(fmt.Sprintf("Priority: %s\n", task.Priority))
		if task.Description != "" {
			context.WriteString(fmt.Sprintf("Description: %s\n", task.Description))
		}
		if task.AcceptanceCriteria != "" {
			context.WriteString(fmt.Sprintf("Acceptance Criteria: %s\n", task.AcceptanceCriteria))
		}

		// Dependencies (if any)
		if len(task.Dependencies) > 0 {
			context.WriteString("Dependencies (all completed): ")
			for i, depID := range task.Dependencies {
				for _, depTask := range allTasks {
					if depTask.ID == depID {
						if i > 0 {
							context.WriteString(", ")
						}
						context.WriteString(depTask.Title)
						break
					}
				}
			}
			context.WriteString("\n")
		}

		// Subtasks (if any)
		subtaskCount := 0
		for _, possibleSub := range allTasks {
			if possibleSub.ParentID != nil && *possibleSub.ParentID == task.ID {
				subtaskCount++
			}
		}
		if subtaskCount > 0 {
			context.WriteString(fmt.Sprintf("Has %d subtask(s)\n", subtaskCount))
		}

		// Tasks that depend on this one
		dependentCount := 0
		for _, possibleDep := range allTasks {
			for _, depID := range possibleDep.Dependencies {
				if depID == task.ID {
					dependentCount++
					break
				}
			}
		}
		if dependentCount > 0 {
			context.WriteString(fmt.Sprintf("Blocks %d other task(s)\n", dependentCount))
		}

		context.WriteString("\n")
	}

	return context.String()
}

func init() {
	nextCmd.Flags().Bool("ai-suggestions", true, "Use AI to provide context-aware task suggestions (default: true)")
	rootCmd.AddCommand(nextCmd)
}
