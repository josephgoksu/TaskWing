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

	"github.com/josephgoksu/taskwing.app/types"
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

const openAIAPIURL = "https://api.openai.com/v1/chat/completions"

// GenerateTasks for OpenAIProvider.
// TODO: Implement the actual API call and error handling.
func (p *OpenAIProvider) GenerateTasks(ctx context.Context, systemPrompt, prdContent string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]types.TaskOutput, error) {
	if apiKey == "" {
		apiKey = p.apiKey // Use provider's key if per-call key is not given
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is not set")
	}

	userMessage := fmt.Sprintf("PRD Content:\n---\n%s\n---", prdContent)

	requestPayload := OpenAIRequestPayload{
		Model: modelName,
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		ResponseFormat: &OpenAIResponseFormat{Type: "json_object"},
	}

	// Use standard parameters for all models
	requestPayload.MaxTokens = maxTokens
	requestPayload.Temperature = temperature

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openAIAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second} // Increased timeout
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to OpenAI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the body for more detailed error information
		errorBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			// If reading the body fails, return the original status error
			return nil, fmt.Errorf("OpenAI API request failed with status %s (and failed to read error body: %v)", resp.Status, readErr)
		}
		// Return the status error along with the body content
		return nil, fmt.Errorf("OpenAI API request failed with status %s: %s", resp.Status, string(errorBodyBytes))
	}

	var responsePayload OpenAIResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&responsePayload); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if len(responsePayload.Choices) == 0 {
		// It's good practice to log the full response if choices are empty unexpectedly.
		fullResponseBytes, _ := json.Marshal(responsePayload) // Best effort marshal
		fmt.Printf("OpenAI response had no choices. Full response: %s\n", string(fullResponseBytes))
		return nil, fmt.Errorf("OpenAI response contained no choices")
	}

	// Log the first choice and usage for debugging before accessing its content.
	fmt.Printf("DEBUG OpenAI Response - First Choice: %+v\n", responsePayload.Choices[0])
	fmt.Printf("DEBUG OpenAI Response - Usage: %+v\n", responsePayload.Usage)

	content := responsePayload.Choices[0].Message.Content
	// Also log the raw content string that will be unmarshalled
	fmt.Printf("DEBUG OpenAI Raw Content to Unmarshal: [%s]\n", content)

	var responseWrapper OpenAITaskResponseWrapper
	if err := json.Unmarshal([]byte(content), &responseWrapper); err != nil {
		// The error message already includes the content, but explicit logging before erroring can be useful.
		// fmt.Printf("Problematic OpenAI content before error: [%s]\n", content) // This would be redundant given the error format below
		return nil, fmt.Errorf("failed to unmarshal tasks JSON from OpenAI response content: %w. Content was: [%s]", err, content)
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

	requestPayload := OpenAIRequestPayload{
		Model: modelName,
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		ResponseFormat: &OpenAIResponseFormat{Type: "json_object"},
	}

	// Use standard parameters for all models
	requestPayload.MaxTokens = maxTokensForEstimation
	requestPayload.Temperature = temperatureForEstimation

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return types.EstimationOutput{}, fmt.Errorf("failed to marshal OpenAI estimation request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openAIAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return types.EstimationOutput{}, fmt.Errorf("failed to create OpenAI estimation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second} // Shorter timeout for estimation
	resp, err := client.Do(req)
	if err != nil {
		return types.EstimationOutput{}, fmt.Errorf("failed to send estimation request to OpenAI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return types.EstimationOutput{}, fmt.Errorf("OpenAI estimation API request failed with status %s (and failed to read error body: %v)", resp.Status, readErr)
		}
		return types.EstimationOutput{}, fmt.Errorf("OpenAI estimation API request failed with status %s: %s", resp.Status, string(errorBodyBytes))
	}

	var responsePayload OpenAIResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&responsePayload); err != nil {
		return types.EstimationOutput{}, fmt.Errorf("failed to decode OpenAI estimation response: %w", err)
	}

	if len(responsePayload.Choices) == 0 {
		fullResponseBytes, _ := json.Marshal(responsePayload)
		fmt.Printf("OpenAI estimation response had no choices. Full response: %s\n", string(fullResponseBytes))
		return types.EstimationOutput{}, fmt.Errorf("OpenAI estimation response contained no choices")
	}

	fmt.Printf("DEBUG OpenAI Estimation Response - First Choice: %+v\n", responsePayload.Choices[0])
	fmt.Printf("DEBUG OpenAI Estimation Response - Usage: %+v\n", responsePayload.Usage)

	content := responsePayload.Choices[0].Message.Content
	fmt.Printf("DEBUG OpenAI Estimation Raw Content to Unmarshal: [%s]\n", content)

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

	requestPayload := OpenAIRequestPayload{
		Model: modelName, // GPT-5 Mini is the default model
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}

	// Use standard parameters for all models
	requestPayload.MaxTokens = maxTokensForImprovement
	requestPayload.Temperature = temperatureForImprovement

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI PRD improvement request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openAIAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create OpenAI PRD improvement request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second} // Longer timeout for potentially large rewrites
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send PRD improvement request to OpenAI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("OpenAI PRD improvement API request failed with status %s (and failed to read error body: %v)", resp.Status, readErr)
		}
		return "", fmt.Errorf("OpenAI PRD improvement API request failed with status %s: %s", resp.Status, string(errorBodyBytes))
	}

	var responsePayload OpenAIResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&responsePayload); err != nil {
		return "", fmt.Errorf("failed to decode OpenAI PRD improvement response: %w", err)
	}

	if len(responsePayload.Choices) == 0 {
		return "", fmt.Errorf("OpenAI PRD improvement response contained no choices")
	}

	improvedContent := responsePayload.Choices[0].Message.Content
	return strings.TrimSpace(improvedContent), nil
}
