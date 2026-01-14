package planner

import (
	"strings"
	"testing"
)

func TestRetryLogic_FormatErrorFeedback(t *testing.T) {
	feedback := formatErrorFeedback("JSON Parse Error", "unexpected end of JSON", `{"incomplete":`)

	if !strings.Contains(feedback, "JSON Parse Error") {
		t.Error("Expected feedback to contain error type")
	}
	if !strings.Contains(feedback, "unexpected end of JSON") {
		t.Error("Expected feedback to contain error message")
	}
	if !strings.Contains(feedback, `{"incomplete":`) {
		t.Error("Expected feedback to contain raw output")
	}
}

func TestRetryLogic_FormatErrorFeedback_Truncation(t *testing.T) {
	// Create a long string that should be truncated
	longOutput := strings.Repeat("a", 600)
	feedback := formatErrorFeedback("Test Error", "test message", longOutput)

	if !strings.Contains(feedback, "[truncated]") {
		t.Error("Expected long output to be truncated")
	}
	// Should not contain the full 600 character string
	if strings.Contains(feedback, longOutput) {
		t.Error("Expected output to be truncated, but found full string")
	}
}

func TestRetryLogic_FormatValidationFeedback(t *testing.T) {
	result := ValidationResult{
		Valid: false,
		Errors: []ValidationError{
			{Field: "Title", Tag: "required", Message: "Title is required"},
			{Field: "Priority", Tag: "priority_range", Value: 150, Message: "Priority must be between 0 and 100"},
		},
	}

	feedback := formatValidationFeedback(result)

	if !strings.Contains(feedback, "SCHEMA VALIDATION ERRORS") {
		t.Error("Expected feedback to contain validation errors header")
	}
	if !strings.Contains(feedback, "Title") {
		t.Error("Expected feedback to mention Title field")
	}
	if !strings.Contains(feedback, "Priority") {
		t.Error("Expected feedback to mention Priority field")
	}
	if !strings.Contains(feedback, "150") {
		t.Error("Expected feedback to show current value")
	}
}

func TestRetryLogic_IsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{"nil error", "", false},
		{"rate limit", "rate limit exceeded", true},
		{"http 429", "HTTP 429 Too Many Requests", true},
		{"quota exceeded", "API quota exceeded for today", true},
		{"timeout", "context deadline exceeded: timeout", true},
		{"connection reset", "connection reset by peer", true},
		{"validation error", "validation failed: Title is required", false},
		{"json parse error", "json: cannot unmarshal", false},
		{"generic error", "something went wrong", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = &testError{msg: tt.errMsg}
			}
			result := isTransientError(err)
			if result != tt.expected {
				t.Errorf("isTransientError(%q) = %v, want %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}

func TestRetryLogic_CopyMap(t *testing.T) {
	original := map[string]any{
		"Goal":    "test goal",
		"Context": "test context",
	}

	copied := copyMap(original)

	// Verify copy has same values
	if copied["Goal"] != "test goal" {
		t.Error("Expected Goal to be copied")
	}
	if copied["Context"] != "test context" {
		t.Error("Expected Context to be copied")
	}

	// Verify modification doesn't affect original
	copied["NewKey"] = "new value"
	if _, exists := original["NewKey"]; exists {
		t.Error("Expected copy to be independent of original")
	}
}

func TestRetryLogic_PromptTemplates(t *testing.T) {
	// Verify plan template has required placeholders
	if !strings.Contains(planPromptTemplate, "{{.Goal}}") {
		t.Error("Plan template missing Goal placeholder")
	}
	if !strings.Contains(planPromptTemplate, "{{.Context}}") {
		t.Error("Plan template missing Context placeholder")
	}
	if !strings.Contains(planPromptTemplate, "{{.ValidationErrors}}") {
		t.Error("Plan template missing ValidationErrors placeholder")
	}

	// Verify clarification template has required placeholders
	if !strings.Contains(clarificationPromptTemplate, "{{.Goal}}") {
		t.Error("Clarification template missing Goal placeholder")
	}
	if !strings.Contains(clarificationPromptTemplate, "{{.History}}") {
		t.Error("Clarification template missing History placeholder")
	}
	if !strings.Contains(clarificationPromptTemplate, "{{.ValidationErrors}}") {
		t.Error("Clarification template missing ValidationErrors placeholder")
	}
}

func TestRetryLogic_GeneratorConfig_DefaultTemperature(t *testing.T) {
	gen := NewGenerator(GeneratorConfig{})

	// Temperature should be 0 (deterministic) by default
	if gen.cfg.Temperature != 0.0 {
		t.Errorf("Expected default temperature 0.0, got %f", gen.cfg.Temperature)
	}
}

func TestRetryLogic_Constants(t *testing.T) {
	// Verify retry constants are sensible
	if MaxGenerationRetries < 1 {
		t.Error("MaxGenerationRetries should be at least 1")
	}
	if MaxGenerationRetries > 10 {
		t.Error("MaxGenerationRetries should not be excessive")
	}
	if RetryDelay < 100*1000000 { // 100ms in nanoseconds
		t.Error("RetryDelay should be at least 100ms")
	}
}

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
