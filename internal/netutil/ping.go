package netutil

import (
	"runtime"
	"strconv"
	"strings"
	"time"

	"netwatch/internal/winexec"
)

// Ping reports whether ip answers an ICMP echo, using the OS `ping` command
// (no admin rights required, no raw sockets). Used to monitor liveness of
// hosts that have no scanned TCP ports. Best-effort: false on any error.
func Ping(ip string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = time.Second
	}
	ms := int(timeout / time.Millisecond)
	if ms < 300 {
		ms = 300
	}

	var cmd = winexec.Command("ping", "-c", "1", "-W", strconv.Itoa(maxInt(ms/1000, 1)), ip)
	if runtime.GOOS == "windows" {
		cmd = winexec.Command("ping", "-n", "1", "-w", strconv.Itoa(ms), ip)
	}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// A genuine reply contains "TTL=" (kept across locales). This rejects the
	// "Destination host unreachable" reply, which can still exit 0 on Windows.
	return strings.Contains(strings.ToLower(string(out)), "ttl=")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
