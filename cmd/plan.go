package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/task"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage development plans",
	Long:  `Create, view, and export development plans using AI agents.`,
}

var planNewCmd = &cobra.Command{
	Use:   "new \"Goal Description\"",
	Short: "Create a new plan from a goal",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		goal := args[0]

		cfg, err := getLLMConfig(cmd)
		if err != nil {
			return fmt.Errorf("llm config: %w", err)
		}

		repo, err := memory.NewDefaultRepository(viper.GetString("memory.path"))
		if err != nil {
			return fmt.Errorf("open repo: %w", err)
		}
		defer repo.Close()

		fmt.Printf("\n🤖 I'll help you plan: \"%s\"\n\n", goal)

		// 1. Clarification Loop
		clarifyingAgent := agents.NewClarifyingAgent(cfg)
		history := ""
		enrichedGoal := goal

		for i := 0; i < 3; i++ { // Max 3 turns
			fmt.Print("   Thinking...\r")
			out, err := clarifyingAgent.Run(ctx, agents.Input{
				ExistingContext: map[string]any{
					"goal":    goal,
					"history": history,
				},
				Verbose: false,
			})
			if err != nil {
				return err
			}

			// Extract metadata (should be safe as we built it)
			if len(out.Findings) == 0 {
				return fmt.Errorf("agent returned no findings")
			}
			finding := out.Findings[0]
			meta := finding.Metadata
			isReady, _ := meta["is_ready_to_plan"].(bool)
			if val, ok := meta["enriched_goal"].(string); ok && val != "" {
				enrichedGoal = val
			}
			questions, _ := meta["questions"].([]interface{})

			if isReady {
				fmt.Println("✅ Goal is clear!")
				break
			}

			if len(questions) > 0 {
				fmt.Println("❓ Please clarify:")
				for _, q := range questions {
					fmt.Printf("   - %s\n", q)
				}
				fmt.Print("\n> ")

				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(answer)

				history += fmt.Sprintf("Q: %v\nA: %s\n", questions, answer)
				fmt.Println()
			} else {
				break
			}
		}

		// 2. Planning

		// Start spinner for planning phase
		s := ui.NewSpinner(fmt.Sprintf("Creating implementation plan for: %s", enrichedGoal))
		s.Start()

		planningAgent := agents.NewPlanningAgent(cfg)

		// RAG: Retrieve relevant context from Knowledge Graph
		ks := knowledge.NewService(repo, cfg)

		// Search for context using the enriched goal
		// We use a generous limit (5) to give the agent options
		scoredNodes, err := ks.Search(ctx, enrichedGoal, 5)

		// Format context for the agent
		var kgContext string
		if err != nil {
			// Don't fail the whole command if search fails, just warn (maybe log if verbose)
			kgContext = "Note: Context search unavailable."
		} else if len(scoredNodes) > 0 {
			var sb strings.Builder
			sb.WriteString("RELEVANT ARCHITECTURAL CONTEXT:\n")
			for _, node := range scoredNodes {
				// Include title/summary and content
				title := node.Node.Summary
				if title == "" {
					title = "Snippet"
				}
				source := node.Node.SourceAgent
				if source == "" {
					source = "unknown"
				}
				// Format: [Type] Title (Source: Agent, Score: 0.xx)
				sb.WriteString(fmt.Sprintf("### [%s] %s\n**Source**: %s | **Score**: %.2f\n%s\n\n",
					strings.ToUpper(node.Node.Type), title, source, node.Score, node.Node.Content))
			}
			kgContext = sb.String()
		} else {
			kgContext = "No specific existing patterns or decisions found relevant to this goal."
		}

		planOut, err := planningAgent.Run(ctx, agents.Input{
			ExistingContext: map[string]any{
				"enriched_goal": enrichedGoal,
				"goal":          goal, // Fallback
				"context":       kgContext,
			},
			Verbose: true, // Show progress
		})
		s.Stop() // Stop spinner

		if err != nil {
			return err
		}

		// 3. Persist Plan
		planFinding := planOut.Findings[0]

		// Unmarshal tasks using the typed struct we verified in agents package
		var tasksData []agents.PlanningTask
		if rawTasks, ok := planFinding.Metadata["tasks"].([]agents.PlanningTask); ok {
			tasksData = rawTasks
		} else {
			// Fallback or error if strict type fails (should not happen with our fix)
			return fmt.Errorf("internal error: invalid tasks format received from agent")
		}

		planID := "plan-" + fmt.Sprintf("%d", time.Now().Unix())
		newPlan := &task.Plan{
			ID:           planID,
			Goal:         goal,
			EnrichedGoal: enrichedGoal,
			Status:       "active",
		}

		if err := repo.CreatePlan(newPlan); err != nil {
			return fmt.Errorf("save plan: %w", err)
		}

		for _, tData := range tasksData {
			newTask := &task.Task{
				PlanID:             planID,
				Title:              tData.Title,
				Description:        tData.Description,
				Priority:           tData.Priority,
				AssignedAgent:      tData.AssignedAgent,
				AcceptanceCriteria: tData.AcceptanceCriteria,
				ValidationSteps:    tData.ValidationSteps,
			}

			if err := repo.CreateTask(newTask); err != nil {
				fmt.Printf("   ⚠️ Failed to save task %s: %v\n", newTask.Title, err)
			}
		}

		// Fetch the complete plan with tasks and print inline
		createdPlan, err := repo.GetPlan(planID)
		if err != nil {
			return fmt.Errorf("fetch created plan: %w", err)
		}

		fmt.Println() // Blank line before output
		printPlanDetails(createdPlan)
		return nil
	},
}

var planListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all plans",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := memory.NewDefaultRepository(viper.GetString("memory.path"))
		if err != nil {
			return err
		}
		defer repo.Close()

		plans, err := repo.ListPlans()
		if err != nil {
			return err
		}

		fmt.Println("ID\t\tCREATED\t\tGOAL")
		for _, p := range plans {
			fmt.Printf("%s\t%s\t%s\n", p.ID, p.CreatedAt.Format("2006-01-02"), p.Goal)
		}
		return nil
	},
}

// printPlanDetails prints a plan and its tasks in Markdown format.
// Used by both `plan new` (inline output) and `plan export`.
func printPlanDetails(plan *task.Plan) {
	fmt.Printf("# Plan: %s\n\n", plan.Goal)
	fmt.Printf("**Refined Goal**: %s\n\n", plan.EnrichedGoal)

	for _, t := range plan.Tasks {
		fmt.Printf("## Task: %s\n", t.Title)
		fmt.Printf("**Priority**: %d | **Agent**: %s\n\n", t.Priority, t.AssignedAgent)
		fmt.Printf("%s\n\n", t.Description)

		if len(t.AcceptanceCriteria) > 0 {
			fmt.Println("### Acceptance Criteria")
			for _, ac := range t.AcceptanceCriteria {
				fmt.Printf("- [ ] %s\n", ac)
			}
			fmt.Println()
		}

		if len(t.ValidationSteps) > 0 {
			fmt.Println("### Validation")
			fmt.Println("```bash")
			for _, vs := range t.ValidationSteps {
				fmt.Println(vs)
			}
			fmt.Println("```")
			fmt.Println()
		}
	}
}

var planExportCmd = &cobra.Command{
	Use:   "export [plan-id]",
	Short: "Export plan to Markdown",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := memory.NewDefaultRepository(viper.GetString("memory.path"))
		if err != nil {
			return err
		}
		defer repo.Close()

		plan, err := repo.GetPlan(args[0])
		if err != nil {
			return err
		}

		printPlanDetails(plan)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.AddCommand(planNewCmd)
	planCmd.AddCommand(planListCmd)
	planCmd.AddCommand(planExportCmd)
}
