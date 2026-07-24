//go:build !windows

package gitdiff

import "os/exec"

func hideWindow(cmd *exec.Cmd) {
	// no-op on non-Windows platforms
}
