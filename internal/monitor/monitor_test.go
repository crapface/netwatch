package monitor

import (
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"netwatch/internal/model"
)

// freePort returns a TCP port that was free a moment ago (best-effort for tests).
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, ps, _ := net.SplitHostPort(ln.Addr().String())
	_ = ln.Close()
	p, _ := strconv.Atoi(ps)
	return p
}

// TestTransitionsAndDebounce verifies the core requirement: a host whose ports
// stop responding is marked DOWN after two consecutive misses, exactly one
// DOWN event fires (debounce), and a live host stays UP.
func TestTransitionsAndDebounce(t *testing.T) {
	upLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upLn.Close()
	_, ups, _ := net.SplitHostPort(upLn.Addr().String())
	upPort, _ := strconv.Atoi(ups)

	downPort := freePort(t) // nothing listening here -> always a miss

	var mu sync.Mutex
	events := map[string][]string{}
	m := New(Config{
		Interval:  40 * time.Millisecond,
		Timeout:   150 * time.Millisecond,
		SendEmail: false,
		OnEvent: func(e model.MonitorEvent) {
			mu.Lock()
			events[e.HostID] = append(events[e.HostID], e.Type)
			mu.Unlock()
		},
	})
	m.SetHosts([]model.Host{
		{ID: "up", IP: "127.0.0.1", OpenPorts: []int{upPort}, Status: model.StatusUnknown},
		{ID: "down", IP: "127.0.0.1", OpenPorts: []int{downPort}, Status: model.StatusUnknown},
	})
	m.Start()
	// Wait for several check passes (>> 2 intervals) so 'down' accrues 2 misses.
	time.Sleep(600 * time.Millisecond)
	m.Stop()

	mu.Lock()
	defer mu.Unlock()
	if got := events["down"]; len(got) != 1 || got[0] != model.EventDown {
		t.Errorf("down host: want exactly one DOWN event, got %v", got)
	}
	if got := events["up"]; len(got) != 0 {
		t.Errorf("up host: want no events, got %v", got)
	}
}
