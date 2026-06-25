//go:build !windows

// Package winexec — non-Windows fallback so the project compiles and vets
// on the build host (Linux/macOS). Behaviour mirrors the Windows version.
package winexec

import "os/exec"

// Command builds a plain *exec.Cmd (no window hiding needed off-Windows).
func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// OpenBrowser opens a file path or URL using the desktop default opener.
func OpenBrowser(target string) error {
	return exec.Command("xdg-open", target).Start()
}
