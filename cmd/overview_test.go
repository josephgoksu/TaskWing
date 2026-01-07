package cmd

import (
	"testing"
)

func TestParseOverviewTemplate(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantShort string
		wantLong  string
	}{
		{
			name: "valid template",
			content: `# Project Overview

## Short Description (one sentence)
A CLI tool for testing.

## Long Description (2-3 paragraphs)
This is the first paragraph.

This is the second paragraph.

---
Instructions: Edit the text above.
`,
			wantShort: "A CLI tool for testing.",
			wantLong:  "This is the first paragraph.\n\nThis is the second paragraph.",
		},
		{
			name: "multiline short description collapses to single line",
			content: `# Project Overview

## Short Description (one sentence)
Line one
Line two

## Long Description (2-3 paragraphs)
Long content here.

---
`,
			wantShort: "Line one Line two",
			wantLong:  "Long content here.",
		},
		{
			name:      "empty content",
			content:   "",
			wantShort: "",
			wantLong:  "",
		},
		{
			name: "missing sections returns empty",
			content: `# Just a title

Some random content without proper sections.
`,
			wantShort: "",
			wantLong:  "",
		},
		{
			name: "only short description section",
			content: `## Short Description (one sentence)
Only short here.
`,
			wantShort: "Only short here.",
			wantLong:  "",
		},
		{
			name: "stops at instruction separator",
			content: `## Short Description (one sentence)
Short text.

## Long Description (2-3 paragraphs)
Long text before separator.

---
This should NOT be included.
More instructions.
`,
			wantShort: "Short text.",
			wantLong:  "Long text before separator.",
		},
		{
			name: "preserves blank lines in long description",
			content: `## Short Description (one sentence)
Short.

## Long Description (2-3 paragraphs)
Para one.

Para two.

Para three.

---
`,
			wantShort: "Short.",
			wantLong:  "Para one.\n\nPara two.\n\nPara three.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotShort, gotLong := parseOverviewTemplate(tt.content)
			if gotShort != tt.wantShort {
				t.Errorf("short = %q, want %q", gotShort, tt.wantShort)
			}
			if gotLong != tt.wantLong {
				t.Errorf("long = %q, want %q", gotLong, tt.wantLong)
			}
		})
	}
}

// TestParseOverviewTemplate_ValidationEdgeCases documents parsing behavior for edge cases
// that result in validation failures in the edit flow. These tests ensure parsing works
// as expected, even when the result would be rejected by SaveProjectOverview.
func TestParseOverviewTemplate_ValidationEdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		wantShort         string
		wantLong          string
		wouldFailValidate bool // True if this would fail validation (either field empty)
	}{
		{
			name: "only short description - would fail validation",
			content: `## Short Description (one sentence)
Valid short description here.
`,
			wantShort:         "Valid short description here.",
			wantLong:          "",
			wouldFailValidate: true,
		},
		{
			name: "only long description - would fail validation",
			content: `## Long Description (2-3 paragraphs)
Valid long description here.
---
`,
			wantShort:         "",
			wantLong:          "Valid long description here.",
			wouldFailValidate: true,
		},
		{
			name: "short empty but long filled - would fail validation",
			content: `## Short Description (one sentence)

## Long Description (2-3 paragraphs)
This is a valid long description.
---
`,
			wantShort:         "",
			wantLong:          "This is a valid long description.",
			wouldFailValidate: true,
		},
		{
			name: "both fields filled - valid",
			content: `## Short Description (one sentence)
Valid short.

## Long Description (2-3 paragraphs)
Valid long.
---
`,
			wantShort:         "Valid short.",
			wantLong:          "Valid long.",
			wouldFailValidate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotShort, gotLong := parseOverviewTemplate(tt.content)
			if gotShort != tt.wantShort {
				t.Errorf("short = %q, want %q", gotShort, tt.wantShort)
			}
			if gotLong != tt.wantLong {
				t.Errorf("long = %q, want %q", gotLong, tt.wantLong)
			}

			// Verify our expectation about validation
			wouldFail := gotShort == "" || gotLong == ""
			if wouldFail != tt.wouldFailValidate {
				t.Errorf("wouldFailValidate = %v, want %v", wouldFail, tt.wouldFailValidate)
			}
		})
	}
}
