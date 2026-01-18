// Package planner provides structured generation with validation and retry logic.
package planner

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"strings"
	"text/template"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

const (
	// MaxGenerationRetries is the maximum number of retry attempts for validation failures
	MaxGenerationRetries = 3

	// RetryDelay is the delay between retries
	RetryDelay = 500 * time.Millisecond
)

// GeneratorConfig configures the structured generator.
type GeneratorConfig struct {
	LLMConfig llm.Config
	// Temperature for generation (0 = deterministic)
	Temperature float32
}

// Generator produces validated structured output from LLM.
type Generator struct {
	cfg       GeneratorConfig
	chatModel *llm.CloseableChatModel
}

// NewGenerator creates a new structured generator.
// Temperature is set to 0 by default for deterministic output.
func NewGenerator(cfg GeneratorConfig) *Generator {
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.0 // Explicit zero for determinism
	}
	return &Generator{cfg: cfg}
}

// Close releases LLM resources.
func (g *Generator) Close() error {
	if g.chatModel != nil {
		return g.chatModel.Close()
	}
	return nil
}

// GenerationResult contains the result of a structured generation.
type GenerationResult[T any] struct {
	Result    T
	RawOutput string
	Attempts  int
	Duration  time.Duration
}

// GeneratePlan generates a validated plan using the LLM with retry logic.
// If validation fails, the error is fed back to the LLM for self-correction.
func (g *Generator) GeneratePlan(ctx context.Context, goal, kgContext string) (*GenerationResult[LLMPlanResponse], error) {
	return generateWithRetry(
		ctx,
		g,
		planPromptTemplate,
		map[string]any{
			"Goal":    goal,
			"Context": kgContext,
		},
		func(r *LLMPlanResponse) ValidationResult {
			return r.Validate()
		},
	)
}

// GenerateClarification generates a validated clarification response.
func (g *Generator) GenerateClarification(ctx context.Context, goal, history, kgContext string) (*GenerationResult[LLMClarificationResponse], error) {
	return generateWithRetry(
		ctx,
		g,
		clarificationPromptTemplate,
		map[string]any{
			"Goal":    goal,
			"History": history,
			"Context": kgContext,
		},
		func(r *LLMClarificationResponse) ValidationResult {
			return r.Validate()
		},
	)
}

// generateWithRetry is the core generation loop with validation and error feedback.
func generateWithRetry[T any](
	ctx context.Context,
	g *Generator,
	promptTemplate string,
	input map[string]any,
	validate func(*T) ValidationResult,
) (*GenerationResult[T], error) {
	start := time.Now()

	// Initialize chat model if needed
	if g.chatModel == nil {
		model, err := llm.NewCloseableChatModel(ctx, g.cfg.LLMConfig)
		if err != nil {
			return nil, fmt.Errorf("create chat model: %w", err)
		}
		g.chatModel = model
	}

	// Parse the prompt template
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var lastRaw string
	var lastErr error
	var validationErrors string

	for attempt := 1; attempt <= MaxGenerationRetries; attempt++ {
		// Build prompt with optional error feedback
		promptInput := copyMap(input)
		if validationErrors != "" {
			promptInput["ValidationErrors"] = validationErrors
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, promptInput); err != nil {
			return nil, fmt.Errorf("execute template: %w", err)
		}
		prompt := buf.String()

		// Generate with temperature=0
		messages := []*schema.Message{
			schema.UserMessage(prompt),
		}

		resp, err := g.chatModel.Generate(ctx, messages)
		if err != nil {
			lastErr = fmt.Errorf("LLM generate: %w", err)
			// Check if we should retry on transient errors
			if isTransientError(err) && attempt < MaxGenerationRetries {
				time.Sleep(RetryDelay * time.Duration(attempt))
				continue
			}
			return nil, lastErr
		}

		lastRaw = resp.Content

		// Parse JSON from response
		var result T
		result, err = utils.ExtractAndParseJSON[T](resp.Content)
		if err != nil {
			lastErr = fmt.Errorf("parse JSON (attempt %d): %w", attempt, err)
			validationErrors = formatErrorFeedback("JSON Parse Error", err.Error(), resp.Content)
			if attempt < MaxGenerationRetries {
				time.Sleep(RetryDelay)
				continue
			}
			return nil, lastErr
		}

		// Validate against schema
		validationResult := validate(&result)
		if !validationResult.Valid {
			lastErr = fmt.Errorf("validation failed (attempt %d): %s", attempt, validationResult.ErrorSummary())
			validationErrors = formatValidationFeedback(validationResult)
			if attempt < MaxGenerationRetries {
				time.Sleep(RetryDelay)
				continue
			}
			return nil, lastErr
		}

		// Success!
		return &GenerationResult[T]{
			Result:    result,
			RawOutput: lastRaw,
			Attempts:  attempt,
			Duration:  time.Since(start),
		}, nil
	}

	return nil, fmt.Errorf("generation failed after %d attempts: %w", MaxGenerationRetries, lastErr)
}

// formatErrorFeedback creates a prompt section for error feedback.
func formatErrorFeedback(errorType, errorMsg, rawOutput string) string {
	// Truncate raw output if too long
	truncated := rawOutput
	if len(truncated) > 500 {
		truncated = truncated[:500] + "... [truncated]"
	}

	return fmt.Sprintf(`
PREVIOUS ATTEMPT FAILED - PLEASE FIX

Error Type: %s
Error: %s

Your previous output (which failed):
%s

Please ensure your response is valid JSON matching the required schema.
`, errorType, errorMsg, truncated)
}

// formatValidationFeedback creates detailed validation error feedback.
func formatValidationFeedback(result ValidationResult) string {
	var sb strings.Builder
	sb.WriteString("\nPREVIOUS ATTEMPT FAILED - SCHEMA VALIDATION ERRORS\n\n")
	sb.WriteString("Please fix the following issues:\n")

	for i, e := range result.Errors {
		sb.WriteString(fmt.Sprintf("%d. Field '%s': %s\n", i+1, e.Field, e.Message))
		if e.Value != nil {
			sb.WriteString(fmt.Sprintf("   Current value: %v\n", e.Value))
		}
	}

	sb.WriteString("\nPlease regenerate the response with these issues corrected.\n")
	return sb.String()
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	maps.Copy(result, m)
	return result
}

// isTransientError checks if an error is transient and worth retrying.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())

	// Rate limit errors
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "quota exceeded") {
		return true
	}

	// Network errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "temporary") {
		return true
	}

	return false
}

// Prompt templates with support for validation error feedback.
const planPromptTemplate = `You are a senior software architect creating a detailed implementation plan.

GOAL:
{{.Goal}}

PROJECT CONTEXT:
{{.Context}}
{{if .ValidationErrors}}
{{.ValidationErrors}}
{{end}}
INSTRUCTIONS:
Generate a structured plan as JSON with the following schema:

{
  "goal_summary": "string (max 100 chars, concise summary for UI)",
  "rationale": "string (min 20 chars, explain why this plan design)",
  "estimated_complexity": "low|medium|high",
  "prerequisites": ["optional list of conditions"],
  "risk_factors": ["optional list of risks"],
  "tasks": [
    {
      "title": "string (max 200 chars, action-oriented)",
      "description": "string (min 10 chars, detailed context)",
      "priority": 0-100 (lower = more important),
      "complexity": "low|medium|high",
      "assigned_agent": "coder|qa|architect|researcher",
      "acceptance_criteria": ["at least one criterion"],
      "validation_steps": ["optional CLI commands to verify"],
      "depends_on": [0-based indices of dependent tasks],
      "scope": "optional category",
      "keywords": ["optional search keywords"],
      "expected_files": ["files this task will create/modify/delete"]
    }
  ]
}

RULES:
- Tasks must be atomic and independently testable
- Each task needs at least one acceptance criterion
- Use 0-based indices for depends_on references
- Priority: 0=critical, 50=normal, 100=nice-to-have
- assigned_agent: coder (implementation), qa (testing), architect (design), researcher (analysis)
- expected_files: List files that will be created, modified, or deleted (e.g., "internal/auth/handler.go", "tests/auth_test.go")
- Output ONLY valid JSON, no markdown or explanation

Generate the plan JSON now:`

const clarificationPromptTemplate = `You are a senior software architect helping refine a development goal.

GOAL:
{{.Goal}}

CONVERSATION HISTORY:
{{.History}}

PROJECT CONTEXT:
{{.Context}}
{{if .ValidationErrors}}
{{.ValidationErrors}}
{{end}}
INSTRUCTIONS:
Analyze the goal and determine if you have enough information to create a plan.

Output JSON with this schema:
{
  "is_ready_to_plan": boolean,
  "goal_summary": "string (max 100 chars, concise summary)",
  "enriched_goal": "string (detailed technical specification if ready)",
  "questions": ["list of clarifying questions if not ready"],
  "assumptions": ["list of assumptions made"],
  "constraints": ["list of identified constraints"]
}

RULES:
- If is_ready_to_plan is true, enriched_goal must be a detailed specification
- If is_ready_to_plan is false, questions must have at least one question
- goal_summary is always required (max 100 characters)
- Output ONLY valid JSON

Generate the clarification response now:`
