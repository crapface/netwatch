// Package report renders a standalone, self-contained HTML report (embedded CSS,
// no external files) summarizing a scan and its monitoring log.
package report

import (
	"html/template"
	"os"
	"time"

	"netwatch/internal/i18n"
	"netwatch/internal/model"
)

type hostRow struct {
	StatusText  string
	StatusClass string
	IP          string
	Hostname    string
	Label       string
	Vendor      string
	MAC         string
	Ports       string
	Notes       string
}

type eventRow struct {
	Time      string
	Host      string
	TypeText  string
	TypeClass string
	Detail    string
}

type reportData struct {
	Title      string
	GeneratedL string
	Generated  string
	SiteName   string
	NetworkL   string
	Network    string
	PortsL     string
	Ports      string
	ScanDateL  string
	ScanDate   string
	HostCountL string
	HostCount  int
	HostsH     string
	EventsH    string
	NoEvents   string
	Footer     string
	ColStatus  string
	ColIP      string
	ColHost    string
	ColLabel   string
	ColVendor  string
	ColMAC     string
	ColPorts   string
	ColNotes   string
	EvTime     string
	EvHost     string
	EvEvent    string
	EvDetail   string
	Hosts      []hostRow
	Events     []eventRow
	HasEvents  bool
}

func statusText(s string) (string, string) {
	switch s {
	case model.StatusUp:
		return i18n.T("status.up"), "up"
	case model.StatusDown:
		return i18n.T("status.down"), "down"
	default:
		return i18n.T("status.unknown"), "unknown"
	}
}

func portsList(ps []int) string {
	return model.Host{OpenPorts: ps}.PortsString()
}

// Generate renders the report for p and writes it to outPath. appVersion is
// shown in the footer.
func Generate(p *model.SiteProfile, outPath, appVersion string) error {
	d := reportData{
		Title:      i18n.T("report.title"),
		GeneratedL: i18n.T("report.generated"),
		Generated:  time.Now().Format("2006-01-02 15:04:05 MST"),
		SiteName:   p.Name,
		NetworkL:   i18n.T("report.network"),
		Network:    p.CIDR,
		PortsL:     i18n.T("report.ports"),
		Ports:      portsList(p.Ports),
		ScanDateL:  i18n.T("report.scan_date"),
		HostCountL: i18n.T("report.host_count"),
		HostCount:  len(p.Hosts),
		HostsH:     i18n.T("report.hosts_heading"),
		EventsH:    i18n.T("report.events_heading"),
		NoEvents:   i18n.T("report.no_events"),
		Footer:     i18n.Tf("report.footer", appVersion),
		ColStatus:  i18n.T("col.status"),
		ColIP:      i18n.T("col.ip"),
		ColHost:    i18n.T("col.hostname"),
		ColLabel:   i18n.T("col.label"),
		ColVendor:  i18n.T("col.vendor"),
		ColMAC:     i18n.T("col.mac"),
		ColPorts:   i18n.T("col.ports"),
		ColNotes:   i18n.T("col.notes"),
		EvTime:     i18n.T("evcol.time"),
		EvHost:     i18n.T("evcol.host"),
		EvEvent:    i18n.T("evcol.event"),
		EvDetail:   i18n.T("evcol.detail"),
	}
	if !p.LastScan.Date.IsZero() {
		d.ScanDate = p.LastScan.Date.Format("2006-01-02 15:04:05 MST")
	} else {
		d.ScanDate = "—"
	}

	for _, h := range p.Hosts {
		txt, cls := statusText(h.Status)
		d.Hosts = append(d.Hosts, hostRow{
			StatusText:  txt,
			StatusClass: cls,
			IP:          h.IP,
			Hostname:    h.Hostname,
			Label:       h.Label,
			Vendor:      h.Vendor,
			MAC:         h.MAC,
			Ports:       model.PortsLabeled(h.OpenPorts, p.PortLabels),
			Notes:       h.Notes,
		})
	}
	for _, e := range p.Events {
		host := e.IP
		if e.Hostname != "" {
			host = e.Hostname + " (" + e.IP + ")"
		}
		cls := "info"
		if e.Type == model.EventDown {
			cls = "down"
		} else if e.Type == model.EventUp {
			cls = "up"
		}
		d.Events = append(d.Events, eventRow{
			Time:      e.Time.Format("2006-01-02 15:04:05"),
			Host:      host,
			TypeText:  e.Type,
			TypeClass: cls,
			Detail:    e.Detail,
		})
	}
	d.HasEvents = len(d.Events) > 0

	tpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return tpl.Execute(f, d)
}

// Self-contained HTML. Font sizes >= 14px, zebra rows, colored status pills,
// long values wrap (readability rules carried from the predecessor project).
const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} — {{.SiteName}}</title>
<style>
  :root{ --bg:#0f1721; --panel:#16212e; --line:#243446; --txt:#e6edf3; --muted:#9fb0c0;
         --up:#22d37c; --down:#ff5d5d; --unknown:#8aa0b4; --accent:#4aa3ff; }
  *{ box-sizing:border-box; }
  body{ margin:0; background:var(--bg); color:var(--txt);
        font:15px/1.55 "Segoe UI",system-ui,Arial,sans-serif; padding:28px; }
  h1{ font-size:24px; margin:0 0 4px; }
  h2{ font-size:19px; margin:30px 0 12px; border-bottom:1px solid var(--line); padding-bottom:6px; }
  .sub{ color:var(--muted); font-size:14px; }
  .meta{ display:flex; flex-wrap:wrap; gap:10px 26px; background:var(--panel);
         border:1px solid var(--line); border-radius:10px; padding:14px 18px; margin-top:16px; }
  .meta div{ font-size:14px; }
  .meta b{ color:var(--muted); font-weight:600; display:block; font-size:12px; text-transform:uppercase; letter-spacing:.04em; }
  table{ width:100%; border-collapse:collapse; margin-top:10px; font-size:14px; }
  th,td{ text-align:left; padding:9px 12px; border-bottom:1px solid var(--line);
         vertical-align:top; word-break:break-word; }
  th{ color:var(--muted); font-size:12px; text-transform:uppercase; letter-spacing:.04em; }
  tbody tr:nth-child(even){ background:rgba(255,255,255,.025); }
  .pill{ display:inline-block; padding:2px 10px; border-radius:999px; font-size:12px; font-weight:700; }
  .pill.up{ background:rgba(34,211,124,.15); color:var(--up); }
  .pill.down{ background:rgba(255,93,93,.15); color:var(--down); }
  .pill.unknown{ background:rgba(138,160,180,.15); color:var(--unknown); }
  .pill.info{ background:rgba(74,163,255,.15); color:var(--accent); }
  code{ font-family:Consolas,"Courier New",monospace; }
  footer{ margin-top:34px; color:var(--muted); font-size:12px; border-top:1px solid var(--line); padding-top:10px; }
</style>
</head>
<body>
  <h1>{{.Title}}</h1>
  <div class="sub">{{.SiteName}}</div>
  <div class="meta">
    <div><b>{{.GeneratedL}}</b>{{.Generated}}</div>
    <div><b>{{.ScanDateL}}</b>{{.ScanDate}}</div>
    <div><b>{{.NetworkL}}</b><code>{{.Network}}</code></div>
    <div><b>{{.PortsL}}</b><code>{{.Ports}}</code></div>
    <div><b>{{.HostCountL}}</b>{{.HostCount}}</div>
  </div>

  <h2>{{.HostsH}}</h2>
  <table>
    <thead><tr>
      <th>{{.ColStatus}}</th><th>{{.ColIP}}</th><th>{{.ColHost}}</th><th>{{.ColLabel}}</th>
      <th>{{.ColVendor}}</th><th>{{.ColMAC}}</th><th>{{.ColPorts}}</th><th>{{.ColNotes}}</th>
    </tr></thead>
    <tbody>
    {{range .Hosts}}
      <tr>
        <td><span class="pill {{.StatusClass}}">{{.StatusText}}</span></td>
        <td><code>{{.IP}}</code></td>
        <td>{{.Hostname}}</td>
        <td>{{.Label}}</td>
        <td>{{.Vendor}}</td>
        <td><code>{{.MAC}}</code></td>
        <td><code>{{.Ports}}</code></td>
        <td>{{.Notes}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>

  <h2>{{.EventsH}}</h2>
  {{if .HasEvents}}
  <table>
    <thead><tr>
      <th>{{.EvTime}}</th><th>{{.EvHost}}</th><th>{{.EvEvent}}</th><th>{{.EvDetail}}</th>
    </tr></thead>
    <tbody>
    {{range .Events}}
      <tr>
        <td><code>{{.Time}}</code></td>
        <td>{{.Host}}</td>
        <td><span class="pill {{.TypeClass}}">{{.TypeText}}</span></td>
        <td>{{.Detail}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>
  {{else}}
  <p class="sub">{{.NoEvents}}</p>
  {{end}}

  <footer>{{.Footer}}</footer>
</body>
</html>
`
