package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"netwatch/internal/model"
)

func TestGenerate(t *testing.T) {
	p := model.DefaultProfile()
	p.Name = "HQ"
	p.CIDR = "192.168.0.0/24"
	p.Ports = []int{8080, 3000}
	p.PortLabels = map[int]string{8080: "Door Controller"}
	p.LastScan = model.ScanInfo{Date: time.Now(), CIDR: p.CIDR, Ports: p.Ports}
	p.Hosts = []model.Host{{
		ID: "ip:192.168.0.85", IP: "192.168.0.85", Hostname: "door1",
		Label: "Front Door", Vendor: "Acme", MAC: "AA:BB:CC:DD:EE:FF",
		OpenPorts: []int{8080}, Status: model.StatusUp, Notes: "lobby reader",
	}}
	p.Events = []model.MonitorEvent{{Time: time.Now(), IP: "192.168.0.85", Type: model.EventDown, Detail: "no response"}}

	path := filepath.Join(t.TempDir(), "r.html")
	if err := Generate(p, path, "1.1.0"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	html := string(b)
	for _, want := range []string{
		"--bg:#ffffff",           // light theme
		"@media print",           // printer-friendly
		"Front Door",             // label rendered
		"lobby reader",           // notes rendered
		"8080 (Door Controller)", // port label rendered
		`class="pill up"`,        // status pill
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report HTML missing %q", want)
		}
	}
}
