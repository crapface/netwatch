// Package scan performs a thorough, cancellable TCP-connect port scan across a
// CIDR range using a bounded worker pool. No raw sockets, so no admin rights
// are required. IP generation is streamed (never materialized) so very large
// subnets (e.g. /16) do not exhaust memory.
package scan

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"netwatch/internal/model"
	"netwatch/internal/netutil"
)

// Config tunes a scan run.
type Config struct {
	Ports       []int
	Concurrency int                     // simultaneous dialers (default 100)
	TimeoutMs   int                     // per-connect timeout (default 1000)
	Retries     int                     // extra attempts on timeout (default 1)
	VendorOf    func(mac string) string // OUI vendor resolver (may be nil)
}

// Progress is reported periodically during a scan.
type Progress struct {
	Scanned int
	Total   int
	Found   int
}

// HostCount returns the number of scannable addresses in a CIDR (network and
// broadcast excluded for prefixes that have them). Used for progress totals.
func HostCount(cidr string) (int, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0, err
	}
	if ipnet.IP.To4() == nil {
		return 0, fmt.Errorf("only IPv4 ranges are supported")
	}
	ones, bits := ipnet.Mask.Size()
	hostBits := bits - ones
	if hostBits > 24 {
		return 0, fmt.Errorf("range too large (maximum /8)")
	}
	total := 1 << uint(hostBits)
	if hostBits >= 2 {
		total -= 2 // network + broadcast
	}
	if total < 0 {
		total = 0
	}
	return total, nil
}

// Run scans cidr for the configured ports and returns discovered hosts.
// It honours ctx for cancellation and calls onProgress periodically.
func Run(ctx context.Context, cidr string, cfg Config, onProgress func(Progress)) ([]model.Host, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	if ipnet.IP.To4() == nil {
		return nil, fmt.Errorf("only IPv4 ranges are supported")
	}
	total, err := HostCount(cidr)
	if err != nil {
		return nil, err
	}
	if len(cfg.Ports) == 0 {
		return nil, fmt.Errorf("no ports configured to scan")
	}

	conc := cfg.Concurrency
	if conc < 1 {
		conc = 100
	}
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Second
	}
	retries := cfg.Retries
	if retries < 0 {
		retries = 0
	}

	ones, bits := ipnet.Mask.Size()
	hostBits := bits - ones
	skipEnds := hostBits >= 2
	base := ipnet.IP.Mask(ipnet.Mask).To4()
	bcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		bcast[i] = base[i] | ^ipnet.Mask[i]
	}

	jobs := make(chan string, conc*2)

	// Streaming IP generator — bounded memory regardless of subnet size.
	go func() {
		defer close(jobs)
		cur := make(net.IP, 4)
		copy(cur, base)
		for {
			if !ipnet.Contains(cur) {
				break
			}
			emit := true
			if skipEnds && (cur.Equal(base) || cur.Equal(bcast)) {
				emit = false
			}
			if emit {
				select {
				case <-ctx.Done():
					return
				case jobs <- cur.String():
				}
			}
			if cur.Equal(bcast) {
				break
			}
			incIP(cur)
		}
	}()

	var scanned, found int64
	results := make(chan model.Host, conc)
	dialer := &net.Dialer{Timeout: timeout}

	var wg sync.WaitGroup
	for w := 0; w < conc; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				open := scanPorts(ctx, dialer, ip, cfg.Ports, retries)
				n := atomic.AddInt64(&scanned, 1)
				if len(open) > 0 {
					atomic.AddInt64(&found, 1)
					results <- model.Host{IP: ip, OpenPorts: open, Status: model.StatusUnknown}
				}
				if onProgress != nil && n%64 == 0 {
					onProgress(Progress{Scanned: int(n), Total: total, Found: int(atomic.LoadInt64(&found))})
				}
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	var hosts []model.Host
	for h := range results {
		hosts = append(hosts, h)
	}

	// Final progress tick.
	if onProgress != nil {
		onProgress(Progress{Scanned: int(atomic.LoadInt64(&scanned)), Total: total, Found: len(hosts)})
	}

	// If cancelled, return what we have so far.
	if ctx.Err() != nil {
		enrich(ctx, hosts, cfg)
		return hosts, ctx.Err()
	}

	enrich(ctx, hosts, cfg)
	return hosts, nil
}

// enrich fills hostname (reverse DNS), MAC (ARP cache) and vendor (OUI) for
// the discovered hosts, then sorts them by IP.
func enrich(ctx context.Context, hosts []model.Host, cfg Config) {
	if len(hosts) == 0 {
		return
	}
	arp := netutil.ARPTable()
	sem := make(chan struct{}, 32)
	var wg sync.WaitGroup
	for i := range hosts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			h := &hosts[i]
			h.Hostname = reverseDNS(ctx, h.IP)
			if mac, ok := arp[h.IP]; ok {
				h.MAC = mac
				if cfg.VendorOf != nil {
					h.Vendor = cfg.VendorOf(mac)
				}
			}
			if h.MAC != "" {
				h.ID = h.MAC
			} else {
				h.ID = "ip:" + h.IP
			}
			sort.Ints(h.OpenPorts)
		}(i)
	}
	wg.Wait()
	sort.Slice(hosts, func(i, j int) bool { return ipLess(hosts[i].IP, hosts[j].IP) })
}

// scanPorts returns the subset of ports that accept a TCP connection.
func scanPorts(ctx context.Context, d *net.Dialer, ip string, ports []int, retries int) []int {
	var open []int
	for _, p := range ports {
		addr := net.JoinHostPort(ip, strconv.Itoa(p))
		ok := false
		for attempt := 0; attempt <= retries; attempt++ {
			select {
			case <-ctx.Done():
				return open
			default:
			}
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err == nil {
				_ = conn.Close()
				ok = true
				break
			}
			// Retry only on timeout; "connection refused" is a definitive closed port.
			if ne, isNet := err.(net.Error); isNet && ne.Timeout() {
				continue
			}
			break
		}
		if ok {
			open = append(open, p)
		}
	}
	return open
}

func reverseDNS(ctx context.Context, ip string) string {
	c, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(c, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	name := names[0]
	if n := len(name); n > 0 && name[n-1] == '.' {
		name = name[:n-1]
	}
	return name
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func ipLess(a, b string) bool {
	ia, ib := net.ParseIP(a).To4(), net.ParseIP(b).To4()
	if ia == nil || ib == nil {
		return a < b
	}
	return bytes.Compare(ia, ib) < 0
}
