//go:build windows

package app

import (
	"os/exec"
	"strconv"
)

// setProcessGroup is a no-op on Windows; process-tree teardown is handled by
// taskkill in killProcessTree instead.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessTree kills the bridge along with its descendants. Mirrors the
// taskkill approach the Electron wrapper already uses for grroxy itself.
func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	if err := kill.Run(); err != nil {
		_ = cmd.Process.Kill()
	}
}
