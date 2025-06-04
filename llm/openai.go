package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	// TODO: Add other parameters like TopP, N, Stream if needed
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
	Tasks []TaskOutput `json:"tasks"`
}

const openAIAPIURL = "https://api.openai.com/v1/chat/completions"

// GenerateTasks for OpenAIProvider.
// TODO: Implement the actual API call and error handling.
func (p *OpenAIProvider) GenerateTasks(prdContent string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]TaskOutput, error) {
	if apiKey == "" {
		apiKey = p.apiKey // Use provider's key if per-call key is not given
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is not set")
	}

	// 1. Construct the prompt
	// This is a simplified version; the actual prompt will be more detailed as per Phase 3 plan.
	systemPrompt := `You are an expert project manager. Analyze the following Product Requirements Document (PRD) and extract all actionable tasks and subtasks. For each task and subtask, provide:
1. title: A concise title.
2. description: A detailed description if available, otherwise use the title.
3. priority: Infer a priority (low, medium, high, urgent) based on keywords and context. Default to medium if unsure.
4. subtasks: A list of any subtasks that directly belong to this task. Each subtask should have the same fields (title, description, priority, subtasks, dependsOnTitles).
5. dependsOnTitles: A list of titles of other tasks defined in THIS PRD that this task directly depends on. Only list titles found within this document.

Return your response as a single JSON object. This object must have a key named "tasks", and the value of "tasks" must be an array of task objects. Ensure the overall response is a valid JSON object. Ensure the description field for each task is always populated, even if it's just a copy of the title.
Example of the expected JSON object structure:
{
  "tasks": [
    {
      "title": "User Login Feature",
      "description": "Implement email/password login.",
      "priority": "high",
      "subtasks": [
        {"title": "Login UI", "description": "Design UI mockups for login.", "priority": "medium", "subtasks": [], "dependsOnTitles": []}
      ],
      "dependsOnTitles": ["Setup Database"]
    }
  ]
}
`
	userMessage := fmt.Sprintf("PRD Content:\n---\n%s\n---", prdContent)

	requestPayload := OpenAIRequestPayload{
		Model: modelName,
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature:    temperature,
		MaxTokens:      maxTokens,
		ResponseFormat: &OpenAIResponseFormat{Type: "json_object"}, // Request JSON output
	}

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI request payload: %w", err)
	}

	req, err := http.NewRequest("POST", openAIAPIURL, bytes.NewBuffer(payloadBytes))
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
		// TODO: Parse error response body from OpenAI for better error messages
		return nil, fmt.Errorf("OpenAI API request failed with status %s", resp.Status)
	}

	var responsePayload OpenAIResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&responsePayload); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if len(responsePayload.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI response contained no choices")
	}

	// Assuming the actual task list is in the first choice's message content as a JSON string.
	// This needs to be robustly parsed.
	content := responsePayload.Choices[0].Message.Content
	var responseWrapper OpenAITaskResponseWrapper                             // Changed from []TaskOutput
	if err := json.Unmarshal([]byte(content), &responseWrapper); err != nil { // Changed to &responseWrapper
		// The LLM might not always return a perfect JSON array string, especially if the content is complex or it hits limits.
		// It might also return a single JSON object if only one task is found, and the prompt asked for an array.
		// Or it might return the JSON object directly without being wrapped in a string.
		// We need to handle this gracefully. For now, simple unmarshal.
		return nil, fmt.Errorf("failed to unmarshal tasks JSON from OpenAI response content: %w. Content was: %s", err, content)
	}

	return responseWrapper.Tasks, nil // Return the Tasks field from the wrapper
	// return nil, fmt.Errorf("OpenAI GenerateTasks not yet fully implemented")
}
