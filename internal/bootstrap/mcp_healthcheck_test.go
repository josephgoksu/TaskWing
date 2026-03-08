package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// TestMCPInstallAndHealthcheck: Drift detection and report generation
// =============================================================================

func TestMCPInstallAndHealthcheck(t *testing.T) {
	t.Run("global_mcp_drift_detection", func(t *testing.T) {
		// Simulate reports where Claude has global MCP drift
		reports := map[string]IntegrationReport{
			"claude": {
				AI:             "claude",
				GlobalMCPDrift: true,
				Issues: []IntegrationIssue{
					{
						AI:            "claude",
						Component:     AIComponentMCPGlobal,
						Ownership:     OwnershipManaged,
						Status:        ComponentStatusStale,
						Reason:        "Global MCP config references stale binary path",
						AutoFixable:   false,
						MutatesGlobal: true,
					},
				},
			},
			"gemini": {
				AI:             "gemini",
				GlobalMCPDrift: false,
			},
			"codex": {
				AI:             "codex",
				GlobalMCPDrift: false,
			},
		}

		driftAIs := GlobalMCPDriftAIs(reports)
		if len(driftAIs) != 1 || driftAIs[0] != "claude" {
			t.Errorf("GlobalMCPDriftAIs = %v, want [claude]", driftAIs)
		}
	})

	t.Run("no_drift_when_all_healthy", func(t *testing.T) {
		reports := map[string]IntegrationReport{
			"claude": {AI: "claude", GlobalMCPDrift: false},
			"gemini": {AI: "gemini", GlobalMCPDrift: false},
		}

		driftAIs := GlobalMCPDriftAIs(reports)
		if len(driftAIs) != 0 {
			t.Errorf("GlobalMCPDriftAIs = %v, want empty", driftAIs)
		}
	})

	t.Run("multiple_drifts_detected", func(t *testing.T) {
		reports := map[string]IntegrationReport{
			"claude": {AI: "claude", GlobalMCPDrift: true},
			"gemini": {AI: "gemini", GlobalMCPDrift: true},
			"codex":  {AI: "codex", GlobalMCPDrift: false},
		}

		driftAIs := GlobalMCPDriftAIs(reports)
		if len(driftAIs) != 2 {
			t.Errorf("GlobalMCPDriftAIs returned %d AIs, want 2", len(driftAIs))
		}
	})

	t.Run("managed_local_drift_detection", func(t *testing.T) {
		reports := map[string]IntegrationReport{
			"claude": {
				AI:                "claude",
				ManagedLocalDrift: true,
				Issues: []IntegrationIssue{
					{
						AI:          "claude",
						Component:   AIComponentCommands,
						Ownership:   OwnershipManaged,
						Status:      ComponentStatusStale,
						Reason:      "Managed commands directory is stale",
						AutoFixable: true,
					},
				},
			},
		}

		if !HasManagedLocalDrift(reports) {
			t.Error("HasManagedLocalDrift = false, want true")
		}
	})

	t.Run("drift_issue_provides_root_cause_evidence", func(t *testing.T) {
		// Verify that drift issues include reason (root-cause evidence)
		issue := IntegrationIssue{
			AI:            "claude",
			Component:     AIComponentMCPGlobal,
			Status:        ComponentStatusStale,
			Reason:        "Global MCP config references stale binary path",
			MutatesGlobal: true,
		}

		if issue.Reason == "" {
			t.Error("Issue.Reason is empty - drift detection must provide root-cause evidence")
		}
		if !issue.MutatesGlobal {
			t.Error("Issue.MutatesGlobal should be true for global MCP drift")
		}
	})
}

// =============================================================================
// TestRepairPlan: Verify repair plan generation from drift reports
// =============================================================================

func TestRepairPlanFromDriftReports(t *testing.T) {
	t.Run("generates_actions_for_managed_drift", func(t *testing.T) {
		reports := map[string]IntegrationReport{
			"claude": {
				AI:                "claude",
				ManagedLocalDrift: true,
				Issues: []IntegrationIssue{
					{
						AI:          "claude",
						Component:   AIComponentCommands,
						Ownership:   OwnershipManaged,
						Status:      ComponentStatusStale,
						AutoFixable: true,
					},
				},
			},
		}

		plan := BuildRepairPlan(reports, RepairPlanOptions{
			TargetAIs: []string{"claude"},
		})

		if len(plan.Actions) == 0 {
			t.Error("BuildRepairPlan returned 0 actions for managed drift, want >= 1")
		}
	})

	t.Run("disables_global_mcp_mutations_by_default", func(t *testing.T) {
		reports := map[string]IntegrationReport{
			"claude": {
				AI:             "claude",
				GlobalMCPDrift: true,
				Issues: []IntegrationIssue{
					{
						AI:            "claude",
						Component:     AIComponentMCPGlobal,
						Status:        ComponentStatusStale,
						MutatesGlobal: true,
						AutoFixable:   true,
					},
				},
			},
		}

		plan := BuildRepairPlan(reports, RepairPlanOptions{
			TargetAIs:              []string{"claude"},
			IncludeGlobalMutations: false, // default: don't mutate global
		})

		// Global MCP action should exist but with Apply=false
		foundGlobal := false
		for _, action := range plan.Actions {
			if action.Component == AIComponentMCPGlobal {
				foundGlobal = true
				if action.Apply {
					t.Error("Global MCP action has Apply=true without IncludeGlobalMutations - should be disabled")
				}
				if action.Reason == "" {
					t.Error("Global MCP action should have a reason when disabled")
				}
			}
		}
		if !foundGlobal {
			t.Error("BuildRepairPlan should include global MCP action (disabled) for reporting")
		}
	})
}

// =============================================================================
// TestClaudeDriftDetection: Filesystem-based Claude MCP drift scenarios
// =============================================================================

func TestClaudeDriftDetection(t *testing.T) {
	t.Run("missing_commands_dir_no_global_mcp", func(t *testing.T) {
		// Empty project: no .claude directory, no global MCP
		basePath := t.TempDir()

		report := EvaluateIntegration(basePath, "claude", false)

		// No commands dir + no global MCP = nothing configured, no drift
		if report.CommandsDirExists {
			t.Error("CommandsDirExists should be false for empty project")
		}
		if report.GlobalMCPDrift {
			t.Error("GlobalMCPDrift should be false when nothing is configured")
		}
	})

	t.Run("managed_commands_stale_version_triggers_drift", func(t *testing.T) {
		basePath := t.TempDir()
		commandsDir := filepath.Join(basePath, ".claude", "commands")
		if err := os.MkdirAll(commandsDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Write managed marker with a stale version
		markerPath := filepath.Join(commandsDir, TaskWingManagedFile)
		if err := os.WriteFile(markerPath, []byte("# Version: 0.0.0-old\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Write all expected command files so the version check is reached
		for name := range expectedSlashCommandFiles(".md") {
			if err := os.WriteFile(filepath.Join(commandsDir, name), []byte("test"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		report := EvaluateIntegration(basePath, "claude", true)

		if !report.ManagedLocalDrift {
			t.Error("ManagedLocalDrift should be true for stale managed version")
		}
		// Verify issue has root-cause evidence
		foundDrift := false
		for _, issue := range report.Issues {
			if issue.Component == AIComponentCommands && issue.Ownership == OwnershipManaged {
				foundDrift = true
				if issue.Status != ComponentStatusStale {
					t.Errorf("Expected stale status for version mismatch, got %q", issue.Status)
				}
				if issue.Reason == "" {
					t.Error("Stale commands issue must include reason (root-cause evidence)")
				}
				if !issue.AutoFixable {
					t.Error("Managed stale commands should be auto-fixable")
				}
			}
		}
		if !foundDrift {
			t.Error("Expected managed commands drift issue for version mismatch")
		}
	})

	t.Run("local_configured_but_global_mcp_missing_triggers_drift", func(t *testing.T) {
		basePath := t.TempDir()
		commandsDir := filepath.Join(basePath, ".claude", "commands")
		if err := os.MkdirAll(commandsDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Write managed marker with current version
		markerPath := filepath.Join(commandsDir, TaskWingManagedFile)
		currentVersion := AIToolConfigVersion("claude")
		if err := os.WriteFile(markerPath, []byte("# Version: "+currentVersion+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Write expected slash command files
		for name := range expectedSlashCommandFiles(".md") {
			if err := os.WriteFile(filepath.Join(commandsDir, name), []byte("test"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		// globalMCPExists=false means drift
		report := EvaluateIntegration(basePath, "claude", false)

		if !report.GlobalMCPDrift {
			t.Error("GlobalMCPDrift should be true when local is configured but global MCP is missing")
		}

		// Verify the issue provides evidence and is marked as global mutation
		foundGlobalIssue := false
		for _, issue := range report.Issues {
			if issue.Component == AIComponentMCPGlobal {
				foundGlobalIssue = true
				if issue.Reason == "" {
					t.Error("Global MCP issue must include reason")
				}
				if !issue.MutatesGlobal {
					t.Error("Global MCP issue must be flagged as MutatesGlobal")
				}
			}
		}
		if !foundGlobalIssue {
			t.Error("Expected global MCP issue when local is configured but global is missing")
		}
	})

	t.Run("repair_plan_requires_explicit_consent_for_global_fix", func(t *testing.T) {
		// Simulate Claude with both local and global drift
		reports := map[string]IntegrationReport{
			"claude": {
				AI:                "claude",
				GlobalMCPDrift:    true,
				ManagedLocalDrift: true,
				Issues: []IntegrationIssue{
					{
						AI:          "claude",
						Component:   AIComponentCommands,
						Ownership:   OwnershipManaged,
						Status:      ComponentStatusStale,
						Reason:      "managed marker version mismatch",
						AutoFixable: true,
					},
					{
						AI:            "claude",
						Component:     AIComponentMCPGlobal,
						Ownership:     OwnershipNone,
						Status:        ComponentStatusMissing,
						Reason:        "global taskwing MCP registration missing",
						AutoFixable:   true,
						MutatesGlobal: true,
					},
				},
			},
		}

		// Default: no global mutations allowed
		plan := BuildRepairPlan(reports, RepairPlanOptions{
			TargetAIs: []string{"claude"},
		})

		localApplied := false
		globalApplied := false
		for _, action := range plan.Actions {
			if action.Component == AIComponentCommands && action.Apply {
				localApplied = true
			}
			if action.Component == AIComponentMCPGlobal {
				if action.Apply {
					globalApplied = true
				}
			}
		}

		if !localApplied {
			t.Error("Local managed fix should be auto-applied")
		}
		if globalApplied {
			t.Error("Global MCP fix must NOT be auto-applied without explicit consent (Gate 3)")
		}

		// Now with explicit consent
		planWithConsent := BuildRepairPlan(reports, RepairPlanOptions{
			TargetAIs:              []string{"claude"},
			IncludeGlobalMutations: true,
		})

		globalConsentApplied := false
		for _, action := range planWithConsent.Actions {
			if action.Component == AIComponentMCPGlobal && action.Apply {
				globalConsentApplied = true
			}
		}
		if !globalConsentApplied {
			t.Error("Global MCP fix should be applied when IncludeGlobalMutations=true")
		}
	})

	t.Run("all_drift_issues_carry_evidence", func(t *testing.T) {
		// Every IntegrationIssue must have a non-empty Reason for traceability
		reports := map[string]IntegrationReport{
			"claude": {
				AI:             "claude",
				GlobalMCPDrift: true,
				Issues: []IntegrationIssue{
					{AI: "claude", Component: AIComponentMCPGlobal, Status: ComponentStatusMissing, Reason: "global taskwing MCP registration missing", MutatesGlobal: true},
					{AI: "claude", Component: AIComponentCommands, Status: ComponentStatusStale, Reason: "managed marker version mismatch"},
					{AI: "claude", Component: AIComponentHooks, Status: ComponentStatusInvalid, Reason: "required Stop hook missing"},
				},
			},
		}

		for _, issue := range reports["claude"].Issues {
			if issue.Reason == "" {
				t.Errorf("Issue for component %q has empty Reason - all drift must carry root-cause evidence", issue.Component)
			}
		}

		plan := BuildRepairPlan(reports, RepairPlanOptions{TargetAIs: []string{"claude"}})
		for _, action := range plan.Actions {
			if action.Reason == "" && !action.Apply {
				t.Errorf("Disabled action for %q has empty Reason - must explain why disabled", action.Component)
			}
		}
	})
}
