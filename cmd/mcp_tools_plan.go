package cmd

// Planning MCP tool: plan-from-document (preview or create)

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/prompts"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterPlanningTools registers the document→plan tool
func RegisterPlanningTools(server *mcp.Server, taskStore store.TaskStore) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "plan-from-document",
		Description: "Create a plan from PRD/text. Args: content or uri, skip_improve (bool), confirm (bool), model, temperature. Preview by default; set confirm=true to create.",
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
			EstimationTemperature:      appCfg.LLM.EstimationTemperature,
			EstimationMaxOutputTokens:  appCfg.LLM.EstimationMaxOutputTokens,
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

		provider, err := llm.NewProvider(&resolved)
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

		// 5) Estimate and generate
		estSys, perr := prompts.GetPrompt(prompts.KeyEstimateTasks, templatesDir)
		if perr != nil {
			return nil, types.NewMCPError("PROMPT_ERROR", "failed to load estimation prompt", map[string]interface{}{"error": perr.Error()})
		}
		estimate, err := provider.EstimateTaskParameters(ctx, estSys, improved, resolved.ModelName, resolved.APIKey, resolved.ProjectID, resolved.EstimationMaxOutputTokens, resolved.EstimationTemperature)
		if err != nil {
			// Log the error but continue with configured tokens
			logInfo("Failed to estimate task parameters: " + err.Error())
		}

		genSys, perr := prompts.GetPrompt(prompts.KeyGenerateTasks, templatesDir)
		if perr != nil {
			return nil, types.NewMCPError("PROMPT_ERROR", "failed to load generation prompt", map[string]interface{}{"error": perr.Error()})
		}

		maxTokens := resolved.MaxOutputTokens
		if estimate.EstimatedTaskCount > 0 {
			calc := (estimate.EstimatedTaskCount * 200) + 2048
			if calc < 4096 {
				calc = 4096
			}
			if calc > 32768 {
				calc = 32768
			}
			maxTokens = calc
		}

		outputs, err := provider.GenerateTasks(ctx, genSys, improved, resolved.ModelName, resolved.APIKey, resolved.ProjectID, maxTokens, resolved.Temperature)
		if err != nil {
			return nil, types.NewMCPError("GENERATION_FAILED", "LLM failed to generate tasks", map[string]interface{}{"error": err.Error()})
		}

		// 6) Build candidates (using internal helpers from generate.go)
		candidates, relMap, err := resolveAndBuildTaskCandidates(outputs)
		if err != nil {
			return nil, types.NewMCPError("CANDIDATE_ERROR", "failed to process LLM output", map[string]interface{}{"error": err.Error()})
		}

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

		// 7) If confirm, delete existing tasks and create
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

			created, errs := createTasksInStore(st, candidates, relMap)
			resp.Created = created
			if len(errs) > 0 {
				return nil, types.NewMCPError("CREATE_PARTIAL", fmt.Sprintf("created %d with %d errors", created, len(errs)), map[string]interface{}{"errors": toStringSlice(errs)})
			}
		}

		elapsed := time.Since(start).Milliseconds()
		if resp.Preview {
			resp.Summary = fmt.Sprintf("Proposed %d tasks (preview) in %dms", resp.ProposedCount, elapsed)
		} else {
			resp.Summary = fmt.Sprintf("Created %d tasks from plan in %dms", resp.Created, elapsed)
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
