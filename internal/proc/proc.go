// Package proc starts child processes as a group, so a timeout or Ctrl-C
// stops the whole tree. Without it only the top process (usually bash) dies:
// its children keep running and hold the output pipes open, so Wait blocks
// until they finish on their own.
package proc

import (
	"os/exec"
	"time"
)

// waitDelay bounds how long Wait waits for output after the group is killed.
const waitDelay = 5 * time.Second

// Group makes cmd the leader of a new process group and, when its context
// ends, kills the whole group instead of only cmd.
func Group(cmd *exec.Cmd) {
	setGroup(cmd)
	cmd.Cancel = func() error { return Kill(cmd) }
	cmd.WaitDelay = waitDelay
}

// Kill stops cmd and every process it started. It is safe to call before
// the command started or after it ended.
func Kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return killGroup(cmd)
}
