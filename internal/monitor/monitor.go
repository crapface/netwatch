// Package monitor watches an approved host list. Each interval it re-checks
// every host's previously-open ports. A host that misses on ALL its ports for
// two consecutive checks is marked DOWN (anti-flap). A DOWN event fires exactly
// once per UP->DOWN transition; an email alert is sent then and only re-sent
// after the host recovers (UP) and goes DOWN again (debounce).
package monitor

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"netwatch/internal/applog"
	"netwatch/internal/mailer"
	"netwatch/internal/model"
)

// Config wires the monitor to the rest of the app via callbacks. All callbacks
// are invoked from monitor goroutines; the GUI must marshal UI work to the UI
// thread inside them.
type Config struct {
	Interval  time.Duration
	Timeout   time.Duration
	Email     model.EmailConfig
	SendEmail bool

	// BuildEmail produces a localized subject/body for a DOWN/UP event.
	BuildEmail func(ev model.MonitorEvent, openPorts []int) (subject, body string)

	// OnUpdate reports a host's runtime change (status/misses/alerted).
	OnUpdate func(id, status string, misses int, alerted bool, when time.Time)
	// OnEvent appends a transition to the monitoring log.
	OnEvent func(model.MonitorEvent)
	// OnTick reports aggregate counts after every full check pass.
	OnTick func(when time.Time, up, down, unknown int)
}

type hostState struct {
	id, ip, hostname string
	ports            []int
	status           string
	misses           int
	alerted          bool
}

// Monitor is a running (or stopped) watcher over a fixed host set.
type Monitor struct {
	cfg     Config
	mu      sync.Mutex
	states  []*hostState
	cancel  context.CancelFunc
	running bool
	wg      sync.WaitGroup
}

// New creates a stopped monitor.
func New(cfg Config) *Monitor {
	if cfg.Interval <= 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second
	}
	return &Monitor{cfg: cfg}
}

// SetHosts seeds the host set, preserving each host's persisted runtime state
// (status/misses/alerted) so a loaded Site resumes seamlessly.
func (m *Monitor) SetHosts(hosts []model.Host) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = m.states[:0]
	for _, h := range hosts {
		st := &hostState{
			id:       h.ID,
			ip:       h.IP,
			hostname: h.Hostname,
			ports:    append([]int(nil), h.OpenPorts...),
			status:   h.Status,
			misses:   h.ConsecutiveMisses,
			alerted:  h.AlertedDown,
		}
		if st.status == "" {
			st.status = model.StatusUnknown
		}
		m.states = append(m.states, st)
	}
}

// Running reports whether the monitor loop is active.
func (m *Monitor) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// Start launches the monitoring loop (no-op if already running or no hosts).
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running || len(m.states) == 0 {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true
	m.mu.Unlock()

	applog.Info("monitor: started, interval=%s, hosts=%d", m.cfg.Interval, len(m.states))
	m.wg.Add(1)
	go m.loop(ctx)
}

// Stop halts the monitoring loop and waits for it to exit.
func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
	applog.Info("monitor: stopped")
}

func (m *Monitor) loop(ctx context.Context) {
	defer m.wg.Done()
	m.checkAll(ctx) // immediate first pass
	t := time.NewTicker(m.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.checkAll(ctx)
		}
	}
}

func (m *Monitor) checkAll(ctx context.Context) {
	m.mu.Lock()
	states := make([]*hostState, len(m.states))
	copy(states, m.states)
	m.mu.Unlock()

	sem := make(chan struct{}, 64)
	var wg sync.WaitGroup
	for _, st := range states {
		wg.Add(1)
		go func(st *hostState) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m.checkOne(ctx, st)
		}(st)
	}
	wg.Wait()

	if ctx.Err() != nil {
		return
	}

	// Aggregate counts.
	var up, down, unknown int
	now := time.Now()
	m.mu.Lock()
	for _, st := range m.states {
		switch st.status {
		case model.StatusUp:
			up++
		case model.StatusDown:
			down++
		default:
			unknown++
		}
	}
	m.mu.Unlock()
	if m.cfg.OnTick != nil {
		m.cfg.OnTick(now, up, down, unknown)
	}
}

func (m *Monitor) checkOne(ctx context.Context, st *hostState) {
	// A manually-added host with no ports can't be probed; leave it unknown
	// and never flag it DOWN.
	if len(st.ports) == 0 {
		return
	}
	reachable := probe(ctx, st.ip, st.ports, m.cfg.Timeout)
	if ctx.Err() != nil {
		return
	}

	m.mu.Lock()
	prev := st.status
	now := time.Now()
	var emit *model.MonitorEvent

	if reachable {
		if st.status == model.StatusDown {
			emit = &model.MonitorEvent{Time: now, HostID: st.id, IP: st.ip, Hostname: st.hostname, Type: model.EventUp, Detail: "host recovered"}
		}
		st.status = model.StatusUp
		st.misses = 0
		st.alerted = false
	} else {
		st.misses++
		if st.misses >= 2 && st.status != model.StatusDown {
			st.status = model.StatusDown
			emit = &model.MonitorEvent{Time: now, HostID: st.id, IP: st.ip, Hostname: st.hostname, Type: model.EventDown, Detail: "no open port responded for 2 consecutive checks"}
		}
	}
	status, misses, alerted := st.status, st.misses, st.alerted
	ports := append([]int(nil), st.ports...)
	hostname := st.hostname
	// Mark alerted at the moment we decide to send the DOWN email.
	sendDown := false
	if emit != nil && emit.Type == model.EventDown && m.cfg.SendEmail && !st.alerted {
		st.alerted = true
		alerted = true
		sendDown = true
	}
	m.mu.Unlock()

	if m.cfg.OnUpdate != nil && (status != prev || emit != nil) {
		m.cfg.OnUpdate(st.id, status, misses, alerted, now)
	}
	if emit != nil {
		applog.Info("monitor: %s -> %s (%s)", st.ip, emit.Type, st.id)
		if m.cfg.OnEvent != nil {
			m.cfg.OnEvent(*emit)
		}
		if sendDown {
			m.sendAlert(*emit, ports, hostname)
		}
	}
}

func (m *Monitor) sendAlert(ev model.MonitorEvent, ports []int, hostname string) {
	if m.cfg.BuildEmail == nil {
		return
	}
	subject, body := m.cfg.BuildEmail(ev, ports)
	go func() {
		if err := mailer.Send(m.cfg.Email, subject, body); err != nil {
			applog.Error("monitor: email alert for %s failed: %v", ev.IP, err)
		} else {
			applog.Info("monitor: email alert sent for %s", ev.IP)
		}
	}()
}

// probe returns true if ANY of the host's ports accepts a TCP connection.
func probe(ctx context.Context, ip string, ports []int, timeout time.Duration) bool {
	d := &net.Dialer{Timeout: timeout}
	for _, p := range ports {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(p)))
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}
