package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/spf13/viper"
)

func TestMissingMemoryGuidance_AcrossKnowledgePlanTask(t *testing.T) {
	withTempWorkingDir(t, func() {
		knowledgeTypeFlag = ""
		knowledgeWorkspaceFlag = ""
		knowledgeAllFlag = false

		viper.Set("json", false)
		viper.Set("quiet", false)
		t.Cleanup(func() {
			viper.Set("json", false)
			viper.Set("quiet", false)
		})

		commands := []struct {
			name string
			run  func() error
		}{
			{name: "knowledge", run: func() error { return runKnowledge(knowledgeCmd, nil) }},
			{name: "plan status", run: func() error { return planStatusCmd.RunE(planStatusCmd, nil) }},
			{name: "task list", run: func() error { return runTaskList(taskCmd, nil) }},
		}

		for _, tc := range commands {
			out := captureStdout(t, func() {
				if err := tc.run(); err != nil {
					t.Fatalf("%s returned error: %v", tc.name, err)
				}
			})

			if !strings.Contains(out, "No project memory found for this repository.") {
				t.Fatalf("%s output missing memory guidance: %q", tc.name, out)
			}
			if !strings.Contains(out, "Run 'taskwing bootstrap' first.") {
				t.Fatalf("%s output missing bootstrap guidance: %q", tc.name, out)
			}
		}
	})
}

func TestMissingMemoryGuidance_JSONShape(t *testing.T) {
	withTempWorkingDir(t, func() {
		viper.Set("json", true)
		t.Cleanup(func() {
			viper.Set("json", false)
		})

		out := captureStdout(t, func() {
			if err := runTaskList(taskCmd, nil); err != nil {
				t.Fatalf("task list returned error: %v", err)
			}
		})

		for _, want := range []string{
			`"ok": false`,
			`"error": "project memory not initialized"`,
			`"command": "taskwing bootstrap"`,
			`"next": "run taskwing bootstrap"`,
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("missing JSON field %q in output: %s", want, out)
			}
		}
	})
}

func withTempWorkingDir(t *testing.T, fn func()) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	config.ClearProjectContext()
	if _, err := config.DetectAndSetProjectContext(); err != nil {
		t.Fatalf("set project context: %v", err)
	}
	t.Cleanup(func() {
		config.ClearProjectContext()
		_ = os.Chdir(originalWD)
	})

	fn()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = original

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("capture stdout: %v", err)
	}
	_ = r.Close()

	return buf.String()
}
