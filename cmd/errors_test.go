package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestPrintError(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr

	tests := []struct {
		name         string
		userMsg      string
		technicalErr error
		verbose      bool
		expectedOut  string
	}{
		{
			name:         "normal mode with error",
			userMsg:      "User friendly message",
			technicalErr: nil,
			verbose:      false,
			expectedOut:  "User friendly message",
		},
		{
			name:         "verbose mode with error",
			userMsg:      "User friendly message",
			technicalErr: &testError{msg: "technical details"},
			verbose:      true,
			expectedOut:  "Error: technical details",
		},
		{
			name:         "normal mode with technical error",
			userMsg:      "User friendly message",
			technicalErr: &testError{msg: "technical details"},
			verbose:      false,
			expectedOut:  "User friendly message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set verbose flag
			viper.Set("verbose", tt.verbose)
			defer viper.Set("verbose", false)

			// Capture stderr output
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Call function
			PrintError(tt.userMsg, tt.technicalErr)

			// Close writer and read output
			_ = w.Close()
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := strings.TrimSpace(buf.String())

			// Restore stderr
			os.Stderr = originalStderr

			// Check output
			if !strings.Contains(output, tt.expectedOut) {
				t.Errorf("PrintError() output = %q, want to contain %q", output, tt.expectedOut)
			}
		})
	}
}

func TestLogError(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr

	tests := []struct {
		name        string
		msg         string
		err         error
		verbose     bool
		shouldPrint bool
	}{
		{
			name:        "verbose mode with error",
			msg:         "Debug message",
			err:         &testError{msg: "error details"},
			verbose:     true,
			shouldPrint: true,
		},
		{
			name:        "verbose mode without error",
			msg:         "Debug message",
			err:         nil,
			verbose:     true,
			shouldPrint: true,
		},
		{
			name:        "non-verbose mode",
			msg:         "Debug message",
			err:         &testError{msg: "error details"},
			verbose:     false,
			shouldPrint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set verbose flag
			viper.Set("verbose", tt.verbose)
			defer viper.Set("verbose", false)

			// Capture stderr output
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Call function
			LogError(tt.msg, tt.err)

			// Close writer and read output
			_ = w.Close()
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := strings.TrimSpace(buf.String())

			// Restore stderr
			os.Stderr = originalStderr

			// Check output
			if tt.shouldPrint && !strings.Contains(output, "[DEBUG]") {
				t.Errorf("LogError() should have printed debug output")
			}
			if !tt.shouldPrint && output != "" {
				t.Errorf("LogError() should not have printed anything, got: %q", output)
			}
		})
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
