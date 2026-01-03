package core

import (
	"errors"
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		// Nil error
		{
			name:      "nil error",
			err:       nil,
			wantRetry: false,
		},

		// JSON parse errors
		{
			name:      "parse json error",
			err:       errors.New("failed to parse JSON response"),
			wantRetry: true,
		},
		{
			name:      "unmarshal error",
			err:       errors.New("json: cannot unmarshal string into Go value"),
			wantRetry: true,
		},
		{
			name:      "invalid character error",
			err:       errors.New("invalid character 'x' looking for beginning of value"),
			wantRetry: true,
		},
		{
			name:      "no json found error",
			err:       errors.New("no JSON found in response"),
			wantRetry: true,
		},
		{
			name:      "no json start error",
			err:       errors.New("no JSON start ({ or [) found"),
			wantRetry: true,
		},

		// Rate limit errors - OpenAI format
		{
			name:      "openai rate limit",
			err:       errors.New("Rate limit exceeded. Please retry after 20s"),
			wantRetry: true,
		},
		{
			name:      "http 429 error",
			err:       errors.New("HTTP 429: Too Many Requests"),
			wantRetry: true,
		},
		{
			name:      "too many requests",
			err:       errors.New("Error: too many requests, please slow down"),
			wantRetry: true,
		},

		// Rate limit errors - Anthropic format
		{
			name:      "anthropic rate limit",
			err:       errors.New("rate_limit_error: Number of request tokens has exceeded your per-minute rate limit"),
			wantRetry: true,
		},

		// Rate limit errors - Google/Gemini format
		{
			name:      "gemini quota exceeded",
			err:       errors.New("quota exceeded for aiplatform.googleapis.com"),
			wantRetry: true,
		},
		{
			name:      "google resource exhausted",
			err:       errors.New("RESOURCE_EXHAUSTED: Quota exceeded"),
			wantRetry: true,
		},

		// Network errors
		{
			name:      "timeout error",
			err:       errors.New("context deadline exceeded (Client.Timeout exceeded)"),
			wantRetry: true,
		},
		{
			name:      "connection reset",
			err:       errors.New("read tcp: connection reset by peer"),
			wantRetry: true,
		},
		{
			name:      "temporary network error",
			err:       errors.New("temporary failure in name resolution"),
			wantRetry: true,
		},

		// Non-retryable errors
		{
			name:      "authentication error",
			err:       errors.New("invalid API key provided"),
			wantRetry: false,
		},
		{
			name:      "permission denied",
			err:       errors.New("permission denied: you do not have access to this model"),
			wantRetry: false,
		},
		{
			name:      "model not found",
			err:       errors.New("model 'gpt-5' does not exist"),
			wantRetry: false,
		},
		{
			name:      "content policy violation",
			err:       errors.New("content policy violation detected"),
			wantRetry: false,
		},
		{
			name:      "generic error",
			err:       errors.New("something went wrong"),
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.wantRetry {
				t.Errorf("isRetryableError(%q) = %v, want %v", tt.err, got, tt.wantRetry)
			}
		})
	}
}

func TestIsRetryableError_CaseInsensitive(t *testing.T) {
	// Verify that error matching is case-insensitive
	tests := []struct {
		err       error
		wantRetry bool
	}{
		{errors.New("RATE LIMIT EXCEEDED"), true},
		{errors.New("Rate Limit Exceeded"), true},
		{errors.New("rate limit exceeded"), true},
		{errors.New("TIMEOUT"), true},
		{errors.New("Timeout"), true},
		{errors.New("UNMARSHAL ERROR"), true},
		{errors.New("Unmarshal Error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.wantRetry {
				t.Errorf("isRetryableError(%q) = %v, want %v (case insensitive)", tt.err, got, tt.wantRetry)
			}
		})
	}
}

func TestRetryConstants(t *testing.T) {
	// Verify retry constants are reasonable
	if MaxRetries < 1 || MaxRetries > 5 {
		t.Errorf("MaxRetries = %d, want between 1 and 5", MaxRetries)
	}

	if RetryBaseDelay < 100*1e6 || RetryBaseDelay > 5000*1e6 { // 100ms to 5s in nanoseconds
		t.Errorf("RetryBaseDelay = %v, want between 100ms and 5s", RetryBaseDelay)
	}
}
