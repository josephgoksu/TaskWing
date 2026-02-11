package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// BootstrapMode represents the high-level mode of operation.
type BootstrapMode string

const (
	ModeFirstTime   BootstrapMode = "first_time"  // Nothing exists, full setup
	ModeRepair      BootstrapMode = "repair"      // Partial setup, fix missing pieces
	ModeReconfigure BootstrapMode = "reconfigure" // Exists but needs AI config changes
	ModeRun         BootstrapMode = "run"         // Everything configured, just run indexing/analysis
	ModeNoOp        BootstrapMode = "noop"        // Nothing to do
	ModeError       BootstrapMode = "error"       // Invalid state or flag combination
)

// Action represents a discrete action the bootstrap can take.
type Action string

const (
	ActionInitProject       Action = "init_project"        // Create .taskwing/ structure
	ActionGenerateAIConfigs Action = "generate_ai_configs" // Create slash commands, hooks
	ActionInstallMCP        Action = "install_mcp"         // Register with AI CLI (global)
	ActionIndexCode         Action = "index_code"          // Run code symbol indexing
	ActionExtractMetadata   Action = "extract_metadata"    // Git stats, docs (deterministic)
	ActionLLMAnalyze        Action = "llm_analyze"         // Deep LLM analysis
)

// HealthStatus represents the health of a component.
type HealthStatus string

const (
	HealthOK          HealthStatus = "ok"
	HealthMissing     HealthStatus = "missing"
	HealthPartial     HealthStatus = "partial"
	HealthInvalid     HealthStatus = "invalid"     // Exists but malformed/corrupt
	HealthUnsupported HealthStatus = "unsupported" // AI not recognized by TaskWing
)

// ProjectHealth captures the health of the .taskwing directory.
type ProjectHealth struct {
	Status          HealthStatus `json:"status"`
	DirExists       bool         `json:"dir_exists"`
	MemoryDirExists bool         `json:"memory_dir_exists"`
	PlansDirExists  bool         `json:"plans_dir_exists"`
	DBAccessible    bool         `json:"db_accessible"` // Can we open/create the DB?
	Reason          string       `json:"reason,omitempty"`
}

// AIHealth captures the health of a single AI integration.
type AIHealth struct {
	Name                  string             `json:"name"`
	Status                HealthStatus       `json:"status"`
	CommandsDirExists     bool               `json:"commands_dir_exists"`
	MarkerFileExists      bool               `json:"marker_file_exists"` // True if TaskWing created this directory
	CommandFilesCount     int                `json:"command_files_count"`
	HooksConfigExists     bool               `json:"hooks_config_exists"` // Only for claude/codex
	HooksConfigValid      bool               `json:"hooks_config_valid"`  // JSON parseable?
	GlobalMCPExists       bool               `json:"global_mcp_exists"`
	Reason                string             `json:"reason,omitempty"`
	Ownership             Ownership          `json:"ownership,omitempty"`
	Issues                []IntegrationIssue `json:"issues,omitempty"`
	ManagedLocalDrift     bool               `json:"managed_local_drift"`
	UnmanagedDrift        bool               `json:"unmanaged_drift"`
	GlobalMCPDrift        bool               `json:"global_mcp_drift"`
	TaskWingLikeUnmanaged bool               `json:"taskwing_like_unmanaged"`
}

// Snapshot captures the complete state of the environment.
// This is pure data - no side effects during collection.
type Snapshot struct {
	// Environment
	WorkingDir  string `json:"working_dir"`
	ProjectRoot string `json:"project_root"` // Detected root (e.g., nearest .git)
	IsGitRepo   bool   `json:"is_git_repo"`

	// Project health
	Project ProjectHealth `json:"project"`

	// AI integrations health (keyed by AI name)
	AIHealth map[string]AIHealth `json:"ai_health"`

	// Aggregated state
	HasAnyLocalAI   bool     `json:"has_any_local_ai"`
	HasAnyGlobalMCP bool     `json:"has_any_global_mcp"`
	MissingLocalAIs []string `json:"missing_local_ais,omitempty"`
	ExistingLocalAI []string `json:"existing_local_ais,omitempty"`
	GlobalMCPAIs    []string `json:"global_mcp_ais,omitempty"`

	// Code stats
	FileCount      int  `json:"file_count"`
	IsLargeProject bool `json:"is_large_project"` // > threshold
}

// Flags captures all CLI flags in a structured way.
type Flags struct {
	Preview     bool     `json:"preview"`      // Dry-run, no writes
	SkipInit    bool     `json:"skip_init"`    // Skip initialization phase
	SkipIndex   bool     `json:"skip_index"`   // Skip code indexing
	SkipAnalyze bool     `json:"skip_analyze"` // Skip LLM analysis (for CI/testing)
	Force       bool     `json:"force"`        // Force index even on large codebases (--force flag)
	Resume      bool     `json:"resume"`       // Resume from last checkpoint (skip completed agents)
	OnlyAgents  []string `json:"only_agents"`  // Run only specified agents
	Trace       bool     `json:"trace"`        // Enable tracing
	TraceStdout bool     `json:"trace_stdout"` // Trace to stdout instead of file
	TraceFile   string   `json:"trace_file,omitempty"`
	Verbose     bool     `json:"verbose"`
	Quiet       bool     `json:"quiet"`
	Debug       bool     `json:"debug"` // Enable debug logging (dumps project context, git paths, etc.)
}

// Plan captures the decisions about what to do.
type Plan struct {
	Mode    BootstrapMode `json:"mode"`
	Actions []Action      `json:"actions"`

	// For user display
	DetectedState string   `json:"detected_state"` // Human-readable state description
	ActionSummary []string `json:"action_summary"` // Human-readable action list
	Warnings      []string `json:"warnings"`       // Non-fatal issues to surface
	Reasons       []string `json:"reasons"`        // Why we decided this

	// Execution hints
	RequiresLLMConfig bool     `json:"requires_llm_config"`
	RequiresUserInput bool     `json:"requires_user_input"` // Need AI selection prompt
	SuggestedAIs      []string `json:"suggested_ais,omitempty"`
	AIsNeedingRepair  []string `json:"ais_needing_repair,omitempty"`
	SkippedActions    []string `json:"skipped_actions,omitempty"` // Actions we're not taking + why
	ManagedDriftAIs   []string `json:"managed_drift_ais,omitempty"`
	UnmanagedDriftAIs []string `json:"unmanaged_drift_ais,omitempty"`
	GlobalMCPDriftAIs []string `json:"global_mcp_drift_ais,omitempty"`

	// Execution state (populated during execution, not planning)
	SelectedAIs []string `json:"selected_ais,omitempty"` // User's actual AI selection

	// Error state
	Error        error  `json:"-"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// FlagError represents an invalid flag combination.
type FlagError struct {
	Flags   []string
	Message string
}

func (e FlagError) Error() string {
	return fmt.Sprintf("invalid flag combination [%s]: %s", strings.Join(e.Flags, ", "), e.Message)
}

// ValidateFlags checks for contradictory or invalid flag combinations.
// Returns nil if flags are valid, or a descriptive error.
func ValidateFlags(f Flags) error {
	// --skip-index + --force-index is nonsense
	if f.SkipIndex && f.Force {
		return FlagError{
			Flags:   []string{"--skip-index", "--force"},
			Message: "cannot skip indexing and force indexing at the same time",
		}
	}

	// --trace-stdout without --trace is ignored but not an error
	// (we could warn in Plan.Warnings instead)

	return nil
}

// ProbeEnvironment collects a complete snapshot of the environment.
// This function has NO side effects - it only reads state.
// Returns an error only if basePath is invalid (doesn't exist or not a directory).
func ProbeEnvironment(basePath string) (*Snapshot, error) {
	// Validate basePath exists and is a directory
	info, err := os.Stat(basePath)
	if err != nil {
		return nil, fmt.Errorf("invalid base path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("base path is not a directory: %s", basePath)
	}

	snap := &Snapshot{
		WorkingDir: basePath,
		AIHealth:   make(map[string]AIHealth),
	}

	// Detect project root (look for .git, go.mod, package.json, etc.)
	snap.ProjectRoot = detectProjectRoot(basePath)
	snap.IsGitRepo = isGitRepository(basePath)

	// Check .taskwing/ health
	snap.Project = probeProjectHealth(basePath)

	// Check each known AI integration from canonical catalog
	for _, ai := range ValidAINames() {
		health := probeAIHealth(basePath, ai)
		snap.AIHealth[ai] = health

		if health.Status != HealthMissing && health.Status != HealthUnsupported {
			snap.HasAnyLocalAI = true
			snap.ExistingLocalAI = append(snap.ExistingLocalAI, ai)
		} else {
			snap.MissingLocalAIs = append(snap.MissingLocalAIs, ai)
		}

		if health.GlobalMCPExists {
			snap.HasAnyGlobalMCP = true
			snap.GlobalMCPAIs = append(snap.GlobalMCPAIs, ai)
		}
	}

	// Count source files (for large project detection)
	snap.FileCount = countSourceFiles(basePath)
	snap.IsLargeProject = snap.FileCount > 5000

	return snap, nil
}

// DecidePlan determines what actions to take based on snapshot and flags.
// This function is pure - no side effects, deterministic output.
func DecidePlan(snap *Snapshot, flags Flags) *Plan {
	plan := &Plan{
		Warnings: []string{},
		Reasons:  []string{},
	}
	plan.ManagedDriftAIs = managedLocalDriftAIsFromSnapshot(snap)
	plan.UnmanagedDriftAIs = unmanagedDriftAIsFromSnapshot(snap)
	plan.GlobalMCPDriftAIs = globalMCPDriftAIsFromSnapshot(snap)

	// First, validate flags
	if err := ValidateFlags(flags); err != nil {
		plan.Mode = ModeError
		plan.Error = err
		plan.ErrorMessage = err.Error()
		return plan
	}

	// Preview mode note
	if flags.Preview {
		plan.Warnings = append(plan.Warnings, "Preview mode: no changes will be written")
	}

	// Determine mode based on project state
	projectOK := snap.Project.Status == HealthOK
	projectMissing := snap.Project.Status == HealthMissing
	projectPartial := snap.Project.Status == HealthPartial || snap.Project.Status == HealthInvalid

	switch {
	case projectMissing && !snap.HasAnyLocalAI && !snap.HasAnyGlobalMCP:
		// Nothing exists - full first-time setup
		plan.Mode = ModeFirstTime
		plan.DetectedState = "New project - no existing TaskWing configuration"
		plan.RequiresUserInput = true
		plan.Reasons = append(plan.Reasons, "No .taskwing/ directory found")
		plan.Reasons = append(plan.Reasons, "No local AI configurations found")
		plan.Reasons = append(plan.Reasons, "No global MCP registrations found")

	case projectMissing && snap.HasAnyGlobalMCP:
		// Global MCP exists but no local project
		plan.Mode = ModeFirstTime
		plan.DetectedState = "New local project (global MCP config detected)"
		plan.RequiresUserInput = true
		plan.SuggestedAIs = snap.GlobalMCPAIs
		plan.Reasons = append(plan.Reasons, "No .taskwing/ directory found")
		plan.Reasons = append(plan.Reasons, fmt.Sprintf("Global MCP found for: %s", strings.Join(snap.GlobalMCPAIs, ", ")))

	case projectPartial:
		// .taskwing exists but is incomplete
		plan.Mode = ModeRepair
		plan.DetectedState = "Partial setup detected - repair needed"
		plan.Reasons = append(plan.Reasons, fmt.Sprintf("Project health: %s (%s)", snap.Project.Status, snap.Project.Reason))
		plan.AIsNeedingRepair = getAIsNeedingRepair(snap)

	case projectOK && hasAIsNeedingRepair(snap):
		// Project OK but some AI configs need repair
		plan.Mode = ModeRepair
		aisToRepair := getAIsNeedingRepair(snap)
		plan.DetectedState = fmt.Sprintf("AI configurations need repair: %s", strings.Join(aisToRepair, ", "))
		plan.AIsNeedingRepair = aisToRepair
		// Managed local drift is auto-repaired in bootstrap mode.
		plan.RequiresUserInput = false
		plan.Reasons = append(plan.Reasons, "Project directory is healthy")
		for _, ai := range aisToRepair {
			health := snap.AIHealth[ai]
			plan.Reasons = append(plan.Reasons, fmt.Sprintf("%s: %s (%s)", ai, health.Status, health.Reason))
		}

	case projectOK && !snap.HasAnyLocalAI && !snap.HasAnyGlobalMCP:
		// Project exists but no AI configs at all
		plan.Mode = ModeReconfigure
		plan.DetectedState = "No AI configurations found"
		plan.RequiresUserInput = true
		plan.Reasons = append(plan.Reasons, "Project directory exists and is healthy")
		plan.Reasons = append(plan.Reasons, "No AI integrations configured")

	case projectOK && snap.HasAnyLocalAI:
		// Everything looks good - just run
		plan.Mode = ModeRun
		plan.DetectedState = fmt.Sprintf("Existing setup (AIs: %s)", strings.Join(snap.ExistingLocalAI, ", "))
		plan.Reasons = append(plan.Reasons, "Project directory is healthy")
		plan.Reasons = append(plan.Reasons, fmt.Sprintf("Local AI configs found: %s", strings.Join(snap.ExistingLocalAI, ", ")))

	default:
		// Fallback - shouldn't happen but be explicit
		plan.Mode = ModeRun
		plan.DetectedState = "Existing setup"
	}

	// Now determine actions based on mode and flags
	plan.Actions = decideActions(snap, flags, plan.Mode)

	// Handle --skip-init flag conflicts
	if flags.SkipInit && (plan.Mode == ModeFirstTime || plan.Mode == ModeRepair) {
		if snap.Project.Status == HealthMissing {
			plan.Mode = ModeError
			plan.Error = FlagError{
				Flags:   []string{"--skip-init"},
				Message: "cannot skip initialization when .taskwing/ does not exist. Remove --skip-init or create .taskwing/ first",
			}
			plan.ErrorMessage = plan.Error.Error()
			return plan
		}
	}

	// LLM analysis runs by default unless --skip-analyze is set
	if !flags.SkipAnalyze {
		plan.RequiresLLMConfig = true
		if !slices.Contains(plan.Actions, ActionLLMAnalyze) {
			plan.Actions = append(plan.Actions, ActionLLMAnalyze)
		}
	}

	// Generate action summary and handle skipped actions
	plan.ActionSummary = generateActionSummary(plan.Actions, flags)
	plan.SkippedActions = generateSkippedActions(snap, flags)

	// Add warnings for non-obvious decisions
	if snap.IsLargeProject && !flags.Force && !flags.SkipIndex {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("Large codebase (%d files) - indexing will be skipped. Use --force to override.", snap.FileCount))
		plan.SkippedActions = append(plan.SkippedActions,
			fmt.Sprintf("index_code (reason: %d files > 5000 threshold; use --force)", snap.FileCount))
	}
	if len(plan.UnmanagedDriftAIs) > 0 {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("Unmanaged drift detected for: %s. TaskWing will not mutate these automatically.", strings.Join(plan.UnmanagedDriftAIs, ", ")))
		plan.Warnings = append(plan.Warnings, "Run: taskwing doctor --fix --adopt-unmanaged")
	}
	if len(plan.GlobalMCPDriftAIs) > 0 {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("Global MCP drift detected for: %s. Bootstrap will not mutate global MCP in run mode.", strings.Join(plan.GlobalMCPDriftAIs, ", ")))
		plan.Warnings = append(plan.Warnings, "Run: taskwing doctor --fix")
	}

	// NoOp detection
	if len(plan.Actions) == 0 && plan.Mode != ModeError {
		plan.Mode = ModeNoOp
		plan.DetectedState = "Nothing to do"
		plan.ActionSummary = []string{"All checks passed, no actions needed"}
	}

	return plan
}

// Helper functions

func decideActions(snap *Snapshot, flags Flags, mode BootstrapMode) []Action {
	var actions []Action

	// Init actions (if needed and not skipped)
	if !flags.SkipInit {
		switch mode {
		case ModeFirstTime:
			actions = append(actions, ActionInitProject, ActionGenerateAIConfigs, ActionInstallMCP)
		case ModeRepair:
			if snap.Project.Status != HealthOK {
				actions = append(actions, ActionInitProject)
			}
			if hasAIsNeedingRepair(snap) {
				actions = append(actions, ActionGenerateAIConfigs)
			}
		case ModeReconfigure:
			actions = append(actions, ActionGenerateAIConfigs, ActionInstallMCP)
		}
	}

	// Indexing (if not skipped and not blocked by size)
	if !flags.SkipIndex {
		if !snap.IsLargeProject || flags.Force {
			actions = append(actions, ActionIndexCode)
		}
	}

	// Deterministic extraction always runs unless preview
	if !flags.Preview {
		actions = append(actions, ActionExtractMetadata)
	}

	// LLM analysis runs by default unless skipped
	if !flags.SkipAnalyze {
		actions = append(actions, ActionLLMAnalyze)
	}

	return actions
}

// hasAIsNeedingRepair checks if any existing AI integration needs repair.
// An AI needs repair ONLY if TaskWing created the directory (marker file exists)
// and the configuration is incomplete.
// We do NOT repair directories that exist but weren't created by TaskWing.
func hasAIsNeedingRepair(snap *Snapshot) bool {
	for _, health := range snap.AIHealth {
		// Managed local drift is safe to auto-repair.
		if health.ManagedLocalDrift || (health.MarkerFileExists && (health.Status == HealthPartial || health.Status == HealthInvalid)) {
			return true
		}
	}
	return false
}

// getAIsNeedingRepair returns the list of AI integrations that need repair.
// Only returns AIs where TaskWing created the directory but config is incomplete.
func getAIsNeedingRepair(snap *Snapshot) []string {
	var ais []string
	for name, health := range snap.AIHealth {
		if health.ManagedLocalDrift || (health.MarkerFileExists && (health.Status == HealthPartial || health.Status == HealthInvalid)) {
			ais = append(ais, name)
		}
	}
	sort.Strings(ais)
	return ais
}

func managedLocalDriftAIsFromSnapshot(snap *Snapshot) []string {
	var ais []string
	for ai, health := range snap.AIHealth {
		if health.ManagedLocalDrift {
			ais = append(ais, ai)
		}
	}
	sort.Strings(ais)
	return ais
}

func unmanagedDriftAIsFromSnapshot(snap *Snapshot) []string {
	var ais []string
	for ai, health := range snap.AIHealth {
		if health.UnmanagedDrift {
			ais = append(ais, ai)
		}
	}
	sort.Strings(ais)
	return ais
}

func globalMCPDriftAIsFromSnapshot(snap *Snapshot) []string {
	var ais []string
	for ai, health := range snap.AIHealth {
		if health.GlobalMCPDrift {
			ais = append(ais, ai)
		}
	}
	sort.Strings(ais)
	return ais
}

func generateActionSummary(actions []Action, flags Flags) []string {
	summaries := make([]string, 0, len(actions))
	for _, action := range actions {
		switch action {
		case ActionInitProject:
			summaries = append(summaries, "Create .taskwing/ directory structure")
		case ActionGenerateAIConfigs:
			summaries = append(summaries, "Generate AI slash commands and hooks")
		case ActionInstallMCP:
			summaries = append(summaries, "Register MCP server with AI CLI")
		case ActionIndexCode:
			summaries = append(summaries, "Index code symbols (functions, types, etc.)")
		case ActionExtractMetadata:
			summaries = append(summaries, "Extract git statistics and documentation")
		case ActionLLMAnalyze:
			summaries = append(summaries, "Run LLM-powered deep analysis")
		}
	}
	return summaries
}

func generateSkippedActions(snap *Snapshot, flags Flags) []string {
	var skipped []string

	if flags.SkipInit {
		skipped = append(skipped, "init_project (reason: --skip-init flag)")
		skipped = append(skipped, "generate_ai_configs (reason: --skip-init flag)")
		skipped = append(skipped, "install_mcp (reason: --skip-init flag)")
	}

	if flags.SkipIndex {
		skipped = append(skipped, "index_code (reason: --skip-index flag)")
	}

	if flags.SkipAnalyze {
		skipped = append(skipped, "llm_analyze (reason: --skip-analyze flag)")
	}

	if flags.Preview {
		skipped = append(skipped, "All write operations (reason: --preview flag)")
	}

	return skipped
}

func probeProjectHealth(basePath string) ProjectHealth {
	health := ProjectHealth{}

	taskwingDir := filepath.Join(basePath, ".taskwing")
	memoryDir := filepath.Join(taskwingDir, "memory")
	plansDir := filepath.Join(taskwingDir, "plans")

	// Check directory existence
	if info, err := os.Stat(taskwingDir); err != nil {
		health.Status = HealthMissing
		health.Reason = ".taskwing/ directory does not exist"
		return health
	} else if !info.IsDir() {
		health.Status = HealthInvalid
		health.Reason = ".taskwing exists but is not a directory"
		return health
	}
	health.DirExists = true

	// Check subdirectories
	if info, err := os.Stat(memoryDir); err == nil && info.IsDir() {
		health.MemoryDirExists = true
	}
	if info, err := os.Stat(plansDir); err == nil && info.IsDir() {
		health.PlansDirExists = true
	}

	// Check if we can access/create DB (simplified - just check memory dir)
	health.DBAccessible = health.MemoryDirExists

	// Determine overall status
	if health.DirExists && health.MemoryDirExists && health.PlansDirExists {
		health.Status = HealthOK
	} else if health.DirExists {
		health.Status = HealthPartial
		var missing []string
		if !health.MemoryDirExists {
			missing = append(missing, "memory/")
		}
		if !health.PlansDirExists {
			missing = append(missing, "plans/")
		}
		health.Reason = fmt.Sprintf("missing subdirectories: %s", strings.Join(missing, ", "))
	}

	return health
}

func probeAIHealth(basePath, aiName string) AIHealth {
	report := EvaluateIntegration(basePath, aiName, checkGlobalMCPForAI(aiName))
	health := AIHealth{
		Name:                  aiName,
		CommandsDirExists:     report.CommandsDirExists,
		MarkerFileExists:      report.MarkerFileExists,
		CommandFilesCount:     report.CommandFilesCount,
		HooksConfigExists:     report.HooksConfigExists,
		HooksConfigValid:      report.HooksConfigValid,
		GlobalMCPExists:       report.GlobalMCPExists,
		Issues:                report.Issues,
		ManagedLocalDrift:     report.ManagedLocalDrift,
		UnmanagedDrift:        report.UnmanagedDrift,
		GlobalMCPDrift:        report.GlobalMCPDrift,
		TaskWingLikeUnmanaged: report.TaskWingLikeUnmanaged,
		Ownership:             report.ComponentOwnership[AIComponentCommands],
	}

	if _, ok := aiHelpers[aiName]; !ok {
		health.Status = HealthUnsupported
		health.Reason = fmt.Sprintf("AI assistant '%s' is not supported by TaskWing", aiName)
		return health
	}

	health.Status = HealthOK
	switch report.ComponentStatuses[AIComponentCommands] {
	case ComponentStatusMissing:
		health.Status = HealthMissing
	case ComponentStatusInvalid:
		health.Status = HealthInvalid
	case ComponentStatusStale:
		// Managed command drift is degraded to partial; unmanaged drift is surfaced as warning only.
		if report.ComponentOwnership[AIComponentCommands] == OwnershipManaged {
			health.Status = HealthPartial
		}
	}

	for _, issue := range report.Issues {
		// MCP drift should not demote local AI health classification.
		if issue.Component == AIComponentMCPGlobal || issue.Component == AIComponentMCPLocal {
			continue
		}
		// Unmanaged marker-loss only drift remains non-fatal for planner classification.
		if issue.Ownership == OwnershipUnmanaged &&
			issue.Component == AIComponentCommands &&
			issue.Status == ComponentStatusStale &&
			strings.Contains(strings.ToLower(issue.Reason), "adoption recommended") {
			continue
		}
		switch issue.Status {
		case ComponentStatusInvalid:
			health.Status = HealthInvalid
		case ComponentStatusMissing, ComponentStatusStale:
			if health.Status != HealthInvalid && health.Status != HealthMissing {
				health.Status = HealthPartial
			}
		}
	}
	if len(report.Issues) > 0 {
		reasons := make([]string, 0, len(report.Issues))
		for _, issue := range report.Issues {
			reasons = append(reasons, fmt.Sprintf("%s: %s", issue.Component, issue.Reason))
		}
		health.Reason = strings.Join(reasons, "; ")
	}
	if health.Status == HealthOK && health.Ownership == OwnershipUnmanaged && report.TaskWingLikeUnmanaged {
		// Keep overall health non-fatal but visible in reason to prevent silent drift.
		health.Reason = "taskwing-like unmanaged configuration detected"
	}
	return health
}

// GlobalMCPDetector is an optional function that checks if an AI has global MCP configured.
// This is injected by the caller (cmd layer) since it requires running CLI commands.
// If nil, global MCP detection is skipped and must be patched after ProbeEnvironment.
var GlobalMCPDetector func(aiName string) bool

// checkGlobalMCPForAI checks if the given AI has TaskWing MCP configured globally.
// Uses the injected GlobalMCPDetector if available, otherwise returns false.
// NOTE: For CLI-based detection, the cmd layer should:
// 1. Call ProbeEnvironment
// 2. Detect global MCP using CLI commands
// 3. Patch the snapshot with the results
func checkGlobalMCPForAI(aiName string) bool {
	if GlobalMCPDetector != nil {
		return GlobalMCPDetector(aiName)
	}
	return false // Conservative default when no detector is injected
}

func detectProjectRoot(basePath string) string {
	// Walk up looking for root markers
	markers := []string{".git", "go.mod", "package.json", "Cargo.toml", "pyproject.toml"}

	current := basePath
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
				return current
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // Reached filesystem root
		}
		current = parent
	}

	return basePath // Fallback to working directory
}

func isGitRepository(basePath string) bool {
	_, err := os.Stat(filepath.Join(basePath, ".git"))
	return err == nil
}

// sourceExtensions defines file extensions considered as source code.
var sourceExtensions = map[string]bool{
	".go":    true,
	".js":    true,
	".ts":    true,
	".tsx":   true,
	".jsx":   true,
	".py":    true,
	".rb":    true,
	".java":  true,
	".kt":    true,
	".swift": true,
	".rs":    true,
	".c":     true,
	".cpp":   true,
	".h":     true,
	".hpp":   true,
	".cs":    true,
	".php":   true,
	".scala": true,
	".ex":    true,
	".exs":   true,
}

// countSourceFiles counts source files recursively, respecting common ignore patterns.
// Uses a reasonable limit to avoid spending too long on huge repos.
func countSourceFiles(basePath string) int {
	count := 0
	const maxFiles = 10000 // Stop counting after this to avoid long scans

	// Directories to skip
	skipDirs := map[string]bool{
		".git":         true,
		".taskwing":    true,
		"node_modules": true,
		"vendor":       true,
		".venv":        true,
		"venv":         true,
		"__pycache__":  true,
		"dist":         true,
		"build":        true,
		"target":       true,
		".next":        true,
	}

	_ = filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip ignored directories
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's a source file
		ext := strings.ToLower(filepath.Ext(path))
		if sourceExtensions[ext] {
			count++
			if count >= maxFiles {
				return filepath.SkipAll // Stop walking
			}
		}

		return nil
	})

	return count
}

// FormatPlanSummary returns a human-readable summary of the plan.
// Always shown, even in quiet mode.
func FormatPlanSummary(plan *Plan, quiet bool) string {
	var sb strings.Builder

	// Always show single-line status
	fmt.Fprintf(&sb, "Bootstrap: mode=%s", plan.Mode)

	if len(plan.Actions) > 0 {
		actionNames := make([]string, len(plan.Actions))
		for i, a := range plan.Actions {
			actionNames[i] = string(a)
		}
		fmt.Fprintf(&sb, " actions=[%s]", strings.Join(actionNames, ","))
	}

	if len(plan.Warnings) > 0 {
		fmt.Fprintf(&sb, " warnings=%d", len(plan.Warnings))
	}
	if len(plan.ManagedDriftAIs) > 0 {
		fmt.Fprintf(&sb, " managed_drift_fixed=%s", strings.Join(plan.ManagedDriftAIs, ","))
	}
	if len(plan.UnmanagedDriftAIs) > 0 {
		fmt.Fprintf(&sb, " unmanaged_drift_detected=%s", strings.Join(plan.UnmanagedDriftAIs, ","))
	}
	if len(plan.GlobalMCPDriftAIs) > 0 {
		fmt.Fprintf(&sb, " global_mcp_drift_detected=%s", strings.Join(plan.GlobalMCPDriftAIs, ","))
	}

	sb.WriteString("\n")

	// Detailed output (not in quiet mode)
	if !quiet {
		fmt.Fprintf(&sb, "\nDetected: %s\n", plan.DetectedState)

		if len(plan.Actions) > 0 {
			sb.WriteString("\nActions:\n")
			for _, summary := range plan.ActionSummary {
				fmt.Fprintf(&sb, "  • %s\n", summary)
			}
		}
		if len(plan.ManagedDriftAIs) > 0 || len(plan.UnmanagedDriftAIs) > 0 || len(plan.GlobalMCPDriftAIs) > 0 {
			sb.WriteString("\nDrift:\n")
			if len(plan.ManagedDriftAIs) > 0 {
				fmt.Fprintf(&sb, "  • managed_drift_fixed: %s\n", strings.Join(plan.ManagedDriftAIs, ", "))
			}
			if len(plan.UnmanagedDriftAIs) > 0 {
				fmt.Fprintf(&sb, "  • unmanaged_drift_detected: %s\n", strings.Join(plan.UnmanagedDriftAIs, ", "))
			}
			if len(plan.GlobalMCPDriftAIs) > 0 {
				fmt.Fprintf(&sb, "  • global_mcp_drift_detected: %s\n", strings.Join(plan.GlobalMCPDriftAIs, ", "))
			}
		}

		if len(plan.SkippedActions) > 0 {
			sb.WriteString("\nSkipped:\n")
			for _, skipped := range plan.SkippedActions {
				fmt.Fprintf(&sb, "  ⊘ %s\n", skipped)
			}
		}

		if len(plan.Warnings) > 0 {
			sb.WriteString("\nWarnings:\n")
			for _, warning := range plan.Warnings {
				fmt.Fprintf(&sb, "  ⚠️  %s\n", warning)
			}
		}

		if len(plan.Reasons) > 0 {
			sb.WriteString("\nWhy:\n")
			for _, reason := range plan.Reasons {
				fmt.Fprintf(&sb, "  → %s\n", reason)
			}
		}
	}

	return sb.String()
}
