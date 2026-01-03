/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	evalpkg "github.com/josephgoksu/TaskWing/internal/eval"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultEvalDirName     = "eval"
	defaultTasksFileName   = "tasks.yaml"
	defaultPromptFileName  = "task.txt"
	defaultResultsFileName = "results.json"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate model outputs against repo constraints",
}

func init() {
	// Persistent flags for all eval commands
	evalCmd.PersistentFlags().StringSliceP("model", "m", nil, "Model(s) to use (provider:model or model)")
}

var evalInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize eval harness in .taskwing/eval",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}
		evalDir := filepath.Join(cwd, ".taskwing", defaultEvalDirName)
		force, _ := cmd.Flags().GetBool("force")

		if err := os.MkdirAll(filepath.Join(evalDir, "prompts"), 0755); err != nil {
			return fmt.Errorf("create eval dirs: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(evalDir, "runs"), 0755); err != nil {
			return fmt.Errorf("create eval runs dir: %w", err)
		}

		tasksPath := filepath.Join(evalDir, defaultTasksFileName)
		promptPath := filepath.Join(evalDir, "prompts", defaultPromptFileName)

		if err := evalpkg.WriteFileIfMissing(tasksPath, evalpkg.DefaultTasksTemplate, force); err != nil {
			return err
		}
		if err := evalpkg.WriteFileIfMissing(promptPath, evalpkg.DefaultPromptTemplate, force); err != nil {
			return err
		}

		fmt.Printf("✓ Initialized eval harness at %s\n", evalDir)
		return nil
	},
}

var evalRunCmd = &cobra.Command{
	Use:          "run",
	Short:        "Run bootstrap + task prompts across models",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}

		models, _ := cmd.Flags().GetStringSlice("model")
		runner, _ := cmd.Flags().GetString("runner")

		// For external runners, --model is optional (used as label for results)
		// Default to a runner-based name if not provided
		if len(models) == 0 {
			if runner != "" && runner != "internal" {
				// External runner: use runner name as default model label
				runnerName := strings.Fields(runner)[0] // Extract command name (e.g., "codex" from "codex exec ...")
				models = []string{runnerName + "-default"}
			} else {
				return errors.New("at least one --model is required")
			}
		}

		bootstrapOnly, _ := cmd.Flags().GetBool("bootstrap-only")
		tasksOnly, _ := cmd.Flags().GetBool("tasks-only")
		noContext, _ := cmd.Flags().GetBool("no-context")
		if bootstrapOnly && tasksOnly {
			return errors.New("--bootstrap-only and --tasks-only cannot both be set")
		}
		doBootstrap := !tasksOnly
		doTasks := !bootstrapOnly

		// Fall back to EVAL_RUNNER env var if runner not set
		if runner == "" {
			runner = os.Getenv("EVAL_RUNNER")
		}
		if runner == "" && doTasks {
			runner = "internal"
		}

		// For baseline runs (--no-context with internal runner), skip bootstrap
		// since we won't inject context anyway. This speeds up baseline tests significantly.
		if noContext && runner == "internal" && !bootstrapOnly {
			doBootstrap = false
		}

		label, _ := cmd.Flags().GetString("label")
		if err := validateRunner(runner, label); err != nil {
			return err
		}

		evalDir := filepath.Join(cwd, ".taskwing", defaultEvalDirName)
		tasksPath, _ := cmd.Flags().GetString("tasks")
		if tasksPath == "" {
			tasksPath = filepath.Join(evalDir, defaultTasksFileName)
		}
		promptPath, _ := cmd.Flags().GetString("prompt")
		if promptPath == "" {
			promptPath = filepath.Join(evalDir, "prompts", defaultPromptFileName)
		}

		var cfg evalpkg.Config
		var promptTemplate []byte
		if doTasks {
			cfg, err = evalpkg.LoadConfig(tasksPath)
			if err != nil {
				return err
			}
			promptTemplate, err = os.ReadFile(promptPath)
			if err != nil {
				if os.IsNotExist(err) {
					// Auto-init prompt template if missing
					if err := os.MkdirAll(filepath.Dir(promptPath), 0755); err == nil {
						if err := os.WriteFile(promptPath, []byte(evalpkg.DefaultPromptTemplate), 0644); err == nil {
							fmt.Printf("✓ Created default prompt template at %s\n", promptPath)
							promptTemplate = []byte(evalpkg.DefaultPromptTemplate)
						}
					}
				}
				if promptTemplate == nil {
					return fmt.Errorf("read prompt template: %w", err)
				}
			}
		}

		stamp := time.Now().Format("20060102-150405")
		outDir, _ := cmd.Flags().GetString("out")
		if outDir == "" {
			if tasksOnly {
				return errors.New("--tasks-only requires --out pointing to an existing run directory")
			}
			outDir = filepath.Join(evalDir, "runs", stamp)
		}
		if tasksOnly {
			if _, err := os.Stat(outDir); err != nil {
				return fmt.Errorf("run directory not found: %w", err)
			}
		} else {
			if err := os.MkdirAll(outDir, 0755); err != nil {
				return fmt.Errorf("create out dir: %w", err)
			}
		}
		if err := os.MkdirAll(filepath.Join(outDir, "memory"), 0755); err != nil {
			return fmt.Errorf("create memory dir: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(outDir, "prompts"), 0755); err != nil {
			return fmt.Errorf("create prompts dir: %w", err)
		}

		judge, _ := cmd.Flags().GetBool("judge")
		injectContext := !noContext

		// Track token usage per model for cost calculation
		modelCosts := make(map[string]evalpkg.CostSummary)

		for _, model := range models {
			provider, modelName := evalpkg.ParseModel(model)
			displayModel := modelName
			if provider != "" {
				displayModel = provider + ":" + modelName
			}
			safeModel := evalpkg.SafeName(displayModel)
			modelOutDir := filepath.Join(outDir, "memory", safeModel)
			if tasksOnly {
				if _, err := os.Stat(modelOutDir); err != nil {
					return fmt.Errorf("missing model memory for %s: %w", displayModel, err)
				}
			} else if err := os.MkdirAll(modelOutDir, 0755); err != nil {
				return fmt.Errorf("create model memory dir: %w", err)
			}

			restore := setEvalViperOverrides(provider, modelName, modelOutDir)
			llmCfg, err := getLLMConfig(cmd)
			if err != nil {
				restore()
				return err
			}
			llmCfg.Model = modelName

			if doBootstrap {
				if !viper.GetBool("quiet") {
					fmt.Printf("\n==> Bootstrap: %s\n", model)
				}
				if err := runBootstrapViaCLI(cmd.Context(), cwd, viper.GetBool("quiet"), llmCfg); err != nil {
					restore()
					return fmt.Errorf("bootstrap failed for %s: %w", model, err)
				}
			}

			if doTasks {
				if !viper.GetBool("quiet") {
					if noContext {
						fmt.Printf("==> Tasks (no context): %s\n", displayModel)
					} else {
						fmt.Printf("==> Tasks: %s\n", displayModel)
					}
				}

				// Context injection now handled via CLI subprocess (tw context)
				// No need to open memory repository directly

				timeout, _ := cmd.Flags().GetDuration("timeout")
				resetRepo, _ := cmd.Flags().GetBool("reset-repo")
				usage, taskErr := runEvalTasks(cmd.Context(), cfg, outDir, promptTemplate, modelName, displayModel, safeModel, runner, cwd, llmCfg, injectContext, timeout, resetRepo)

				// Calculate and store cost
				cost := llm.CalculateCost(modelName, usage.InputTokens, usage.OutputTokens)
				modelCosts[safeModel] = evalpkg.CostSummary{
					InputTokens:  usage.InputTokens,
					OutputTokens: usage.OutputTokens,
					TotalTokens:  usage.InputTokens + usage.OutputTokens,
					CostUSD:      cost,
				}

				if taskErr != nil {
					restore()
					return fmt.Errorf("tasks failed for %s: %w", displayModel, taskErr)
				}
			}

			restore()
		}

		if judge && doTasks {
			// Calculate context mode for report
			contextMode := "taskwing"
			if noContext {
				if runner == "internal" {
					contextMode = "none"
				} else {
					contextMode = "raw"
				}
			}

			// Get LLM config for judge (use first model's config)
			judgeLLMCfg, _ := getLLMConfig(cmd)
			// Parse model to strip provider prefix (e.g., "openai:gpt-4o" -> "gpt-4o")
			if judgeLLMCfg.Model != "" {
				_, parsedModel := evalpkg.ParseModel(judgeLLMCfg.Model)
				judgeLLMCfg.Model = parsedModel
			}
			if err := runJudge(cmd.Context(), cfg, outDir, judgeLLMCfg, modelCosts, label, contextMode, runner); err != nil {
				return err
			}
			if !viper.GetBool("quiet") {
				fmt.Printf("\n✓ Results written to %s\n", filepath.Join(outDir, defaultResultsFileName))
			}
		}

		return nil
	},
}

func validateRunner(runner, label string) error {
	if runner == "internal" || runner == "" {
		return nil
	}
	// Require label for external runners to avoid benchmark confusion
	if label == "" {
		return errors.New("--label is required when using external runners (e.g., --label codex-only)")
	}
	// Check if runner command exists
	parts := strings.Fields(runner)
	if len(parts) > 0 {
		if _, err := exec.LookPath(parts[0]); err != nil {
			return fmt.Errorf("runner command %q not found in PATH", parts[0])
		}
	}
	return nil
}

var evalJudgeCmd = &cobra.Command{
	Use:   "judge",
	Short: "Evaluate task outputs against hard-fail rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}
		evalDir := filepath.Join(cwd, ".taskwing", defaultEvalDirName)
		outDir, _ := cmd.Flags().GetString("run")
		if outDir == "" {
			return errors.New("--run is required")
		}
		tasksPath, _ := cmd.Flags().GetString("tasks")
		if tasksPath == "" {
			tasksPath = filepath.Join(evalDir, defaultTasksFileName)
		}
		cfg, err := evalpkg.LoadConfig(tasksPath)
		if err != nil {
			return err
		}
		// Get LLM config for judge
		llmCfg, llmErr := getLLMConfig(cmd)
		if llmErr != nil {
			return fmt.Errorf("get llm config: %w", llmErr)
		}
		// Parse model to strip provider prefix (e.g., "openai:gpt-4o" -> "gpt-4o")
		if llmCfg.Model != "" {
			_, parsedModel := evalpkg.ParseModel(llmCfg.Model)
			llmCfg.Model = parsedModel
		}
		if err := runJudge(cmd.Context(), cfg, outDir, llmCfg, nil, "", "", ""); err != nil {
			return err
		}
		fmt.Printf("✓ Results written to %s\n", filepath.Join(outDir, defaultResultsFileName))
		return nil
	},
}

var evalReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Summarize eval results",
	RunE: func(cmd *cobra.Command, args []string) error {
		outDir, _ := cmd.Flags().GetString("run")
		if outDir == "" {
			return errors.New("--run is required")
		}
		_, _ = cmd.Flags().GetBool("verbose") // Kept for API compat, UI always shows full report
		resultsPath := filepath.Join(outDir, defaultResultsFileName)
		data, err := os.ReadFile(resultsPath)
		if err != nil {
			return fmt.Errorf("read results: %w", err)
		}
		var results evalpkg.Results
		if err := json.Unmarshal(data, &results); err != nil {
			return fmt.Errorf("parse results: %w", err)
		}

		if isJSON() {
			return printJSON(results)
		}

		// Build UI data
		reportData := buildEvalReportData(results)
		ui.RenderEvalReport(reportData)

		return nil
	},
}

var evalBenchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Aggregate runs into historical benchmark",
	Long: `Scan all eval runs and aggregate results into a benchmark report.

Shows how each model performs over time with trend indicators.

Examples:
  tw eval benchmark                    # All runs
  tw eval benchmark --since 2025-12-01 # From date
  tw eval benchmark --last 5           # Last 5 runs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}

		evalDir := filepath.Join(cwd, ".taskwing", defaultEvalDirName)
		runsDir := filepath.Join(evalDir, "runs")

		since, _ := cmd.Flags().GetString("since")
		last, _ := cmd.Flags().GetInt("last")
		outputPath, _ := cmd.Flags().GetString("output")

		// Scan runs directory
		entries, err := os.ReadDir(runsDir)
		if err != nil {
			return fmt.Errorf("read runs dir: %w", err)
		}

		var runs []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			runID := e.Name()
			// Filter by --since if provided
			if since != "" {
				runDate, parseErr := time.Parse("20060102", runID[:8])
				if parseErr == nil {
					sinceDate, sinceErr := time.Parse("2006-01-02", since)
					if sinceErr == nil && runDate.Before(sinceDate) {
						continue
					}
				}
			}
			// Check results.json exists
			resultsPath := filepath.Join(runsDir, runID, defaultResultsFileName)
			if _, statErr := os.Stat(resultsPath); statErr == nil {
				runs = append(runs, runID)
			}
		}

		if len(runs) == 0 {
			fmt.Println("No eval runs found.")
			return nil
		}

		sort.Strings(runs)

		// Apply --last filter
		if last > 0 && len(runs) > last {
			runs = runs[len(runs)-last:]
		}

		// Build benchmark data
		benchmarkData := buildBenchmarkData(runsDir, runs)

		if isJSON() || outputPath != "" {
			payload, marshalErr := json.MarshalIndent(benchmarkData, "", "  ")
			if marshalErr != nil {
				return fmt.Errorf("marshal benchmark: %w", marshalErr)
			}
			if outputPath != "" {
				if writeErr := os.WriteFile(outputPath, payload, 0644); writeErr != nil {
					return fmt.Errorf("write output: %w", writeErr)
				}
				fmt.Printf("✓ Benchmark written to %s\n", outputPath)
				return nil
			}
			fmt.Println(string(payload))
			return nil
		}

		ui.RenderBenchmark(benchmarkData)
		return nil
	},
}

var evalGenerateTasksCmd = &cobra.Command{
	Use:   "generate-tasks",
	Short: "Generate evaluation tasks from bootstrapped memory",
	Long: `Read project constraints from memory and generate evaluation tasks.

This makes eval repo-agnostic by auto-generating tasks based on the actual
constraints discovered during bootstrap.

Examples:
  tw eval generate-tasks            # Generate 5 tasks
  tw eval generate-tasks --count 10 # Generate 10 tasks`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}

		count, _ := cmd.Flags().GetInt("count")
		if count <= 0 {
			count = 5
		}

		llmCfg, err := getLLMConfig(cmd)
		if err != nil {
			return err
		}

		// Get context via CLI subprocess (replaces internal knowledge.Service)
		cliRunner := evalpkg.NewRunner(cwd).WithTimeout(2 * time.Minute)
		cliRunner.Quiet = true

		// Pass LLM config to subprocess via environment variables
		if llmCfg.APIKey != "" {
			cliRunner = cliRunner.WithEnv("TASKWING_LLM_APIKEY=" + llmCfg.APIKey)
		}
		if llmCfg.Provider != "" {
			cliRunner = cliRunner.WithEnv("TASKWING_LLM_PROVIDER=" + string(llmCfg.Provider))
		}

		if !viper.GetBool("quiet") {
			fmt.Println("🔍 Reading project constraints from memory via CLI...")
		}

		// Use tw context to get architectural constraints and workflows
		ctx := cmd.Context()
		result, err := cliRunner.Execute(ctx, "context", "architectural constraints decisions rules workflows processes patterns")
		if err != nil {
			return fmt.Errorf("context retrieval: %w (run 'tw bootstrap' first)", err)
		}

		contextStr := result.Stdout
		if len(contextStr) < 100 {
			return errors.New("not enough context in memory. Run 'tw bootstrap' first to populate knowledge")
		}

		if !viper.GetBool("quiet") {
			fmt.Printf("🧠 Generating %d evaluation tasks...\n", count)
		}

		// Generate tasks via LLM
		generatePrompt := fmt.Sprintf(`You are generating evaluation tasks for an AI coding assistant.

## Project Constraints, Workflows, and Decisions
%s

## Instructions
Generate %d evaluation tasks that test whether an AI assistant follows these constraints and workflows.
Prioritize tasks that test **Workflows** (e.g. "Add new API endpoint") or **Violatable Constraints** (e.g. "Must use X library").
Each task should be a realistic coding request that could violate the rules if done incorrectly.

## CRITICAL: Criteria Format
- **expected**: Write SEMANTIC pass criteria, NOT exact code.
  - GOOD: "Must explain that OIDC requires id-token write permission"
  - BAD: "Return a snippet with permissions: { id-token: write }"
- **failure_signals**: List behavioral violations, NOT syntax comparisons.
  - GOOD: "Uses static AWS keys instead of OIDC role assumption"
  - BAD: "Missing exact text 'id-token: write'"

The goal is: if the answer correctly follows the constraint in spirit, it should pass.
Allow flexibility in HOW the answer is phrased while catching real violations.

Output ONLY valid YAML (no markdown code blocks):
tasks:
  - id: T1
    title: "Short title"
    prompt: "Realistic coding request that triggers a workflow or constraint"
    expected: "Semantic description of what a correct answer MUST include (principles, not syntax)"
    failure_signals: "Behavioral signs of failure (constraint violations, not phrasing differences)"
  - id: T2
    ...`, contextStr, count)

		chatModel, err := llm.NewCloseableChatModel(ctx, llmCfg)
		if err != nil {
			return fmt.Errorf("create llm: %w", err)
		}
		defer chatModel.Close()

		resp, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage(generatePrompt)})
		if err != nil {
			return fmt.Errorf("generate tasks: %w", err)
		}

		// Write to tasks.yaml
		evalDir := filepath.Join(cwd, ".taskwing", defaultEvalDirName)
		if err := os.MkdirAll(evalDir, 0755); err != nil {
			return fmt.Errorf("create eval dir: %w", err)
		}

		tasksPath := filepath.Join(evalDir, defaultTasksFileName)

		// Check if file exists and confirm unless --force
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			if _, statErr := os.Stat(tasksPath); statErr == nil {
				fmt.Printf("⚠️  %s already exists. Overwrite? [y/N]: ", tasksPath)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					return errors.New("aborted")
				}
			}
		}

		// Clean up response (remove markdown if present)
		content := strings.TrimSpace(resp.Content)
		if strings.HasPrefix(content, "```") {
			lines := strings.Split(content, "\n")
			var yamlLines []string
			inBlock := false
			for _, line := range lines {
				if strings.HasPrefix(line, "```") {
					inBlock = !inBlock
					continue
				}
				if inBlock {
					yamlLines = append(yamlLines, line)
				}
			}
			content = strings.Join(yamlLines, "\n")
		}

		// Add header
		fullContent := fmt.Sprintf(`# Auto-generated evaluation tasks
# Generated from project memory by: tw eval generate-tasks
# Regenerate anytime with: tw eval generate-tasks --count %d

%s`, count, content)

		if err := os.WriteFile(tasksPath, []byte(fullContent), 0644); err != nil {
			return fmt.Errorf("write tasks: %w", err)
		}

		fmt.Printf("✓ Generated %d tasks at %s\n", count, tasksPath)
		return nil
	},
}

func init() {
	evalInitCmd.Flags().Bool("force", false, "Overwrite existing eval templates")

	evalRunCmd.Flags().String("runner", "", "Runner command with {model} and {prompt} or {prompt_file} (default: internal)")
	evalRunCmd.Flags().String("tasks", "", "Path to tasks.yaml")
	evalRunCmd.Flags().String("prompt", "", "Path to prompt template")
	evalRunCmd.Flags().String("out", "", "Output directory (default: .taskwing/eval/runs/<timestamp>)")
	evalRunCmd.Flags().Bool("judge", true, "Run judge after tasks complete")
	evalRunCmd.Flags().Bool("bootstrap-only", false, "Run bootstrap only (skip tasks)")
	evalRunCmd.Flags().Bool("tasks-only", false, "Run tasks only (skip bootstrap)")
	evalRunCmd.Flags().Bool("no-context", false, "Skip context injection (baseline run without TaskWing)")
	evalRunCmd.Flags().String("label", "", "Tag for benchmark grouping (required for external runners)")
	evalRunCmd.Flags().Duration("timeout", 10*time.Minute, "Timeout for external runner execution")
	evalRunCmd.Flags().Bool("reset-repo", false, "Reset git state between tasks (for agentic runners like Codex)")

	evalJudgeCmd.Flags().String("run", "", "Run directory containing task outputs")
	evalJudgeCmd.Flags().String("tasks", "", "Path to tasks.yaml")

	evalReportCmd.Flags().String("run", "", "Run directory containing results.json")
	evalReportCmd.Flags().Bool("verbose", false, "Show per-task results")

	evalBenchmarkCmd.Flags().String("since", "", "Only include runs from this date (YYYY-MM-DD)")
	evalBenchmarkCmd.Flags().Int("last", 0, "Only include last N runs")
	evalBenchmarkCmd.Flags().String("output", "", "Write benchmark to file (JSON)")

	evalGenerateTasksCmd.Flags().Int("count", 5, "Number of tasks to generate")
	evalGenerateTasksCmd.Flags().Bool("force", false, "Overwrite existing tasks.yaml without confirmation")

	evalCmd.AddCommand(evalInitCmd, evalRunCmd, evalJudgeCmd, evalReportCmd, evalBenchmarkCmd, evalGenerateTasksCmd)
	rootCmd.AddCommand(evalCmd)
}

// runBootstrapViaCLI invokes `tw bootstrap` as a subprocess.
// This ensures eval tests the actual CLI behavior, not a reimplementation.
func runBootstrapViaCLI(ctx context.Context, cwd string, quiet bool, llmCfg llm.Config) error {
	runner := evalpkg.NewRunner(cwd).WithTimeout(10 * time.Minute)
	runner.Quiet = quiet

	// Pass LLM config to subprocess via environment variables
	// This ensures the subprocess uses the same configuration as the parent
	if llmCfg.APIKey != "" {
		runner = runner.WithEnv("TASKWING_LLM_APIKEY=" + llmCfg.APIKey)
	}
	if llmCfg.Provider != "" {
		runner = runner.WithEnv("TASKWING_LLM_PROVIDER=" + string(llmCfg.Provider))
	}
	if llmCfg.Model != "" {
		runner = runner.WithEnv("TASKWING_LLM_MODEL=" + llmCfg.Model)
	}
	if llmCfg.BaseURL != "" {
		runner = runner.WithEnv("TASKWING_LLM_BASEURL=" + llmCfg.BaseURL)
	}

	args := []string{"bootstrap"}
	if quiet {
		args = append(args, "--quiet")
	}

	result, err := runner.Execute(ctx, args...)
	if err != nil {
		return fmt.Errorf("bootstrap failed: %w\nOutput: %s", err, result.Combined())
	}

	return nil
}

func runEvalTasks(ctx context.Context, cfg evalpkg.Config, outDir string, promptTemplate []byte, model string, displayModel string, safeModel string, runner string, repoPath string, llmCfg llm.Config, injectContext bool, timeout time.Duration, resetRepo bool) (evalpkg.TokenUsage, error) {
	// Create CLI runner for context retrieval (replaces internal knowledge.Service)
	cliRunner := evalpkg.NewRunner(repoPath).WithTimeout(2 * time.Minute)
	cliRunner.Quiet = true

	// Pass LLM config to subprocess via environment variables
	if llmCfg.APIKey != "" {
		cliRunner = cliRunner.WithEnv("TASKWING_LLM_APIKEY=" + llmCfg.APIKey)
	}
	if llmCfg.Provider != "" {
		cliRunner = cliRunner.WithEnv("TASKWING_LLM_PROVIDER=" + string(llmCfg.Provider))
	}

	var totalUsage evalpkg.TokenUsage
	var runErrs []error
	totalTasks := len(cfg.Tasks)
	for i, task := range cfg.Tasks {
		// Reset repo state between tasks if requested (for agentic runners like Codex)
		if resetRepo && i > 0 {
			resetCmd := exec.CommandContext(ctx, "sh", "-c", "git checkout . && git clean -fd")
			resetCmd.Dir = repoPath
			if out, err := resetCmd.CombinedOutput(); err != nil {
				if !viper.GetBool("quiet") {
					fmt.Printf("  ⚠️  git reset failed: %v (%s)\n", err, string(out))
				}
			} else if !viper.GetBool("quiet") {
				fmt.Printf("  🔄 Reset repo state\n")
			}
		}

		if !viper.GetBool("quiet") {
			fmt.Printf("  [%d/%d] %s...", i+1, totalTasks, task.ID)
		}
		prompt := strings.ReplaceAll(string(promptTemplate), "{{repo}}", repoPath)
		prompt = strings.ReplaceAll(prompt, "{{task}}", strings.TrimSpace(task.Prompt))

		// Context injection via CLI: invoke `tw context` as subprocess
		if injectContext && task.Prompt != "" {
			if !viper.GetBool("quiet") {
				fmt.Println("  Retrieving context via CLI...")
			}

			// Use tw context command to get relevant knowledge
			result, err := cliRunner.Execute(ctx, "context", task.Prompt)
			if err != nil {
				if !viper.GetBool("quiet") {
					fmt.Printf("  ⚠️  Context retrieval warning: %v\n", err)
				}
			} else if result.Stdout != "" {
				if !viper.GetBool("quiet") {
					fmt.Printf("  Injected context (%d bytes)\n", len(result.Stdout))
				}
				// Append context to prompt
				prompt += "\n\n## Project Context\n" + result.Stdout
			}
		}

		promptPath := filepath.Join(outDir, "prompts", fmt.Sprintf("task-%s-%s.txt", task.ID, safeModel))
		if err := os.WriteFile(promptPath, []byte(prompt), 0644); err != nil {
			return totalUsage, fmt.Errorf("write prompt: %w", err)
		}

		outputPath := filepath.Join(outDir, fmt.Sprintf("task-%s-%s.txt", task.ID, safeModel))
		start := time.Now()

		var out []byte
		var err error
		var taskUsage evalpkg.TokenUsage
		if runner == "internal" {
			// Apply timeout to internal runner too
			ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
			out, taskUsage, err = runEvalTaskInternal(ctxWithTimeout, llmCfg, prompt)
			cancel()
			totalUsage.InputTokens += taskUsage.InputTokens
			totalUsage.OutputTokens += taskUsage.OutputTokens
			if ctxWithTimeout.Err() == context.DeadlineExceeded {
				err = fmt.Errorf("timeout exceeded (%v)", timeout)
			}
		} else {
			cmdStr := strings.ReplaceAll(runner, "{model}", model)
			if strings.Contains(cmdStr, "{prompt_file}") {
				cmdStr = strings.ReplaceAll(cmdStr, "{prompt_file}", shellEscape(promptPath))
			} else {
				cmdStr = strings.ReplaceAll(cmdStr, "{prompt}", shellEscape(prompt))
			}

			// Run with timeout
			ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
			cmd := exec.CommandContext(ctxWithTimeout, "sh", "-c", cmdStr)
			cmd.Env = os.Environ()
			out, err = cmd.CombinedOutput()
			cancel() // Ensure resources are cleaned up

			if ctxWithTimeout.Err() == context.DeadlineExceeded {
				err = fmt.Errorf("timeout exceeded (%v)", timeout)
			}
		}
		duration := time.Since(start)

		if err != nil {
			out = append(out, []byte(fmt.Sprintf("\n[error] %v\n", err))...)
		}
		if writeErr := os.WriteFile(outputPath, append(out, []byte(fmt.Sprintf("\n[meta] model=%s duration=%s\n", displayModel, duration.Round(time.Millisecond)))...), 0644); writeErr != nil {
			return totalUsage, fmt.Errorf("write output: %w", writeErr)
		}
		if err != nil {
			if !viper.GetBool("quiet") {
				fmt.Printf(" ✗ (%.1fs)\n", duration.Seconds())
			}
			runErrs = append(runErrs, fmt.Errorf("task %s: %w", task.ID, err))
			continue
		}
		if !viper.GetBool("quiet") {
			fmt.Printf(" ✓ (%.1fs)\n", duration.Seconds())
		}
	}
	if len(runErrs) > 0 {
		return totalUsage, fmt.Errorf("runner failed: %v", runErrs)
	}
	return totalUsage, nil
}

// runEvalTaskInternal runs a single eval task with retry logic
func runEvalTaskInternal(ctx context.Context, llmCfg llm.Config, prompt string) ([]byte, evalpkg.TokenUsage, error) {
	var usage evalpkg.TokenUsage
	var lastErr error

	// Retry up to 3 times
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second) // Backoff
		}

		chatModel, err := llm.NewCloseableChatModel(ctx, llmCfg)
		if err != nil {
			if !viper.GetBool("quiet") {
				fmt.Printf(" [attempt %d failed to create model: %v]", attempt+1, err)
			}
			lastErr = err
			continue
		}

		resp, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
		chatModel.Close() // Close immediately after use, before potential return
		if err != nil {
			if !viper.GetBool("quiet") {
				fmt.Printf(" [attempt %d failed: %v]", attempt+1, err)
			}
			lastErr = fmt.Errorf("llm generate (attempt %d): %w", attempt+1, err)
			continue
		}

		// Note: Token usage is not directly available from schema.Message
		// Token tracking requires callback integration (future enhancement)

		return []byte(resp.Content), usage, nil
	}

	return nil, usage, lastErr
}

// runLLMJudge uses an LLM to semantically evaluate model output.
// Runs the judge 3 times and takes the median score for consistency.
func runLLMJudge(ctx context.Context, llmCfg llm.Config, task evalpkg.Task, output string) (evalpkg.JudgeResult, error) {
	// Build expected behavior from task fields
	expected := task.Expected
	if expected == "" && task.PassFail.Pass != "" {
		expected = task.PassFail.Pass
	}

	failureSignals := task.FailureSignals
	if failureSignals == "" && task.PassFail.Fail != "" {
		failureSignals = task.PassFail.Fail
	}

	judgePrompt := fmt.Sprintf(`You are evaluating an AI coding assistant's response.

## Task Given to AI
%s

## Expected Behavior
%s

## Failure Signals (violations)
%s

## AI's Response
%s

## Scoring Rubric (0-10)
- 10: Perfect - all MUST and SHOULD requirements met, correct tech stack, correct file paths
- 8-9: Excellent - all MUST met, most SHOULD met, correct tech stack
- 6-7: Good - all MUST met, some SHOULD missing, correct tech stack
- 4-5: Partial - some MUST requirements missing OR minor tech stack confusion
- 2-3: Poor - WRONG TECH STACK or fundamentally wrong file paths/structure
- 0-1: Fail - completely wrong language/framework, dangerous, or nonsensical

## CRITICAL: Tech Stack Correctness
HEAVILY PENALIZE responses that:
- Use the WRONG programming language (e.g., TypeScript when repo uses Go, or vice versa)
- Reference WRONG file paths (e.g., src/types/openapi.ts when repo uses internal/api/types.gen.go)
- Suggest WRONG package managers (e.g., npm when repo uses go mod)
- Assume WRONG frameworks (e.g., Next.js when repo uses Chi/Gin)
- Miss repo-specific generated types or conventions mentioned in expected behavior

If the response assumes the wrong tech stack, it CANNOT score above 3, regardless of how well-structured the answer is otherwise.

## Instructions
Evaluate the response against the expected behavior and failure signals.
Consider semantic meaning, not just keyword matching.
Pay special attention to whether file paths and tech stack match what's expected.

Output ONLY valid JSON:
{"score": 8, "reason": "brief explanation"}`,
		task.Prompt, expected, failureSignals, output)

	// Run judge 3 times for consistency (median score reduces variance)
	const judgeRuns = 3
	type judgeRun struct {
		score  int
		reason string
	}
	var runs []judgeRun
	var lastErr error

	for i := 0; i < judgeRuns; i++ {
		result, err := runSingleJudge(ctx, llmCfg, judgePrompt)
		if err != nil {
			lastErr = err
			continue
		}
		runs = append(runs, judgeRun{score: result.Score, reason: result.Reason})
	}

	if len(runs) == 0 {
		return evalpkg.JudgeResult{}, fmt.Errorf("all judge runs failed: %w", lastErr)
	}

	// Sort by score to find median
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].score < runs[j].score
	})

	// Take median (middle element after sorting)
	medianIdx := len(runs) / 2
	medianRun := runs[medianIdx]

	result := evalpkg.JudgeResult{
		Score:  medianRun.score,
		Reason: medianRun.reason,
		Pass:   medianRun.score >= evalpkg.ScoreThreshold,
	}

	return result, nil
}

// runSingleJudge executes a single judge evaluation
func runSingleJudge(ctx context.Context, llmCfg llm.Config, prompt string) (evalpkg.JudgeResult, error) {
	chatModel, err := llm.NewCloseableChatModel(ctx, llmCfg)
	if err != nil {
		return evalpkg.JudgeResult{}, fmt.Errorf("create judge model: %w", err)
	}
	defer chatModel.Close()

	resp, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		return evalpkg.JudgeResult{}, fmt.Errorf("judge generate: %w", err)
	}

	// Parse JSON response
	var result evalpkg.JudgeResult
	content := strings.TrimSpace(resp.Content)
	// Handle markdown code blocks
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		content = strings.Join(jsonLines, "\n")
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return evalpkg.JudgeResult{Score: 0, Pass: false, Reason: "Judge parse error: " + content}, nil
	}

	return result, nil
}

func runJudge(ctx context.Context, cfg evalpkg.Config, outDir string, llmCfg llm.Config, modelCosts map[string]evalpkg.CostSummary, label, contextMode, runner string) error {
	results := evalpkg.Results{
		GeneratedAt: time.Now().UTC(),
		Costs:       modelCosts,
		Label:       label,
		ContextMode: contextMode,
		Runner:      runner,
	}

	outputs, err := filepath.Glob(filepath.Join(outDir, "task-*-*.txt"))
	if err != nil {
		return fmt.Errorf("list outputs: %w", err)
	}
	if len(outputs) == 0 {
		return fmt.Errorf("no task outputs found in %s", outDir)
	}

	// Build task lookup for LLM judge
	taskLookup := make(map[string]evalpkg.Task)
	for _, t := range cfg.Tasks {
		taskLookup[t.ID] = t
	}

	// Check if any task has Expected field (use LLM judge)
	useLLMJudge := false
	for _, t := range cfg.Tasks {
		if t.Expected != "" || t.FailureSignals != "" || t.PassFail.Pass != "" {
			useLLMJudge = true
			break
		}
	}

	for _, outputPath := range outputs {
		base := filepath.Base(outputPath)
		parts := strings.Split(base, "-")
		if len(parts) < 3 {
			continue
		}
		taskID := parts[1]
		model := strings.TrimSuffix(strings.Join(parts[2:], "-"), ".txt")

		data, err := os.ReadFile(outputPath)
		if err != nil {
			return fmt.Errorf("read output: %w", err)
		}
		text := string(data)

		var hardFail bool
		var score int
		var checks map[string]evalpkg.RuleCheck
		var judgeReason string

		task, hasTask := taskLookup[taskID]
		if useLLMJudge && hasTask && (task.Expected != "" || task.FailureSignals != "" || task.PassFail.Pass != "") {
			// Use LLM judge for semantic evaluation
			if !viper.GetBool("quiet") {
				fmt.Printf("  Judging %s/%s with LLM...", taskID, shortenModelForDisplay(model))
			}
			judgeResult, judgeErr := runLLMJudge(ctx, llmCfg, task, text)
			if judgeErr != nil {
				if !viper.GetBool("quiet") {
					fmt.Printf(" error: %v\n", judgeErr)
				}
				hardFail = true
				judgeReason = "Judge error: " + judgeErr.Error()
			} else {
				score = judgeResult.Score
				hardFail = !judgeResult.Pass
				judgeReason = judgeResult.Reason
				if !viper.GetBool("quiet") {
					if judgeResult.Pass {
						fmt.Printf(" %d/10 ✓\n", score)
					} else {
						fmt.Printf(" %d/10 ✗\n", score)
					}
				}
			}
		} else {
			// Fallback to regex-based evaluation
			checks, hardFail = evaluateRules(cfg.HardFailRules, taskID, text)
			if !hardFail {
				score = 10 // Treat regex pass as perfect score
			} else {
				score = 0 // Treat regex fail as zero score
			}
		}

		results.TaskResults = append(results.TaskResults, evalpkg.TaskResult{
			Task:        taskID,
			Model:       model,
			Score:       score,
			HardFail:    hardFail,
			Checks:      checks,
			JudgeReason: judgeReason,
			OutputFile:  outputPath,
		})
	}

	results.Summary = summarize(results.TaskResults)

	outPath := filepath.Join(outDir, defaultResultsFileName)
	payload, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	if err := os.WriteFile(outPath, payload, 0644); err != nil {
		return fmt.Errorf("write results: %w", err)
	}
	return nil
}

// shortenModelForDisplay returns a short model name for console output
func shortenModelForDisplay(model string) string {
	model = strings.TrimPrefix(model, "openai_")
	model = strings.TrimPrefix(model, "anthropic_")
	if len(model) > 15 {
		return model[:12] + "..."
	}
	return model
}

func evaluateRules(rules []evalpkg.Rule, taskID string, text string) (map[string]evalpkg.RuleCheck, bool) {
	checks := make(map[string]evalpkg.RuleCheck)
	failed := false

	for _, rule := range rules {
		if len(rule.TaskIDs) > 0 && !slices.Contains(rule.TaskIDs, taskID) {
			continue
		}
		check := evalpkg.RuleCheck{RequireAll: map[string]bool{}, RequireAny: map[string]bool{}, Forbid: map[string]bool{}, AllowIf: map[string]bool{}}
		matchesAll := true
		matchesAny := false

		for _, pattern := range rule.RequireAll {
			match := regexMatch(pattern, text)
			check.RequireAll[pattern] = match
			if !match {
				matchesAll = false
			}
		}
		if len(rule.RequireAll) == 0 {
			matchesAll = true
		}

		for _, pattern := range rule.RequireAny {
			match := regexMatch(pattern, text)
			check.RequireAny[pattern] = match
			if match {
				matchesAny = true
			}
		}
		if len(rule.RequireAny) == 0 {
			matchesAny = true
		}

		for _, pattern := range rule.Forbid {
			check.Forbid[pattern] = regexMatch(pattern, text)
		}

		for _, pattern := range rule.AllowIf {
			check.AllowIf[pattern] = regexMatch(pattern, text)
		}

		violatesForbid := false
		for _, hit := range check.Forbid {
			if hit {
				violatesForbid = true
				break
			}
		}
		allowed := false
		for _, hit := range check.AllowIf {
			if hit {
				allowed = true
				break
			}
		}

		check.Pass = matchesAll && matchesAny && (!violatesForbid || allowed)
		checks[rule.ID] = check
		if !check.Pass {
			failed = true
		}
	}

	return checks, failed
}

func regexMatch(pattern string, text string) bool {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return false
	}
	return re.MatchString(text)
}

func summarize(results []evalpkg.TaskResult) map[string]evalpkg.Summary {
	summary := make(map[string]evalpkg.Summary)
	for _, r := range results {
		item := summary[r.Model]
		item.Total++
		item.TotalScore += r.Score
		if r.HardFail {
			item.HardFail++
		}
		summary[r.Model] = item
	}
	// Calculate average scores
	for model, item := range summary {
		if item.Total > 0 {
			item.AvgScore = float64(item.TotalScore) / float64(item.Total)
		}
		summary[model] = item
	}
	return summary
}

func failedRules(checks map[string]evalpkg.RuleCheck) []string {
	var failed []string
	for id, check := range checks {
		if !check.Pass {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	return failed
}

// buildEvalReportData converts eval results to ui.EvalReportData for styled rendering
func buildEvalReportData(results evalpkg.Results) ui.EvalReportData {
	data := ui.EvalReportData{
		Models:  make(map[string]ui.EvalModelSummary),
		Results: make([]ui.EvalTaskResult, 0, len(results.TaskResults)),
	}

	// Convert summary
	for model, summary := range results.Summary {
		data.Models[model] = ui.EvalModelSummary{
			Total:    summary.Total,
			HardFail: summary.HardFail,
		}
	}

	// Convert results and collect unique tasks
	taskSet := make(map[string]bool)
	for _, r := range results.TaskResults {
		taskSet[r.Task] = true
		data.Results = append(data.Results, ui.EvalTaskResult{
			Task:        r.Task,
			Model:       r.Model,
			Pass:        !r.HardFail,
			FailedRules: failedRules(r.Checks),
			JudgeReason: r.JudgeReason,
			Score:       r.Score,
		})
	}

	// Extract unique tasks in order
	for task := range taskSet {
		data.Tasks = append(data.Tasks, task)
	}
	sort.Strings(data.Tasks)

	return data
}

// buildBenchmarkData aggregates multiple runs into benchmark data
func buildBenchmarkData(runsDir string, runs []string) ui.BenchmarkData {
	data := ui.BenchmarkData{
		Runs:   runs,
		Matrix: make(map[string]map[string]ui.BenchmarkRun),
	}

	modelSet := make(map[string]bool)
	taskIDSet := make(map[string]bool)

	for _, runID := range runs {
		resultsPath := filepath.Join(runsDir, runID, defaultResultsFileName)
		fileData, err := os.ReadFile(resultsPath)
		if err != nil {
			continue
		}

		var results evalpkg.Results
		if err := json.Unmarshal(fileData, &results); err != nil {
			continue
		}

		// Parse run date from runID (format: 20060102-150405)
		runDate, _ := time.Parse("20060102-150405", runID)

		for model, summary := range results.Summary {
			// If label is present, use it for grouping row name
			rowName := model
			if results.Label != "" {
				rowName = fmt.Sprintf("%s (%s)", results.Label, model)
			}

			modelSet[rowName] = true
			passed := summary.Total - summary.HardFail
			passRate := 0.0
			if summary.Total > 0 {
				passRate = float64(passed) / float64(summary.Total)
			}

			if data.Matrix[rowName] == nil {
				data.Matrix[rowName] = make(map[string]ui.BenchmarkRun)
			}

			// Collect task scores
			taskScores := make(map[string]int)
			for _, tr := range results.TaskResults {
				if tr.Model == model {
					taskScores[tr.Task] = tr.Score
					taskIDSet[tr.Task] = true
				}
			}

			run := ui.BenchmarkRun{
				RunID:      runID,
				RunDate:    runDate,
				Model:      model,
				Label:      results.Label,
				PassRate:   passRate,
				AvgScore:   summary.AvgScore,
				TaskScores: taskScores,
				Pass:       passed,
				Total:      summary.Total,
			}

			// Add cost data if available
			if results.Costs != nil {
				if cost, ok := results.Costs[model]; ok {
					run.InputTokens = cost.InputTokens
					run.OutputTokens = cost.OutputTokens
					run.CostUSD = cost.CostUSD
				}
			}

			data.Matrix[rowName][runID] = run
		}
	}

	// Extract unique models
	for model := range modelSet {
		data.Models = append(data.Models, model)
	}
	sort.Strings(data.Models)

	// Extract unique task IDs
	for t := range taskIDSet {
		data.TaskIDs = append(data.TaskIDs, t)
	}
	sort.Strings(data.TaskIDs)

	return data
}

func shellEscape(input string) string {
	if input == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(input, "'", "'\\''") + "'"
}

func setEvalViperOverrides(provider, model, memoryPath string) func() {
	prev := map[string]*string{}

	set := func(key, value string) {
		if viper.IsSet(key) {
			v := viper.GetString(key)
			prev[key] = &v
		} else {
			prev[key] = nil
		}
		viper.Set(key, value)
	}

	set("memory.path", memoryPath)
	set("llm.model", model)
	if provider != "" {
		set("llm.provider", provider)
	}

	return func() {
		for key, value := range prev {
			if value == nil {
				viper.Set(key, nil)
			} else {
				viper.Set(key, *value)
			}
		}
	}
}
