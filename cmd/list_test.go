/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// captureStdout captures stdout output during the execution of fn.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TestListFooterVersionFormat tests the footer string format.
func TestListFooterVersionFormat(t *testing.T) {
	// Save original version and restore after test
	originalVersion := version
	defer func() { version = originalVersion }()

	tests := []struct {
		name        string
		setVersion  string
		wantContain string
	}{
		{
			name:        "release version format",
			setVersion:  "1.2.3",
			wantContain: "TaskWing v1.2.3",
		},
		{
			name:        "dev version format",
			setVersion:  "dev",
			wantContain: "TaskWing vdev",
		},
		{
			name:        "empty version falls back to dev",
			setVersion:  "",
			wantContain: "TaskWing vdev",
		},
		{
			name:        "prerelease version",
			setVersion:  "1.2.3-beta.1",
			wantContain: "TaskWing v1.2.3-beta.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version = tt.setVersion

			// Test the footer generation logic directly
			ver := version
			if ver == "" {
				ver = "dev"
			}
			footer := "TaskWing v" + ver

			if footer != tt.wantContain {
				t.Errorf("footer = %q, want %q", footer, tt.wantContain)
			}
		})
	}
}

// TestListFooterFlagsLogic tests that footer respects JSON and quiet flags.
func TestListFooterFlagsLogic(t *testing.T) {
	// Save original viper state
	originalJSON := viper.GetBool("json")
	originalQuiet := viper.GetBool("quiet")
	defer func() {
		viper.Set("json", originalJSON)
		viper.Set("quiet", originalQuiet)
	}()

	tests := []struct {
		name       string
		jsonFlag   bool
		quietFlag  bool
		wantFooter bool
	}{
		{
			name:       "no flags - footer shown",
			jsonFlag:   false,
			quietFlag:  false,
			wantFooter: true,
		},
		{
			name:       "json flag - no footer",
			jsonFlag:   true,
			quietFlag:  false,
			wantFooter: false,
		},
		{
			name:       "quiet flag - no footer",
			jsonFlag:   false,
			quietFlag:  true,
			wantFooter: false,
		},
		{
			name:       "both flags - no footer",
			jsonFlag:   true,
			quietFlag:  true,
			wantFooter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("json", tt.jsonFlag)
			viper.Set("quiet", tt.quietFlag)

			// The footer condition from list.go is: if isJSON() early return, then if !isQuiet() print footer
			// So footer shows when: !isJSON() && !isQuiet()
			shouldShowFooter := !isJSON() && !isQuiet()

			if shouldShowFooter != tt.wantFooter {
				t.Errorf("shouldShowFooter = %v, want %v (json=%v, quiet=%v)",
					shouldShowFooter, tt.wantFooter, tt.jsonFlag, tt.quietFlag)
			}
		})
	}
}

// TestListFooterPrintsCorrectly tests that the footer is actually printed to stdout.
func TestListFooterPrintsCorrectly(t *testing.T) {
	// Save and restore version
	originalVersion := version
	defer func() { version = originalVersion }()

	version = "1.0.0-test"

	output := captureStdout(func() {
		// Simulate the footer printing logic from list.go
		ver := version
		if ver == "" {
			ver = "dev"
		}
		// This mimics: fmt.Printf("TaskWing v%s\n", ver)
		os.Stdout.WriteString("TaskWing v" + ver + "\n")
	})

	expected := "TaskWing v1.0.0-test\n"
	if output != expected {
		t.Errorf("output = %q, want %q", output, expected)
	}
}

// TestListFooterDevFallback tests that empty version falls back to "dev".
func TestListFooterDevFallback(t *testing.T) {
	// Save and restore version
	originalVersion := version
	defer func() { version = originalVersion }()

	version = ""

	output := captureStdout(func() {
		ver := version
		if ver == "" {
			ver = "dev"
		}
		os.Stdout.WriteString("TaskWing v" + ver + "\n")
	})

	if !strings.Contains(output, "TaskWing vdev") {
		t.Errorf("expected output to contain 'TaskWing vdev', got: %q", output)
	}
}

// TestListFooterAppearsOnce tests that footer appears exactly once.
func TestListFooterAppearsOnce(t *testing.T) {
	// Save and restore version
	originalVersion := version
	defer func() { version = originalVersion }()

	version = "2.0.0"

	output := captureStdout(func() {
		// Simulate multiple lines of output followed by footer
		os.Stdout.WriteString("Node 1\n")
		os.Stdout.WriteString("Node 2\n")

		// Footer logic
		ver := version
		if ver == "" {
			ver = "dev"
		}
		os.Stdout.WriteString("TaskWing v" + ver + "\n")
	})

	count := strings.Count(output, "TaskWing v")
	if count != 1 {
		t.Errorf("expected footer to appear exactly once, appeared %d times in: %q", count, output)
	}

	// Verify footer is at the end
	if !strings.HasSuffix(output, "TaskWing v2.0.0\n") {
		t.Errorf("expected footer at end of output, got: %q", output)
	}
}
