package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/cobra"
)

var goalCmd = &cobra.Command{
	Use:   "goal \"Goal Description\"",
	Short: "Turn a goal into an active execution plan",
	Long: `Create and activate a plan from a goal in one command.

This command runs clarification and plan generation automatically, then prints
the next action to start execution in your AI tool.`,
	Args: cobra.ExactArgs(1),
	RunE: runGoal,
}

func init() {
	rootCmd.AddCommand(goalCmd)
}

func runGoal(cmd *cobra.Command, args []string) error {
	goal := args[0]

	repo, err := openRepoOrHandleMissingMemory()
	if err != nil {
		return err
	}
	if repo == nil {
		return nil
	}
	defer func() { _ = repo.Close() }()

	cfg, err := getLLMConfigForRole(cmd, llm.RoleBootstrap)
	if err != nil {
		return fmt.Errorf("llm config: %w", err)
	}

	planApp := app.NewPlanApp(app.NewContextWithConfig(repo, cfg))

	clarifyCtx, clarifyCancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer clarifyCancel()

	clarifyRes, err := planApp.Clarify(clarifyCtx, app.ClarifyOptions{
		Goal:       goal,
		AutoAnswer: true,
	})
	if err != nil {
		if errors.Is(clarifyCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("clarification timed out after 2 minutes")
		}
		return fmt.Errorf("clarification failed: %w", err)
	}
	if !clarifyRes.Success {
		return fmt.Errorf("clarification failed: %s", clarifyRes.Message)
	}

	enrichedGoal := clarifyRes.EnrichedGoal
	if enrichedGoal == "" {
		enrichedGoal = goal
	}

	genCtx, genCancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer genCancel()

	genRes, err := planApp.Generate(genCtx, app.GenerateOptions{
		Goal:         goal,
		EnrichedGoal: enrichedGoal,
		Save:         true,
	})
	if err != nil {
		if errors.Is(genCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("plan generation timed out after 2 minutes")
		}
		return fmt.Errorf("plan generation failed: %w", err)
	}
	if !genRes.Success {
		return fmt.Errorf("plan generation failed: %s", genRes.Message)
	}

	if isJSON() {
		return printJSON(map[string]any{
			"success":    true,
			"plan_id":    genRes.PlanID,
			"task_count": len(genRes.Tasks),
			"next_steps": []string{
				"taskwing slash next",
				"or call MCP tool task with action=next",
			},
		})
	}

	fmt.Printf("Plan created and activated: %s (%d task(s))\n", genRes.PlanID, len(genRes.Tasks))
	fmt.Println("Next:")
	fmt.Println("  1. In your AI tool, run /tw-next")
	fmt.Println("  2. Or use MCP tool task with action=next")
	return nil
}
