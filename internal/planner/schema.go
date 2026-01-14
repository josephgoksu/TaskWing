// Package planner provides strict schema definitions for LLM-generated plans.
// These schemas enforce structure on LLM outputs to prevent malformed responses.
package planner

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is a singleton validator instance
var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom validation for priority range
	_ = validate.RegisterValidation("priority_range", func(fl validator.FieldLevel) bool {
		p := fl.Field().Int()
		return p >= 0 && p <= 100
	})

	// Register custom validation for non-empty trimmed strings
	_ = validate.RegisterValidation("nonempty", func(fl validator.FieldLevel) bool {
		s := strings.TrimSpace(fl.Field().String())
		return s != ""
	})
}

// LLMPlanResponse represents the expected JSON structure from LLM plan generation.
// This is the schema that the LLM must conform to.
type LLMPlanResponse struct {
	// GoalSummary is a concise (max 100 char) summary of the goal for UI display
	GoalSummary string `json:"goal_summary" validate:"required,nonempty,max=100"`

	// Rationale explains why this plan was designed this way
	Rationale string `json:"rationale" validate:"required,nonempty,min=20"`

	// Tasks are the individual steps to complete the goal
	Tasks []LLMTaskSchema `json:"tasks" validate:"required,min=1,max=50,dive"`

	// EstimatedComplexity is an overall complexity rating
	EstimatedComplexity string `json:"estimated_complexity" validate:"required,oneof=low medium high"`

	// Prerequisites are conditions that must be met before starting
	Prerequisites []string `json:"prerequisites,omitempty"`

	// RiskFactors are potential issues that could block progress
	RiskFactors []string `json:"risk_factors,omitempty"`
}

// LLMTaskSchema represents a single task in the LLM response.
// Each task must have a title, description, and priority at minimum.
type LLMTaskSchema struct {
	// Title is a concise action-oriented description (max 200 chars)
	Title string `json:"title" validate:"required,nonempty,max=200"`

	// Description provides detailed context for the task
	Description string `json:"description" validate:"required,nonempty,min=10"`

	// Priority is the importance level (0-100, lower = more important)
	Priority int `json:"priority" validate:"priority_range"`

	// Complexity indicates the estimated difficulty
	Complexity string `json:"complexity" validate:"required,oneof=low medium high"`

	// AssignedAgent is the type of agent to handle this task
	AssignedAgent string `json:"assigned_agent" validate:"required,oneof=coder qa architect researcher"`

	// AcceptanceCriteria are checkable conditions for task completion
	AcceptanceCriteria []string `json:"acceptance_criteria" validate:"required,min=1,dive,nonempty"`

	// ValidationSteps are CLI commands to verify the task
	ValidationSteps []string `json:"validation_steps,omitempty"`

	// DependsOn lists task indices (0-based) that must complete first
	DependsOn []int `json:"depends_on,omitempty"`

	// Scope categorizes the task for context retrieval
	Scope string `json:"scope,omitempty"`

	// Keywords for semantic search
	Keywords []string `json:"keywords,omitempty"`
}

// LLMClarificationResponse represents the expected JSON structure from goal clarification.
type LLMClarificationResponse struct {
	// IsReadyToPlan indicates if enough information is available
	IsReadyToPlan bool `json:"is_ready_to_plan"`

	// EnrichedGoal is the refined technical specification (required when ready to plan)
	EnrichedGoal string `json:"enriched_goal"`

	// GoalSummary is a concise summary for UI display
	GoalSummary string `json:"goal_summary" validate:"required,nonempty,max=100"`

	// Questions are clarifying questions to ask the user (if not ready)
	Questions []string `json:"questions,omitempty"`

	// Assumptions made during enrichment
	Assumptions []string `json:"assumptions,omitempty"`

	// Constraints identified from the goal
	Constraints []string `json:"constraints,omitempty"`
}

// Validate checks the LLMClarificationResponse with conditional logic.
// Returns a ValidationResult with detailed error information.
func (c *LLMClarificationResponse) Validate() ValidationResult {
	// First run struct validation
	result := validateStruct(c)

	// Add conditional validation
	if c.IsReadyToPlan {
		// EnrichedGoal is required when ready to plan
		if strings.TrimSpace(c.EnrichedGoal) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "EnrichedGoal",
				Tag:     "required_if",
				Message: "EnrichedGoal is required when IsReadyToPlan is true",
			})
		}
	} else {
		// Questions are required when not ready to plan
		if len(c.Questions) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "Questions",
				Tag:     "required_if",
				Message: "Questions are required when IsReadyToPlan is false",
			})
		}
	}

	return result
}

// ValidationError provides structured error information for schema validation failures
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   any    `json:"value,omitempty"`
	Message string `json:"message"`
}

// ValidationResult contains the result of schema validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validate checks the LLMPlanResponse against the schema rules.
// Returns a ValidationResult with detailed error information.
func (r *LLMPlanResponse) Validate() ValidationResult {
	return validateStruct(r)
}

// Validate checks the LLMTaskSchema against the schema rules.
func (t *LLMTaskSchema) Validate() ValidationResult {
	return validateStruct(t)
}

// validateStruct is a helper that validates any struct and returns ValidationResult
func validateStruct(s any) ValidationResult {
	err := validate.Struct(s)
	if err == nil {
		return ValidationResult{Valid: true}
	}

	var errors []ValidationError
	for _, err := range err.(validator.ValidationErrors) {
		errors = append(errors, ValidationError{
			Field:   err.Field(),
			Tag:     err.Tag(),
			Value:   err.Value(),
			Message: formatValidationError(err),
		})
	}

	return ValidationResult{
		Valid:  false,
		Errors: errors,
	}
}

// formatValidationError creates a human-readable error message
func formatValidationError(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", err.Field())
	case "nonempty":
		return fmt.Sprintf("%s cannot be empty or whitespace", err.Field())
	case "min":
		if err.Kind().String() == "string" {
			return fmt.Sprintf("%s must be at least %s characters", err.Field(), err.Param())
		}
		return fmt.Sprintf("%s must have at least %s items", err.Field(), err.Param())
	case "max":
		if err.Kind().String() == "string" {
			return fmt.Sprintf("%s must be at most %s characters", err.Field(), err.Param())
		}
		return fmt.Sprintf("%s must have at most %s items", err.Field(), err.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", err.Field(), err.Param())
	case "priority_range":
		return fmt.Sprintf("%s must be between 0 and 100", err.Field())
	case "required_if":
		return fmt.Sprintf("%s is required when condition is met", err.Field())
	default:
		return fmt.Sprintf("%s failed validation: %s", err.Field(), err.Tag())
	}
}

// ErrorSummary returns a single string summarizing all validation errors
func (r ValidationResult) ErrorSummary() string {
	if r.Valid {
		return ""
	}
	var parts []string
	for _, e := range r.Errors {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, "; ")
}
