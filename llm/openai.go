package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/types"
)

// OpenAIProvider implements the Provider interface for OpenAI LLMs.
type OpenAIProvider struct {
	apiKey  string
	timeout time.Duration
	debug   bool
}

// NewOpenAIProvider creates a new OpenAIProvider with options.
func NewOpenAIProvider(apiKey string, timeout time.Duration, debug bool) *OpenAIProvider {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &OpenAIProvider{apiKey: apiKey, timeout: timeout, debug: debug}
}

// OpenAIRequestPayload defines the structure for the OpenAI API request.
type OpenAIRequestPayload struct {
	Model          string                `json:"model"`
	Messages       []OpenAIMessage       `json:"messages"`
	Temperature    float64               `json:"temperature,omitempty"`
	MaxTokens      int                   `json:"max_tokens,omitempty"`
	ResponseFormat *OpenAIResponseFormat `json:"response_format,omitempty"`
}

// OpenAIResponseFormat specifies the output format for OpenAI (e.g., JSON object).
type OpenAIResponseFormat struct {
	Type string `json:"type"` // e.g., "json_object"
}

// OpenAIMessage defines a message in the conversation for OpenAI.
type OpenAIMessage struct {
	Role    string `json:"role"` // "system", "user", or "assistant"
	Content string `json:"content"`
}

// OpenAIResponsePayload defines the structure for the OpenAI API response.
type OpenAIResponsePayload struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

// OpenAIChoice defines a choice in the OpenAI response.
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage defines token usage statistics from OpenAI.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAITaskResponseWrapper is used to unmarshal the JSON object returned by OpenAI
// when response_format is json_object and the prompt requests a list of tasks.
type OpenAITaskResponseWrapper struct {
	Tasks []types.TaskOutput `json:"tasks"`
}

// buildTasksSchema returns a JSON Schema for an object with a required 'tasks' array.
func buildTasksSchema() map[string]interface{} {
	// Define a base task object
	requiredFields := []string{"title", "description", "acceptanceCriteria", "priority", "tempId", "subtasks", "dependsOnIds"}
	baseTask := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"title":              map[string]interface{}{"type": "string"},
			"description":        map[string]interface{}{"type": "string"},
			"acceptanceCriteria": map[string]interface{}{"type": "string"},
			"priority":           map[string]interface{}{"type": "string"},
			"tempId":             map[string]interface{}{"type": "integer", "minimum": 1},
			"dependsOnIds": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "integer"},
			},
		},
		"required": requiredFields,
	}
	// Define a shallow subtask object (one level) using the same fields and no deep recursion
	shallowSubtask := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"title":              map[string]interface{}{"type": "string"},
			"description":        map[string]interface{}{"type": "string"},
			"acceptanceCriteria": map[string]interface{}{"type": "string"},
			"priority":           map[string]interface{}{"type": "string"},
			"tempId":             map[string]interface{}{"type": "integer", "minimum": 1},
			"dependsOnIds": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "integer"},
			},
			// Allow nested 'subtasks' but constrain inner objects to have no free-form properties
			"subtasks": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties":           map[string]interface{}{},
				},
			},
		},
		"required": requiredFields,
	}
	// Attach shallow subtasks to base task
	baseTask["properties"].(map[string]interface{})["subtasks"] = map[string]interface{}{
		"type":  "array",
		"items": shallowSubtask,
	}

	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"tasks": map[string]interface{}{
				"type":  "array",
				"items": baseTask,
			},
		},
		"required": []string{"tasks"},
	}
}

// buildEnhancedTaskSchema returns a JSON Schema for a single enhanced task object.
func buildEnhancedTaskSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"title":              map[string]interface{}{"type": "string"},
			"description":        map[string]interface{}{"type": "string"},
			"acceptanceCriteria": map[string]interface{}{"type": "string"},
			"priority":           map[string]interface{}{"type": "string"},
		},
		"required": []string{"title", "description", "acceptanceCriteria", "priority"},
	}
}

// buildSubtasksSchema returns a JSON Schema for an array of subtask objects.
func buildSubtasksSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"subtasks": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"title":              map[string]interface{}{"type": "string"},
						"description":        map[string]interface{}{"type": "string"},
						"acceptanceCriteria": map[string]interface{}{"type": "string"},
						"priority":           map[string]interface{}{"type": "string"},
					},
					"required": []string{"title", "description", "acceptanceCriteria", "priority"},
				},
				"minItems": 3,
				"maxItems": 7,
			},
		},
		"required": []string{"subtasks"},
	}
}

// buildSuggestionsSchema returns a JSON Schema for task suggestion objects.
func buildSuggestionsSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"suggestions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"taskId":          map[string]interface{}{"type": "string"},
						"reasoning":       map[string]interface{}{"type": "string"},
						"confidenceScore": map[string]interface{}{"type": "number", "minimum": 0.0, "maximum": 1.0},
						"estimatedEffort": map[string]interface{}{"type": "string"},
						"projectPhase":    map[string]interface{}{"type": "string"},
						"recommendedActions": map[string]interface{}{
							"type":     "array",
							"items":    map[string]interface{}{"type": "string"},
							"minItems": 1,
							"maxItems": 5,
						},
					},
					"required": []string{"taskId", "reasoning", "confidenceScore", "estimatedEffort", "projectPhase", "recommendedActions"},
				},
				"minItems": 0,
				"maxItems": 5,
			},
		},
		"required": []string{"suggestions"},
	}
}

// buildDependenciesSchema returns a JSON Schema for dependency suggestion objects.
func buildDependenciesSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"dependencies": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"sourceTaskId":    map[string]interface{}{"type": "string"},
						"targetTaskId":    map[string]interface{}{"type": "string"},
						"reasoning":       map[string]interface{}{"type": "string"},
						"confidenceScore": map[string]interface{}{"type": "number", "minimum": 0.0, "maximum": 1.0},
						"dependencyType":  map[string]interface{}{"type": "string", "enum": []string{"technical", "logical", "sequential"}},
					},
					"required": []string{"sourceTaskId", "targetTaskId", "reasoning", "confidenceScore", "dependencyType"},
				},
				"minItems": 0,
				"maxItems": 5,
			},
		},
		"required": []string{"dependencies"},
	}
}

// parseEnhancedTaskFallback makes a best-effort to build an EnhancedTask from free-form text.
func parseEnhancedTaskFallback(text, defaultTitle string) types.EnhancedTask {
	// First, attempt to extract an inline JSON object if present
	if et, ok := tryExtractEnhancedTaskJSON(text); ok {
		return sanitizeEnhancedTask(et, defaultTitle)
	}
	et := types.EnhancedTask{}
	lines := strings.Split(text, "\n")
	for _, l := range lines {
		line := strings.TrimSpace(l)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "title:"):
			et.Title = strings.TrimSpace(line[len("title:"):])
		case strings.HasPrefix(lower, "description:"):
			et.Description = strings.TrimSpace(line[len("description:"):])
		case strings.HasPrefix(lower, "acceptance criteria:"):
			et.AcceptanceCriteria = strings.TrimSpace(line[len("acceptance criteria:"):])
		case strings.HasPrefix(lower, "acceptancecriteria:"):
			et.AcceptanceCriteria = strings.TrimSpace(line[len("acceptancecriteria:"):])
		case strings.HasPrefix(lower, "priority:"):
			et.Priority = strings.TrimSpace(line[len("priority:"):])
		}
	}
	// If we didn't find structured labels, treat the whole text as description
	if et.Title == "" {
		et.Title = defaultTitle
	}
	if et.Description == "" {
		et.Description = strings.TrimSpace(text)
		if et.Description == "" {
			et.Description = defaultTitle
		}
	}
	if et.Priority == "" {
		et.Priority = "medium"
	}
	return sanitizeEnhancedTask(et, defaultTitle)
}

// tryExtractEnhancedTaskJSON tries to parse a JSON object from the given string.
// It supports the entire string being JSON or a substring between the first '{' and last '}'.
func tryExtractEnhancedTaskJSON(s string) (types.EnhancedTask, bool) {
	var et types.EnhancedTask
	ss := strings.TrimSpace(s)
	// Direct JSON
	if strings.HasPrefix(ss, "{") && strings.HasSuffix(ss, "}") {
		if err := json.Unmarshal([]byte(ss), &et); err == nil {
			return et, true
		}
	}
	// Substring JSON
	start := strings.Index(ss, "{")
	end := strings.LastIndex(ss, "}")
	if start >= 0 && end > start {
		sub := ss[start : end+1]
		if err := json.Unmarshal([]byte(sub), &et); err == nil {
			return et, true
		}
	}
	return types.EnhancedTask{}, false
}

// sanitizeEnhancedTask cleans fields and applies safe defaults.
func sanitizeEnhancedTask(in types.EnhancedTask, defaultTitle string) types.EnhancedTask {
	et := in
	// Ensure title
	if strings.TrimSpace(et.Title) == "" {
		et.Title = defaultTitle
	}
	// Remove any embedded acceptance criteria block from description
	if idx := strings.Index(strings.ToLower(et.Description), strings.ToLower("Acceptance Criteria:")); idx >= 0 {
		et.Description = strings.TrimSpace(et.Description[:idx])
	}
	// Trim and default
	et.Title = strings.TrimSpace(et.Title)
	et.Description = strings.TrimSpace(et.Description)
	et.AcceptanceCriteria = strings.TrimSpace(et.AcceptanceCriteria)
	if strings.TrimSpace(et.Priority) == "" {
		et.Priority = "medium"
	}
	return et
}

const (
	openAIResponsesURL = "https://api.openai.com/v1/responses"
)

// GenerateTasks for OpenAIProvider.
func (p *OpenAIProvider) GenerateTasks(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.TaskOutput, error) {
	if apiKey == "" {
		apiKey = p.apiKey // Use provider's key if per-call key is not given
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is not set")
	}

	userMessage := fmt.Sprintf("Please analyze this PRD content and return a JSON response with the requested format:\n---\n%s\n---", prdContent)

	content, err := p.callOpenAIAndExtract(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
	if err != nil {
		return nil, err
	}

	var responseWrapper OpenAITaskResponseWrapper
	if err := json.Unmarshal([]byte(content), &responseWrapper); err != nil {
		return nil, fmt.Errorf("failed to parse tasks JSON from AI response: %w", err)
	}

	return responseWrapper.Tasks, nil
	// return nil, fmt.Errorf("OpenAI GenerateTasks not yet fully implemented")
}

// ImprovePRD sends the PRD content to OpenAI with a prompt to refine and improve it.
func (p *OpenAIProvider) ImprovePRD(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForImprovement int, temperatureForImprovement float64) (string, error) {
	if apiKey == "" {
		apiKey = p.apiKey
	}
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key is not set for PRD improvement")
	}

	userMessage := fmt.Sprintf("Please improve the following PRD content and return the enhanced version:\n---\n%s\n---", prdContent)

	content, err := p.callOpenAIAndExtractText(ctx, apiKey, modelName, systemPrompt, userMessage, temperatureForImprovement, maxTokensForImprovement)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(content), nil
}

// EnhanceTask sends task input to OpenAI for AI-powered enhancement and returns structured task details.
func (p *OpenAIProvider) EnhanceTask(ctx context.Context, systemPrompt, taskInput, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) (types.EnhancedTask, error) {
	if apiKey == "" {
		apiKey = p.apiKey
	}
	if apiKey == "" {
		return types.EnhancedTask{}, fmt.Errorf("OpenAI API key is not set for task enhancement")
	}

	// Build user message with task input and context
	userMessage := fmt.Sprintf("Task input: %s", taskInput)
	if contextInfo != "" {
		userMessage += fmt.Sprintf("\n\nContext: %s", contextInfo)
	}

	// Use a schema tailored for a single enhanced task output
	content, err := p.callOpenAIAndExtractEnhanced(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
	if err != nil {
		// Fallback: retry once without JSON schema in plain text mode
		content2, err2 := p.callOpenAIAndExtractText(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
		if err2 != nil {
			// Last-resort: craft a minimal, sane enhancement from original input
			base := taskInput
			if idx := strings.Index(base, "\n"); idx >= 0 {
				base = base[:idx]
			}
			if idx := strings.Index(strings.ToLower(base), "acceptance criteria:"); idx >= 0 {
				base = strings.TrimSpace(base[:idx])
			}
			if strings.TrimSpace(base) == "" {
				base = taskInput
			}
			return types.EnhancedTask{
				Title:              base,
				Description:        base,
				AcceptanceCriteria: "- task is clarified and scoped\n- requirements reviewed with stakeholders",
				Priority:           "medium",
			}, nil
		}
		content = content2
	}

	var enhancedTask types.EnhancedTask
	// Try direct JSON or embedded JSON first
	if et, ok := tryExtractEnhancedTaskJSON(content); ok {
		return sanitizeEnhancedTask(et, taskInput), nil
	}
	if err := json.Unmarshal([]byte(content), &enhancedTask); err != nil {
		// Fallback: try to coerce from a text response
		coerced := parseEnhancedTaskFallback(content, taskInput)
		// Ensure at least a title is present
		if strings.TrimSpace(coerced.Title) == "" {
			coerced.Title = taskInput
		}
		if strings.TrimSpace(coerced.Description) == "" {
			coerced.Description = taskInput
		}
		return sanitizeEnhancedTask(coerced, taskInput), nil
	}

	return sanitizeEnhancedTask(enhancedTask, taskInput), nil
}

// BreakdownTask analyzes a task and suggests relevant subtasks
func (p *OpenAIProvider) BreakdownTask(ctx context.Context, systemPrompt, taskTitle, taskDescription, acceptanceCriteria, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.EnhancedTask, error) {
	if apiKey == "" {
		apiKey = p.apiKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is not set for task breakdown")
	}

	// Build user message with task details and context
	userMessage := "Analyze this task and suggest 3-7 relevant subtasks:\n\n"
	userMessage += fmt.Sprintf("Title: %s\n", taskTitle)
	if taskDescription != "" && taskDescription != taskTitle {
		userMessage += fmt.Sprintf("Description: %s\n", taskDescription)
	}
	if acceptanceCriteria != "" {
		userMessage += fmt.Sprintf("Acceptance Criteria: %s\n", acceptanceCriteria)
	}
	if contextInfo != "" {
		userMessage += fmt.Sprintf("\nContext: %s", contextInfo)
	}

	// Use a schema tailored for an array of enhanced tasks
	content, err := p.callOpenAIAndExtractSubtasks(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
	if err != nil {
		return nil, err
	}

	var subtaskWrapper struct {
		Subtasks []types.EnhancedTask `json:"subtasks"`
	}
	if err := json.Unmarshal([]byte(content), &subtaskWrapper); err != nil {
		return nil, fmt.Errorf("failed to parse subtasks JSON from AI response: %w", err)
	}

	return subtaskWrapper.Subtasks, nil
}

// SuggestNextTask provides context-aware suggestions for which task to work on next
func (p *OpenAIProvider) SuggestNextTask(ctx context.Context, systemPrompt, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.TaskSuggestion, error) {
	// Build user message with context info
	userMessage := fmt.Sprintf("Analyze the current project context and suggest the most strategic tasks to work on next:\n\n%s", contextInfo)

	// Call OpenAI API with task suggestions schema
	content, err := p.callOpenAIAndExtractSuggestions(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI for task suggestions: %w", err)
	}

	// Parse the JSON response
	var suggestionsWrapper struct {
		Suggestions []types.TaskSuggestion `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(content), &suggestionsWrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task suggestions response: %w (content: %s)", err, content)
	}

	return suggestionsWrapper.Suggestions, nil
}

// DetectDependencies analyzes tasks and suggests dependency relationships
func (p *OpenAIProvider) DetectDependencies(ctx context.Context, systemPrompt, taskInfo, contextInfo string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.DependencySuggestion, error) {
	// Build user message with task and context info
	userMessage := fmt.Sprintf("Analyze this task for potential dependencies:\n\n%s\n\nProject Context:\n%s", taskInfo, contextInfo)

	// Call OpenAI API with dependencies schema
	content, err := p.callOpenAIAndExtractDependencies(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI for dependency detection: %w", err)
	}

	// Parse the JSON response
	var dependenciesWrapper struct {
		Dependencies []types.DependencySuggestion `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(content), &dependenciesWrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dependencies response: %w (content: %s)", err, content)
	}

	return dependenciesWrapper.Dependencies, nil
}

// callOpenAIAndExtract sends a JSON-schema constrained request (for tasks array) and returns content.
func (p *OpenAIProvider) callOpenAIAndExtract(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	return p.callResponsesAPIWithSchema(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens, "task_generation", buildTasksSchema(), true)
}

// callOpenAIAndExtractText calls the API without JSON formatting for plain text responses
func (p *OpenAIProvider) callOpenAIAndExtractText(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	// Use the Responses API without JSON format
	return p.callResponsesAPIWithSchema(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens, "", nil, false)
}

// callOpenAIAndExtractEnhanced requests a single enhanced task using a tailored schema.
func (p *OpenAIProvider) callOpenAIAndExtractEnhanced(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	return p.callResponsesAPIWithSchema(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens, "enhanced_task", buildEnhancedTaskSchema(), true)
}

// callOpenAIAndExtractSubtasks requests an array of subtasks using a tailored schema.
func (p *OpenAIProvider) callOpenAIAndExtractSubtasks(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	return p.callResponsesAPIWithSchema(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens, "subtask_breakdown", buildSubtasksSchema(), true)
}

// callOpenAIAndExtractSuggestions requests an array of task suggestions using a tailored schema.
func (p *OpenAIProvider) callOpenAIAndExtractSuggestions(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	return p.callResponsesAPIWithSchema(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens, "task_suggestions", buildSuggestionsSchema(), true)
}

// callOpenAIAndExtractDependencies requests an array of dependency suggestions using a tailored schema.
func (p *OpenAIProvider) callOpenAIAndExtractDependencies(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	return p.callResponsesAPIWithSchema(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens, "dependency_detection", buildDependenciesSchema(), true)
}

func (p *OpenAIProvider) callResponsesAPIWithSchema(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int, schemaName string, schema map[string]interface{}, useJsonFormat bool) (string, error) {
	// Build the Responses API payload using structured content blocks
	payload := map[string]interface{}{
		"model": modelName,
		"input": []map[string]interface{}{
			{
				"role":    "system",
				"content": []map[string]interface{}{{"type": "input_text", "text": systemPrompt}},
			},
			{
				"role":    "user",
				"content": []map[string]interface{}{{"type": "input_text", "text": userMessage}},
			},
		},
	}

	// Only set max_output_tokens if explicitly provided (>0)
	if maxTokens > 0 {
		payload["max_output_tokens"] = maxTokens
	}

	// Configure text.format per the Responses API
	if useJsonFormat && schema != nil {
		payload["text"] = map[string]interface{}{
			"format": map[string]interface{}{
				"type":   "json_schema",
				"name":   schemaName,
				"schema": schema,
				"strict": true,
			},
		}
	} else {
		payload["text"] = map[string]interface{}{
			"format": map[string]interface{}{"type": "text"},
		}
	}

	// Add temperature for supported models
	modelLower := strings.ToLower(modelName)
	supportsTemperature := (strings.Contains(modelLower, "gpt-5") && !strings.Contains(modelLower, "gpt-5-nano")) ||
		strings.Contains(modelLower, "gpt-4.1") ||
		strings.Contains(modelLower, "gpt-4") ||
		strings.Contains(modelLower, "gpt-3.5") ||
		strings.Contains(modelLower, "text-davinci") ||
		strings.Contains(modelLower, "gpt-4o")

	if supportsTemperature && !strings.Contains(modelLower, "o1") &&
		!strings.Contains(modelLower, "o3") && !strings.Contains(modelLower, "o4") {
		// Only include temperature if the caller actually set a non-default value
		if temperature > 0 {
			payload["temperature"] = temperature
		}
	}

	// Helper to send a request with the given payload
	send := func(pl map[string]interface{}) (*http.Response, []byte, error) {
		body, _ := json.Marshal(pl)
		req, err := http.NewRequestWithContext(ctx, "POST", openAIResponsesURL, bytes.NewBuffer(body))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create responses request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: p.timeout}
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		raw, _ := io.ReadAll(resp.Body)
		if p.debug {
			dur := time.Since(start)
			fmt.Printf("[LLM] OpenAI Responses %s in %v (status %s, bytes %d)\n", modelName, dur, resp.Status, len(raw))
		}
		return resp, raw, nil
	}

	// Enable SSE streaming in debug mode to surface partial progress
	enableStream := p.debug
	if enableStream {
		payload["stream"] = true
		if p.debug {
			fmt.Printf("[LLM] Sending STREAM request to OpenAI: model=%s schema=%t max_tokens=%v temp=%v timeout=%s\n",
				modelName, useJsonFormat && schema != nil, payload["max_output_tokens"], payload["temperature"], p.timeout)
		}
		// Build and send request for streaming
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, "POST", openAIResponsesURL, bytes.NewBuffer(body))
		if err != nil {
			return "", fmt.Errorf("failed to create responses request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		client := &http.Client{Timeout: p.timeout}
		resp, err := client.Do(req)
		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout exceeded") {
				return "", fmt.Errorf("OpenAI API request timed out after %v. This may be due to a complex request or network issues. Try with a smaller document or check your connection", p.timeout)
			}
			return "", fmt.Errorf("failed to call responses (stream): %w", err)
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
		}()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("OpenAI API error (%s): %s", resp.Status, strings.TrimSpace(string(b)))
		}

		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/event-stream") {
			// Fallback: not streaming; read whole body and continue below
			raw, _ := io.ReadAll(resp.Body)
			// Simulate non-stream parsing path
			// Parse generic as below
			var generic map[string]interface{}
			if err := json.Unmarshal(raw, &generic); err == nil {
				if ot, ok := generic["output_text"].(string); ok && strings.TrimSpace(ot) != "" {
					return ot, nil
				}
			}
			// As last resort return raw as string
			return string(raw), nil
		}

		// Stream parse SSE events; accumulate output_text deltas
		scanner := bufio.NewScanner(resp.Body)
		// allow large frames
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 10*1024*1024)
		var builder strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" || data == "{}" || data == "null" {
				continue
			}
			var ev map[string]interface{}
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				// ignore non-JSON keepalive lines
				continue
			}
			if t, ok := ev["type"].(string); ok {
				switch t {
				case "response.output_text.delta":
					if d, ok := ev["delta"].(string); ok && d != "" {
						builder.WriteString(d)
						// Stream to stderr for visibility
						fmt.Fprint(os.Stderr, d)
					}
				case "response.completed":
					// done event, break loop
					// Add newline after stream for readability
					fmt.Fprintln(os.Stderr)
					return builder.String(), nil
				case "response.error":
					// Attempt to surface message
					if em, ok := ev["error"].(map[string]interface{}); ok {
						if msg, ok := em["message"].(string); ok {
							return "", fmt.Errorf("OpenAI stream error: %s", msg)
						}
					}
					return "", fmt.Errorf("OpenAI stream error")
				}
			}
		}
		// End of stream without explicit completed; return accumulated
		fmt.Fprintln(os.Stderr)
		return builder.String(), nil
	}

	// Non-stream fallback path
	if p.debug {
		fmt.Printf("[LLM] Sending request to OpenAI: model=%s schema=%t max_tokens=%v temp=%v timeout=%s\n",
			modelName, useJsonFormat && schema != nil, payload["max_output_tokens"], payload["temperature"], p.timeout)
	}
	resp, raw, err := send(payload)
	if err != nil {
		// Check for timeout errors and attempt one fallback retry with fewer tokens
		if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout exceeded") {
			// Reduce the max tokens and retry once to encourage faster completion
			fallbackTokens := 1024
			if maxTokens > 0 {
				fallbackTokens = maxTokens / 2
				if fallbackTokens < 256 {
					fallbackTokens = 256
				}
			}
			payload["max_output_tokens"] = fallbackTokens

			resp, raw, err = send(payload)
			if err != nil {
				if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout exceeded") {
					return "", fmt.Errorf("OpenAI API request timed out after %v even after reducing tokens to %d. Try with a smaller document or lower llm.maxOutputTokens", p.timeout, fallbackTokens)
				}
				return "", fmt.Errorf("failed to call responses (retry with %d tokens): %w", fallbackTokens, err)
			}
		} else {
			return "", fmt.Errorf("failed to call responses: %w", err)
		}
	}
	// Retry once without temperature if model rejects it
	if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(raw), "Unsupported parameter: 'temperature'") {
		// Remove temperature and retry once
		delete(payload, "temperature")
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		resp, raw, err = send(payload)
		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout exceeded") {
				return "", fmt.Errorf("OpenAI API request timed out after %v. This may be due to a complex request or network issues. Try with a smaller document or check your connection", p.timeout)
			}
			return "", fmt.Errorf("failed to call responses (retry without temperature): %w", err)
		}
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				fmt.Printf("Warning: failed to close response body: %v\n", err)
			}
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error (%s): %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	// Parse OpenAI Responses API format
	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err == nil {
		// 1) Try aggregated output_text first if present
		if ot, ok := generic["output_text"].(string); ok && strings.TrimSpace(ot) != "" {
			return ot, nil
		}

		// 2) Try the "output" array format
		if outputs, ok := generic["output"].([]interface{}); ok && len(outputs) > 0 {
			for _, output := range outputs {
				if outputObj, ok := output.(map[string]interface{}); ok {
					// Look for text type output
					if outputType, ok := outputObj["type"].(string); ok && outputType == "text" {
						if txt, ok := outputObj["text"].(string); ok && txt != "" {
							return txt, nil
						}
					}
					// Also check for message type outputs
					if outputType, ok := outputObj["type"].(string); ok && outputType == "message" {
						if contents, ok := outputObj["content"].([]interface{}); ok && len(contents) > 0 {
							if c0, ok := contents[0].(map[string]interface{}); ok {
								if txt, ok := c0["text"].(string); ok && txt != "" {
									return txt, nil
								}
							}
						}
					}
				}
			}
		}

		// 3) If still nothing and the response is incomplete due to token limit, retry once without schema in text mode
		if status, ok := generic["status"].(string); ok && status == "incomplete" {
			if details, ok := generic["incomplete_details"].(map[string]interface{}); ok {
				if reason, ok := details["reason"].(string); ok && reason == "max_output_tokens" {
					// Retry with text format and without explicit token cap
					if useJsonFormat {
						return p.callResponsesAPIWithSchema(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, 0, "", nil, false)
					}
				}
			}
		}

		// Fallback: try choices format (for backwards compatibility)
		var cc OpenAIResponsePayload
		if err := json.Unmarshal(raw, &cc); err == nil && len(cc.Choices) > 0 && cc.Choices[0].Message.Content != "" {
			return cc.Choices[0].Message.Content, nil
		}

		// Try direct content access
		if txt, ok := generic["content"].(string); ok && txt != "" {
			return txt, nil
		}

		// No recognizable content fields found; fallthrough to detailed raw error
	}
	return "", fmt.Errorf("failed to extract content from responses body. Raw response: %s", string(raw))
}
