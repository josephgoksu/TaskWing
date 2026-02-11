package cmd

import (
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
)

func TestChecksFromIntegrationReports_Healthy(t *testing.T) {
	reports := map[string]bootstrap.IntegrationReport{
		"opencode": {
			AI:     "opencode",
			Issues: nil,
		},
	}

	checks := checksFromIntegrationReports(reports)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Name != "Integration (opencode)" {
		t.Fatalf("unexpected check name: %s", checks[0].Name)
	}
	if checks[0].Status != "ok" {
		t.Fatalf("expected ok status, got %s", checks[0].Status)
	}
}

func TestChecksFromIntegrationReports_HintSelection(t *testing.T) {
	reports := map[string]bootstrap.IntegrationReport{
		"claude": {
			AI: "claude",
			Issues: []bootstrap.IntegrationIssue{
				{
					AI:            "claude",
					Component:     bootstrap.AIComponentMCPGlobal,
					Status:        bootstrap.ComponentStatusMissing,
					Reason:        "global taskwing-mcp registration missing",
					MutatesGlobal: true,
				},
			},
		},
		"codex": {
			AI: "codex",
			Issues: []bootstrap.IntegrationIssue{
				{
					AI:            "codex",
					Component:     bootstrap.AIComponentCommands,
					Status:        bootstrap.ComponentStatusInvalid,
					Reason:        "commands invalid",
					AdoptRequired: true,
				},
			},
		},
	}

	checks := checksFromIntegrationReports(reports)
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(checks))
	}

	var sawGlobalHint bool
	var sawAdoptHint bool
	for _, check := range checks {
		if strings.Contains(check.Hint, "--yes --ai claude") {
			sawGlobalHint = true
		}
		if strings.Contains(check.Hint, "--adopt-unmanaged --ai codex") {
			sawAdoptHint = true
		}
	}
	if !sawGlobalHint {
		t.Fatalf("expected global mutation hint in checks: %+v", checks)
	}
	if !sawAdoptHint {
		t.Fatalf("expected adopt-unmanaged hint in checks: %+v", checks)
	}
}

func TestDoctor_OpenCodeFixReevaluatesState(t *testing.T) {
	tmpDir := t.TempDir()
	init := bootstrap.NewInitializer(tmpDir)

	if err := init.CreateSlashCommands("opencode", false); err != nil {
		t.Fatalf("create opencode commands: %v", err)
	}
	if err := init.InstallHooksConfig("opencode", false); err != nil {
		t.Fatalf("create opencode plugin: %v", err)
	}

	reportsBefore := bootstrap.EvaluateIntegrations(tmpDir, map[string]bool{})
	if !hasIntegrationIssue(reportsBefore["opencode"], bootstrap.AIComponentMCPLocal) {
		t.Fatalf("expected opencode mcp_local issue before repair, got %+v", reportsBefore["opencode"].Issues)
	}

	plan := bootstrap.BuildRepairPlan(reportsBefore, bootstrap.RepairPlanOptions{
		TargetAIs:              []string{"opencode"},
		IncludeGlobalMutations: true,
	})
	if len(plan.Actions) == 0 {
		t.Fatal("expected non-empty repair plan for opencode drift")
	}

	applied, skipped, blocked, err := applyRepairPlan(tmpDir, plan, doctorFixOptions{
		Fix:       true,
		Yes:       true,
		TargetAIs: []string{"opencode"},
	})
	if err != nil {
		t.Fatalf("apply repair plan: %v", err)
	}
	if len(applied) == 0 {
		t.Fatalf("expected at least one applied action; skipped=%d blocked=%d", len(skipped), len(blocked))
	}

	reportsAfter := bootstrap.EvaluateIntegrations(tmpDir, map[string]bool{})
	if hasIntegrationIssue(reportsAfter["opencode"], bootstrap.AIComponentMCPLocal) {
		t.Fatalf("expected opencode mcp_local issue to be resolved, got %+v", reportsAfter["opencode"].Issues)
	}
}

func hasIntegrationIssue(report bootstrap.IntegrationReport, component bootstrap.AIComponent) bool {
	for _, issue := range report.Issues {
		if issue.Component == component {
			return true
		}
	}
	return false
}
