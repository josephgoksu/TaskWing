package runner

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

const (
	retryBaseDelay = 500 * time.Millisecond
	retryMaxDelay  = 30 * time.Second
	defaultRetries = 4
)

// RetryableInvoke calls runner.Invoke with exponential backoff and jitter.
// It retries on timeout, CLI failure, and JSON parse errors.
// It does not retry on context cancellation.
func RetryableInvoke(ctx context.Context, r Runner, req InvokeRequest, maxRetries int) (*InvokeResult, error) {
	if maxRetries <= 0 {
		maxRetries = defaultRetries
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result, err := r.Invoke(ctx, req)
		if err == nil {
			return result, nil
		}

		if !isRetryable(err) {
			return nil, err
		}

		lastErr = err

		if attempt < maxRetries {
			delay := backoffDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// isRetryable returns true for errors that warrant a retry.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Retry on timeouts, CLI failures, and JSON parse errors
	for _, keyword := range []string{"timeout", "timed out", "JSON", "json", "exit status", "signal"} {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

// backoffDelay returns exponential backoff with jitter.
func backoffDelay(attempt int) time.Duration {
	delay := time.Duration(float64(retryBaseDelay) * math.Pow(2, float64(attempt)))
	if delay > retryMaxDelay {
		delay = retryMaxDelay
	}
	// Add jitter: ±25%
	jitter := time.Duration(float64(delay) * (0.5*rand.Float64() - 0.25))
	return delay + jitter
}
