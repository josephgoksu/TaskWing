package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaluateIntegration_ManagedCommandsMissing_AutoFixable(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)
	if err := init.CreateSlashCommands("claude", false); err != nil {
		t.Fatalf("create slash commands: %v", err)
	}
	if err := init.InstallHooksConfig("claude", false); err != nil {
		t.Fatalf("install hooks: %v", err)
	}

	// Simulate managed drift by deleting one expected command file.
	missingPath := filepath.Join(tmpDir, ".claude", "commands", "tw-next.md")
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("remove managed command: %v", err)
	}

	report := EvaluateIntegration(tmpDir, "claude", true)
	if !report.ManagedLocalDrift {
		t.Fatal("expected managed local drift for missing managed command")
	}
	if report.UnmanagedDrift {
		t.Fatal("did not expect unmanaged drift")
	}

	plan := BuildRepairPlan(map[string]IntegrationReport{"claude": report}, RepairPlanOptions{
		IncludeGlobalMutations:   true,
		IncludeUnmanagedAdoption: false,
	})
	if len(plan.Actions) == 0 {
		t.Fatal("expected repair actions")
	}
	foundCommandsRepair := false
	for _, action := range plan.Actions {
		if action.Component == AIComponentCommands {
			foundCommandsRepair = true
			if !action.Apply {
				t.Fatalf("expected commands repair to be applicable: %+v", action)
			}
		}
	}
	if !foundCommandsRepair {
		t.Fatal("expected commands repair action")
	}
}

func TestEvaluateIntegration_UnmanagedTaskWingLike_RequiresAdoption(t *testing.T) {
	tmpDir := t.TempDir()
	cmdDir := filepath.Join(tmpDir, ".claude", "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	// TaskWing-like files but no marker.
	if err := os.WriteFile(filepath.Join(cmdDir, "tw-brief.md"), []byte("---\ndescription: brief\n---\n!taskwing slash brief\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	report := EvaluateIntegration(tmpDir, "claude", false)
	if !report.UnmanagedDrift {
		t.Fatal("expected unmanaged drift")
	}
	if !report.TaskWingLikeUnmanaged {
		t.Fatal("expected taskwing-like unmanaged signal")
	}

	plan := BuildRepairPlan(map[string]IntegrationReport{"claude": report}, RepairPlanOptions{
		IncludeGlobalMutations:   true,
		IncludeUnmanagedAdoption: false,
	})
	if len(plan.Actions) == 0 {
		t.Fatal("expected repair plan actions")
	}
	foundBlockedAdoption := false
	for _, action := range plan.Actions {
		if strings.HasPrefix(action.Primitive, "adopt_and_") {
			if action.Apply {
				t.Fatalf("expected adoption action to be blocked without opt-in: %+v", action)
			}
			foundBlockedAdoption = true
		}
	}
	if !foundBlockedAdoption {
		t.Fatal("expected blocked adoption action")
	}
}

func TestEvaluateIntegration_HooksMissingStop_IsInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	cmdDir := filepath.Join(tmpDir, ".claude", "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	for _, cmd := range SlashCommands {
		if err := os.WriteFile(filepath.Join(cmdDir, cmd.BaseName+".md"), []byte("---\ndescription: test\n---\n!taskwing slash brief\n"), 0644); err != nil {
			t.Fatalf("write command: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(`{"hooks":{"SessionStart":[],"SessionEnd":[]}}`), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	report := EvaluateIntegration(tmpDir, "claude", true)
	foundInvalidHooks := false
	for _, issue := range report.Issues {
		if issue.Component == AIComponentHooks && issue.Status == ComponentStatusInvalid {
			foundInvalidHooks = true
			break
		}
	}
	if !foundInvalidHooks {
		t.Fatalf("expected invalid hooks issue, got: %+v", report.Issues)
	}
}

func TestEvaluateIntegration_HooksWrongStopCommand_IsInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	cmdDir := filepath.Join(tmpDir, ".claude", "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	for _, cmd := range SlashCommands {
		if err := os.WriteFile(filepath.Join(cmdDir, cmd.BaseName+".md"), []byte("---\ndescription: test\n---\n!taskwing slash brief\n"), 0644); err != nil {
			t.Fatalf("write command: %v", err)
		}
	}
	settings := `{
	  "hooks": {
	    "SessionStart": [{"hooks":[{"type":"command","command":"taskwing hook session-init"}]}],
	    "Stop": [{"hooks":[{"type":"command","command":"echo noop"}]}],
	    "SessionEnd": [{"hooks":[{"type":"command","command":"taskwing hook session-end"}]}]
	  }
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(settings), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	report := EvaluateIntegration(tmpDir, "claude", true)
	foundInvalid := false
	for _, issue := range report.Issues {
		if issue.Component == AIComponentHooks && issue.Status == ComponentStatusInvalid &&
			strings.Contains(issue.Reason, "continue-check") {
			foundInvalid = true
			break
		}
	}
	if !foundInvalid {
		t.Fatalf("expected invalid Stop command issue, got: %+v", report.Issues)
	}
}

func TestEvaluateIntegration_UnconfiguredAI_NoIssues(t *testing.T) {
	tmpDir := t.TempDir()

	claude := EvaluateIntegration(tmpDir, "claude", false)
	if len(claude.Issues) != 0 {
		t.Fatalf("expected no issues for unconfigured claude, got: %+v", claude.Issues)
	}

	gemini := EvaluateIntegration(tmpDir, "gemini", false)
	if len(gemini.Issues) != 0 {
		t.Fatalf("expected no issues for unconfigured gemini, got: %+v", gemini.Issues)
	}

	opencode := EvaluateIntegration(tmpDir, "opencode", false)
	if len(opencode.Issues) != 0 {
		t.Fatalf("expected no issues for unconfigured opencode, got: %+v", opencode.Issues)
	}
}

func TestEvaluateIntegration_GlobalMCPOnlyWhenConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	report := EvaluateIntegration(tmpDir, "claude", false)
	for _, issue := range report.Issues {
		if issue.Component == AIComponentMCPGlobal {
			t.Fatalf("did not expect global mcp issue for unconfigured claude: %+v", issue)
		}
	}

	init := NewInitializer(tmpDir)
	if err := init.CreateSlashCommands("claude", false); err != nil {
		t.Fatalf("create slash commands: %v", err)
	}
	if err := init.InstallHooksConfig("claude", false); err != nil {
		t.Fatalf("install hooks: %v", err)
	}

	configured := EvaluateIntegration(tmpDir, "claude", false)
	foundGlobal := false
	for _, issue := range configured.Issues {
		if issue.Component == AIComponentMCPGlobal {
			foundGlobal = true
			break
		}
	}
	if !foundGlobal {
		t.Fatalf("expected global mcp issue for configured claude, got: %+v", configured.Issues)
	}
}

func TestEvaluateIntegration_CursorLocalMCP_NonCanonicalKeyIsInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	cursorDir := filepath.Join(tmpDir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("mkdir cursor dir: %v", err)
	}

	raw := map[string]any{
		"mcpServers": map[string]any{
			"taskwing-mcp-my-project": map[string]any{
				"command": "taskwing mcp",
			},
		},
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal cursor config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cursorDir, "mcp.json"), data, 0644); err != nil {
		t.Fatalf("write cursor config: %v", err)
	}

	report := EvaluateIntegration(tmpDir, "cursor", false)
	found := false
	for _, issue := range report.Issues {
		if issue.Component == AIComponentMCPLocal && issue.Status == ComponentStatusInvalid {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected invalid non-canonical local MCP issue, got: %+v", report.Issues)
	}
}
