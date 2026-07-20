package main

import "os/exec"

// openInExplorer opens the given folder in Windows Explorer.
func openInExplorer(path string) error {
	return exec.Command("explorer", path).Start()
}
