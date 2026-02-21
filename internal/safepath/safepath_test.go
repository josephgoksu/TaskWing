package safepath

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoin_ValidPaths(t *testing.T) {
	base := t.TempDir()
	// Create a nested file.
	nested := filepath.Join(base, "sub", "file.txt")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		untrusted string
	}{
		{"simple file", "sub/file.txt"},
		{"directory", "sub"},
		{"empty returns base", ""},
		{"dot returns base", "."},
	}
	// Resolve symlinks on base for comparison (macOS /var -> /private/var).
	realBase, _ := filepath.EvalSymlinks(base)
	absBase, _ := filepath.Abs(realBase)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := SafeJoin(base, tc.untrusted)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !isWithin(absBase, result) && result != absBase {
				t.Errorf("result %q not within base %q", result, absBase)
			}
		})
	}
}

func TestSafeJoin_TraversalBlocked(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name      string
		untrusted string
	}{
		{"dotdot", ".."},
		{"dotdot slash", "../etc/passwd"},
		{"nested dotdot", "sub/../../etc/passwd"},
		{"absolute escape", "/etc/passwd"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SafeJoin(base, tc.untrusted)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, ErrOutsideBase) {
				t.Errorf("expected ErrOutsideBase, got: %v", err)
			}
		})
	}
}

func TestSafeJoin_SymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	// Create a symlink inside base pointing outside.
	link := filepath.Join(base, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := SafeJoin(base, "escape")
	if !errors.Is(err, ErrOutsideBase) {
		t.Errorf("expected ErrOutsideBase for symlink escape, got: %v", err)
	}
}

func TestSafeJoin_EmptyBase(t *testing.T) {
	_, err := SafeJoin("", "foo")
	if !errors.Is(err, ErrInvalidPath) {
		t.Errorf("expected ErrInvalidPath for empty base, got: %v", err)
	}
}

func TestValidateAbsPath_Valid(t *testing.T) {
	base := t.TempDir()
	child := filepath.Join(base, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := ValidateAbsPath(base, child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	absBase, _ := filepath.Abs(base)
	if !isWithin(absBase, result) {
		t.Errorf("result %q not within base %q", result, absBase)
	}
}

func TestValidateAbsPath_OutsideBase(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	_, err := ValidateAbsPath(base, outside)
	if !errors.Is(err, ErrOutsideBase) {
		t.Errorf("expected ErrOutsideBase, got: %v", err)
	}
}

func TestValidateAbsPath_EmptyInputs(t *testing.T) {
	_, err := ValidateAbsPath("", "/foo")
	if !errors.Is(err, ErrInvalidPath) {
		t.Errorf("expected ErrInvalidPath for empty base, got: %v", err)
	}
	_, err = ValidateAbsPath("/foo", "")
	if !errors.Is(err, ErrInvalidPath) {
		t.Errorf("expected ErrInvalidPath for empty candidate, got: %v", err)
	}
}
