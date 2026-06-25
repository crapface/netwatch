// NetWatch — a portable Windows LAN scanner & monitor.
//
// Everything (logs, OUI cache, profiles) lives next to the executable, so the
// app is fully copy-and-paste portable: no installer, no registry, no %APPDATA%.
package main

import (
	"os"
	"path/filepath"

	"netwatch/internal/applog"
	"netwatch/internal/ui"
)

// version is stamped into reports and saved .site files. Bump on every change.
const version = "1.1.0"

// appDir returns the directory containing the executable (falls back to cwd
// when run via `go run`).
func appDir() string {
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); dir != "" {
			return dir
		}
	}
	wd, _ := os.Getwd()
	return wd
}

func main() {
	dir := appDir()
	_ = applog.Init(dir)
	applog.Info("app: NetWatch %s starting (dir=%s)", version, dir)
	defer applog.Close()

	if err := ui.Run(dir, version); err != nil {
		applog.Error("app: fatal: %v", err)
		os.Exit(1)
	}
	applog.Info("app: stopped")
}
