package netutil

import (
	"regexp"
	"strings"

	"netwatch/internal/winexec"
)

var (
	reIP  = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)
	reMAC = regexp.MustCompile(`\b(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}\b`)
)

// ARPTable returns ip -> normalized MAC ("AA:BB:CC:DD:EE:FF") from the OS ARP
// cache by parsing `arp -a`. Best-effort: an empty map is returned on any error,
// and entries only exist for hosts on the same layer-2 segment that have been
// contacted (the scan's TCP connects populate the cache). Graceful by design.
func ARPTable() map[string]string {
	out := map[string]string{}
	b, err := winexec.Command("arp", "-a").Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		ip := reIP.FindString(line)
		mac := reMAC.FindString(line)
		if ip == "" || mac == "" {
			continue
		}
		if m := NormalizeMAC(mac); m != "" {
			out[ip] = m
		}
	}
	return out
}

func hexOnly(s string) string {
	b := make([]byte, 0, 12)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			b = append(b, c)
		}
	}
	return strings.ToUpper(string(b))
}

// NormalizeMAC converts any MAC notation to upper "AA:BB:CC:DD:EE:FF".
func NormalizeMAC(mac string) string {
	s := hexOnly(mac)
	if len(s) < 12 {
		return ""
	}
	s = s[:12]
	parts := make([]string, 0, 6)
	for i := 0; i < 12; i += 2 {
		parts = append(parts, s[i:i+2])
	}
	return strings.Join(parts, ":")
}

// OUIKey returns the first 6 hex chars of a MAC (the IEEE OUI prefix).
func OUIKey(mac string) string {
	s := hexOnly(mac)
	if len(s) < 6 {
		return ""
	}
	return s[:6]
}
