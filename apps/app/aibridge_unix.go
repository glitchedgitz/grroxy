//go:build !windows

package app

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the bridge in its own process group so we can signal the
// bridge and the claude/codex CLI it spawns as a unit.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree kills the bridge along with its descendants.
func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// Only signal the group if the child actually leads one — otherwise a
	// negative PID would target our own group and take grroxy down with it.
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil && pgid == cmd.Process.Pid {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	_ = cmd.Process.Kill()
}
