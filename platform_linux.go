//go:build linux

package main

import "os/exec"

// openInExplorer opens the given folder in the system file manager.
func openInExplorer(path string) error {
	return exec.Command("xdg-open", path).Start()
}
