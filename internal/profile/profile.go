// Package profile saves and loads the entire app state as a ".site" JSON file.
package profile

import (
	"encoding/json"
	"fmt"
	"os"

	"netwatch/internal/model"
)

// SchemaVersion identifies the on-disk format.
const SchemaVersion = "1"

// Save writes the profile to path as indented JSON, stamping schema/app version.
func Save(path string, p *model.SiteProfile, appVersion string) error {
	p.SchemaVersion = SchemaVersion
	p.AppVersion = appVersion
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads a .site file, filling defaults for any missing/invalid fields so
// older or hand-edited files still open cleanly.
func Load(path string) (*model.SiteProfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := model.DefaultProfile()
	if err := json.Unmarshal(b, p); err != nil {
		return nil, fmt.Errorf("not a valid .site file: %w", err)
	}
	if p.Concurrency <= 0 {
		p.Concurrency = 100
	}
	if p.TimeoutMs <= 0 {
		p.TimeoutMs = 1000
	}
	if p.Retries < 0 {
		p.Retries = 0
	}
	if p.IntervalSec <= 0 {
		p.IntervalSec = 60
	}
	if len(p.Ports) == 0 {
		p.Ports = []int{8080, 3000}
	}
	if p.Language == "" {
		p.Language = "en"
	}
	if p.Hosts == nil {
		p.Hosts = []model.Host{}
	}
	if p.Events == nil {
		p.Events = []model.MonitorEvent{}
	}
	if p.PortLabels == nil {
		p.PortLabels = map[int]string{}
	}
	return p, nil
}
