//go:build windows

// Package winexec runs child processes without flashing a console window
// and opens files/URLs in the default browser. Windows implementation.
package winexec

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW

// Command builds an *exec.Cmd whose console window is hidden.
func Command(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return c
}

// OpenBrowser opens a file path or URL with the default handler.
func OpenBrowser(target string) error {
	// rundll32 url.dll handles both local files and http(s) URLs reliably.
	return Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
}
