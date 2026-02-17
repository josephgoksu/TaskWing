package cmd

import (
	"strings"
	"testing"
)

func assertContainsInOrder(t *testing.T, content string, parts ...string) {
	t.Helper()

	start := 0
	for _, part := range parts {
		idx := strings.Index(content[start:], part)
		if idx < 0 {
			t.Fatalf("expected content to include %q after position %d", part, start)
		}
		start += idx + len(part)
	}
}

func TestSlashContracts_CoreCommandsIncludeWorkflowContract(t *testing.T) {
	core := map[string]string{
		"next":  slashNextContent,
		"done":  slashDoneContent,
		"plan":  slashPlanContent,
		"debug": slashDebugContent,
	}

	for name, content := range core {
		if !strings.Contains(content, "TaskWing Workflow Contract v1") {
			t.Fatalf("/tw-%s missing workflow contract banner", name)
		}
	}
}

func TestSlashContract_NextHasImplementationGate(t *testing.T) {
	if !strings.Contains(slashNextContent, "Implementation Start Gate (Hard Gate)") {
		t.Fatal("/tw-next missing implementation hard gate")
	}
	if !strings.Contains(slashNextContent, "REFUSAL: I can't start implementation yet.") {
		t.Fatal("/tw-next missing refusal language for checkpoint gate")
	}

	assertContainsInOrder(t, slashNextContent,
		"## Step 5: Present Unified Task Brief",
		"## Step 6: Implementation Start Gate (Hard Gate)",
		"## Step 7: Begin Implementation (Only After Approval)",
	)
}

func TestSlashContract_DoneHasVerificationGate(t *testing.T) {
	if !strings.Contains(slashDoneContent, "## Step 2: Collect Fresh Verification Evidence") {
		t.Fatal("/tw-done missing verification collection step")
	}
	if !strings.Contains(slashDoneContent, "REFUSAL: I can't mark this task done yet.") {
		t.Fatal("/tw-done missing refusal language for verification gate")
	}

	assertContainsInOrder(t, slashDoneContent,
		"## Step 2: Collect Fresh Verification Evidence",
		"## Step 4: Completion Gate (Hard Gate)",
		"## Step 5: Mark Complete",
	)
}

func TestSlashContract_PlanRequiresClarificationApproval(t *testing.T) {
	if !strings.Contains(slashPlanContent, "Hard gate for this command:") {
		t.Fatal("/tw-plan missing hard gate definition")
	}
	if !strings.Contains(slashPlanContent, "REFUSAL: I can't move past planning yet.") {
		t.Fatal("/tw-plan missing refusal language for clarification checkpoint")
	}

	assertContainsInOrder(t, slashPlanContent,
		"## Step 2: Ask Clarifying Questions (Loop)",
		"## Step 3: Clarification Checkpoint Approval (Hard Gate)",
		"## Step 4: Generate Plan",
	)
}

func TestSlashContract_DebugRequiresRootCauseEvidence(t *testing.T) {
	if !strings.Contains(slashDebugContent, "## Phase 2: Root-Cause Evidence Collection (Hard Gate)") {
		t.Fatal("/tw-debug missing root-cause hard gate")
	}
	if !strings.Contains(slashDebugContent, "REFUSAL: I can't propose a fix yet.") {
		t.Fatal("/tw-debug missing refusal language for root-cause gate")
	}

	assertContainsInOrder(t, slashDebugContent,
		"## Phase 1: Capture Problem Statement",
		"## Phase 2: Root-Cause Evidence Collection (Hard Gate)",
		"## Phase 3: Present Investigation Plan",
		"## Phase 4: Fix Proposal (Only After Evidence Gate Passes)",
	)
}

func TestSlashContract_LightweightCommandsRemainReadOnly(t *testing.T) {
	lightweight := map[string]string{
		"status":   slashStatusContent,
		"brief":    slashBriefContent,
		"explain":  slashExplainContent,
		"simplify": slashSimplifyContent,
	}

	for name, content := range lightweight {
		if !strings.Contains(content, "must not be used to bypass planning, verification, or debug gates") &&
			!strings.Contains(content, "must not bypass planning, verification, or debugging gates") &&
			!strings.Contains(content, "Do not use it to bypass plan, verification, or debug gates") {
			t.Fatalf("/tw-%s missing lightweight guardrail language", name)
		}
	}
}
