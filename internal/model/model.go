// Package model holds the shared data types used across NetWatch.
// Keeping them in one dependency-free package avoids import cycles between
// the scanner, monitor, profile (.site) serializer, HTML report and GUI.
package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Monitoring status values for a host.
const (
	StatusUnknown = "unknown"
	StatusUp      = "up"
	StatusDown    = "down"
)

// Event types written to the monitoring log.
const (
	EventUp   = "UP"
	EventDown = "DOWN"
	EventInfo = "INFO"
)

// Host is a single discovered device on the network.
type Host struct {
	ID        string `json:"id"`         // stable unique id (MAC if known, else "ip:<addr>")
	IP        string `json:"ip"`         // IPv4 address
	Hostname  string `json:"hostname"`   // reverse-DNS name (may be empty)
	MAC       string `json:"mac"`        // colon-separated MAC (may be empty for routed hosts)
	Vendor    string `json:"vendor"`     // OUI vendor (may be empty)
	OpenPorts []int  `json:"open_ports"` // ports that accepted a TCP connection
	Status    string `json:"status"`     // monitoring status: up|down|unknown

	// Monitor runtime state — persisted so a saved Site can resume cleanly.
	ConsecutiveMisses int       `json:"consecutive_misses"`
	AlertedDown       bool      `json:"alerted_down"` // DOWN alert already sent (debounce)
	LastChange        time.Time `json:"last_change,omitempty"`
}

// PortsString renders the open-port list as "80, 443, 8080".
func (h Host) PortsString() string {
	if len(h.OpenPorts) == 0 {
		return ""
	}
	parts := make([]string, len(h.OpenPorts))
	for i, p := range h.OpenPorts {
		parts[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(parts, ", ")
}

// EmailConfig is the SMTP configuration stored in the Site profile.
// NOTE: Password is stored in plain text by design (the user is warned in the UI).
type EmailConfig struct {
	Enabled            bool   `json:"enabled"`
	Server             string `json:"server"`
	Port               int    `json:"port"`
	TLSMode            string `json:"tls_mode"` // none | starttls | ssl
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	Username           string `json:"username"`
	Password           string `json:"password"` // PLAIN TEXT — documented warning
	From               string `json:"from"`
	To                 string `json:"to"`
}

// MonitorEvent is a single UP/DOWN/INFO transition in the monitoring log.
type MonitorEvent struct {
	Time     time.Time `json:"time"`
	HostID   string    `json:"host_id"`
	IP       string    `json:"ip"`
	Hostname string    `json:"hostname"`
	Type     string    `json:"type"` // UP | DOWN | INFO
	Detail   string    `json:"detail"`
}

// ScanInfo records metadata about the most recent scan.
type ScanInfo struct {
	Date  time.Time `json:"date"`
	CIDR  string    `json:"cidr"`
	Ports []int     `json:"ports"`
	Count int       `json:"count"`
}

// MonitorState captures whether monitoring was running and when it last checked.
type MonitorState struct {
	Running   bool      `json:"running"`
	LastCheck time.Time `json:"last_check"`
}

// SiteProfile is the entire saved state of a "Site" (.site JSON file).
type SiteProfile struct {
	SchemaVersion string         `json:"schema_version"`
	AppVersion    string         `json:"app_version"`
	Name          string         `json:"name"`
	CIDR          string         `json:"cidr"`
	Ports         []int          `json:"ports"`
	Email         EmailConfig    `json:"email"`
	Hosts         []Host         `json:"hosts"`
	Events        []MonitorEvent `json:"events"`
	Monitoring    MonitorState   `json:"monitoring"`
	LastScan      ScanInfo       `json:"last_scan"`
	Concurrency   int            `json:"concurrency"`
	TimeoutMs     int            `json:"timeout_ms"`
	Retries       int            `json:"retries"`
	IntervalSec   int            `json:"interval_sec"`
	Language      string         `json:"language"`
}

// DefaultProfile returns a fresh profile with sensible defaults.
func DefaultProfile() *SiteProfile {
	return &SiteProfile{
		SchemaVersion: "1",
		Name:          "Untitled",
		CIDR:          "",
		Ports:         []int{8080, 3000},
		Email: EmailConfig{
			Port:    587,
			TLSMode: "starttls",
		},
		Hosts:       []Host{},
		Events:      []MonitorEvent{},
		Concurrency: 100,
		TimeoutMs:   1000,
		Retries:     1,
		IntervalSec: 60,
		Language:    "en",
	}
}

// NormalizePorts de-duplicates and sorts a port list, dropping out-of-range values.
func NormalizePorts(ports []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, p := range ports {
		if p < 1 || p > 65535 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}
