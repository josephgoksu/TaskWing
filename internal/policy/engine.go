package policy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/spf13/afero"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
)

// DefaultPolicyPackage is the default Rego package path for TaskWing policies.
const DefaultPolicyPackage = "taskwing.policy"

// Engine wraps OPA for policy evaluation.
// It loads policies from .rego files and evaluates them against input data.
// All evaluation happens locally without external network calls.
type Engine struct {
	// policies contains loaded policy files
	policies []*PolicyFile

	// builtinCtx provides the context for custom built-in functions
	builtinCtx *BuiltinContext

	// policyPackage is the Rego package to query (default: "taskwing.policy")
	policyPackage string

	// compiled indicates whether the engine has been prepared
	compiled bool
}

// EngineConfig holds configuration for creating an Engine.
type EngineConfig struct {
	// WorkDir is the working directory for resolving file paths in policies.
	WorkDir string

	// PoliciesDir is the directory containing .rego policy files.
	// If empty, defaults to {WorkDir}/.taskwing/policies
	PoliciesDir string

	// PolicyPackage is the Rego package to query.
	// If empty, defaults to "taskwing.policy"
	PolicyPackage string

	// Fs is the filesystem to use for loading policies.
	// If nil, uses the OS filesystem.
	Fs afero.Fs

	// CodeIntel is the optional code intelligence repository.
	// If provided, enables symbol-aware built-in functions.
	CodeIntel codeintel.Repository
}

// NewEngine creates a new policy engine with the given configuration.
// It loads policies from the configured directory and prepares built-in functions.
func NewEngine(cfg EngineConfig) (*Engine, error) {
	// Set defaults
	if cfg.Fs == nil {
		cfg.Fs = afero.NewOsFs()
	}
	if cfg.PoliciesDir == "" && cfg.WorkDir != "" {
		cfg.PoliciesDir = GetPoliciesPath(cfg.WorkDir)
	}
	if cfg.PolicyPackage == "" {
		cfg.PolicyPackage = DefaultPolicyPackage
	}

	// Create builtin context
	builtinCtx := &BuiltinContext{
		WorkDir:   cfg.WorkDir,
		Fs:        cfg.Fs,
		CodeIntel: cfg.CodeIntel,
	}

	// Register custom built-ins (this registers globally with OPA)
	RegisterBuiltins(builtinCtx)

	// Load policies
	loader := NewLoader(cfg.Fs, cfg.PoliciesDir)
	policies, err := loader.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("load policies: %w", err)
	}

	return &Engine{
		policies:      policies,
		builtinCtx:    builtinCtx,
		policyPackage: cfg.PolicyPackage,
		compiled:      true,
	}, nil
}

// NewEngineWithPolicies creates an engine with explicitly provided policies.
// This is useful for testing or when policies come from sources other than files.
func NewEngineWithPolicies(workDir string, policies []*PolicyFile) *Engine {
	builtinCtx := &BuiltinContext{
		WorkDir: workDir,
		Fs:      afero.NewOsFs(),
	}
	RegisterBuiltins(builtinCtx)

	return &Engine{
		policies:      policies,
		builtinCtx:    builtinCtx,
		policyPackage: DefaultPolicyPackage,
		compiled:      true,
	}
}

// PolicyCount returns the number of loaded policies.
func (e *Engine) PolicyCount() int {
	return len(e.policies)
}

// PolicyNames returns the names of all loaded policies.
func (e *Engine) PolicyNames() []string {
	names := make([]string, len(e.policies))
	for i, p := range e.policies {
		names[i] = p.Name
	}
	return names
}

// Evaluate runs all loaded policies against the provided input.
// Returns a PolicyDecision containing the result and any violations.
//
// The input should be a map or struct that will be available as `input` in Rego.
// Typical structure:
//
//	{
//	  "task": { "id": "...", "files_modified": [...] },
//	  "plan": { "id": "...", "goal": "..." },
//	  "context": { "protected_zones": [...] }
//	}
//
// The function queries the "deny" and "warn" rules in the policy package.
// Any strings returned by "deny" rules become violations that block the action.
// Any strings returned by "warn" rules become warnings that are logged but don't block.
func (e *Engine) Evaluate(ctx context.Context, input any) (*PolicyDecision, error) {
	if len(e.policies) == 0 {
		// No policies loaded - allow by default
		return &PolicyDecision{
			DecisionID:  uuid.New().String(),
			PolicyPath:  e.policyPackage,
			Result:      PolicyResultAllow,
			Violations:  nil,
			Input:       input,
			EvaluatedAt: time.Now().UTC(),
		}, nil
	}

	// Build the query modules from loaded policies
	modules := make([]func(*rego.Rego), len(e.policies))
	for i, p := range e.policies {
		content := p.Content
		path := p.Path
		modules[i] = rego.Module(path, content)
	}

	// Collect violations from "deny" rules
	violations, err := e.querySet(ctx, input, "deny", modules)
	if err != nil {
		return nil, fmt.Errorf("query deny rules: %w", err)
	}

	// Collect warnings from "warn" rules
	warnings, err := e.querySet(ctx, input, "warn", modules)
	if err != nil {
		// Warnings are optional - don't fail if not defined
		warnings = nil
	}

	// Build decision
	decision := &PolicyDecision{
		DecisionID:  uuid.New().String(),
		PolicyPath:  e.policyPackage,
		Input:       input,
		EvaluatedAt: time.Now().UTC(),
	}

	if len(violations) > 0 {
		decision.Result = PolicyResultDeny
		decision.Violations = violations
	} else {
		decision.Result = PolicyResultAllow
	}

	// Store warnings in the input for audit purposes (they don't affect result)
	if len(warnings) > 0 {
		// We could add a Warnings field to PolicyDecision, but for now just log them
		// through the audit store
		_ = warnings // Acknowledge but don't use directly in decision
	}

	return decision, nil
}

// querySet queries a set-generating rule (like deny or warn) and returns all string values.
func (e *Engine) querySet(ctx context.Context, input any, ruleName string, modules []func(*rego.Rego)) ([]string, error) {
	query := fmt.Sprintf("data.%s.%s", e.policyPackage, ruleName)

	// Build rego options
	opts := []func(*rego.Rego){
		rego.Query(query),
		rego.Input(input),
	}
	opts = append(opts, modules...)

	// Create and evaluate query
	r := rego.New(opts...)
	rs, err := r.Eval(ctx)
	if err != nil {
		// Check if this is just "rule not defined" error
		if strings.Contains(err.Error(), "undefined") {
			return nil, nil // Rule not defined is OK
		}
		return nil, err
	}

	// Extract string values from result set
	var results []string
	for _, result := range rs {
		for _, expr := range result.Expressions {
			if set, ok := expr.Value.([]any); ok {
				for _, item := range set {
					if s, ok := item.(string); ok {
						results = append(results, s)
					}
				}
			}
		}
	}

	return results, nil
}

// EvaluateTask is a convenience method for evaluating a task against policies.
// It constructs the standard input format from task data.
func (e *Engine) EvaluateTask(ctx context.Context, task *TaskInput, plan *PlanInput, contextData *ContextInput) (*PolicyDecision, error) {
	input := &PolicyInput{
		Task:    task,
		Plan:    plan,
		Context: contextData,
	}
	return e.Evaluate(ctx, input)
}

// EvaluateFiles is a convenience method for checking if file modifications are allowed.
// Returns the decision and any violation messages.
func (e *Engine) EvaluateFiles(ctx context.Context, filesModified, filesCreated []string) (*PolicyDecision, error) {
	input := &PolicyInput{
		Task: &TaskInput{
			ID:            "file-check",
			Title:         "File modification check",
			FilesModified: filesModified,
			FilesCreated:  filesCreated,
		},
	}
	return e.Evaluate(ctx, input)
}

// MustEvaluate is like Evaluate but panics on error.
// Only use in tests or where errors are truly unexpected.
func (e *Engine) MustEvaluate(ctx context.Context, input any) *PolicyDecision {
	decision, err := e.Evaluate(ctx, input)
	if err != nil {
		panic(fmt.Sprintf("policy evaluation failed: %v", err))
	}
	return decision
}

// GetPolicies returns the loaded policy files.
func (e *Engine) GetPolicies() []*PolicyFile {
	return e.policies
}

// ReloadPolicies reloads policies from disk.
// This is useful if policies have been modified while the engine is running.
func (e *Engine) ReloadPolicies(fs afero.Fs, policiesDir string) error {
	loader := NewLoader(fs, policiesDir)
	policies, err := loader.LoadAll()
	if err != nil {
		return fmt.Errorf("reload policies: %w", err)
	}
	e.policies = policies
	return nil
}

// AddPolicy adds a policy to the engine at runtime.
// The policy content should be valid Rego source code.
func (e *Engine) AddPolicy(name, content string) {
	e.policies = append(e.policies, &PolicyFile{
		Name:    name,
		Path:    name + ".rego",
		Content: content,
	})
}

// ClearPolicies removes all loaded policies.
func (e *Engine) ClearPolicies() {
	e.policies = nil
}

// ValidatePolicy checks if a policy has valid Rego syntax.
// Returns nil if valid, or an error describing the syntax problem.
func ValidatePolicy(content string) error {
	// Try to compile the policy
	_, err := rego.New(
		rego.Query("data"),
		rego.Module("validation.rego", content),
	).PrepareForEval(context.Background())
	if err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}
	return nil
}

// PolicyEvaluatorAdapter adapts the Engine to the task.PolicyEvaluator interface.
// This is a separate type to avoid import cycles - task defines the interface,
// policy implements it via this adapter.
type PolicyEvaluatorAdapter struct {
	engine     *Engine
	auditStore *AuditStore
	sessionID  string
}

// NewPolicyEvaluatorAdapter creates a new adapter for the Engine.
// The auditStore is optional - if nil, decisions won't be persisted.
func NewPolicyEvaluatorAdapter(engine *Engine, auditStore *AuditStore, sessionID string) *PolicyEvaluatorAdapter {
	return &PolicyEvaluatorAdapter{
		engine:     engine,
		auditStore: auditStore,
		sessionID:  sessionID,
	}
}

// PolicyTaskInput mirrors task.PolicyTaskInput to avoid import cycles.
type PolicyTaskInputAdapter struct {
	ID            string
	Title         string
	Description   string
	FilesModified []string
	FilesCreated  []string
}

// PolicyPlanInputAdapter mirrors task.PolicyPlanInput to avoid import cycles.
type PolicyPlanInputAdapter struct {
	ID   string
	Goal string
}

// EvaluateTaskPolicy evaluates a task against loaded policies.
// Implements the task.PolicyEvaluator interface.
func (a *PolicyEvaluatorAdapter) EvaluateTaskPolicy(ctx context.Context, taskID, taskTitle, taskDescription string, filesModified, filesCreated []string, planID, planGoal string) (allowed bool, violations []string, decisionID string, err error) {
	if a.engine == nil {
		return true, nil, "", nil
	}

	// Build policy input
	taskInput := &TaskInput{
		ID:            taskID,
		Title:         taskTitle,
		FilesModified: filesModified,
		FilesCreated:  filesCreated,
	}

	var planInput *PlanInput
	if planID != "" {
		planInput = &PlanInput{
			ID:   planID,
			Goal: planGoal,
		}
	}

	decision, err := a.engine.EvaluateTask(ctx, taskInput, planInput, nil)
	if err != nil {
		return false, nil, "", err
	}

	// Record decision if audit store is available
	if a.auditStore != nil && decision != nil {
		decision.SessionID = a.sessionID
		_ = a.auditStore.SaveDecision(decision) // Ignore save errors
	}

	return decision.IsAllowed(), decision.Violations, decision.DecisionID, nil
}

// EvaluateFilesPolicy checks if file modifications are allowed.
// Implements the task.PolicyEvaluator interface.
func (a *PolicyEvaluatorAdapter) EvaluateFilesPolicy(ctx context.Context, filesModified, filesCreated []string) (allowed bool, violations []string, decisionID string, err error) {
	if a.engine == nil {
		return true, nil, "", nil
	}

	decision, err := a.engine.EvaluateFiles(ctx, filesModified, filesCreated)
	if err != nil {
		return false, nil, "", err
	}

	// Record decision
	if a.auditStore != nil && decision != nil {
		decision.SessionID = a.sessionID
		_ = a.auditStore.SaveDecision(decision)
	}

	return decision.IsAllowed(), decision.Violations, decision.DecisionID, nil
}

// PolicyCount returns the number of loaded policies.
func (a *PolicyEvaluatorAdapter) PolicyCount() int {
	if a.engine == nil {
		return 0
	}
	return a.engine.PolicyCount()
}
