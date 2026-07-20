package main

import (
	"os/exec"
	"syscall"
)

// openInExplorer opens the given folder in Windows Explorer.
func openInExplorer(path string) error {
	cmd := exec.Command("explorer", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
