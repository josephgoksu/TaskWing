package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	goalCmd.Flags().Bool("auto-answer", false, "Automatically answer clarification questions using project context")
	goalCmd.Flags().Int("max-rounds", 5, "Maximum clarify rounds before stopping")
}

func runGoal(cmd *cobra.Command, args []string) error {
	goal := args[0]
	autoAnswer, _ := cmd.Flags().GetBool("auto-answer")
	maxRounds, _ := cmd.Flags().GetInt("max-rounds")
	if maxRounds <= 0 {
		maxRounds = 5
	}

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

	interactive := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) && !isJSON()

	var clarifyRes *app.ClarifyResult
	if autoAnswer || !interactive {
		clarifyCtx, clarifyCancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer clarifyCancel()

		clarifyRes, err = planApp.Clarify(clarifyCtx, app.ClarifyOptions{
			Goal:       goal,
			AutoAnswer: autoAnswer,
			MaxRounds:  maxRounds,
		})
		if err != nil {
			if errors.Is(clarifyCtx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("clarification timed out after 2 minutes")
			}
			return fmt.Errorf("clarification failed: %w", err)
		}
	} else {
		reader := bufio.NewReader(os.Stdin)
		clarifyRes, err = runInteractiveGoalClarifyLoop(cmd.Context(), reader, planApp, goal, maxRounds)
		if err != nil {
			return err
		}
	}

	if clarifyRes == nil {
		return fmt.Errorf("clarification failed: empty result")
	}
	if !clarifyRes.Success {
		return fmt.Errorf("clarification failed: %s", clarifyRes.Message)
	}
	if !clarifyRes.IsReadyToPlan {
		if isJSON() {
			return printJSON(map[string]any{
				"success":            false,
				"clarify_session_id": clarifyRes.ClarifySessionID,
				"is_ready_to_plan":   false,
				"questions":          clarifyRes.Questions,
				"enriched_goal":      clarifyRes.EnrichedGoal,
				"round_index":        clarifyRes.RoundIndex,
				"max_rounds_reached": clarifyRes.MaxRoundsReached,
				"message":            "clarification is unresolved; answer questions and retry clarify",
			})
		}
		fmt.Printf("Clarification unresolved (session: %s, round: %d)\n", clarifyRes.ClarifySessionID, clarifyRes.RoundIndex)
		if len(clarifyRes.Questions) > 0 {
			fmt.Println("Pending questions:")
			for i, q := range clarifyRes.Questions {
				fmt.Printf("  %d. %s\n", i+1, q)
			}
		}
		fmt.Println("No plan generated. Continue clarification first.")
		return nil
	}

	enrichedGoal := strings.TrimSpace(clarifyRes.EnrichedGoal)
	if enrichedGoal == "" {
		enrichedGoal = goal
	}

	genCtx, genCancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer genCancel()

	genRes, err := planApp.Generate(genCtx, app.GenerateOptions{
		Goal:             goal,
		ClarifySessionID: clarifyRes.ClarifySessionID,
		EnrichedGoal:     enrichedGoal,
		Save:             true,
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
			"success":            true,
			"clarify_session_id": clarifyRes.ClarifySessionID,
			"plan_id":            genRes.PlanID,
			"task_count":         len(genRes.Tasks),
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

func runInteractiveGoalClarifyLoop(ctx context.Context, reader *bufio.Reader, planApp *app.PlanApp, goal string, maxRounds int) (*app.ClarifyResult, error) {
	clarifyCtx, clarifyCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer clarifyCancel()
	result, err := planApp.Clarify(clarifyCtx, app.ClarifyOptions{
		Goal:      goal,
		MaxRounds: maxRounds,
	})
	if err != nil {
		if errors.Is(clarifyCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("clarification timed out after 2 minutes")
		}
		return nil, fmt.Errorf("clarification failed: %w", err)
	}

	for result != nil && result.Success && !result.IsReadyToPlan && !result.MaxRoundsReached {
		if len(result.Questions) == 0 {
			return result, nil
		}
		fmt.Printf("Clarify round %d (%s):\n", result.RoundIndex, result.ClarifySessionID)
		answers := make([]app.ClarifyAnswer, 0, len(result.Questions))
		for i, q := range result.Questions {
			fmt.Printf("  %d. %s\n", i+1, q)
			fmt.Print("     answer> ")
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return nil, fmt.Errorf("read answer: %w", readErr)
			}
			answers = append(answers, app.ClarifyAnswer{
				Question: q,
				Answer:   strings.TrimSpace(line),
			})
		}

		nextCtx, nextCancel := context.WithTimeout(ctx, 2*time.Minute)
		result, err = planApp.Clarify(nextCtx, app.ClarifyOptions{
			Goal:             goal,
			ClarifySessionID: result.ClarifySessionID,
			Answers:          answers,
			MaxRounds:        maxRounds,
		})
		nextCancel()
		if err != nil {
			if errors.Is(nextCtx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("clarification timed out after 2 minutes")
			}
			return nil, fmt.Errorf("clarification failed: %w", err)
		}
	}

	return result, nil
}
