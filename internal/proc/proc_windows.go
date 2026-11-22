//go:build windows

package proc

import "os/exec"

// Windows has no process groups in this sense; TaskWing needs bash, so
// Windows users run it under WSL, where the unix version applies.
func setGroup(cmd *exec.Cmd) {}

func killGroup(cmd *exec.Cmd) error { return cmd.Process.Kill() }
