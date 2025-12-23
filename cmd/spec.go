/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/spec"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Create and manage feature specifications",
	Long: `Create feature specifications using AI persona agents.

Each spec is analyzed by multiple expert personas:
  • PM - User stories and acceptance criteria
  • Architect - Technical design
  • Engineer - Task breakdown
  • QA - Test strategy
  • Monetization - Revenue impact (optional)
  • UX - Design recommendations (optional)

Examples:
  tw spec create "add Stripe integration"
  tw spec create "fix login bug" --type bugfix
  tw spec list`,
}

var specCreateCmd = &cobra.Command{
	Use:   "create [feature description]",
	Short: "Create a new feature specification",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		featureRequest := strings.Join(args, " ")

		// Get flags
		specType, _ := cmd.Flags().GetString("type")
		skipAgents, _ := cmd.Flags().GetStringSlice("skip")
		outputPath, _ := cmd.Flags().GetString("output")
		verbose, _ := cmd.Flags().GetBool("verbose")

		// Get LLM config
		apiKey := viper.GetString("llm.apiKey")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY not set")
		}

		model := viper.GetString("llm.model")
		if model == "" {
			model = llm.DefaultOpenAIModel
		}

		providerStr := viper.GetString("llm.provider")
		if providerStr == "" {
			providerStr = llm.DefaultProvider
		}

		provider, err := llm.ValidateProvider(providerStr)
		if err != nil {
			return err
		}

		llmCfg := llm.Config{
			Provider: provider,
			Model:    model,
			APIKey:   apiKey,
			BaseURL:  viper.GetString("llm.baseURL"),
		}

		// Get chain config
		var chainConfig spec.ChainConfig
		if specType != "" {
			chainConfig = spec.Preset(specType)
		} else {
			chainConfig = spec.Preset("feature")
		}

		// Add manually skipped agents
		for _, s := range skipAgents {
			chainConfig.Skip = append(chainConfig.Skip, spec.PersonaType(s))
		}

		chainConfig.Verbose = verbose || viper.GetBool("verbose")

		// Print header
		cwd, _ := os.Getwd()
		fmt.Println()
		fmt.Println("┌──────────────────────────────────────────────────────────────┐")
		fmt.Println("│  🎯 TaskWing Spec - Feature Specification                    │")
		fmt.Println("└──────────────────────────────────────────────────────────────┘")
		fmt.Println()
		fmt.Printf("  📝 Feature: %s\n", featureRequest)
		fmt.Printf("  ⚡ Model: %s\n", model)
		fmt.Println()

		// Create and run chain
		chain := spec.NewChain(llmCfg, chainConfig)
		result, err := chain.Execute(context.Background(), featureRequest)
		if err != nil {
			return fmt.Errorf("chain execution failed: %w", err)
		}

		// Determine output path
		if outputPath == "" {
			// Default: .taskwing/specs/<feature-slug>/spec.md
			slug := slugify(featureRequest)
			specDir := filepath.Join(cwd, ".taskwing", "specs", slug)
			if err := os.MkdirAll(specDir, 0755); err != nil {
				return fmt.Errorf("create spec directory: %w", err)
			}
			outputPath = filepath.Join(specDir, "spec.md")
		}

		// Write spec file
		if err := os.WriteFile(outputPath, []byte(result.MarkdownSpec), 0644); err != nil {
			return fmt.Errorf("write spec file: %w", err)
		}

		// Print summary
		fmt.Println()
		fmt.Printf("✅ Spec created: %s\n", outputPath)
		fmt.Printf("   Agents run: %d\n", len(result.Outputs))

		return nil
	},
}

var specListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all feature specifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		store, err := spec.NewStore(cwd)
		if err != nil {
			return err
		}

		specs, err := store.ListSpecs()
		if err != nil {
			return err
		}

		if len(specs) == 0 {
			fmt.Println("No specs found. Create one with: tw spec create \"feature description\"")
			return nil
		}

		if viper.GetBool("json") {
			data, _ := json.MarshalIndent(specs, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println()
		fmt.Println("┌──────────────────────────────────────────────────────────────┐")
		fmt.Println("│  📋 Feature Specifications                                   │")
		fmt.Println("└──────────────────────────────────────────────────────────────┘")
		fmt.Println()

		for _, s := range specs {
			statusIcon := "📝"
			switch s.Status {
			case spec.StatusApproved:
				statusIcon = "✅"
			case spec.StatusInProgress:
				statusIcon = "🚧"
			case spec.StatusDone:
				statusIcon = "✓"
			}
			fmt.Printf("  %s %s\n", statusIcon, s.Title)
			fmt.Printf("     ID: %s | Tasks: %d | Created: %s\n\n",
				s.ID, s.TaskCount, s.CreatedAt.Format("2006-01-02"))
		}

		return nil
	},
}

// Task commands
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage development tasks",
	Long: `List and manage tasks from feature specifications.

Examples:
  tw task list
  tw task list --spec add-stripe-integration
  tw task start task-abc123`,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		store, err := spec.NewStore(cwd)
		if err != nil {
			return err
		}

		specSlug, _ := cmd.Flags().GetString("spec")
		tasks, err := store.ListTasks(specSlug)
		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}

		if viper.GetBool("json") {
			data, _ := json.MarshalIndent(tasks, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println()
		fmt.Println("┌──────────────────────────────────────────────────────────────┐")
		fmt.Println("│  📋 Tasks                                                    │")
		fmt.Println("└──────────────────────────────────────────────────────────────┘")
		fmt.Println()

		for _, t := range tasks {
			statusIcon := "[ ]"
			switch t.Status {
			case spec.StatusInProgress:
				statusIcon = "[/]"
			case spec.StatusDone:
				statusIcon = "[x]"
			}
			fmt.Printf("  %s %s (%s)\n", statusIcon, t.Title, t.Estimate)
			fmt.Printf("      ID: %s | Priority: %d\n\n", t.ID, t.Priority)
		}

		return nil
	},
}

var taskStartCmd = &cobra.Command{
	Use:   "start [task-id]",
	Short: "Start a task and output context for AI tools",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		cwd, _ := os.Getwd()
		store, err := spec.NewStore(cwd)
		if err != nil {
			return err
		}

		// Get task context
		ctx, err := store.GetTaskContext(taskID)
		if err != nil {
			return fmt.Errorf("task not found: %w", err)
		}

		// Mark as in-progress
		if err := store.UpdateTaskStatus(taskID, spec.StatusInProgress); err != nil {
			return err
		}

		// Output context as JSON (for AI tools)
		if viper.GetBool("json") {
			data, _ := json.MarshalIndent(ctx, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Human-readable output
		fmt.Println()
		fmt.Println("┌──────────────────────────────────────────────────────────────┐")
		fmt.Println("│  🚀 Task Started                                             │")
		fmt.Println("└──────────────────────────────────────────────────────────────┘")
		fmt.Println()
		fmt.Printf("  📌 %s\n", ctx.Task.Title)
		fmt.Printf("  📝 %s\n", ctx.Task.Description)
		fmt.Printf("  ⏱️  Estimate: %s\n", ctx.Task.Estimate)
		fmt.Println()

		if len(ctx.Task.Files) > 0 {
			fmt.Println("  📂 Files to modify:")
			for _, f := range ctx.Task.Files {
				fmt.Printf("     - %s\n", f)
			}
			fmt.Println()
		}

		fmt.Println("  💡 Tip: Use --json to get full context for AI tools")

		return nil
	},
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	// Keep only alphanumeric and hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug := result.String()
	// Trim hyphens and limit length
	slug = strings.Trim(slug, "-")
	if len(slug) > 50 {
		slug = slug[:50]
	}
	return slug
}

func init() {
	rootCmd.AddCommand(specCmd)
	specCmd.AddCommand(specCreateCmd)
	specCmd.AddCommand(specListCmd)

	specCreateCmd.Flags().String("type", "", "Preset type: feature, bugfix, refactor, full")
	specCreateCmd.Flags().StringSlice("skip", nil, "Agents to skip: pm, architect, qa, monetization, ux")
	specCreateCmd.Flags().String("output", "", "Output path for spec file")
	specCreateCmd.Flags().BoolP("verbose", "v", false, "Verbose output")

	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskStartCmd)

	taskListCmd.Flags().String("spec", "", "Filter tasks by spec slug")
}
