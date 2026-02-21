package ui

import (
	"testing"
)

func TestStylesRenderNonEmpty(t *testing.T) {
	styles := []struct {
		name  string
		style interface{ Render(...string) string }
	}{
		{"StyleCheckOK", StyleCheckOK},
		{"StyleCheckWarn", StyleCheckWarn},
		{"StyleCheckFail", StyleCheckFail},
		{"StyleCheckName", StyleCheckName},
		{"StyleCheckHint", StyleCheckHint},
		{"StyleAskHeader", StyleAskHeader},
		{"StyleAskMeta", StyleAskMeta},
		{"StyleCitationPath", StyleCitationPath},
		{"StyleCitationBadge", StyleCitationBadge},
		{"StyleTableRowEven", StyleTableRowEven},
		{"StyleTableRowOdd", StyleTableRowOdd},
		{"StyleTableHeader", StyleTableHeader},
	}

	for _, s := range styles {
		t.Run(s.name, func(t *testing.T) {
			result := s.style.Render("test")
			if result == "" {
				t.Errorf("%s.Render(\"test\") returned empty string", s.name)
			}
		})
	}
}

func TestCategoryBadge(t *testing.T) {
	types := []string{"decision", "feature", "constraint", "pattern", "plan", "note", "metadata", "documentation"}
	for _, nodeType := range types {
		t.Run(nodeType, func(t *testing.T) {
			badge := CategoryBadge(nodeType)
			if badge == "" {
				t.Errorf("CategoryBadge(%q) returned empty string", nodeType)
			}
		})
	}

	// Unknown type should also work
	badge := CategoryBadge("unknown_type")
	if badge == "" {
		t.Error("CategoryBadge for unknown type returned empty string")
	}
}
