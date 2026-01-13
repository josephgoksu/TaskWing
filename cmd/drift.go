/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

// driftCmd represents the drift command
var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Detect architectural drift between documented rules and code",
	Long: `Analyze the codebase for violations of documented architectural rules.

Extracts rules from constraints, patterns, and decisions in the knowledge base,
then checks the code against these rules using call graph analysis.

Examples:
  tw drift                           # Full drift analysis
  tw drift --constraint "repository" # Check specific constraint
  tw drift --path "internal/services" # Focus on specific paths
  tw drift --json                    # Output as JSON`,
	RunE: runDrift,
}

var (
	driftConstraint string
	driftPath       string
)

func init() {
	rootCmd.AddCommand(driftCmd)
	driftCmd.Flags().StringVar(&driftConstraint, "constraint", "", "Check only rules matching this name")
	driftCmd.Flags().StringVar(&driftPath, "path", "", "Limit analysis to files matching this pattern")
}

func runDrift(cmd *cobra.Command, args []string) error {
	// 1. Initialize repository
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// 2. Create app context with LLM config
	llmCfg, err := getLLMConfigForRole(cmd, llm.RoleQuery)
	if err != nil {
		return err
	}

	appCtx := app.NewContextWithConfig(repo, llmCfg)
	driftApp := app.NewDriftApp(appCtx)

	// 3. Show progress
	ctx := context.Background()
	if !isQuiet() {
		fmt.Println()
		fmt.Println("🔍 Architecture Drift Analysis")
		fmt.Println("══════════════════════════════")
		fmt.Println()
		fmt.Print("📋 Extracting architectural rules...")
	}

	// 4. Build request
	req := app.DriftRequest{
		Constraint: driftConstraint,
	}
	if driftPath != "" {
		req.Paths = []string{driftPath}
	}

	// 5. Run analysis
	report, err := driftApp.Analyze(ctx, req)
	if err != nil {
		if !isQuiet() {
			fmt.Println(" failed")
		}
		return fmt.Errorf("drift analysis failed: %w", err)
	}

	if !isQuiet() {
		fmt.Printf(" found %d rules\n", report.RulesChecked)
		fmt.Println()
	}

	// 6. Output
	if isJSON() {
		return printJSON(report)
	}

	// Render report
	ui.RenderDriftReport(report, isVerbose())

	return nil
}
