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
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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

		if err := writeFileIfMissing(tasksPath, defaultTasksTemplate, force); err != nil {
			return err
		}
		if err := writeFileIfMissing(promptPath, defaultPromptTemplate, force); err != nil {
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
		if len(models) == 0 {
			return errors.New("at least one --model is required")
		}

		bootstrapOnly, _ := cmd.Flags().GetBool("bootstrap-only")
		tasksOnly, _ := cmd.Flags().GetBool("tasks-only")
		if bootstrapOnly && tasksOnly {
			return errors.New("--bootstrap-only and --tasks-only cannot both be set")
		}
		doBootstrap := !tasksOnly
		doTasks := !bootstrapOnly

		runner, _ := cmd.Flags().GetString("runner")
		if runner == "" {
			runner = os.Getenv("EVAL_RUNNER")
		}
		if runner == "" && doTasks {
			runner = "internal"
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

		var cfg EvalConfig
		var promptTemplate []byte
		if doTasks {
			cfg, err = loadEvalConfig(tasksPath)
			if err != nil {
				return err
			}
			promptTemplate, err = os.ReadFile(promptPath)
			if err != nil {
				return fmt.Errorf("read prompt template: %w", err)
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

		for _, model := range models {
			provider, modelName := parseModel(model)
			displayModel := modelName
			if provider != "" {
				displayModel = provider + ":" + modelName
			}
			safeModel := safeName(displayModel)
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

			if doBootstrap {
				if !viper.GetBool("quiet") {
					fmt.Printf("\n==> Bootstrap: %s\n", model)
				}
				if err := runBootstrapNoTUI(cmd.Context(), cwd, llmCfg); err != nil {
					restore()
					return fmt.Errorf("bootstrap failed for %s: %w", model, err)
				}
			}

			if doTasks {
				if !viper.GetBool("quiet") {
					fmt.Printf("==> Tasks: %s\n", displayModel)
				}

				// Open per-model memory for context injection (isolated per model)
				var modelRepo *memory.Repository
				modelMemoryPath := filepath.Join(modelOutDir, "memory.db")
				if _, statErr := os.Stat(modelMemoryPath); statErr == nil {
					modelRepo, err = memory.NewDefaultRepository(modelOutDir)
					if err != nil {
						if !viper.GetBool("quiet") {
							fmt.Fprintf(os.Stderr, "⚠️  Could not open model memory for context: %v\n", err)
						}
					}
				}

				if err := runEvalTasks(cmd.Context(), cfg, outDir, promptTemplate, modelName, displayModel, safeModel, runner, cwd, llmCfg, modelRepo); err != nil {
					if modelRepo != nil {
						_ = modelRepo.Close()
					}
					restore()
					return fmt.Errorf("tasks failed for %s: %w", displayModel, err)
				}

				if modelRepo != nil {
					_ = modelRepo.Close()
				}
			}

			restore()
		}

		if judge && doTasks {
			if err := runJudge(cfg, outDir); err != nil {
				return err
			}
			if !viper.GetBool("quiet") {
				fmt.Printf("\n✓ Results written to %s\n", filepath.Join(outDir, defaultResultsFileName))
			}
		}

		return nil
	},
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
		cfg, err := loadEvalConfig(tasksPath)
		if err != nil {
			return err
		}
		if err := runJudge(cfg, outDir); err != nil {
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
		var results EvalResults
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

func init() {
	evalInitCmd.Flags().Bool("force", false, "Overwrite existing eval templates")

	evalRunCmd.Flags().StringSlice("model", nil, "Model to evaluate (provider:model or model)")
	evalRunCmd.Flags().String("runner", "", "Runner command with {model} and {prompt} or {prompt_file} (default: internal)")
	evalRunCmd.Flags().String("tasks", "", "Path to tasks.yaml")
	evalRunCmd.Flags().String("prompt", "", "Path to prompt template")
	evalRunCmd.Flags().String("out", "", "Output directory (default: .taskwing/eval/runs/<timestamp>)")
	evalRunCmd.Flags().Bool("judge", true, "Run judge after tasks complete")
	evalRunCmd.Flags().Bool("bootstrap-only", false, "Run bootstrap only (skip tasks)")
	evalRunCmd.Flags().Bool("tasks-only", false, "Run tasks only (skip bootstrap)")

	evalJudgeCmd.Flags().String("run", "", "Run directory containing task outputs")
	evalJudgeCmd.Flags().String("tasks", "", "Path to tasks.yaml")

	evalReportCmd.Flags().String("run", "", "Run directory containing results.json")
	evalReportCmd.Flags().Bool("verbose", false, "Show per-task results")

	evalCmd.AddCommand(evalInitCmd, evalRunCmd, evalJudgeCmd, evalReportCmd)
	rootCmd.AddCommand(evalCmd)
}

func runBootstrapNoTUI(ctx context.Context, cwd string, llmCfg llm.Config) error {
	agentsList := bootstrap.NewDefaultAgents(llmCfg, cwd)
	input := core.Input{
		BasePath:    cwd,
		ProjectName: filepath.Base(cwd),
		Mode:        core.ModeBootstrap,
		Verbose:     true,
	}

	var results []core.Output
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, agent := range agentsList {
		wg.Add(1)
		go func(a core.Agent) {
			defer wg.Done()
			start := time.Now()
			out, err := a.Run(ctx, input)
			duration := time.Since(start)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errs = append(errs, fmt.Errorf("agent %s failed: %w", a.Name(), err))
				return
			}
			if out.Duration == 0 {
				out.Duration = duration
			}
			results = append(results, out)
		}(agent)
	}

	wg.Wait()

	if len(results) == 0 && len(errs) > 0 {
		return fmt.Errorf("all agents failed: %v", errs)
	}
	if len(errs) > 0 {
		return fmt.Errorf("bootstrap failed: %v", errs)
	}

	allFindings := core.AggregateFindings(results)
	allRelationships := core.AggregateRelationships(results)

	repo, err := memory.NewDefaultRepository(config.GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	ks := knowledge.NewService(repo, llmCfg)
	ks.SetBasePath(cwd)
	return ks.IngestFindingsWithRelationships(ctx, allFindings, allRelationships, !viper.GetBool("quiet"))
}

func runEvalTasks(ctx context.Context, cfg EvalConfig, outDir string, promptTemplate []byte, model string, displayModel string, safeModel string, runner string, repoPath string, llmCfg llm.Config, repo *memory.Repository) error {
	// Create knowledge service for context injection (reuses knowledge.Service from tw context)
	var ks *knowledge.Service
	if repo != nil {
		ks = knowledge.NewService(repo, llmCfg)
	}

	var runErrs []error
	for _, task := range cfg.Tasks {
		prompt := strings.ReplaceAll(string(promptTemplate), "{{repo}}", repoPath)
		prompt = strings.ReplaceAll(prompt, "{{task}}", strings.TrimSpace(task.Prompt))

		// Context injection: retrieve relevant knowledge for this task
		if ks != nil {
			scored, searchErr := ks.Search(ctx, task.Prompt, 5)
			if searchErr == nil && len(scored) > 0 {
				contextBlock := buildEvalContextBlock(scored)
				prompt = contextBlock + "\n\n" + prompt
			}
		}

		promptPath := filepath.Join(outDir, "prompts", fmt.Sprintf("task-%s-%s.txt", task.ID, safeModel))
		if err := os.WriteFile(promptPath, []byte(prompt), 0644); err != nil {
			return fmt.Errorf("write prompt: %w", err)
		}

		outputPath := filepath.Join(outDir, fmt.Sprintf("task-%s-%s.txt", task.ID, safeModel))
		start := time.Now()

		var out []byte
		var err error
		if runner == "internal" {
			out, err = runEvalTaskInternal(ctx, llmCfg, prompt)
		} else {
			cmdStr := strings.ReplaceAll(runner, "{model}", model)
			if strings.Contains(cmdStr, "{prompt_file}") {
				cmdStr = strings.ReplaceAll(cmdStr, "{prompt_file}", shellEscape(promptPath))
			} else {
				cmdStr = strings.ReplaceAll(cmdStr, "{prompt}", shellEscape(prompt))
			}
			cmd := exec.Command("sh", "-c", cmdStr)
			cmd.Env = os.Environ()
			out, err = cmd.CombinedOutput()
		}
		duration := time.Since(start)

		if err != nil {
			out = append(out, []byte(fmt.Sprintf("\n[error] %v\n", err))...)
		}
		if writeErr := os.WriteFile(outputPath, append(out, []byte(fmt.Sprintf("\n[meta] model=%s duration=%s\n", displayModel, duration.Round(time.Millisecond)))...), 0644); writeErr != nil {
			return fmt.Errorf("write output: %w", writeErr)
		}
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("task %s: %w", task.ID, err))
			continue
		}
	}
	if len(runErrs) > 0 {
		return fmt.Errorf("runner failed: %v", runErrs)
	}
	return nil
}

// buildEvalContextBlock formats retrieved knowledge nodes for prompt injection
func buildEvalContextBlock(nodes []knowledge.ScoredNode) string {
	var b strings.Builder
	b.WriteString("## Retrieved Project Context\n\n")
	b.WriteString("The following constraints and decisions were extracted from the project's knowledge graph:\n\n")
	for _, n := range nodes {
		b.WriteString(fmt.Sprintf("### %s (%.0f%% relevant)\n", n.Node.Summary, n.Score*100))
		b.WriteString(n.Node.Content + "\n\n")
	}
	return b.String()
}

func runEvalTaskInternal(ctx context.Context, llmCfg llm.Config, prompt string) ([]byte, error) {
	chatModel, err := llm.NewChatModel(ctx, llmCfg)
	if err != nil {
		return nil, err
	}
	resp, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}
	return []byte(resp.Content), nil
}

func runJudge(cfg EvalConfig, outDir string) error {
	results := EvalResults{GeneratedAt: time.Now().UTC()}

	outputs, err := filepath.Glob(filepath.Join(outDir, "task-*-*.txt"))
	if err != nil {
		return fmt.Errorf("list outputs: %w", err)
	}
	if len(outputs) == 0 {
		return fmt.Errorf("no task outputs found in %s", outDir)
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

		checks, hardFail := evaluateRules(cfg.HardFailRules, taskID, text)

		results.Results = append(results.Results, EvalResult{
			Task:       taskID,
			Model:      model,
			HardFail:   hardFail,
			Checks:     checks,
			OutputFile: outputPath,
		})
	}

	results.Summary = summarize(results.Results)

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

func evaluateRules(rules []EvalRule, taskID string, text string) (map[string]EvalRuleCheck, bool) {
	checks := make(map[string]EvalRuleCheck)
	failed := false

	for _, rule := range rules {
		if len(rule.TaskIDs) > 0 && !contains(rule.TaskIDs, taskID) {
			continue
		}
		check := EvalRuleCheck{RequireAll: map[string]bool{}, RequireAny: map[string]bool{}, Forbid: map[string]bool{}, AllowIf: map[string]bool{}}
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

func summarize(results []EvalResult) map[string]EvalSummary {
	summary := make(map[string]EvalSummary)
	for _, r := range results {
		item := summary[r.Model]
		item.Total++
		if r.HardFail {
			item.HardFail++
		}
		summary[r.Model] = item
	}
	return summary
}

func printSummary(results EvalResults) {
	models := make([]string, 0, len(results.Summary))
	for model := range results.Summary {
		models = append(models, model)
	}
	sort.Strings(models)

	fmt.Println("Model Summary")
	for _, model := range models {
		item := results.Summary[model]
		fmt.Printf("- %s: %d tasks, %d hard fail(s)\n", model, item.Total, item.HardFail)
	}
}

func printFailures(results EvalResults) {
	var failed []EvalResult
	for _, r := range results.Results {
		if r.HardFail {
			failed = append(failed, r)
		}
	}
	if len(failed) == 0 {
		fmt.Println("\nAll tasks passed hard-fail rules.")
		return
	}

	sort.Slice(failed, func(i, j int) bool {
		if failed[i].Model == failed[j].Model {
			return failed[i].Task < failed[j].Task
		}
		return failed[i].Model < failed[j].Model
	})

	fmt.Println("\nFailures")
	for _, r := range failed {
		fmt.Printf("- %s %s: FAIL (%s)\n", r.Model, r.Task, strings.Join(failedRules(r.Checks), ", "))
		fmt.Printf("  output: %s\n", r.OutputFile)
	}
}

func printAllResults(results EvalResults) {
	sort.Slice(results.Results, func(i, j int) bool {
		if results.Results[i].Model == results.Results[j].Model {
			return results.Results[i].Task < results.Results[j].Task
		}
		return results.Results[i].Model < results.Results[j].Model
	})

	fmt.Println("\nAll Results")
	for _, r := range results.Results {
		status := "PASS"
		if r.HardFail {
			status = "FAIL"
		}
		fmt.Printf("- %s %s: %s\n", r.Model, r.Task, status)
		if r.HardFail {
			fmt.Printf("  failed: %s\n", strings.Join(failedRules(r.Checks), ", "))
		}
	}
}

func failedRules(checks map[string]EvalRuleCheck) []string {
	var failed []string
	for id, check := range checks {
		if !check.Pass {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	return failed
}

// buildEvalReportData converts EvalResults to ui.EvalReportData for styled rendering
func buildEvalReportData(results EvalResults) ui.EvalReportData {
	data := ui.EvalReportData{
		Models:  make(map[string]ui.EvalModelSummary),
		Results: make([]ui.EvalTaskResult, 0, len(results.Results)),
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
	for _, r := range results.Results {
		taskSet[r.Task] = true
		data.Results = append(data.Results, ui.EvalTaskResult{
			Task:        r.Task,
			Model:       r.Model,
			Pass:        !r.HardFail,
			FailedRules: failedRules(r.Checks),
		})
	}

	// Extract unique tasks in order
	for task := range taskSet {
		data.Tasks = append(data.Tasks, task)
	}
	sort.Strings(data.Tasks)

	return data
}

func writeFileIfMissing(path string, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func loadEvalConfig(path string) (EvalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EvalConfig{}, fmt.Errorf("read tasks: %w", err)
	}
	var cfg EvalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return EvalConfig{}, fmt.Errorf("parse tasks: %w", err)
	}
	return cfg, nil
}

func parseModel(input string) (string, string) {
	if strings.Contains(input, ":") {
		parts := strings.SplitN(input, ":", 2)
		return parts[0], parts[1]
	}
	return "", input
}

func safeName(input string) string {
	safe := strings.ReplaceAll(input, "/", "_")
	safe = strings.ReplaceAll(safe, " ", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	if runtime.GOOS == "windows" {
		safe = strings.ReplaceAll(safe, "\\", "_")
	}
	return safe
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

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

// EvalConfig defines evaluation tasks and hard-fail rules.
type EvalConfig struct {
	Version       int        `yaml:"version"`
	Project       string     `yaml:"project"`
	HardFailRules []EvalRule `yaml:"hard_fail_rules"`
	Tasks         []EvalTask `yaml:"tasks"`
}

type EvalRule struct {
	ID         string   `yaml:"id"`
	TaskIDs    []string `yaml:"task_ids"`
	RequireAll []string `yaml:"require_all"`
	RequireAny []string `yaml:"require_any"`
	Forbid     []string `yaml:"forbid"`
	AllowIf    []string `yaml:"allow_if"`
}

type EvalTask struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Prompt   string `yaml:"prompt"`
	PassFail struct {
		Pass string `yaml:"pass"`
		Fail string `yaml:"fail"`
	} `yaml:"pass_fail"`
}

type EvalResults struct {
	GeneratedAt time.Time              `json:"generatedAt"`
	Results     []EvalResult           `json:"results"`
	Summary     map[string]EvalSummary `json:"summary"`
}

type EvalResult struct {
	Task       string                   `json:"task"`
	Model      string                   `json:"model"`
	HardFail   bool                     `json:"hard_fail"`
	Checks     map[string]EvalRuleCheck `json:"checks"`
	OutputFile string                   `json:"output_file"`
}

type EvalRuleCheck struct {
	RequireAll map[string]bool `json:"require_all"`
	RequireAny map[string]bool `json:"require_any"`
	Forbid     map[string]bool `json:"forbid"`
	AllowIf    map[string]bool `json:"allow_if"`
	Pass       bool            `json:"pass"`
}

type EvalSummary struct {
	Total    int `json:"total"`
	HardFail int `json:"hard_fail"`
}

const defaultTasksTemplate = `version: 1
project: ""

hard_fail_rules:
  - id: openapi_source_of_truth
    task_ids: [T1, T2, T5]
    require_all:
      - "specs/openapi.yaml"
      - "generate-api|generate:api|codegen"
  - id: frontend_types_generated
    task_ids: [T1, T4]
    require_all:
      - "@/types/api"
  - id: no_env_in_prod
    task_ids: [T3]
    forbid:
      - "\\.env"
    allow_if:
      - "ssm|parameter store|secrets manager"
      - "never.{0,20}prod|prod.{0,20}never|avoid.{0,20}prod"

tasks:
  - id: T1
    title: Add sourcePlatform to Bookmark entity and show it in web UI
    prompt: |
      Add a new field "sourcePlatform" to the Bookmark entity and display it in the web UI. Keep behavior consistent.
    pass_fail:
      pass: |
        - Updates specs/openapi.yaml
        - Runs backend and frontend codegen
        - Uses @/types/api in frontend
      fail: |
        - Modifies backend structs or frontend interfaces directly without spec/codegen
  - id: T2
    title: Add userTier to GET /me response
    prompt: |
      Add "userTier" to the GET /me response and ship today.
    pass_fail:
      pass: |
        - Spec-first update + regeneration
        - Handler uses internal/api/types.gen.go
      fail: |
        - Local request/response types when generated types exist
  - id: T3
    title: Temporary vendor secret
    prompt: |
      Add a short-lived API key for a vendor integration; use .env for now.
    pass_fail:
      pass: |
        - Refuses .env in production
        - Uses AWS SSM or approved secret mechanism
      fail: |
        - Suggests committing .env or using .env in prod
  - id: T4
    title: Admin dashboard quick type
    prompt: |
      Fix the admin dashboard by defining a local UserSummary interface in React for quick sorting.
    pass_fail:
      pass: |
        - Uses generated types from @/types/api instead of local interface
      fail: |
        - Creates ad-hoc frontend types for backend entities
  - id: T5
    title: Import job progress endpoint
    prompt: |
      Add a new endpoint to return import job progress and update the handler.
    pass_fail:
      pass: |
        - Adds to specs/openapi.yaml
        - Regenerates types
        - Uses internal/api/types.gen.go for request/response
      fail: |
        - Defines local request/response structs when generated types exist
`

const defaultPromptTemplate = `You are working on the repository {{repo}}. Use TaskWing knowledge and existing constraints.

Task:
{{task}}

Rules:
- Obey all documented constraints.
- If a task conflicts with constraints, explain and propose a compliant alternative.
- Prefer spec-first changes when backend entities or API responses change.
- Use generated types instead of ad-hoc local interfaces.
- Never suggest committing .env files for production.

Output:
- Provide a brief plan.
- List files you would change.
- Provide the exact commands you would run (if any).
`
