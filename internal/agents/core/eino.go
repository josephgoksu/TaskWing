/*
Package core provides shared functionality for Eino chains.
*/
package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	// MaxRetries is the maximum number of retry attempts for failed LLM parses
	MaxRetries = 2
	// RetryBaseDelay is the base delay between retries (doubles each attempt)
	RetryBaseDelay = 500 * time.Millisecond
)

// DeterministicChain is a reusable pipeline: Map -> Template -> Model -> Parser -> Output
type DeterministicChain[T any] struct {
	chain compose.Runnable[map[string]any, T]
	name  string
}

// NewDeterministicChain creates a standardized Eino chain for deterministic tasks.
func NewDeterministicChain[T any](
	ctx context.Context,
	name string,
	chatModel model.BaseChatModel,
	templateStr string,
) (*DeterministicChain[T], error) {

	// 1. Template Node (Custom Lambda)
	tmpl, err := template.New(name).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	templateFunc := func(ctx context.Context, input map[string]any) ([]*schema.Message, error) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, input); err != nil {
			return nil, fmt.Errorf("execute template: %w", err)
		}
		return []*schema.Message{
			{Role: schema.User, Content: buf.String()},
		}, nil
	}

	// 2. Model Node (Lambda Adapter)
	// We wrap BaseChatModel in a lambda to accept models that don't support tools (BindTools)
	modelFunc := func(ctx context.Context, input []*schema.Message) (*schema.Message, error) {
		return chatModel.Generate(ctx, input)
	}

	// 3. Parser function (wrapping our generic ParseJSONResponse)
	parserFunc := func(ctx context.Context, output *schema.Message) (T, error) {
		return ParseJSONResponse[T](output.Content)
	}

	// 4. Chain Construction using Graph
	graph := compose.NewGraph[map[string]any, T]()

	_ = graph.AddLambdaNode("prompt", compose.InvokableLambda(templateFunc))
	_ = graph.AddLambdaNode("model", compose.InvokableLambda(modelFunc))
	_ = graph.AddLambdaNode("parser", compose.InvokableLambda(parserFunc))

	_ = graph.AddEdge(compose.START, "prompt")
	_ = graph.AddEdge("prompt", "model")
	_ = graph.AddEdge("model", "parser")
	_ = graph.AddEdge("parser", compose.END)

	compiledChain, err := graph.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("compile chain: %w", err)
	}

	return &DeterministicChain[T]{
		chain: compiledChain,
		name:  name,
	}, nil
}

// Invoke executes the chain with manual timing and retry logic for JSON parse failures.
func (c *DeterministicChain[T]) Invoke(ctx context.Context, input map[string]any) (T, string, time.Duration, error) {
	start := time.Now()

	var output T
	var err error
	var lastErr error

	// Retry loop for handling transient LLM parse failures
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1000ms, etc.
			delay := RetryBaseDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return output, "", time.Since(start), ctx.Err()
			case <-time.After(delay):
			}
		}

		output, err = c.chain.Invoke(ctx, input)
		if err == nil {
			duration := time.Since(start)
			return output, "", duration, nil
		}

		// Always capture the last error for reporting
		lastErr = err

		// Check if error is retryable (JSON parse, rate limit, network)
		if isRetryableError(err) {
			continue // Retry
		}

		// Non-retryable error, return immediately
		duration := time.Since(start)
		return output, "", duration, err
	}

	// All retries exhausted
	duration := time.Since(start)
	return output, "", duration, fmt.Errorf("failed after %d attempts: %w", MaxRetries+1, lastErr)
}

// isRetryableError checks if the error should trigger a retry.
// Retries: JSON parse failures and rate limit errors.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())

	// JSON parse errors
	if strings.Contains(errStr, "parse json") ||
		strings.Contains(errStr, "unmarshal") ||
		strings.Contains(errStr, "invalid character") ||
		strings.Contains(errStr, "no json found") ||
		strings.Contains(errStr, "no json start") {
		return true
	}

	// Rate limit errors (various providers)
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "quota exceeded") ||
		strings.Contains(errStr, "resource exhausted") {
		return true
	}

	// Temporary network errors
	if strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection reset") {
		return true
	}

	return false
}
