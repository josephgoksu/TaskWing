package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/types"
)

// OpenAIProvider implements the Provider interface for OpenAI LLMs.
type OpenAIProvider struct {
	apiKey string
	// Potentially add http.Client for custom timeouts, etc.
}

// NewOpenAIProvider creates a new OpenAIProvider.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{apiKey: apiKey}
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
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"tasks": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"title":       map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"acceptanceCriteria": map[string]interface{}{
							"type":        "string",
							"description": "Acceptance criteria for the task (single or newline-separated list)",
						},
						"priority": map[string]interface{}{"type": "string"},
						"tempId":   map[string]interface{}{"type": "integer"},
					},
					"required": []string{"title", "description", "acceptanceCriteria", "priority", "tempId"},
				},
			},
		},
		"required": []string{"tasks"},
		"strict":   true,
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
		"strict":   true,
	}
}

// parseEnhancedTaskFallback makes a best-effort to build an EnhancedTask from free-form text.
func parseEnhancedTaskFallback(text, defaultTitle string) types.EnhancedTask {
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
		// Debug: show the actual content that failed to unmarshal
		fmt.Printf("DEBUG: Failed to unmarshal JSON content: %s\n", content)
		return nil, fmt.Errorf("failed to unmarshal tasks JSON from OpenAI response content: %w", err)
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
		return types.EnhancedTask{}, err
	}

	var enhancedTask types.EnhancedTask
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
		return coerced, nil
	}

	return enhancedTask, nil
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

func (p *OpenAIProvider) callResponsesAPIWithSchema(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int, schemaName string, schema map[string]interface{}, useJsonFormat bool) (string, error) {
	// Build the Responses API payload using simple string content
	payload := map[string]interface{}{
		"model": modelName,
		"input": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
	}

	// Only set max_output_tokens if explicitly provided (>0)
	if maxTokens > 0 {
		payload["max_output_tokens"] = maxTokens
	}

	// Configure text.format per the Responses API
	textConfig := map[string]interface{}{}
	if useJsonFormat && schema != nil {
		textConfig["format"] = map[string]interface{}{
			"type":   "json_schema",
			"name":   schemaName,
			"schema": schema,
		}
	} else {
		textConfig["format"] = map[string]interface{}{"type": "text"}
	}
	payload["text"] = textConfig

	// Add temperature for supported models
	modelLower := strings.ToLower(modelName)
	supportsTemperature := strings.Contains(modelLower, "gpt-4") ||
		strings.Contains(modelLower, "gpt-3.5") ||
		strings.Contains(modelLower, "text-davinci") ||
		strings.Contains(modelLower, "gpt-4o")

	if supportsTemperature && !strings.Contains(modelLower, "o1") {
		payload["temperature"] = temperature
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", openAIResponsesURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create responses request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	// Use longer timeout for complex document processing
	timeout := 180 * time.Second // 3 minutes
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		// Check for timeout errors and provide helpful message
		if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout exceeded") {
			return "", fmt.Errorf("OpenAI API request timed out after %v. This may be due to a complex request or network issues. Try with a smaller document or check your connection", timeout)
		}
		return "", fmt.Errorf("failed to call responses: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the request for this
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("responses status %s: %s", resp.Status, string(b))
	}
	raw, _ := io.ReadAll(resp.Body)

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

		// Debug log the raw response structure for troubleshooting
		fmt.Printf("Debug: Raw API response structure: %+v\n", generic)
	}
	return "", fmt.Errorf("failed to extract content from responses body. Raw response: %s", string(raw))
}
