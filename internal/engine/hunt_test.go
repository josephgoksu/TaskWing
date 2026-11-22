package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHuntInsideTreeRefusesEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "pkg")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ok"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"pkg/x_test.go", "pkg", "../x", "a/../../x", "/etc/x", ""} {
		if p, err := insideTree(root, rel); err == nil {
			t.Errorf("insideTree(%q) = %q, want an error", rel, p)
		}
	}
	for _, rel := range []string{"ok/x_test.go", "new/dir/x_test.go"} {
		if p, err := insideTree(root, rel); err != nil || p != filepath.Join(root, rel) {
			t.Errorf("insideTree(%q) = %q, %v", rel, p, err)
		}
	}
}

func TestHuntOwnsAnyHiddenTest(t *testing.T) {
	tests := map[string]bool{"billing/taskwing_exam_b1_test.go": true}
	if got := ownsAny(Card{Owns: []string{"Billing/**"}}, tests); got == "" {
		t.Error("a folder pattern covering the hidden test was allowed")
	}
	if got := ownsAny(Card{Owns: []string{"billing/retry.go"}}, tests); got != "" {
		t.Errorf("an exact file was refused: %s", got)
	}
}

func TestHuntOverlapsEarlier(t *testing.T) {
	earlier := []bug{{ID: "b1", Owns: []string{"a.go"}, proven: true}, {ID: "b2", Owns: []string{"b.go"}}}
	if got := overlapsEarlier(Card{Owns: []string{"a.go"}}, earlier); got != "b1" {
		t.Errorf("got %q, want b1", got)
	}
	if got := overlapsEarlier(Card{Owns: []string{"b.go"}}, earlier); got != "" {
		t.Errorf("an unproven bug blocked: %q", got)
	}
}
