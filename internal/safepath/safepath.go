// Package safepath provides helpers to prevent path traversal attacks.
// All functions validate that resolved paths remain within an expected
// base directory, even when symlinks are involved.
package safepath

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel errors for path validation failures.
var (
	ErrOutsideBase = errors.New("path escapes base directory")
	ErrInvalidPath = errors.New("invalid path")
)

// SafeJoin resolves an untrusted relative path against a trusted base
// directory and returns the absolute, cleaned result. It returns
// ErrOutsideBase if the resolved path falls outside base.
func SafeJoin(base, untrusted string) (string, error) {
	if base == "" {
		return "", ErrInvalidPath
	}

	absBase, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return "", ErrInvalidPath
	}

	// Resolve symlinks on base so the containment check is reliable.
	realBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		// Base doesn't exist — fall back to the cleaned absolute path.
		realBase = absBase
	}

	if untrusted == "" {
		return realBase, nil
	}

	// Reject absolute paths — untrusted input must be relative.
	if filepath.IsAbs(untrusted) {
		return "", ErrOutsideBase
	}

	joined := filepath.Join(realBase, filepath.Clean(untrusted))
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", ErrInvalidPath
	}

	// Resolve symlinks on the result when the path exists.
	realJoined := absJoined
	if _, statErr := os.Lstat(absJoined); statErr == nil {
		if resolved, evalErr := filepath.EvalSymlinks(absJoined); evalErr == nil {
			realJoined = resolved
		}
	}

	if !isWithin(realBase, realJoined) {
		return "", ErrOutsideBase
	}
	return absJoined, nil
}

// ValidateAbsPath checks that candidate is an absolute path residing
// within base. It returns the cleaned candidate or an error.
func ValidateAbsPath(base, candidate string) (string, error) {
	if base == "" || candidate == "" {
		return "", ErrInvalidPath
	}

	absBase, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return "", ErrInvalidPath
	}
	realBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		realBase = absBase
	}

	absCandidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", ErrInvalidPath
	}
	realCandidate := absCandidate
	if _, statErr := os.Lstat(absCandidate); statErr == nil {
		if resolved, evalErr := filepath.EvalSymlinks(absCandidate); evalErr == nil {
			realCandidate = resolved
		}
	}

	if !isWithin(realBase, realCandidate) {
		return "", ErrOutsideBase
	}
	return absCandidate, nil
}

// isWithin reports whether child is equal to or a descendant of base.
func isWithin(base, child string) bool {
	if child == base {
		return true
	}
	prefix := base + string(filepath.Separator)
	return strings.HasPrefix(child, prefix)
}
