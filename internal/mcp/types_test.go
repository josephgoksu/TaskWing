package mcp

import "testing"

func TestCodeAction_IsValid(t *testing.T) {
	tests := []struct {
		action CodeAction
		want   bool
	}{
		{CodeActionFind, true},
		{CodeActionSearch, true},
		{CodeActionExplain, true},
		{CodeActionCallers, true},
		{CodeActionImpact, true},
		{CodeActionSimplify, true},
		{"invalid", false},
		{"", false},
		{"FIND", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.want {
				t.Errorf("CodeAction(%q).IsValid() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestTaskAction_IsValid(t *testing.T) {
	tests := []struct {
		action TaskAction
		want   bool
	}{
		{TaskActionNext, true},
		{TaskActionCurrent, true},
		{TaskActionStart, true},
		{TaskActionComplete, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.want {
				t.Errorf("TaskAction(%q).IsValid() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestPlanAction_IsValid(t *testing.T) {
	tests := []struct {
		action PlanAction
		want   bool
	}{
		{PlanActionClarify, true},
		{PlanActionGenerate, true},
		{PlanActionAudit, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.want {
				t.Errorf("PlanAction(%q).IsValid() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestValidCodeActions(t *testing.T) {
	actions := ValidCodeActions()
	if len(actions) != 6 {
		t.Errorf("ValidCodeActions() returned %d actions, want 6", len(actions))
	}
}

func TestValidTaskActions(t *testing.T) {
	actions := ValidTaskActions()
	if len(actions) != 4 {
		t.Errorf("ValidTaskActions() returned %d actions, want 4", len(actions))
	}
}

func TestValidPlanActions(t *testing.T) {
	actions := ValidPlanActions()
	if len(actions) != 3 {
		t.Errorf("ValidPlanActions() returned %d actions, want 3", len(actions))
	}
}
