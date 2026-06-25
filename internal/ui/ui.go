// Package ui builds and drives the NetWatch GUI using lxn/walk (native Win32
// widgets). All long-running work (scan, monitor, OUI download, email) runs in
// goroutines; UI mutations are marshaled back with mw.Synchronize.
package ui

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder for the embedded About image
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"netwatch/internal/applog"
	"netwatch/internal/i18n"
	"netwatch/internal/mailer"
	"netwatch/internal/model"
	"netwatch/internal/monitor"
	"netwatch/internal/netutil"
	"netwatch/internal/oui"
	"netwatch/internal/profile"
	"netwatch/internal/report"
	"netwatch/internal/scan"
	"netwatch/internal/updates"
	"netwatch/internal/winexec"
)

//go:embed assets/about.jpg
var aboutJPEG []byte

// resumeText is the sanitized professional bio shown in the About dialog.
// Kept free of internal hostnames/IPs per the public-repo policy.
const resumeText = `Eric Pezoa
Senior Systems Engineer — Location Intelligence Group / Digital & TECH, CBRE
Phoenix, Arizona

SUMMARY
Senior systems engineer specializing in enterprise GIS, cloud infrastructure,
and automation tooling. I design, deploy and operate high-availability ArcGIS
Enterprise and ArcGIS Online environments, and build the internal tools that
make large geospatial platforms observable and maintainable.

CORE SKILLS
• ArcGIS Enterprise (Portal / Server / Data Store, HA) and ArcGIS Online admin
• Microsoft Azure — CLI-driven ops, Application Gateway, Key Vault, PostgreSQL
• SAML / Microsoft Entra ID federation
• Automation & tooling in Go and PowerShell; SQLite; single-file web dashboards
• Model Context Protocol (MCP) servers and agentic workflows
• Networking, monitoring and security hardening

SELECTED WORK
• Operate an ArcGIS Enterprise 11.5 high-availability portal for the enterprise.
• Built internal troubleshooting and metrics tools — single-file HTML with OAuth
  and versioned changelogs; Go + SQLite dashboards (Chart.js).
• Authored automation and MCP services that wrap internal APIs for self-service.
• NetWatch (this app) — a portable Windows LAN scanner & monitor (Go + lxn/walk).

LINKS
GitHub: github.com/crapface
Full bio: lakes.pezoa.com/about`

// App holds all GUI state.
type App struct {
	version  string
	appDir   string
	ouiCache string

	mw   *walk.MainWindow
	tabs *walk.TabWidget

	hostModel  *HostModel
	eventModel *EventModel
	prof       *model.SiteProfile
	mon        *monitor.Monitor

	scanMu     sync.Mutex
	scanning   bool
	scanCancel context.CancelFunc

	flashStop chan struct{}
	brYellow  *walk.SolidColorBrush
	brWhite   *walk.SolidColorBrush

	suppressLang bool
	monStopped   bool

	w widgets
}

// widgets keeps references to every control that must be retranslated or read.
type widgets struct {
	// toolbar
	tbSave    *walk.PushButton
	tbLoad    *walk.PushButton
	tbReport  *walk.PushButton
	tbAbout   *walk.PushButton
	tbUpdate  *walk.PushButton
	langLabel *walk.Label
	langCombo *walk.ComboBox

	tabScanner  *walk.TabPage
	tabMonitor  *walk.TabPage
	tabSettings *walk.TabPage

	rangeLabel    *walk.Label
	rangeEdit     *walk.LineEdit
	btnDetect     *walk.PushButton
	btnScan       *walk.PushButton
	btnCancel     *walk.PushButton
	scanProgress  *walk.ProgressBar
	scanStatus    *walk.Label
	hostTV        *walk.TableView
	btnAddHost    *walk.PushButton
	btnEditHost   *walk.PushButton
	btnRemoveHost *walk.PushButton
	scanHint      *walk.Label
	flashComp     *walk.Composite
	btnStartMon   *walk.PushButton

	monTV        *walk.TableView
	btnStopMon   *walk.PushButton
	monSummary   *walk.Label
	monStatus    *walk.Label
	monEventsLbl *walk.Label
	eventTV      *walk.TableView

	gbPorts       *walk.GroupBox
	portsHint     *walk.Label
	portsList     *walk.ListBox
	portFieldLbl  *walk.Label
	portEdit      *walk.NumberEdit
	portLabelLbl  *walk.Label
	portLabelEdit *walk.LineEdit
	btnAddPort    *walk.PushButton
	btnRemPort    *walk.PushButton

	gbScan       *walk.GroupBox
	concLabel    *walk.Label
	concEdit     *walk.NumberEdit
	timeoutLabel *walk.Label
	timeoutEdit  *walk.NumberEdit
	retriesLabel *walk.Label
	retriesEdit  *walk.NumberEdit

	gbEmail      *walk.GroupBox
	emailEnabled *walk.CheckBox
	serverLabel  *walk.Label
	server       *walk.LineEdit
	portLabel    *walk.Label
	smtpPort     *walk.NumberEdit
	tlsLabel     *walk.Label
	tlsCombo     *walk.ComboBox
	skipVerify   *walk.CheckBox
	userLabel    *walk.Label
	username     *walk.LineEdit
	passLabel    *walk.Label
	password     *walk.LineEdit
	fromLabel    *walk.Label
	from         *walk.LineEdit
	toLabel      *walk.Label
	to           *walk.LineEdit
	btnTestEmail *walk.PushButton
	warnLabel    *walk.Label

	gbMon         *walk.GroupBox
	intervalLabel *walk.Label
	intervalEdit  *walk.NumberEdit

	gbOUI        *walk.GroupBox
	btnUpdateOUI *walk.PushButton
	btnReresolve *walk.PushButton
	ouiStatus    *walk.Label

	gbProfile     *walk.GroupBox
	siteNameLabel *walk.Label
	siteName      *walk.LineEdit
	btnSave       *walk.PushButton
	btnLoad       *walk.PushButton
	btnReport     *walk.PushButton
}

// Run builds the window and runs the message loop until the window closes.
func Run(appDir, version string) error {
	a := &App{
		version:    version,
		appDir:     appDir,
		ouiCache:   filepath.Join(appDir, "oui_cache.json"),
		hostModel:  NewHostModel(),
		eventModel: NewEventModel(),
		prof:       model.DefaultProfile(),
	}
	i18n.SetLang(a.prof.Language)

	if err := a.build().Create(); err != nil {
		return err
	}
	a.postCreate()
	a.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) { a.shutdown() })
	a.mw.Run()
	return nil
}

func hostColumns() []TableViewColumn {
	return []TableViewColumn{
		{Title: "Status", Width: 110},
		{Title: "IP", Width: 110},
		{Title: "Hostname", Width: 150},
		{Title: "Label", Width: 140},
		{Title: "Vendor", Width: 150},
		{Title: "MAC", Width: 130},
		{Title: "Ports", Width: 150},
		{Title: "Notes", Width: 150},
		{Title: "ID", Width: 110},
	}
}

func eventColumns() []TableViewColumn {
	return []TableViewColumn{
		{Title: "Time", Width: 150},
		{Title: "Host", Width: 240},
		{Title: "Event", Width: 80},
		{Title: "Detail", Width: 360},
	}
}

func (a *App) build() MainWindow {
	codes := i18n.Codes()
	langNames := make([]string, len(codes))
	for i, c := range codes {
		langNames[i] = i18n.DisplayName(c)
	}
	return MainWindow{
		AssignTo: &a.mw,
		Title:    i18n.T("app.title"),
		MinSize:  Size{Width: 980, Height: 640},
		Size:     Size{Width: 1140, Height: 760},
		Layout:   VBox{},
		Children: []Widget{
			// Always-visible toolbar: Save / Load / Report / About + language.
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{AssignTo: &a.w.tbSave, Text: "Save Site", OnClicked: a.onSave},
					PushButton{AssignTo: &a.w.tbLoad, Text: "Load Site", OnClicked: a.onLoad},
					PushButton{AssignTo: &a.w.tbReport, Text: "Generate Report", OnClicked: a.onReport},
					PushButton{AssignTo: &a.w.tbAbout, Text: "About", OnClicked: a.onAbout},
					PushButton{AssignTo: &a.w.tbUpdate, Text: "Check for Updates", OnClicked: a.onCheckUpdates},
					HSpacer{},
					Label{AssignTo: &a.w.langLabel, Text: "Language:"},
					ComboBox{
						AssignTo:              &a.w.langCombo,
						Editable:              false,
						Model:                 langNames,
						OnCurrentIndexChanged: a.onLangChanged,
						MinSize:               Size{Width: 150},
						MaxSize:               Size{Width: 180},
					},
				},
			},
			TabWidget{
				AssignTo: &a.tabs,
				Pages: []TabPage{
					a.scannerPage(),
					a.monitorPage(),
					a.settingsPage(),
				},
			},
		},
	}
}

func (a *App) scannerPage() TabPage {
	return TabPage{
		AssignTo: &a.w.tabScanner,
		Title:    "Scanner",
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{},
				Children: []Widget{
					Label{AssignTo: &a.w.rangeLabel, Text: "Network range (CIDR):"},
					LineEdit{AssignTo: &a.w.rangeEdit, MinSize: Size{Width: 180}},
					PushButton{AssignTo: &a.w.btnDetect, Text: "Detect", OnClicked: a.onDetect},
					PushButton{AssignTo: &a.w.btnScan, Text: "Scan", OnClicked: a.onScan},
					PushButton{AssignTo: &a.w.btnCancel, Text: "Cancel", Enabled: false, OnClicked: a.onCancel},
					HSpacer{},
				},
			},
			ProgressBar{AssignTo: &a.w.scanProgress},
			Label{AssignTo: &a.w.scanStatus, Text: "Idle."},
			TableView{
				AssignTo:         &a.w.hostTV,
				Model:            a.hostModel,
				Columns:          hostColumns(),
				StyleCell:        a.styleHostCell,
				ColumnsOrderable: true,
				OnItemActivated:  a.onEditHost, // double-click / Enter to edit details
				MinSize:          Size{Height: 250},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{AssignTo: &a.w.btnAddHost, Text: "Add host", OnClicked: a.onAddHost},
					PushButton{AssignTo: &a.w.btnEditHost, Text: "Edit host", OnClicked: a.onEditHost},
					PushButton{AssignTo: &a.w.btnRemoveHost, Text: "Remove host", OnClicked: a.onRemoveHost},
					HSpacer{},
				},
			},
			Label{AssignTo: &a.w.scanHint, Text: ""},
			Composite{
				AssignTo: &a.w.flashComp,
				Visible:  false,
				Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
				Children: []Widget{
					PushButton{
						AssignTo:  &a.w.btnStartMon,
						Text:      "START MONITORING",
						MinSize:   Size{Height: 52},
						Font:      Font{PointSize: 12, Bold: true},
						OnClicked: a.onStartMon,
					},
				},
			},
		},
	}
}

func (a *App) monitorPage() TabPage {
	return TabPage{
		AssignTo: &a.w.tabMonitor,
		Title:    "Monitor",
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{AssignTo: &a.w.btnStopMon, Text: "Stop Monitoring", Enabled: false, OnClicked: a.onStopMon},
					HSpacer{},
					Label{AssignTo: &a.w.monSummary, Text: ""},
				},
			},
			Label{AssignTo: &a.w.monStatus, Text: ""},
			TableView{
				AssignTo:        &a.w.monTV,
				Model:           a.hostModel,
				Columns:         hostColumns(),
				StyleCell:       a.styleHostCell,
				OnItemActivated: a.onEditHostMon, // double-click / Enter to edit details
				MinSize:         Size{Height: 220},
			},
			Label{AssignTo: &a.w.monEventsLbl, Text: "Monitoring event log"},
			TableView{
				AssignTo: &a.w.eventTV,
				Model:    a.eventModel,
				Columns:  eventColumns(),
				MinSize:  Size{Height: 150},
			},
		},
	}
}

func (a *App) settingsPage() TabPage {
	return TabPage{
		AssignTo: &a.w.tabSettings,
		Title:    "Settings",
		Layout:   VBox{},
		Children: []Widget{
			ScrollView{
				Layout: VBox{},
				Children: []Widget{
					GroupBox{
						AssignTo: &a.w.gbPorts,
						Title:    "Scan ports",
						Layout:   VBox{},
						Children: []Widget{
							Label{AssignTo: &a.w.portsHint, Text: ""},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									ListBox{AssignTo: &a.w.portsList, MinSize: Size{Width: 220, Height: 110}},
									Composite{
										Layout: VBox{},
										Children: []Widget{
											Composite{
												Layout: HBox{MarginsZero: true},
												Children: []Widget{
													Label{AssignTo: &a.w.portFieldLbl, Text: "Port:"},
													NumberEdit{AssignTo: &a.w.portEdit, Decimals: 0, MinValue: 1, MaxValue: 65535, MinSize: Size{Width: 90}},
													Label{AssignTo: &a.w.portLabelLbl, Text: "Label:"},
													LineEdit{AssignTo: &a.w.portLabelEdit, MinSize: Size{Width: 160}},
													PushButton{AssignTo: &a.w.btnAddPort, Text: "Add", OnClicked: a.onAddPort},
												},
											},
											PushButton{AssignTo: &a.w.btnRemPort, Text: "Remove selected", OnClicked: a.onRemovePort},
											VSpacer{},
										},
									},
									HSpacer{},
								},
							},
						},
					},
					GroupBox{
						AssignTo: &a.w.gbScan,
						Title:    "Scan tuning",
						Layout:   Grid{Columns: 2},
						Children: []Widget{
							Label{AssignTo: &a.w.concLabel, Text: "Max concurrency:"},
							NumberEdit{AssignTo: &a.w.concEdit, Decimals: 0, MinValue: 1, MaxValue: 2000},
							Label{AssignTo: &a.w.timeoutLabel, Text: "Connect timeout (ms):"},
							NumberEdit{AssignTo: &a.w.timeoutEdit, Decimals: 0, MinValue: 50, MaxValue: 60000},
							Label{AssignTo: &a.w.retriesLabel, Text: "Retries on timeout:"},
							NumberEdit{AssignTo: &a.w.retriesEdit, Decimals: 0, MinValue: 0, MaxValue: 10},
						},
					},
					GroupBox{
						AssignTo: &a.w.gbEmail,
						Title:    "Email notifications (SMTP)",
						Layout:   Grid{Columns: 2},
						Children: []Widget{
							CheckBox{AssignTo: &a.w.emailEnabled, Text: "Enable email alerts", ColumnSpan: 2},
							Label{AssignTo: &a.w.serverLabel, Text: "SMTP server:"},
							LineEdit{AssignTo: &a.w.server},
							Label{AssignTo: &a.w.portLabel, Text: "Port:"},
							NumberEdit{AssignTo: &a.w.smtpPort, Decimals: 0, MinValue: 1, MaxValue: 65535},
							Label{AssignTo: &a.w.tlsLabel, Text: "Encryption:"},
							ComboBox{AssignTo: &a.w.tlsCombo, Editable: false},
							CheckBox{AssignTo: &a.w.skipVerify, Text: "Skip TLS verify", ColumnSpan: 2},
							Label{AssignTo: &a.w.userLabel, Text: "Username:"},
							LineEdit{AssignTo: &a.w.username},
							Label{AssignTo: &a.w.passLabel, Text: "Password:"},
							LineEdit{AssignTo: &a.w.password, PasswordMode: true},
							Label{AssignTo: &a.w.fromLabel, Text: "From:"},
							LineEdit{AssignTo: &a.w.from},
							Label{AssignTo: &a.w.toLabel, Text: "Recipient:"},
							LineEdit{AssignTo: &a.w.to},
							Composite{
								ColumnSpan: 2,
								Layout:     HBox{MarginsZero: true},
								Children: []Widget{
									PushButton{AssignTo: &a.w.btnTestEmail, Text: "Test Email", OnClicked: a.onTestEmail},
									HSpacer{},
								},
							},
							Label{AssignTo: &a.w.warnLabel, Text: "", ColumnSpan: 2, TextColor: walk.RGB(200, 60, 60)},
						},
					},
					GroupBox{
						AssignTo: &a.w.gbMon,
						Title:    "Monitoring",
						Layout:   Grid{Columns: 2},
						Children: []Widget{
							Label{AssignTo: &a.w.intervalLabel, Text: "Check interval (seconds):"},
							NumberEdit{AssignTo: &a.w.intervalEdit, Decimals: 0, MinValue: 5, MaxValue: 86400},
						},
					},
					GroupBox{
						AssignTo: &a.w.gbOUI,
						Title:    "Vendor database (IEEE OUI)",
						Layout:   HBox{},
						Children: []Widget{
							PushButton{AssignTo: &a.w.btnUpdateOUI, Text: "Update OUI Data", OnClicked: a.onUpdateOUI},
							PushButton{AssignTo: &a.w.btnReresolve, Text: "Re-resolve vendors", OnClicked: a.onReresolve},
							Label{AssignTo: &a.w.ouiStatus, Text: ""},
							HSpacer{},
						},
					},
					GroupBox{
						AssignTo: &a.w.gbProfile,
						Title:    "Site profile",
						Layout:   VBox{},
						Children: []Widget{
							Composite{
								Layout: HBox{},
								Children: []Widget{
									Label{AssignTo: &a.w.siteNameLabel, Text: "Site name:"},
									LineEdit{AssignTo: &a.w.siteName, MinSize: Size{Width: 220}},
									HSpacer{},
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{AssignTo: &a.w.btnSave, Text: "Save Site", OnClicked: a.onSave},
									PushButton{AssignTo: &a.w.btnLoad, Text: "Load Site", OnClicked: a.onLoad},
									PushButton{AssignTo: &a.w.btnReport, Text: "Generate Report", OnClicked: a.onReport},
									HSpacer{},
								},
							},
						},
					},
				},
			},
		},
	}
}

// postCreate finishes setup that needs live widget handles.
func (a *App) postCreate() {
	a.brYellow, _ = walk.NewSolidColorBrush(walk.RGB(255, 221, 0))
	a.brWhite, _ = walk.NewSolidColorBrush(walk.RGB(255, 255, 255))

	_ = os.MkdirAll(filepath.Join(a.appDir, "profiles"), 0o755)

	if n, err := oui.Load(a.ouiCache); err == nil && n > 0 {
		applog.Info("oui: loaded %d prefixes from cache", n)
	}

	a.setTLSCombo()
	a.syncUIFromProfile()
	a.onDetect()
	a.renderMonStatus()
}

// ---- helpers ---------------------------------------------------------------

func (a *App) info(msg string) {
	walk.MsgBox(a.mw, i18n.T("dlg.info"), msg, walk.MsgBoxIconInformation)
}
func (a *App) errBox(msg string) { walk.MsgBox(a.mw, i18n.T("dlg.error"), msg, walk.MsgBoxIconError) }

func (a *App) isScanning() bool {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	return a.scanning
}

func (a *App) setScanning(v bool, cancel context.CancelFunc) {
	a.scanMu.Lock()
	a.scanning = v
	a.scanCancel = cancel
	a.scanMu.Unlock()
}

func (a *App) styleHostCell(style *walk.CellStyle) {
	if style.Col() != colStatus {
		return
	}
	// Fill the status cell so UP/DOWN is unmistakable, not just a small glyph.
	switch a.hostModel.StatusOf(style.Row()) {
	case model.StatusUp:
		style.TextColor = walk.RGB(0, 120, 55)
		style.BackgroundColor = walk.RGB(216, 244, 226)
	case model.StatusDown:
		style.TextColor = walk.RGB(185, 28, 28)
		style.BackgroundColor = walk.RGB(255, 224, 224)
	default:
		style.TextColor = walk.RGB(90, 100, 112)
	}
}

func (a *App) tlsModeFromIndex(i int) string {
	switch i {
	case 0:
		return "none"
	case 2:
		return "ssl"
	default:
		return "starttls"
	}
}

func (a *App) indexFromMode(mode string) int {
	switch strings.ToLower(mode) {
	case "none":
		return 0
	case "ssl":
		return 2
	default:
		return 1
	}
}

func (a *App) setTLSCombo() {
	idx := a.w.tlsCombo.CurrentIndex()
	if idx < 0 {
		idx = 1
	}
	_ = a.w.tlsCombo.SetModel([]string{i18n.T("tls.none"), i18n.T("tls.starttls"), i18n.T("tls.ssl")})
	_ = a.w.tlsCombo.SetCurrentIndex(idx)
}

func (a *App) setLangCombo(code string) {
	a.suppressLang = true
	_ = a.w.langCombo.SetCurrentIndex(a.langIndex(code))
	a.suppressLang = false
}

func (a *App) langIndex(code string) int {
	for i, c := range i18n.Codes() {
		if c == code {
			return i
		}
	}
	return 0
}

func (a *App) countStatuses() (up, down, unknown int) {
	for _, h := range a.hostModel.Items() {
		switch h.Status {
		case model.StatusUp:
			up++
		case model.StatusDown:
			down++
		default:
			unknown++
		}
	}
	return
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func parsePorts(s string) []int {
	var out []int
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' || r == '\t' }) {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			out = append(out, n)
		}
	}
	return model.NormalizePorts(out)
}

// ---- sync ------------------------------------------------------------------

func (a *App) syncProfileFromUI() {
	a.prof.CIDR = strings.TrimSpace(a.w.rangeEdit.Text())
	a.prof.Concurrency = int(a.w.concEdit.Value())
	a.prof.TimeoutMs = int(a.w.timeoutEdit.Value())
	a.prof.Retries = int(a.w.retriesEdit.Value())
	a.prof.IntervalSec = int(a.w.intervalEdit.Value())
	a.prof.Name = strings.TrimSpace(a.w.siteName.Text())
	if a.prof.Name == "" {
		a.prof.Name = "Untitled"
	}
	a.prof.Email = model.EmailConfig{
		Enabled:            a.w.emailEnabled.Checked(),
		Server:             strings.TrimSpace(a.w.server.Text()),
		Port:               int(a.w.smtpPort.Value()),
		TLSMode:            a.tlsModeFromIndex(a.w.tlsCombo.CurrentIndex()),
		InsecureSkipVerify: a.w.skipVerify.Checked(),
		Username:           a.w.username.Text(),
		Password:           a.w.password.Text(),
		From:               strings.TrimSpace(a.w.from.Text()),
		To:                 strings.TrimSpace(a.w.to.Text()),
	}
	a.prof.Language = i18n.Lang()
}

func (a *App) syncUIFromProfile() {
	i18n.SetLang(a.prof.Language)
	a.setLangCombo(a.prof.Language)
	_ = a.w.rangeEdit.SetText(a.prof.CIDR)
	_ = a.w.concEdit.SetValue(float64(a.prof.Concurrency))
	_ = a.w.timeoutEdit.SetValue(float64(a.prof.TimeoutMs))
	_ = a.w.retriesEdit.SetValue(float64(a.prof.Retries))
	_ = a.w.intervalEdit.SetValue(float64(a.prof.IntervalSec))
	_ = a.w.siteName.SetText(a.prof.Name)
	e := a.prof.Email
	a.w.emailEnabled.SetChecked(e.Enabled)
	_ = a.w.server.SetText(e.Server)
	_ = a.w.smtpPort.SetValue(float64(e.Port))
	_ = a.w.tlsCombo.SetCurrentIndex(a.indexFromMode(e.TLSMode))
	a.w.skipVerify.SetChecked(e.InsecureSkipVerify)
	_ = a.w.username.SetText(e.Username)
	_ = a.w.password.SetText(e.Password)
	_ = a.w.from.SetText(e.From)
	_ = a.w.to.SetText(e.To)
	_ = a.w.portEdit.SetValue(8080)
	if a.prof.PortLabels == nil {
		a.prof.PortLabels = map[int]string{}
	}
	a.refreshPorts()
	a.retranslate()
}

func (a *App) refreshPorts() {
	a.prof.Ports = model.NormalizePorts(a.prof.Ports)
	items := make([]string, len(a.prof.Ports))
	for i, p := range a.prof.Ports {
		if lbl := strings.TrimSpace(a.prof.PortLabels[p]); lbl != "" {
			items[i] = fmt.Sprintf("%d — %s", p, lbl)
		} else {
			items[i] = itoa(p)
		}
	}
	_ = a.w.portsList.SetModel(items)
	a.hostModel.SetPortLabels(a.prof.PortLabels)
}

// reresolveVendors re-fetches the ARP table and fills missing MAC/vendor info
// for the current hosts (so vendors appear after an OUI update, or for hosts
// whose ARP entry wasn't ready during the scan). Returns hosts with a vendor.
func (a *App) reresolveVendors() int {
	arp := netutil.ARPTable()
	withVendor := 0
	for i := range a.prof.Hosts {
		h := &a.prof.Hosts[i]
		if h.MAC == "" {
			if mac, ok := arp[h.IP]; ok {
				h.MAC = mac
			}
		}
		if h.MAC != "" && h.Vendor == "" {
			if v := oui.VendorOf(h.MAC); v != "" {
				h.Vendor = v
			}
		}
		if h.Vendor != "" {
			withVendor++
		}
	}
	a.hostModel.SetItems(a.prof.Hosts)
	return withVendor
}

// ---- retranslate -----------------------------------------------------------

func (a *App) retranslate() {
	_ = a.mw.SetTitle(i18n.T("app.title"))
	a.w.tbSave.SetText(i18n.T("set.save_site"))
	a.w.tbLoad.SetText(i18n.T("set.load_site"))
	a.w.tbReport.SetText(i18n.T("set.gen_report"))
	a.w.tbAbout.SetText(i18n.T("about.button"))
	a.w.tbUpdate.SetText(i18n.T("upd.button"))
	a.w.langLabel.SetText(i18n.T("lang.label"))

	_ = a.w.tabScanner.SetTitle(i18n.T("tab.scanner"))
	_ = a.w.tabMonitor.SetTitle(i18n.T("tab.monitor"))
	_ = a.w.tabSettings.SetTitle(i18n.T("tab.settings"))

	a.w.rangeLabel.SetText(i18n.T("scan.range_label"))
	a.w.btnDetect.SetText(i18n.T("scan.detect"))
	a.w.btnScan.SetText(i18n.T("scan.scan"))
	a.w.btnCancel.SetText(i18n.T("scan.cancel"))
	a.w.btnAddHost.SetText(i18n.T("host.add"))
	a.w.btnEditHost.SetText(i18n.T("host.edit"))
	a.w.btnRemoveHost.SetText(i18n.T("host.remove"))
	a.w.btnStartMon.SetText(i18n.T("scan.start_monitoring"))

	a.w.btnStopMon.SetText(i18n.T("mon.stop"))
	a.w.monEventsLbl.SetText(i18n.T("mon.events"))

	a.w.gbPorts.SetTitle(i18n.T("set.ports_group"))
	a.w.portsHint.SetText(i18n.T("set.ports_hint"))
	a.w.portFieldLbl.SetText(i18n.T("set.port_field"))
	a.w.portLabelLbl.SetText(i18n.T("set.port_label_field"))
	a.w.btnAddPort.SetText(i18n.T("set.add_port"))
	a.w.btnRemPort.SetText(i18n.T("set.remove_port"))

	a.w.gbScan.SetTitle(i18n.T("set.scan_group"))
	a.w.concLabel.SetText(i18n.T("set.concurrency"))
	a.w.timeoutLabel.SetText(i18n.T("set.timeout"))
	a.w.retriesLabel.SetText(i18n.T("set.retries"))

	a.w.gbEmail.SetTitle(i18n.T("set.email_group"))
	a.w.emailEnabled.SetText(i18n.T("set.email_enabled"))
	a.w.serverLabel.SetText(i18n.T("set.smtp_server"))
	a.w.portLabel.SetText(i18n.T("set.smtp_port"))
	a.w.tlsLabel.SetText(i18n.T("set.tls"))
	a.w.skipVerify.SetText(i18n.T("set.skipverify"))
	a.w.userLabel.SetText(i18n.T("set.username"))
	a.w.passLabel.SetText(i18n.T("set.password"))
	a.w.fromLabel.SetText(i18n.T("set.from"))
	a.w.toLabel.SetText(i18n.T("set.to"))
	a.w.btnTestEmail.SetText(i18n.T("set.test_email"))
	a.w.warnLabel.SetText(i18n.T("set.plaintext_warning"))

	a.w.gbMon.SetTitle(i18n.T("set.monitor_group"))
	a.w.intervalLabel.SetText(i18n.T("set.interval"))

	a.w.gbOUI.SetTitle(i18n.T("set.oui_group"))
	a.w.btnUpdateOUI.SetText(i18n.T("set.update_oui"))
	a.w.btnReresolve.SetText(i18n.T("set.reresolve"))

	a.w.gbProfile.SetTitle(i18n.T("set.profile_group"))
	a.w.siteNameLabel.SetText(i18n.T("set.site_name"))
	a.w.btnSave.SetText(i18n.T("set.save_site"))
	a.w.btnLoad.SetText(i18n.T("set.load_site"))
	a.w.btnReport.SetText(i18n.T("set.gen_report"))

	a.setTLSCombo()
	a.setColumnTitles()
	a.hostModel.PublishRowsReset()

	if oui.Loaded() {
		a.w.ouiStatus.SetText(i18n.Tf("set.oui_loaded", oui.Count()))
	} else {
		a.w.ouiStatus.SetText(i18n.T("set.oui_none"))
	}
	if !a.isScanning() && len(a.hostModel.Items()) == 0 {
		a.w.scanStatus.SetText(i18n.T("scan.idle"))
	}
	if len(a.hostModel.Items()) > 0 {
		a.w.scanHint.SetText(i18n.T("scan.hint_after"))
	}
	a.renderMonStatus()
	if up, down, unknown := a.countStatuses(); up+down+unknown > 0 {
		a.w.monSummary.SetText(i18n.Tf("mon.summary", up+down+unknown, up, down))
	}
}

func (a *App) setColumnTitles() {
	host := []string{
		i18n.T("col.status"), i18n.T("col.ip"), i18n.T("col.hostname"), i18n.T("col.label"),
		i18n.T("col.vendor"), i18n.T("col.mac"), i18n.T("col.ports"), i18n.T("col.notes"), i18n.T("col.id"),
	}
	for _, tv := range []*walk.TableView{a.w.hostTV, a.w.monTV} {
		if tv == nil {
			continue
		}
		cols := tv.Columns()
		for i := 0; i < cols.Len() && i < len(host); i++ {
			_ = cols.At(i).SetTitle(host[i])
		}
	}
	ev := []string{i18n.T("evcol.time"), i18n.T("evcol.host"), i18n.T("evcol.event"), i18n.T("evcol.detail")}
	if a.w.eventTV != nil {
		cols := a.w.eventTV.Columns()
		for i := 0; i < cols.Len() && i < len(ev); i++ {
			_ = cols.At(i).SetTitle(ev[i])
		}
	}
}

func (a *App) renderMonStatus() {
	if a.mon != nil && a.mon.Running() {
		last := "—"
		if !a.prof.Monitoring.LastCheck.IsZero() {
			last = a.prof.Monitoring.LastCheck.Format("15:04:05")
		}
		a.w.monStatus.SetText(i18n.Tf("mon.running", a.prof.IntervalSec, last))
		return
	}
	if a.monStopped {
		a.w.monStatus.SetText(i18n.T("mon.stopped"))
		return
	}
	a.w.monStatus.SetText(i18n.T("mon.start_hint"))
}

// ---- language --------------------------------------------------------------

func (a *App) onLangChanged() {
	if a.suppressLang {
		return
	}
	codes := i18n.Codes()
	idx := a.w.langCombo.CurrentIndex()
	code := "en"
	if idx >= 0 && idx < len(codes) {
		code = codes[idx]
	}
	i18n.SetLang(code)
	a.prof.Language = code
	a.retranslate()
	applog.Info("ui: language switched to %s", code)
}

// ---- detect / scan ---------------------------------------------------------

func (a *App) onDetect() {
	a.w.scanStatus.SetText(i18n.T("scan.detecting"))
	go func() {
		cidr, iface, err := netutil.DetectCIDR()
		a.mw.Synchronize(func() {
			if err != nil {
				a.w.scanStatus.SetText(i18n.T("scan.detect_failed"))
				applog.Warn("detect: %v", err)
				return
			}
			_ = a.w.rangeEdit.SetText(cidr)
			a.prof.CIDR = cidr
			a.w.scanStatus.SetText(i18n.Tf("scan.detected", cidr, iface))
			applog.Info("detect: %s on %s", cidr, iface)
		})
	}()
}

func (a *App) onScan() {
	if a.isScanning() {
		a.info(i18n.T("msg.scan_running"))
		return
	}
	cidr := strings.TrimSpace(a.w.rangeEdit.Text())
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		a.errBox(i18n.Tf("msg.invalid_cidr", cidr))
		return
	}
	a.syncProfileFromUI()
	total, err := scan.HostCount(cidr)
	if err != nil {
		a.errBox(err.Error())
		return
	}
	cfg := scan.Config{
		Ports:       a.prof.Ports,
		Concurrency: a.prof.Concurrency,
		TimeoutMs:   a.prof.TimeoutMs,
		Retries:     a.prof.Retries,
		VendorOf:    oui.VendorOf,
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.setScanning(true, cancel)
	a.w.btnScan.SetEnabled(false)
	a.w.btnCancel.SetEnabled(true)
	a.w.btnDetect.SetEnabled(false)
	a.stopFlash()
	a.w.flashComp.SetVisible(false)
	a.w.scanHint.SetText("")
	a.w.scanProgress.SetRange(0, total)
	a.w.scanProgress.SetValue(0)
	a.w.scanStatus.SetText(i18n.Tf("scan.starting", cidr, total, model.Host{OpenPorts: a.prof.Ports}.PortsString()))
	applog.Info("scan: start %s ports=%v conc=%d timeout=%dms", cidr, a.prof.Ports, a.prof.Concurrency, a.prof.TimeoutMs)

	start := time.Now()
	go func() {
		hosts, runErr := scan.Run(ctx, cidr, cfg, func(p scan.Progress) {
			a.mw.Synchronize(func() {
				a.w.scanProgress.SetValue(p.Scanned)
				a.w.scanStatus.SetText(i18n.Tf("scan.progress", p.Scanned, p.Total, p.Found))
			})
		})
		dur := time.Since(start).Round(time.Second)
		a.mw.Synchronize(func() { a.applyScan(hosts, runErr, total, dur) })
	}()
}

func (a *App) applyScan(hosts []model.Host, runErr error, total int, dur time.Duration) {
	a.setScanning(false, nil)
	a.w.btnScan.SetEnabled(true)
	a.w.btnCancel.SetEnabled(false)
	a.w.btnDetect.SetEnabled(true)
	a.w.scanProgress.SetValue(total)

	// Merge: keep manually-added hosts that the scan didn't rediscover.
	merged := hosts
	for _, old := range a.prof.Hosts {
		if !old.Manual {
			continue
		}
		found := false
		for _, h := range hosts {
			if h.IP == old.IP {
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, old)
		}
	}
	a.prof.Hosts = merged
	a.prof.LastScan = model.ScanInfo{
		Date:  time.Now(),
		CIDR:  strings.TrimSpace(a.w.rangeEdit.Text()),
		Ports: append([]int(nil), a.prof.Ports...),
		Count: len(merged),
	}
	a.hostModel.SetItems(merged)

	switch {
	case runErr != nil && errors.Is(runErr, context.Canceled):
		a.w.scanStatus.SetText(i18n.Tf("scan.cancelled", len(hosts), total))
		applog.Warn("scan: cancelled with %d hosts", len(hosts))
	case runErr != nil:
		a.w.scanStatus.SetText(runErr.Error())
		applog.Error("scan: %v", runErr)
	default:
		a.w.scanStatus.SetText(i18n.Tf("scan.done", len(hosts), dur.String()))
		applog.Info("scan: complete, %d hosts in %s", len(hosts), dur)
	}

	if len(a.prof.Hosts) > 0 {
		a.w.scanHint.SetText(i18n.T("scan.hint_after"))
		a.w.flashComp.SetVisible(true)
		a.startFlash()
	}
}

func (a *App) onCancel() {
	a.scanMu.Lock()
	cancel := a.scanCancel
	a.scanMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ---- flashing button -------------------------------------------------------

func (a *App) startFlash() {
	a.stopFlash()
	stop := make(chan struct{})
	a.flashStop = stop
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		on := false
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				on = !on
				br := a.brWhite
				if on {
					br = a.brYellow
				}
				b := br
				a.mw.Synchronize(func() {
					if a.w.flashComp != nil {
						a.w.flashComp.SetBackground(b)
					}
				})
			}
		}
	}()
}

func (a *App) stopFlash() {
	if a.flashStop != nil {
		close(a.flashStop)
		a.flashStop = nil
	}
	if a.w.flashComp != nil && a.brWhite != nil {
		a.w.flashComp.SetBackground(a.brWhite)
	}
}

// ---- monitoring ------------------------------------------------------------

func (a *App) onStartMon() {
	if a.mon != nil && a.mon.Running() {
		return
	}
	if len(a.hostModel.Items()) == 0 {
		a.errBox(i18n.T("mon.no_hosts"))
		return
	}
	a.syncProfileFromUI()
	a.stopFlash()
	a.w.flashComp.SetVisible(false)

	cfg := monitor.Config{
		Interval:   time.Duration(a.prof.IntervalSec) * time.Second,
		Timeout:    time.Duration(a.prof.TimeoutMs) * time.Millisecond,
		Email:      a.prof.Email,
		SendEmail:  a.prof.Email.Enabled,
		BuildEmail: a.buildEmail,
		OnUpdate:   a.onMonUpdate,
		OnEvent:    a.onMonEvent,
		OnTick:     a.onMonTick,
	}
	a.mon = monitor.New(cfg)
	a.mon.SetHosts(a.prof.Hosts)
	a.mon.Start()
	a.prof.Monitoring.Running = true
	a.monStopped = false
	a.w.btnStopMon.SetEnabled(true)
	a.renderMonStatus()
	_ = a.tabs.SetCurrentIndex(1)
	applog.Info("ui: monitoring started")
}

func (a *App) onStopMon() {
	if a.mon != nil {
		a.mon.Stop()
	}
	a.prof.Monitoring.Running = false
	a.monStopped = true
	a.w.btnStopMon.SetEnabled(false)
	a.renderMonStatus()
	if len(a.hostModel.Items()) > 0 {
		a.w.flashComp.SetVisible(true)
		a.startFlash()
	}
	applog.Info("ui: monitoring stopped")
}

func (a *App) onMonUpdate(id, status string, misses int, alerted bool, when time.Time) {
	a.mw.Synchronize(func() {
		a.hostModel.UpdateStatus(id, status, misses, alerted)
		for i := range a.prof.Hosts {
			if a.prof.Hosts[i].ID == id {
				a.prof.Hosts[i].Status = status
				a.prof.Hosts[i].ConsecutiveMisses = misses
				a.prof.Hosts[i].AlertedDown = alerted
				a.prof.Hosts[i].LastChange = when
				break
			}
		}
	})
}

func (a *App) onMonEvent(ev model.MonitorEvent) {
	a.mw.Synchronize(func() {
		a.prof.Events = append(a.prof.Events, ev)
		a.eventModel.Append(ev)
	})
}

func (a *App) onMonTick(when time.Time, up, down, unknown int) {
	a.mw.Synchronize(func() {
		a.prof.Monitoring.LastCheck = when
		a.w.monSummary.SetText(i18n.Tf("mon.summary", up+down+unknown, up, down))
		a.renderMonStatus()
	})
}

func (a *App) buildEmail(ev model.MonitorEvent, ports []int) (string, string) {
	subjKey := "email.down_subject"
	if ev.Type == model.EventUp {
		subjKey = "email.up_subject"
	}
	subject := i18n.Tf(subjKey, ev.IP)

	host := ev.Hostname
	if host == "" {
		host = "—"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", i18n.T("email.lbl_site"), a.prof.Name)
	fmt.Fprintf(&b, "%s: %s\n", i18n.T("email.lbl_host"), host)
	fmt.Fprintf(&b, "%s: %s\n", i18n.T("email.lbl_ip"), ev.IP)
	fmt.Fprintf(&b, "%s: %s\n", i18n.T("email.lbl_ports"), model.PortsLabeled(ports, a.prof.PortLabels))
	fmt.Fprintf(&b, "%s: %s\n", i18n.T("email.lbl_time"), ev.Time.Format("2006-01-02 15:04:05 MST"))
	if ev.Detail != "" {
		fmt.Fprintf(&b, "\n%s\n", ev.Detail)
	}
	return subject, b.String()
}

// ---- email test ------------------------------------------------------------

func (a *App) onTestEmail() {
	a.syncProfileFromUI()
	e := a.prof.Email
	if e.Server == "" || e.Port == 0 || strings.TrimSpace(e.To) == "" {
		a.errBox(i18n.T("msg.email_not_configured"))
		return
	}
	a.w.btnTestEmail.SetEnabled(false)
	go func() {
		err := mailer.Send(e, i18n.T("email.test_subject"), i18n.T("email.test_body"))
		a.mw.Synchronize(func() {
			a.w.btnTestEmail.SetEnabled(true)
			if err != nil {
				applog.Error("email: test failed: %v", err)
				a.errBox(i18n.Tf("msg.test_email_fail", err.Error()))
				return
			}
			applog.Info("email: test sent to %s", e.To)
			a.info(i18n.Tf("msg.test_email_ok", e.To))
		})
	}()
}

// ---- OUI update ------------------------------------------------------------

func (a *App) onUpdateOUI() {
	a.w.btnUpdateOUI.SetEnabled(false)
	a.w.ouiStatus.SetText(i18n.Tf("set.oui_updating", ""))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	go func() {
		defer cancel()
		n, err := oui.Update(ctx, a.ouiCache, func(note string) {
			a.mw.Synchronize(func() { a.w.ouiStatus.SetText(i18n.Tf("set.oui_updating", note)) })
		})
		a.mw.Synchronize(func() {
			a.w.btnUpdateOUI.SetEnabled(true)
			if err != nil {
				applog.Error("oui: update failed: %v", err)
				a.w.ouiStatus.SetText(i18n.T("set.oui_none"))
				a.errBox(i18n.Tf("msg.oui_failed", err.Error()))
				return
			}
			applog.Info("oui: updated, %d prefixes", n)
			a.w.ouiStatus.SetText(i18n.Tf("set.oui_loaded", n))
			a.reresolveVendors() // fill vendors on already-scanned hosts
			a.info(i18n.Tf("msg.oui_done", n))
		})
	}()
}

func (a *App) onReresolve() {
	if !oui.Loaded() {
		a.errBox(i18n.T("set.oui_none"))
		return
	}
	n := a.reresolveVendors()
	a.info(i18n.Tf("msg.reresolved", n))
}

// ---- ports -----------------------------------------------------------------

func (a *App) onAddPort() {
	p := int(a.w.portEdit.Value())
	if p < 1 || p > 65535 {
		a.errBox(i18n.T("msg.need_port"))
		return
	}
	if a.prof.PortLabels == nil {
		a.prof.PortLabels = map[int]string{}
	}
	exists := contains(a.prof.Ports, p)
	lbl := strings.TrimSpace(a.w.portLabelEdit.Text())
	if !exists {
		a.prof.Ports = append(a.prof.Ports, p)
	}
	if lbl != "" {
		a.prof.PortLabels[p] = lbl
	} else if exists {
		a.info(i18n.T("msg.port_exists"))
		return
	}
	a.refreshPorts()
	_ = a.w.portLabelEdit.SetText("")
}

func (a *App) onRemovePort() {
	idx := a.w.portsList.CurrentIndex()
	if idx < 0 || idx >= len(a.prof.Ports) {
		return
	}
	p := a.prof.Ports[idx]
	a.prof.Ports = append(a.prof.Ports[:idx], a.prof.Ports[idx+1:]...)
	delete(a.prof.PortLabels, p)
	a.refreshPorts()
}

// ---- manual hosts ----------------------------------------------------------

func (a *App) onAddHost() {
	h := model.Host{Status: model.StatusUnknown, Manual: true}
	if !a.hostDialog("hd.add_title", &h, true) {
		return
	}
	for _, e := range a.prof.Hosts {
		if e.IP == h.IP {
			a.errBox(i18n.T("msg.host_exists"))
			return
		}
	}
	if h.MAC != "" {
		h.ID = h.MAC
	} else {
		h.ID = "ip:" + h.IP
	}
	a.prof.Hosts = append(a.prof.Hosts, h)
	a.hostModel.SetItems(a.prof.Hosts)
	a.reresolveVendors()
	applog.Info("ui: manual host added %s (%s)", h.IP, h.Label)
	a.w.flashComp.SetVisible(true)
	a.startFlash()
}

// onEditHost edits the row selected in the Scanner table (button or double-click).
func (a *App) onEditHost() { a.editHostAt(a.w.hostTV.CurrentIndex()) }

// onEditHostMon edits the row selected in the Monitor table (double-click).
func (a *App) onEditHostMon() { a.editHostAt(a.w.monTV.CurrentIndex()) }

func (a *App) editHostAt(idx int) {
	if idx < 0 || idx >= len(a.prof.Hosts) {
		return
	}
	h := a.prof.Hosts[idx]
	if !a.hostDialog("hd.edit_title", &h, false) {
		return
	}
	a.prof.Hosts[idx] = h
	a.hostModel.SetItems(a.prof.Hosts)
}

func (a *App) onRemoveHost() {
	idx := a.w.hostTV.CurrentIndex()
	if idx < 0 || idx >= len(a.prof.Hosts) {
		return
	}
	applog.Info("ui: host removed %s", a.prof.Hosts[idx].IP)
	a.prof.Hosts = append(a.prof.Hosts[:idx], a.prof.Hosts[idx+1:]...)
	a.hostModel.SetItems(a.prof.Hosts)
}

// hostDialog edits h in place. ipEditable controls whether the IP can change
// (true when adding, false when editing). Returns true if the user accepted.
func (a *App) hostDialog(titleKey string, h *model.Host, ipEditable bool) bool {
	var dlg *walk.Dialog
	var ipLE, labelLE, portsLE, vendorLE *walk.LineEdit
	var notesTE *walk.TextEdit
	var okPB, cancelPB *walk.PushButton

	cmd, _ := Dialog{
		AssignTo:      &dlg,
		Title:         i18n.T(titleKey),
		MinSize:       Size{Width: 480, Height: 380},
		Layout:        VBox{},
		DefaultButton: &okPB,
		CancelButton:  &cancelPB,
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{Text: i18n.T("hd.ip")},
					LineEdit{AssignTo: &ipLE, Text: h.IP, ReadOnly: !ipEditable},
					Label{Text: i18n.T("hd.label")},
					LineEdit{AssignTo: &labelLE, Text: h.Label},
					Label{Text: i18n.T("hd.ports")},
					LineEdit{AssignTo: &portsLE, Text: model.Host{OpenPorts: h.OpenPorts}.PortsString()},
					Label{Text: i18n.T("hd.vendor")},
					LineEdit{AssignTo: &vendorLE, Text: h.Vendor},
					Label{Text: i18n.T("hd.notes")},
					TextEdit{AssignTo: &notesTE, Text: h.Notes, MinSize: Size{Height: 80}},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &okPB, Text: i18n.T("gen.ok"), OnClicked: func() {
						if ipEditable {
							ip := net.ParseIP(strings.TrimSpace(ipLE.Text()))
							if ip == nil || ip.To4() == nil {
								a.errBox(i18n.T("msg.need_ip"))
								return
							}
						}
						dlg.Accept()
					}},
					PushButton{AssignTo: &cancelPB, Text: i18n.T("gen.cancel"), OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}.Run(a.mw)

	if cmd != walk.DlgCmdOK {
		return false
	}
	if ipEditable {
		h.IP = strings.TrimSpace(ipLE.Text())
	}
	h.Label = strings.TrimSpace(labelLE.Text())
	h.Vendor = strings.TrimSpace(vendorLE.Text())
	h.Notes = notesTE.Text()
	h.OpenPorts = parsePorts(portsLE.Text())
	return true
}

// ---- about -----------------------------------------------------------------

func (a *App) loadAboutImage() walk.Image {
	// Prefer an external about.jpg next to the exe (swap art without rebuilding).
	ext := filepath.Join(a.appDir, "about.jpg")
	if _, err := os.Stat(ext); err == nil {
		if im, err := walk.NewImageFromFile(ext); err == nil {
			return im
		}
	}
	if img, _, err := image.Decode(bytes.NewReader(aboutJPEG)); err == nil {
		if bmp, err := walk.NewBitmapFromImage(img); err == nil {
			return bmp
		}
	}
	return nil
}

func (a *App) onAbout() {
	var dlg *walk.Dialog
	var closePB *walk.PushButton

	imgChildren := []Widget{}
	if im := a.loadAboutImage(); im != nil {
		imgChildren = append(imgChildren, ImageView{
			Image:   im,
			Mode:    ImageViewModeZoom,
			MinSize: Size{Width: 300, Height: 420},
		})
	}
	imgChildren = append(imgChildren, Composite{
		Layout: VBox{},
		Children: []Widget{
			TextEdit{
				Text:     resumeText,
				ReadOnly: true,
				VScroll:  true,
				MinSize:  Size{Width: 420, Height: 380},
			},
			LinkLabel{
				Text:            `Full bio: <a href="https://lakes.pezoa.com/about">lakes.pezoa.com/about</a>`,
				OnLinkActivated: func(link *walk.LinkLabelLink) { _ = winexec.OpenBrowser(link.URL()) },
			},
		},
	})

	_, _ = Dialog{
		AssignTo:      &dlg,
		Title:         i18n.T("about.title"),
		MinSize:       Size{Width: 800, Height: 560},
		Layout:        VBox{},
		DefaultButton: &closePB,
		CancelButton:  &closePB,
		Children: []Widget{
			Composite{Layout: HBox{}, Children: imgChildren},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &closePB, Text: i18n.T("gen.close"), OnClicked: func() { dlg.Accept() }},
				},
			},
		},
	}.Run(a.mw)
}

// ---- check for updates -----------------------------------------------------

func (a *App) onCheckUpdates() {
	a.w.tbUpdate.SetEnabled(false)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		res, err := updates.Check(ctx, a.version)
		a.mw.Synchronize(func() {
			a.w.tbUpdate.SetEnabled(true)
			if err != nil {
				applog.Warn("updates: check failed: %v", err)
				a.errBox(i18n.Tf("upd.failed", err.Error()))
				return
			}
			if res.UpdateAvail {
				applog.Info("updates: v%s available (have v%s)", res.Latest, a.version)
				if walk.MsgBox(a.mw, i18n.T("dlg.info"), i18n.Tf("upd.available", res.Latest, a.version), walk.MsgBoxYesNo|walk.MsgBoxIconInformation) == win.IDYES {
					if err := winexec.OpenBrowser(res.URL); err != nil {
						applog.Warn("updates: open page failed: %v", err)
					}
				}
			} else {
				applog.Info("updates: up to date (v%s)", a.version)
				a.info(i18n.Tf("upd.uptodate", a.version))
			}
		})
	}()
}

// ---- profile save / load ---------------------------------------------------

func (a *App) onSave() {
	a.syncProfileFromUI()
	dlg := new(walk.FileDialog)
	dlg.Title = i18n.T("dlg.save_title")
	dlg.Filter = "NetWatch Site (*.site)|*.site|All files (*.*)|*.*"
	dlg.InitialDirPath = filepath.Join(a.appDir, "profiles")
	if ok, err := dlg.ShowSave(a.mw); err != nil || !ok {
		return
	}
	path := dlg.FilePath
	if !strings.EqualFold(filepath.Ext(path), ".site") {
		path += ".site"
	}
	if err := profile.Save(path, a.prof, a.version); err != nil {
		applog.Error("profile: save failed: %v", err)
		a.errBox(i18n.Tf("msg.save_failed", err.Error()))
		return
	}
	applog.Info("profile: saved %s", path)
	a.info(i18n.Tf("msg.saved", path))
}

func (a *App) onLoad() {
	dlg := new(walk.FileDialog)
	dlg.Title = i18n.T("dlg.load_title")
	dlg.Filter = "NetWatch Site (*.site)|*.site|All files (*.*)|*.*"
	dlg.InitialDirPath = filepath.Join(a.appDir, "profiles")
	if ok, err := dlg.ShowOpen(a.mw); err != nil || !ok {
		return
	}
	p, err := profile.Load(dlg.FilePath)
	if err != nil {
		applog.Error("profile: load failed: %v", err)
		a.errBox(i18n.Tf("msg.load_failed", err.Error()))
		return
	}

	if a.mon != nil {
		a.mon.Stop()
	}
	a.prof = p
	a.hostModel.SetItems(p.Hosts)
	a.eventModel.SetItems(p.Events)
	a.syncUIFromProfile()
	a.reresolveVendors()
	a.monStopped = false
	a.prof.Monitoring.Running = false
	a.w.btnStopMon.SetEnabled(false)

	wasRunning := p.Monitoring.Running
	applog.Info("profile: loaded %s (%d hosts, %d events)", dlg.FilePath, len(p.Hosts), len(p.Events))
	a.info(i18n.Tf("msg.loaded", filepath.Base(dlg.FilePath), len(p.Hosts), len(p.Events)))

	if len(p.Hosts) > 0 {
		a.w.flashComp.SetVisible(true)
		a.startFlash()
	}
	if wasRunning && len(p.Hosts) > 0 {
		if walk.MsgBox(a.mw, i18n.T("dlg.info"), i18n.T("msg.resume_prompt"), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == win.IDYES {
			a.onStartMon()
		}
	}
}

// ---- report ----------------------------------------------------------------

func (a *App) onReport() {
	a.syncProfileFromUI()
	if len(a.prof.Hosts) == 0 {
		a.info(i18n.T("msg.no_data"))
		return
	}
	name := sanitize(a.prof.Name)
	if name == "" {
		name = "Site"
	}
	fname := fmt.Sprintf("Report_%s_%s.html", name, time.Now().Format("20060102_150405"))
	path := filepath.Join(a.appDir, fname)
	if err := report.Generate(a.prof, path, a.version); err != nil {
		applog.Error("report: %v", err)
		a.errBox(i18n.Tf("msg.report_failed", err.Error()))
		return
	}
	applog.Info("report: generated %s", path)
	a.info(i18n.Tf("msg.report_saved", path))
	if err := winexec.OpenBrowser(path); err != nil {
		applog.Warn("report: could not open browser: %v", err)
	}
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('_')
		}
	}
	return b.String()
}

// ---- shutdown --------------------------------------------------------------

func (a *App) shutdown() {
	if a.mon != nil {
		a.mon.Stop()
	}
	a.stopFlash()
	applog.Info("app: closing")
}
