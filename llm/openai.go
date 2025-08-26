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

// openAIEstimationData is used to unmarshal the JSON object returned by OpenAI
// for the estimation call.
type openAIEstimationData struct {
	EstimatedTaskCount  int    `json:"estimatedTaskCount"`
	EstimatedComplexity string `json:"estimatedComplexity"` // e.g., "low", "medium", "high"
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

	userMessage := fmt.Sprintf("PRD Content:\n---\n%s\n---", prdContent)

	content, err := p.callOpenAIAndExtract(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
	if err != nil {
		return nil, err
	}

	var responseWrapper OpenAITaskResponseWrapper
	if err := json.Unmarshal([]byte(content), &responseWrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks JSON from OpenAI response content: %w", err)
	}

	return responseWrapper.Tasks, nil
	// return nil, fmt.Errorf("OpenAI GenerateTasks not yet fully implemented")
}

// EstimateTaskParameters for OpenAIProvider.
func (p *OpenAIProvider) EstimateTaskParameters(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForEstimation int, temperatureForEstimation float64) (types.EstimationOutput, error) {
	if apiKey == "" {
		apiKey = p.apiKey // Use provider's key if per-call key is not given
	}
	if apiKey == "" {
		return types.EstimationOutput{}, fmt.Errorf("OpenAI API key is not set for estimation")
	}

	userMessage := fmt.Sprintf("PRD Content:\n---\n%s\n---", prdContent)

	content, err := p.callOpenAIAndExtract(ctx, apiKey, modelName, systemPrompt, userMessage, temperatureForEstimation, maxTokensForEstimation)
	if err != nil {
		return types.EstimationOutput{}, err
	}

	var estimationData openAIEstimationData
	if err := json.Unmarshal([]byte(content), &estimationData); err != nil {
		return types.EstimationOutput{}, fmt.Errorf("failed to unmarshal estimation JSON from OpenAI response content: %w. Content was: [%s]", err, content)
	}

	return types.EstimationOutput{
		EstimatedTaskCount:  estimationData.EstimatedTaskCount,
		EstimatedComplexity: estimationData.EstimatedComplexity,
	}, nil
}

// ImprovePRD sends the PRD content to OpenAI with a prompt to refine and improve it.
func (p *OpenAIProvider) ImprovePRD(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokensForImprovement int, temperatureForImprovement float64) (string, error) {
	if apiKey == "" {
		apiKey = p.apiKey
	}
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key is not set for PRD improvement")
	}

	userMessage := fmt.Sprintf("Please improve the following PRD content:\n---\n%s\n---", prdContent)

	content, err := p.callOpenAIAndExtract(ctx, apiKey, modelName, systemPrompt, userMessage, temperatureForImprovement, maxTokensForImprovement)
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

	content, err := p.callOpenAIAndExtract(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
	if err != nil {
		return types.EnhancedTask{}, err
	}

	var enhancedTask types.EnhancedTask
	if err := json.Unmarshal([]byte(content), &enhancedTask); err != nil {
		return types.EnhancedTask{}, fmt.Errorf("failed to unmarshal enhanced task JSON from OpenAI response: %w. Content was: [%s]", err, content)
	}

	return enhancedTask, nil
}

// callOpenAIAndExtract tries Responses API first, then Chat Completions, and extracts the text content.
func (p *OpenAIProvider) callOpenAIAndExtract(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	// Only use the Responses API per project preference
	return p.callResponsesAPI(ctx, apiKey, modelName, systemPrompt, userMessage, temperature, maxTokens)
}

func (p *OpenAIProvider) callResponsesAPI(ctx context.Context, apiKey, modelName, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	payload := map[string]interface{}{
		"model":             modelName,
		"input":             []map[string]string{{"role": "system", "content": systemPrompt}, {"role": "user", "content": userMessage}},
		"max_output_tokens": maxTokens,
		// Use text.format for Responses API JSON output with correct type
		"text": map[string]interface{}{"format": map[string]interface{}{"type": "json_object"}},
	}

	// Some models don't support temperature parameter - exclude it for safer compatibility
	// Skip temperature for o1 models and any unknown/unsupported models
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
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
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

	// Try choices-like first
	var cc OpenAIResponsePayload
	if err := json.Unmarshal(raw, &cc); err == nil && len(cc.Choices) > 0 && cc.Choices[0].Message.Content != "" {
		return cc.Choices[0].Message.Content, nil
	}
	// Parse OpenAI Responses API format
	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err == nil {
		if out, ok := generic["output"].([]interface{}); ok && len(out) > 0 {
			// Look for message type outputs with content
			for _, output := range out {
				if outputObj, ok := output.(map[string]interface{}); ok {
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
		if txt, ok := generic["content"].(string); ok && txt != "" {
			return txt, nil
		}
	}
	return "", fmt.Errorf("failed to extract content from responses body")
}
