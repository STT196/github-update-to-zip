//go:build darwin

package main

import "os/exec"

// openInExplorer opens the given folder in Finder.
func openInExplorer(path string) error {
	return exec.Command("open", path).Start()
}
