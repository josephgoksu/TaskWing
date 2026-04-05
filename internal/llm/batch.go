// Package llm - OpenAI Batch API client for 50% cost reduction.
// Uses the go-openai library (already a dependency via Eino) for typed batch operations.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	openai "github.com/meguminnnnnnnnn/go-openai"
)

// BatchRequest represents a single agent's chat completion request for batch submission.
type BatchRequest struct {
	CustomID string         // Identifier to match responses back (e.g., "agent-deps")
	Model    string         // Model to use (e.g., "gpt-4o-mini")
	Messages []BatchMessage // The chat messages
}

// BatchMessage represents a chat message for the batch request body.
type BatchMessage struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// BatchResult represents a parsed response for a single request in the batch.
type BatchResult struct {
	CustomID   string // Matches the request's CustomID
	Content    string // The assistant's response content
	StatusCode int    // HTTP status code (200 = success)
	Error      string // Error message if status != 200
}

// BatchClient wraps the go-openai client for batch operations.
type BatchClient struct {
	client *openai.Client
}

// NewBatchClient creates a new batch client using the go-openai library.
func NewBatchClient(apiKey string, baseURL string) *BatchClient {
	var client *openai.Client
	if baseURL != "" {
		cfg := openai.DefaultConfig(apiKey)
		cfg.BaseURL = baseURL
		client = openai.NewClientWithConfig(cfg)
	} else {
		client = openai.NewClient(apiKey)
	}
	return &BatchClient{client: client}
}

// Submit uploads a JSONL file and creates a batch job.
func (b *BatchClient) Submit(ctx context.Context, requests []BatchRequest) (string, error) {
	if len(requests) == 0 {
		return "", fmt.Errorf("no requests to submit")
	}

	// Build the upload request using the library's typed helpers
	uploadReq := openai.UploadBatchFileRequest{
		FileName: "taskwing_batch.jsonl",
	}

	for _, req := range requests {
		var msgs []openai.ChatCompletionMessage
		for _, m := range req.Messages {
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}

		uploadReq.AddChatCompletion(req.CustomID, openai.ChatCompletionRequest{
			Model:    req.Model,
			Messages: msgs,
		})
	}

	// Upload file + create batch in one call
	resp, err := b.client.CreateBatchWithUploadFile(ctx, openai.CreateBatchWithUploadFileRequest{
		Endpoint:               openai.BatchEndpointChatCompletions,
		CompletionWindow:       "24h",
		UploadBatchFileRequest: uploadReq,
	})
	if err != nil {
		return "", fmt.Errorf("create batch: %w", err)
	}

	return resp.ID, nil
}

// Poll checks the current status of a batch job.
func (b *BatchClient) Poll(ctx context.Context, batchID string) (*openai.Batch, error) {
	resp, err := b.client.RetrieveBatch(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("retrieve batch: %w", err)
	}
	return &resp.Batch, nil
}

// GetResults downloads and parses the output file from a completed batch.
func (b *BatchClient) GetResults(ctx context.Context, outputFileID string) ([]BatchResult, error) {
	rawResp, err := b.client.GetFileContent(ctx, outputFileID)
	if err != nil {
		return nil, fmt.Errorf("get file content: %w", err)
	}
	defer func() { _ = rawResp.Close() }()

	data, err := io.ReadAll(rawResp)
	if err != nil {
		return nil, fmt.Errorf("read file content: %w", err)
	}

	// Parse JSONL - each line is a batch response object
	var results []BatchResult
	decoder := json.NewDecoder(io.NopCloser(io.Reader(nil)))
	_ = decoder // just for type check

	// Manual line-by-line JSONL parsing since json.Decoder.More() doesn't handle newline-delimited JSON well
	lines := splitJSONL(data)
	for _, line := range lines {
		var resp batchResponseLine
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // skip malformed lines
		}

		result := BatchResult{
			CustomID:   resp.CustomID,
			StatusCode: resp.Response.StatusCode,
		}

		if resp.Response.StatusCode == 200 && len(resp.Response.Body.Choices) > 0 {
			result.Content = resp.Response.Body.Choices[0].Message.Content
		} else if resp.Response.Body.Error != nil {
			result.Error = resp.Response.Body.Error.Message
		}

		results = append(results, result)
	}

	return results, nil
}

// WaitForCompletion polls until the batch completes, fails, or context is cancelled.
func (b *BatchClient) WaitForCompletion(ctx context.Context, batchID string, pollInterval time.Duration, progressFn func(*openai.Batch)) ([]BatchResult, error) {
	for {
		batch, err := b.Poll(ctx, batchID)
		if err != nil {
			return nil, err
		}

		if progressFn != nil {
			progressFn(batch)
		}

		switch batch.Status {
		case "completed":
			if batch.OutputFileID == nil || *batch.OutputFileID == "" {
				return nil, fmt.Errorf("batch completed but no output file")
			}
			return b.GetResults(ctx, *batch.OutputFileID)

		case "failed":
			return nil, fmt.Errorf("batch failed (completed: %d, failed: %d)",
				batch.RequestCounts.Completed, batch.RequestCounts.Failed)

		case "expired":
			return nil, fmt.Errorf("batch expired before completion")

		case "cancelled", "cancelling":
			return nil, fmt.Errorf("batch was cancelled")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// Internal types for parsing batch response JSONL

type batchResponseLine struct {
	CustomID string `json:"custom_id"`
	Response struct {
		StatusCode int `json:"status_code"`
		Body       struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
		} `json:"body"`
	} `json:"response"`
}

// splitJSONL splits newline-delimited JSON into individual lines.
func splitJSONL(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := data[start:i]
			if len(line) > 0 {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(data) {
		line := data[start:]
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}
