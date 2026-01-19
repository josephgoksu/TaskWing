package git

import (
	"fmt"
	"testing"
)

func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		name      string
		planID    string
		planTitle string
		want      string
	}{
		{
			name:      "standard title",
			planID:    "plan-abc12345",
			planTitle: "Add OAuth2 authentication",
			want:      "feat/add-oauth2-authentication-c12345",
		},
		{
			name:      "empty title falls back to ID",
			planID:    "plan-xyz789",
			planTitle: "",
			want:      "feat/xyz789",
		},
		{
			name:      "short plan ID",
			planID:    "abc",
			planTitle: "Fix bug",
			want:      "feat/fix-bug-abc",
		},
		{
			name:      "different plans same title get different branches",
			planID:    "plan-111111",
			planTitle: "Add authentication",
			want:      "feat/add-authentication-111111",
		},
		{
			name:      "different plans same title - second plan",
			planID:    "plan-222222",
			planTitle: "Add authentication",
			want:      "feat/add-authentication-222222",
		},
		{
			name:      "long title gets truncated",
			planID:    "plan-abc123",
			planTitle: "This is a very long plan title that exceeds the maximum allowed length for branch names",
			want:      "feat/this-is-a-very-long-plan-title-that-exceeds-abc123",
		},
		{
			name:      "special characters removed",
			planID:    "plan-test99",
			planTitle: "Fix bug #123: User can't login!",
			want:      "feat/fix-bug-123-user-cant-login-test99",
		},
		{
			name:      "underscores converted to hyphens",
			planID:    "plan-under1",
			planTitle: "add_new_feature",
			want:      "feat/add-new-feature-under1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBranchName(tt.planID, tt.planTitle)
			if got != tt.want {
				t.Errorf("GenerateBranchName(%q, %q) = %q, want %q",
					tt.planID, tt.planTitle, got, tt.want)
			}
		})
	}
}

func TestGenerateBranchName_Uniqueness(t *testing.T) {
	// Critical test: two plans with identical titles MUST produce different branch names
	branch1 := GenerateBranchName("plan-aaaaaa", "Add authentication")
	branch2 := GenerateBranchName("plan-bbbbbb", "Add authentication")

	if branch1 == branch2 {
		t.Errorf("CRITICAL: Two different plans produced identical branch names: %q", branch1)
	}
}

func TestUnrelatedBranchError(t *testing.T) {
	err := &UnrelatedBranchError{
		CurrentBranch:  "feat/other-work",
		ExpectedBranch: "feat/add-auth-abc123",
	}

	// Test error message
	expected := `currently on branch "feat/other-work" which is unrelated to plan branch "feat/add-auth-abc123"`
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	// Test type detection
	if !IsUnrelatedBranchError(err) {
		t.Error("IsUnrelatedBranchError() should return true for UnrelatedBranchError")
	}

	// Test non-matching error
	otherErr := fmt.Errorf("some other error")
	if IsUnrelatedBranchError(otherErr) {
		t.Error("IsUnrelatedBranchError() should return false for other errors")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"UPPERCASE", "uppercase"},
		{"under_score", "under-score"},
		{"multiple   spaces", "multiple-spaces"},
		{"special!@#$chars", "specialchars"},
		{"--leading-trailing--", "leading-trailing"},
		{"café résumé", "caf-rsum"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
