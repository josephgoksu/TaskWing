package engine

import (
	"os"
	"testing"
)

// The tests make real git repositories. A developer's global git config
// (commit signing, hooks, templates) must not change what they see.
func TestMain(m *testing.M) {
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Exit(m.Run())
}
