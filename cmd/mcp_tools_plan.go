package cmd

// Planning MCP tool: plan-from-document (preview or create)

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/prompts"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// test seam: allow overriding provider factory in tests
var newLLMProvider = llm.NewProvider

// RegisterPlanningTools registers the document→plan tool
func RegisterPlanningTools(server *mcp.Server, taskStore store.TaskStore) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "plan-from-document",
		Description: "Create a plan from PRD/text using iterative generation. Args: content or uri, skip_improve (bool), confirm (bool), model, temperature. Preview by default; set confirm=true to create.",
	}, planFromDocumentHandler(taskStore))
	return nil
}

func planFromDocumentHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.PlanFromDocumentParams, types.PlanFromDocumentResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.PlanFromDocumentParams]) (*mcp.CallToolResultFor[types.PlanFromDocumentResponse], error) {
		args := params.Arguments
		logToolCall("plan-from-document", args)

		// 1) Read PRD content
		var prdContent string
		if args.Content != "" {
			prdContent = args.Content
		} else if args.URI != "" {
			data, err := os.ReadFile(args.URI)
			if err != nil {
				return nil, types.NewMCPError("READ_FAILED", fmt.Sprintf("failed to read uri: %s", args.URI), map[string]interface{}{"error": err.Error()})
			}
			prdContent = string(data)
		} else {
			return nil, types.NewMCPError("MISSING_INPUT", "Provide either 'content' or 'uri'", nil)
		}

		start := time.Now()

		// 2) Resolve config and provider
		appCfg := GetConfig()
		if args.Model != "" {
			appCfg.LLM.ModelName = args.Model
		}
		if args.Temperature != 0 {
			appCfg.LLM.Temperature = args.Temperature
		}

		// Prepare llm config from app config
		resolved := types.LLMConfig{
			Provider:                   appCfg.LLM.Provider,
			ModelName:                  appCfg.LLM.ModelName,
			APIKey:                     appCfg.LLM.APIKey,
			ProjectID:                  appCfg.LLM.ProjectID,
			MaxOutputTokens:            appCfg.LLM.MaxOutputTokens,
			Temperature:                appCfg.LLM.Temperature,
			ImprovementTemperature:     appCfg.LLM.ImprovementTemperature,
			ImprovementMaxOutputTokens: appCfg.LLM.ImprovementMaxOutputTokens,
		}
		if resolved.Provider == "openai" && resolved.APIKey == "" {
			if v := os.Getenv("OPENAI_API_KEY"); v != "" {
				resolved.APIKey = v
			} else if v := os.Getenv(envPrefix + "_LLM_APIKEY"); v != "" {
				resolved.APIKey = v
			}
		}

		if resolved.Provider == "" || resolved.ModelName == "" || (resolved.Provider == "openai" && resolved.APIKey == "") {
			return nil, types.NewMCPError("LLM_CONFIG_MISSING", "LLM provider/model/apikey not configured", map[string]interface{}{"provider": resolved.Provider, "model": resolved.ModelName})
		}

		provider, err := createLLMProvider(&resolved)
		if err != nil {
			return nil, types.NewMCPError("LLM_PROVIDER_ERROR", "failed to create LLM provider", map[string]interface{}{"error": err.Error()})
		}

		// 3) Templates dir
		templatesDir := filepath.Join(appCfg.Project.RootDir, appCfg.Project.TemplatesDir)

		// 4) Optional improvement
		improved := prdContent
		if !args.SkipImprove {
			sys, perr := prompts.GetPrompt(prompts.KeyImprovePRD, templatesDir)
			if perr != nil {
				return nil, types.NewMCPError("PROMPT_ERROR", "failed to load improvement prompt", map[string]interface{}{"error": perr.Error()})
			}
			improved, err = provider.ImprovePRD(ctx, sys, prdContent, resolved.ModelName, resolved.APIKey, resolved.ProjectID, resolved.ImprovementMaxOutputTokens, resolved.ImprovementTemperature)
			if err != nil {
				// Non-fatal: continue with original content
				improved = prdContent
			}
		}

		// 5) Generate tasks using iterative approach (now the default)
		genSys, perr := prompts.GetPrompt(prompts.KeyGenerateNextWorkItem, templatesDir)
		if perr != nil {
			return nil, types.NewMCPError("PROMPT_ERROR", "failed to load iterative generation prompt", map[string]interface{}{"error": perr.Error()})
		}

		maxTokens := resolved.MaxOutputTokens

		// Use iterative generation approach for consistency with CLI default behavior
		var allTaskCandidates []models.Task
		createdTitles := make(map[string]struct{})
		iterations := 0

		// Adaptive iteration limits based on PRD size
		prdWordCount := len(strings.Fields(improved))
		var maxIterations int
		if prdWordCount < 50 {
			maxIterations = 3 // Very simple PRDs
		} else if prdWordCount < 200 {
			maxIterations = 5 // Simple PRDs
		} else if prdWordCount < 500 {
			maxIterations = 7 // Medium PRDs
		} else {
			maxIterations = 10 // Complex PRDs
		}

		for iterations < maxIterations {
			iterations++

			// Build context with already created titles
			var contextBuilder strings.Builder
			contextBuilder.WriteString(improved)
			contextBuilder.WriteString("\n\n---\nAlready created task titles (avoid duplicates):\n")
			for title := range createdTitles {
				contextBuilder.WriteString("- ")
				contextBuilder.WriteString(title)
				contextBuilder.WriteString("\n")
			}
			contextContent := contextBuilder.String()

			outputs, err := provider.GenerateTasks(ctx, genSys, contextContent, resolved.ModelName, resolved.APIKey, resolved.ProjectID, maxTokens, resolved.Temperature)
			if err != nil {
				return nil, types.NewMCPError("GENERATION_FAILED", fmt.Sprintf("LLM failed to generate tasks in iteration %d", iterations), map[string]interface{}{"error": err.Error(), "iteration": iterations})
			}

			if len(outputs) == 0 {
				// Natural completion - no more tasks to generate
				break
			}

			// Process this iteration's outputs
			candidates, _, err := resolveAndBuildTaskCandidates(outputs)
			if err != nil {
				return nil, types.NewMCPError("CANDIDATE_ERROR", fmt.Sprintf("failed to process LLM output in iteration %d", iterations), map[string]interface{}{"error": err.Error(), "iteration": iterations})
			}

			if len(candidates) == 0 {
				break
			}

			// Accumulate results
			allTaskCandidates = append(allTaskCandidates, candidates...)

			// Update created titles for context
			for _, t := range candidates {
				createdTitles[strings.ToLower(strings.TrimSpace(t.Title))] = struct{}{}
			}

			// Early termination if we've created enough tasks for simple PRDs
			if prdWordCount < 100 && len(allTaskCandidates) >= 5 {
				break
			} else if prdWordCount < 300 && len(allTaskCandidates) >= 8 {
				break
			}
		}

		// Use accumulated results
		candidates := allTaskCandidates

		// Build proposed preview list
		proposed := make([]types.ProposedTask, 0, len(candidates))
		for _, t := range candidates {
			proposed = append(proposed, types.ProposedTask{
				Title:              t.Title,
				Description:        t.Description,
				AcceptanceCriteria: t.AcceptanceCriteria,
				Priority:           string(t.Priority),
			})
		}

		resp := types.PlanFromDocumentResponse{
			Preview:       !args.Confirm,
			Proposed:      proposed,
			ProposedCount: len(proposed),
			Created:       0,
		}

		// 7) If confirm, delete existing tasks and create (iterative approach requires special handling)
		if args.Confirm {
			// Fresh store instance
			st, gerr := GetStore()
			if gerr != nil {
				return nil, types.NewMCPError("STORE_ERROR", "failed to init task store", map[string]interface{}{"error": gerr.Error()})
			}
			defer func() { _ = st.Close() }()

			// Clean slate: delete existing
			if err := st.DeleteAllTasks(); err != nil {
				return nil, types.NewMCPError("DELETE_FAILED", "failed to clear existing tasks before create", map[string]interface{}{"error": err.Error()})
			}

			// For iterative results, we need to recreate the relationship map from all candidates
			// This is a simplified approach - in practice, we'd need to properly handle the complex relationships
			// For now, create tasks in order without complex dependencies
			var totalCreated int
			var allErrors []error

			for _, task := range candidates {
				_, err := st.CreateTask(task)
				if err != nil {
					allErrors = append(allErrors, err)
				} else {
					totalCreated++
				}
			}

			resp.Created = totalCreated
			if len(allErrors) > 0 {
				return nil, types.NewMCPError("CREATE_PARTIAL", fmt.Sprintf("created %d with %d errors", totalCreated, len(allErrors)), map[string]interface{}{"errors": toStringSlice(allErrors)})
			}

			// Find the best task to start with
			nextTaskRecommendation := findRecommendedStartingTask(st)
			if nextTaskRecommendation != "" {
				resp.Summary = fmt.Sprintf("Created %d tasks. Recommended starting task: %s", totalCreated, nextTaskRecommendation)
			}
		}

		elapsed := time.Since(start).Milliseconds()
		if resp.Preview {
			resp.Summary = fmt.Sprintf("Proposed %d tasks across %d iterations (preview) in %dms", resp.ProposedCount, iterations, elapsed)
		} else if resp.Summary == "" {
			resp.Summary = fmt.Sprintf("Created %d tasks from iterative plan (%d iterations) in %dms", resp.Created, iterations, elapsed)
		}
		resp.ImprovedPRD = improved

		text := resp.Summary
		return &mcp.CallToolResultFor[types.PlanFromDocumentResponse]{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: resp,
		}, nil
	}
}

func toStringSlice(errs []error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Error()
	}
	return out
}

// findRecommendedStartingTask analyzes created tasks and recommends which one to start with
func findRecommendedStartingTask(store store.TaskStore) string {
	// Get all tasks
	tasks, err := store.ListTasks(nil, nil)
	if err != nil || len(tasks) == 0 {
		return ""
	}

	// Find tasks with no dependencies and highest priority
	var candidates []models.Task
	for _, task := range tasks {
		if task.Status == models.StatusTodo && len(task.Dependencies) == 0 {
			candidates = append(candidates, task)
		}
	}

	if len(candidates) == 0 {
		// No tasks without dependencies, just pick the highest priority todo
		for _, task := range tasks {
			if task.Status == models.StatusTodo {
				candidates = append(candidates, task)
				break // Just take one for simplicity
			}
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// Sort by priority
	priorityOrder := map[models.TaskPriority]int{
		models.PriorityUrgent: 0,
		models.PriorityHigh:   1,
		models.PriorityMedium: 2,
		models.PriorityLow:    3,
	}

	bestTask := candidates[0]
	bestScore := priorityOrder[bestTask.Priority]

	for _, task := range candidates[1:] {
		score := priorityOrder[task.Priority]
		if score < bestScore {
			bestTask = task
			bestScore = score
		}
	}

	// Return a short identifier and title
	shortID := bestTask.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf("%s: %s", shortID, bestTask.Title)
}
