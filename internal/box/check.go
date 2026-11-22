package box

import (
	"errors"
	"os/exec"
)

// Check finds a setup problem before a run spends time and tokens on a plan:
// a missing boat CLI or no base box to fork.
func (p *Boat) Check() error {
	if _, err := exec.LookPath(p.bin()); err != nil {
		return errors.New("the boat CLI is not installed (or not on PATH). Install it, or try TaskWing on this machine first: --provider local --no-pr")
	}
	if p.Base == "" {
		return errors.New("no boat base box set: run taskwing config boat-base <id> (how to make one: docs/guide.md)")
	}
	return nil
}

// Check has nothing to find for local folders.
func (p *Local) Check() error { return nil }
