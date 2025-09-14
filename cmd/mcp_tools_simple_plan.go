package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/prompts"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterSimplePlanTools(server *mcp.Server, taskStore store.TaskStore) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate-plan",
		Description: "Generate a concise set of subtasks for a parent task (preview by default; confirm to create).",
	}, generatePlanHandler(taskStore))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "iterate-plan-step",
		Description: "Refine or split a specific subtask (preview by default; confirm to apply).",
	}, iteratePlanStepHandler(taskStore))
	return nil
}

func generatePlanHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.GeneratePlanParams, types.GeneratePlanResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.GeneratePlanParams]) (*mcp.CallToolResultFor[types.GeneratePlanResponse], error) {
		p := params.Arguments
		if p.TaskID == "" {
			return nil, types.NewMCPError("MISSING_TASK", "task_id is required", nil)
		}
		parent, err := taskStore.GetTask(p.TaskID)
		if err != nil {
			return nil, types.NewMCPError("TASK_NOT_FOUND", fmt.Sprintf("%v", err), nil)
		}

		sys, err := prompts.GetPrompt(prompts.KeyBreakdownTask, GetConfig().Project.RootDir+"/"+GetConfig().Project.TemplatesDir)
		if err != nil {
			return nil, types.NewMCPError("PROMPT", err.Error(), nil)
		}
		provider, err := createLLMProvider(&GetConfig().LLM)
		if err != nil {
			var guidance map[string]interface{}
			if strings.Contains(err.Error(), "API key") {
				guidance = map[string]interface{}{
					"help": "To use AI planning features, set up your LLM API key",
					"setup_commands": []string{
						"export OPENAI_API_KEY=your_key_here",
						"taskwing init --setup-llm",
					},
				}
			}
			return nil, types.NewMCPError("LLM_CONFIG", err.Error(), guidance)
		}

		// Context with chain memory if available
		tc, _ := BuildTaskContext(taskStore)
		var b strings.Builder
		b.WriteString("Parent Task:\n")
		b.WriteString(parent.Title)
		b.WriteString("\n\nDescription:\n")
		b.WriteString(parent.Description)
		if ac := strings.TrimSpace(parent.AcceptanceCriteria); ac != "" {
			b.WriteString("\n\nAcceptance Criteria:\n")
			b.WriteString(ac)
		}
		b.WriteString("\n\nProject Summary:\n")
		b.WriteString(fmt.Sprintf("Total tasks: %d, Done: %d\n", tc.TotalTasks, tc.TasksByStatus[string(models.StatusDone)]))

		steps, err := provider.BreakdownTask(ctx, sys, parent.Title, parent.Description, parent.AcceptanceCriteria, b.String(), GetConfig().LLM.ModelName, GetConfig().LLM.APIKey, GetConfig().LLM.ProjectID, GetConfig().LLM.MaxOutputTokens, GetConfig().LLM.Temperature)
		if err != nil {
			guidance := map[string]interface{}{
				"task_title": parent.Title,
				"error_type": "ai_processing",
				"help":       "AI task breakdown failed. Check your API key and try again.",
			}
			if strings.Contains(err.Error(), "API key") || strings.Contains(err.Error(), "unauthorized") {
				guidance["setup_commands"] = []string{
					"export OPENAI_API_KEY=your_key_here",
					"taskwing init --setup-llm",
				}
			}
			return nil, types.NewMCPError("PLAN_FAIL", err.Error(), guidance)
		}
		n := p.Count
		if n <= 0 {
			n = 5
		}
		if n < 3 {
			n = 3
		}
		if n > 7 {
			n = 7
		}
		if len(steps) > n {
			steps = steps[:n]
		}

		resp := types.GeneratePlanResponse{
			Preview:      !p.Confirm,
			ParentTask:   parent.Title,
			ParentTaskID: parent.ID,
		}
		for _, s := range steps {
			resp.Proposed = append(resp.Proposed, types.ProposedTask{
				Title:              s.Title,
				Description:        s.Description,
				AcceptanceCriteria: s.AcceptanceCriteria,
				Priority:           strings.ToLower(s.Priority),
			})
		}
		resp.ProposedCount = len(resp.Proposed)

		if !p.Confirm {
			resp.NextSteps = []string{
				fmt.Sprintf("Run with confirm=true to create %d subtasks", len(resp.Proposed)),
				fmt.Sprintf("View parent task: taskwing show %s", parent.ID[:8]),
				"Modify count with count parameter (3-7)",
			}
			return &mcp.CallToolResultFor[types.GeneratePlanResponse]{StructuredContent: resp}, nil
		}

		// Apply
		created := make([]models.Task, 0, len(steps))
		for _, s := range steps {
			t := models.Task{Title: s.Title, Description: s.Description, AcceptanceCriteria: s.AcceptanceCriteria, Status: models.StatusTodo, Priority: mapPriorityOrDefault(s.Priority)}
			pid := parent.ID
			t.ParentID = &pid
			nt, err := taskStore.CreateTask(t)
			if err != nil {
				return nil, types.NewMCPError("CREATE", err.Error(), nil)
			}
			created = append(created, nt)
		}
		for i := 1; i < len(created); i++ {
			prev := created[i-1].ID
			if _, err := taskStore.UpdateTask(created[i].ID, map[string]interface{}{"dependencies": append(created[i].Dependencies, prev)}); err != nil {
				return nil, types.NewMCPError("LINK", err.Error(), nil)
			}
		}
		sort.Slice(created, func(i, j int) bool { return created[i].CreatedAt.Before(created[j].CreatedAt) })
		resp.Created = len(created)

		// Add created tasks details and next steps
		for _, c := range created {
			resp.CreatedTasks = append(resp.CreatedTasks, types.TaskResponse{
				ID:                 c.ID,
				Title:              c.Title,
				Description:        c.Description,
				AcceptanceCriteria: c.AcceptanceCriteria,
				Status:             string(c.Status),
				Priority:           string(c.Priority),
				ParentID:           c.ParentID,
				Dependencies:       c.Dependencies,
				CreatedAt:          c.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt:          c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}

		resp.NextSteps = []string{
			fmt.Sprintf("View all subtasks: taskwing ls --parent %s", parent.ID[:8]),
			fmt.Sprintf("Start first task: taskwing start %s", created[0].ID[:8]),
			fmt.Sprintf("View parent: taskwing show %s", parent.ID[:8]),
		}

		return &mcp.CallToolResultFor[types.GeneratePlanResponse]{StructuredContent: resp}, nil
	}
}

func iteratePlanStepHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.IteratePlanStepParams, types.IteratePlanStepResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.IteratePlanStepParams]) (*mcp.CallToolResultFor[types.IteratePlanStepResponse], error) {
		p := params.Arguments
		if p.TaskID == "" || p.StepID == "" {
			return nil, types.NewMCPError("MISSING_ARGS", "task_id and step_id are required", nil)
		}
		parent, err := taskStore.GetTask(p.TaskID)
		if err != nil {
			return nil, types.NewMCPError("TASK_NOT_FOUND", err.Error(), nil)
		}
		subs, err := taskStore.ListTasks(func(t models.Task) bool { return t.ParentID != nil && *t.ParentID == parent.ID }, nil)
		if err != nil {
			return nil, types.NewMCPError("LIST_FAIL", err.Error(), nil)
		}
		var target *models.Task
		for i := range subs {
			if subs[i].ID == p.StepID {
				target = &subs[i]
				break
			}
		}
		if target == nil {
			return nil, types.NewMCPError("STEP_NOT_FOUND", "subtask not found under parent", nil)
		}

		if p.Split {
			sys, err := prompts.GetPrompt(prompts.KeyBreakdownTask, GetConfig().Project.RootDir+"/"+GetConfig().Project.TemplatesDir)
			if err != nil {
				return nil, types.NewMCPError("PROMPT", err.Error(), nil)
			}
			provider, err := createLLMProvider(&GetConfig().LLM)
			if err != nil {
				var guidance map[string]interface{}
				if strings.Contains(err.Error(), "API key") {
					guidance = map[string]interface{}{
						"help": "To use AI planning features, set up your LLM API key",
						"setup_commands": []string{
							"export OPENAI_API_KEY=your_key_here",
							"taskwing init --setup-llm",
						},
					}
				}
				return nil, types.NewMCPError("LLM_CONFIG", err.Error(), guidance)
			}
			steps, err := provider.BreakdownTask(ctx, sys, target.Title, target.Description, target.AcceptanceCriteria, p.Prompt, GetConfig().LLM.ModelName, GetConfig().LLM.APIKey, GetConfig().LLM.ProjectID, GetConfig().LLM.MaxOutputTokens, GetConfig().LLM.Temperature)
			if err != nil {
				return nil, types.NewMCPError("SPLIT_FAIL", err.Error(), nil)
			}
			if len(steps) > 3 {
				steps = steps[:3]
			}
			resp := types.IteratePlanStepResponse{
				Preview:    !p.Confirm,
				ParentTask: parent.Title,
				TargetStep: target.Title,
				Operation:  "split",
			}
			for _, s := range steps {
				resp.Proposed = append(resp.Proposed, types.ProposedTask{Title: s.Title, Description: s.Description, AcceptanceCriteria: s.AcceptanceCriteria, Priority: strings.ToLower(s.Priority)})
			}
			if !p.Confirm {
				resp.NextSteps = []string{
					fmt.Sprintf("Run with confirm=true to replace '%s' with %d new steps", target.Title, len(resp.Proposed)),
					fmt.Sprintf("View parent task: taskwing show %s", parent.ID[:8]),
				}
				return &mcp.CallToolResultFor[types.IteratePlanStepResponse]{StructuredContent: resp}, nil
			}
			// Apply: create new subtasks, delete old
			for _, s := range steps {
				t := models.Task{Title: s.Title, Description: s.Description, AcceptanceCriteria: s.AcceptanceCriteria, Status: models.StatusTodo, Priority: mapPriorityOrDefault(s.Priority)}
				pid := parent.ID
				t.ParentID = &pid
				if _, err := taskStore.CreateTask(t); err != nil {
					return nil, types.NewMCPError("CREATE", err.Error(), nil)
				}
			}
			if err := taskStore.DeleteTask(target.ID); err != nil {
				return nil, types.NewMCPError("DELETE_OLD", err.Error(), nil)
			}
			return &mcp.CallToolResultFor[types.IteratePlanStepResponse]{StructuredContent: resp}, nil
		}

		// Refine
		sys, err := prompts.GetPrompt(prompts.KeyEnhanceTask, GetConfig().Project.RootDir+"/"+GetConfig().Project.TemplatesDir)
		if err != nil {
			return nil, types.NewMCPError("PROMPT", err.Error(), nil)
		}
		provider, err := createLLMProvider(&GetConfig().LLM)
		if err != nil {
			return nil, types.NewMCPError("LLM", err.Error(), nil)
		}
		enhanced, err := provider.EnhanceTask(ctx, sys, target.Title+"\n\n"+target.Description, p.Prompt, GetConfig().LLM.ModelName, GetConfig().LLM.APIKey, GetConfig().LLM.ProjectID, GetConfig().LLM.MaxOutputTokens, GetConfig().LLM.Temperature)
		if err != nil {
			return nil, types.NewMCPError("REFINE_FAIL", err.Error(), nil)
		}
		resp := types.IteratePlanStepResponse{Preview: !p.Confirm}
		resp.Proposed = []types.ProposedTask{{Title: enhanced.Title, Description: enhanced.Description, AcceptanceCriteria: enhanced.AcceptanceCriteria, Priority: strings.ToLower(enhanced.Priority)}}
		if !p.Confirm {
			return &mcp.CallToolResultFor[types.IteratePlanStepResponse]{StructuredContent: resp}, nil
		}
		if _, err := taskStore.UpdateTask(target.ID, map[string]interface{}{"title": enhanced.Title, "description": enhanced.Description, "acceptanceCriteria": enhanced.AcceptanceCriteria, "priority": strings.ToLower(enhanced.Priority)}); err != nil {
			return nil, types.NewMCPError("APPLY_FAIL", err.Error(), nil)
		}
		return &mcp.CallToolResultFor[types.IteratePlanStepResponse]{StructuredContent: resp}, nil
	}
}
