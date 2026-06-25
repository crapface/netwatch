package profile

import (
	"path/filepath"
	"testing"
	"time"

	"netwatch/internal/model"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	p := model.DefaultProfile()
	p.Name = "HQ"
	p.CIDR = "10.0.0.0/24"
	p.Hosts = []model.Host{{ID: "a", IP: "10.0.0.5", OpenPorts: []int{22, 443}, Status: model.StatusUp}}
	p.Events = []model.MonitorEvent{{Time: time.Now(), HostID: "a", IP: "10.0.0.5", Type: model.EventDown}}
	p.Monitoring.Running = true

	path := filepath.Join(t.TempDir(), "hq.site")
	if err := Save(path, p, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.Name != "HQ" || q.CIDR != "10.0.0.0/24" {
		t.Errorf("scalar mismatch: %+v", q)
	}
	if len(q.Hosts) != 1 || q.Hosts[0].IP != "10.0.0.5" || len(q.Hosts[0].OpenPorts) != 2 {
		t.Errorf("hosts mismatch: %+v", q.Hosts)
	}
	if len(q.Events) != 1 || q.Events[0].Type != model.EventDown {
		t.Errorf("events mismatch: %+v", q.Events)
	}
	if !q.Monitoring.Running {
		t.Errorf("monitoring.running not preserved")
	}
	if q.AppVersion != "1.0.0" {
		t.Errorf("app version not stamped: %q", q.AppVersion)
	}
}

func TestLoadInvalid(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.site")); err == nil {
		t.Errorf("expected error loading nonexistent file")
	}
}
