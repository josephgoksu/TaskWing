/*
Package core provides shared functionality for Eino chains.
*/
package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"text/template"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

const (
	// MaxRetries is the maximum number of retry attempts for transient errors (JSON parse, rate limit).
	// Increased from 2 to 4 to handle timeout errors with exponential backoff.
	MaxRetries = 4
	// RetryBaseDelay is the base delay between retries (used with exponential backoff + jitter)
	RetryBaseDelay = 500 * time.Millisecond
	// RetryMaxDelay caps the backoff delay to prevent excessive waits
	RetryMaxDelay = 30 * time.Second
	// JitterFactor is the fraction of delay to randomize (+/- 50%)
	JitterFactor = 0.5
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

// Invoke executes the chain with manual timing and retry logic for transient failures.
// Retry policy:
// - Timeout errors (context deadline, HTTP timeout): exponential backoff with jitter, up to MaxRetries attempts
// - JSON parse errors: exponential backoff, up to MaxRetries attempts
// - Rate limit errors: exponential backoff with longer initial delay
// - Permanent errors (invalid request, auth): no retry
func (c *DeterministicChain[T]) Invoke(ctx context.Context, input map[string]any) (T, string, time.Duration, error) {
	start := time.Now()

	var output T
	var err error
	var lastErr error

	// Retry loop for handling transient LLM failures
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff with jitter
			delay := calculateBackoffWithJitter(attempt)
			errType := classifyError(lastErr)

			if shouldLogRetryDetails() {
				log.Printf("[eino] chain=%s attempt=%d/%d error_type=%s delay=%v last_error=%v",
					c.name, attempt, MaxRetries+1, errType, delay, lastErr)
			}

			select {
			case <-ctx.Done():
				return output, "", time.Since(start), ctx.Err()
			case <-time.After(delay):
			}
		}

		output, err = c.chain.Invoke(ctx, input)
		if err == nil {
			duration := time.Since(start)
			if attempt > 0 {
				if shouldLogRetryDetails() {
					log.Printf("[eino] chain=%s recovered after %d retries, total_duration=%v",
						c.name, attempt, duration)
				}
			}
			return output, "", duration, nil
		}

		// Always capture the last error for reporting
		lastErr = err

		// Check if error is retryable (timeout, JSON parse, rate limit, network)
		if isRetryableError(err) {
			continue // Retry
		}

		// Non-retryable error, return immediately
		duration := time.Since(start)
		return output, "", duration, err
	}

	// All retries exhausted
	duration := time.Since(start)
	if shouldLogRetryDetails() {
		log.Printf("[eino] chain=%s exhausted all %d retries, total_duration=%v, last_error=%v",
			c.name, MaxRetries+1, duration, lastErr)
	}
	return output, "", duration, fmt.Errorf("failed after %d attempts: %w", MaxRetries+1, lastErr)
}

func shouldLogRetryDetails() bool {
	return viper.GetBool("verbose")
}

// calculateBackoffWithJitter returns exponential backoff delay with jitter.
// Formula: base * 2^(attempt-1) * (1 +/- jitter), capped at RetryMaxDelay
func calculateBackoffWithJitter(attempt int) time.Duration {
	// Exponential: 500ms, 1s, 2s, 4s, ...
	baseDelay := RetryBaseDelay * time.Duration(1<<(attempt-1))

	// Cap at max delay
	if baseDelay > RetryMaxDelay {
		baseDelay = RetryMaxDelay
	}

	// Add jitter: +/- 50% of base delay
	jitter := float64(baseDelay) * JitterFactor * (2*rand.Float64() - 1) // -0.5 to +0.5
	delay := time.Duration(float64(baseDelay) + jitter)

	// Ensure minimum delay of 100ms
	if delay < 100*time.Millisecond {
		delay = 100 * time.Millisecond
	}

	return delay
}

// classifyError returns a human-readable error type for logging.
func classifyError(err error) string {
	if err == nil {
		return "none"
	}
	if isTimeoutError(err) {
		return "timeout"
	}
	if isRateLimitError(err) {
		return "rate_limit"
	}
	if isJSONParseError(err) {
		return "json_parse"
	}
	if isNetworkError(err) {
		return "network"
	}
	return "unknown"
}

// isRetryableError checks if the error should trigger a retry.
// Retries: timeout, JSON parse, rate limit, and transient network errors.
// Does NOT retry: permanent errors (auth failures, invalid requests).
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	return isTimeoutError(err) || isJSONParseError(err) || isRateLimitError(err) || isNetworkError(err)
}

// isTimeoutError checks for context deadline exceeded and HTTP timeout errors.
// These are explicitly detected to enable targeted retry with exponential backoff.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context.DeadlineExceeded (typed check)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// String-based checks for various timeout manifestations
	errStr := strings.ToLower(err.Error())
	timeoutPatterns := []string{
		"context deadline exceeded",
		"client.timeout exceeded",
		"timeout exceeded while awaiting headers",
		"i/o timeout",
		"request timeout",
		"operation timed out",
		"deadline exceeded",
	}
	for _, pattern := range timeoutPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// isJSONParseError checks for JSON parsing failures from LLM responses.
func isJSONParseError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "parse json") ||
		strings.Contains(errStr, "unmarshal") ||
		strings.Contains(errStr, "invalid character") ||
		strings.Contains(errStr, "no json found") ||
		strings.Contains(errStr, "no json start") ||
		strings.Contains(errStr, "unexpected end of json")
}

// isRateLimitError checks for rate limiting from various LLM providers.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "quota exceeded") ||
		strings.Contains(errStr, "resource exhausted")
}

// isNetworkError checks for transient network failures.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "broken pipe")
}
