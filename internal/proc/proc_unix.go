//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

func setGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func killGroup(cmd *exec.Cmd) error {
	// A negative pid signals the whole group. Fall back to the process itself
	// when it is not a group leader (Group was not called).
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
