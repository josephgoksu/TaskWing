/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// decisionCmd represents the decision command
var decisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "Manage architectural decisions",
	Long: `Manage architectural decisions attached to features.

Decisions capture the WHY behind your code - the reasoning, tradeoffs,
and context that led to specific technical choices.

Examples:
  taskwing decision add "Auth" "Use JWT" --reason "Stateless scaling"
  taskwing decision list "Auth"
  taskwing decision delete <decision-id>`,
}

// decision add command
var decisionAddCmd = &cobra.Command{
	Use:   "add <feature> <title>",
	Short: "Add a decision to a feature",
	Long: `Record an architectural decision for a feature.

Example:
  taskwing decision add "Auth" "Use JWT" --reason "Stateless horizontal scaling" --tradeoff "Need refresh logic"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		featureName := args[0]
		title := args[1]
		reason, _ := cmd.Flags().GetString("reason")
		tradeoff, _ := cmd.Flags().GetString("tradeoff")

		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		// Find feature ID
		features, _ := store.ListFeatures()
		var featureID string
		for _, f := range features {
			if strings.EqualFold(f.Name, featureName) || f.ID == featureName {
				featureID = f.ID
				break
			}
		}

		if featureID == "" {
			return fmt.Errorf("feature not found: %s", featureName)
		}

		d := memory.Decision{
			Title:     title,
			Summary:   reason, // Use reason as summary for brevity
			Reasoning: reason,
			Tradeoffs: tradeoff,
		}

		if err := store.AddDecision(featureID, d); err != nil {
			return fmt.Errorf("add decision: %w", err)
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(map[string]string{
				"status":   "created",
				"feature":  featureName,
				"decision": title,
			}, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("✓ Decision '%s' added to %s\n", title, featureName)
		}

		return nil
	},
}

// decision list command
var decisionListCmd = &cobra.Command{
	Use:   "list [feature]",
	Short: "List decisions for a feature (or all if no feature specified)",
	Long: `List architectural decisions.

Without arguments, lists all decisions across all features.
With a feature name, lists decisions for that specific feature.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		features, _ := store.ListFeatures()

		// If no feature specified, list all decisions
		if len(args) == 0 {
			if viper.GetBool("json") {
				allDecisions := make(map[string][]memory.Decision)
				for _, f := range features {
					decisions, _ := store.GetDecisions(f.ID)
					if len(decisions) > 0 {
						allDecisions[f.Name] = decisions
					}
				}
				output, _ := json.MarshalIndent(allDecisions, "", "  ")
				fmt.Println(string(output))
				return nil
			}

			totalDecisions := 0
			for _, f := range features {
				decisions, _ := store.GetDecisions(f.ID)
				if len(decisions) == 0 {
					continue
				}
				totalDecisions += len(decisions)
				fmt.Printf("## %s (%d decisions)\n\n", f.Name, len(decisions))
				for i, d := range decisions {
					fmt.Printf("  %d. %s\n", i+1, d.Title)
					if d.Reasoning != "" {
						// Truncate long reasoning
						reason := d.Reasoning
						if len(reason) > 100 {
							reason = reason[:100] + "..."
						}
						fmt.Printf("     Why: %s\n", reason)
					}
					if d.Tradeoffs != "" {
						tradeoffs := d.Tradeoffs
						if len(tradeoffs) > 80 {
							tradeoffs = tradeoffs[:80] + "..."
						}
						fmt.Printf("     Trade-offs: %s\n", tradeoffs)
					}
					fmt.Println()
				}
			}
			if totalDecisions == 0 {
				fmt.Println("No decisions found. Add one with: taskwing decision add \"Feature\" \"Title\" --reason \"Why\"")
			} else {
				fmt.Printf("Total: %d decisions across %d features\n", totalDecisions, len(features))
			}
			return nil
		}

		// Feature specified
		featureName := args[0]
		var featureID, actualName string
		for _, f := range features {
			if strings.EqualFold(f.Name, featureName) || f.ID == featureName {
				featureID = f.ID
				actualName = f.Name
				break
			}
		}

		if featureID == "" {
			return fmt.Errorf("feature not found: %s", featureName)
		}

		decisions, err := store.GetDecisions(featureID)
		if err != nil {
			return fmt.Errorf("list decisions: %w", err)
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(decisions, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		if len(decisions) == 0 {
			fmt.Printf("No decisions found for '%s'\n", actualName)
			fmt.Printf("Add one with: taskwing decision add \"%s\" \"Decision title\" --reason \"Why\"\n", actualName)
			return nil
		}

		fmt.Printf("Decisions for %s (%d):\n\n", actualName, len(decisions))
		for i, d := range decisions {
			fmt.Printf("%d. %s\n", i+1, d.Title)
			fmt.Printf("   ID: %s\n", d.ID)
			if d.Reasoning != "" {
				fmt.Printf("   Why: %s\n", d.Reasoning)
			}
			if d.Tradeoffs != "" {
				fmt.Printf("   Trade-offs: %s\n", d.Tradeoffs)
			}
			fmt.Printf("   Date: %s\n\n", d.CreatedAt.Format("2006-01-02"))
		}

		return nil
	},
}

// decision delete command
var decisionDeleteCmd = &cobra.Command{
	Use:   "delete <decision-id>",
	Short: "Delete a decision",
	Long: `Delete an architectural decision by its ID.

Get the decision ID from 'taskwing decision list <feature>'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		decisionID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		if !force {
			fmt.Printf("Delete decision '%s'? Use --force to confirm.\n", decisionID)
			return nil
		}

		if err := store.DeleteDecision(decisionID); err != nil {
			return fmt.Errorf("delete decision: %w", err)
		}

		fmt.Printf("✓ Decision '%s' deleted\n", decisionID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(decisionCmd)

	// Add subcommands
	decisionCmd.AddCommand(decisionAddCmd)
	decisionCmd.AddCommand(decisionListCmd)
	decisionCmd.AddCommand(decisionDeleteCmd)

	// Flags for add
	decisionAddCmd.Flags().String("reason", "", "Why this decision was made")
	decisionAddCmd.Flags().String("tradeoff", "", "Known trade-offs of this decision")

	// Flags for delete
	decisionDeleteCmd.Flags().Bool("force", false, "Confirm deletion")
}
