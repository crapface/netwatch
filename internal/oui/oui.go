// Package oui downloads, parses and caches the IEEE OUI database, mapping the
// first 6 hex digits of a MAC address to a hardware vendor name.
package oui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	"netwatch/internal/netutil"
)

// SourceURLs are tried in order; IEEE serves the same file over https and http.
var SourceURLs = []string{
	"https://standards-oui.ieee.org/oui/oui.txt",
	"http://standards-oui.ieee.org/oui/oui.txt",
}

// "28-6F-B9   (hex)\t\tNokia Shanghai Bell Co., Ltd."
var ouiLineRe = regexp.MustCompile(`^\s*([0-9A-Fa-f]{2})[-:]([0-9A-Fa-f]{2})[-:]([0-9A-Fa-f]{2})\s+\(hex\)\s+(.+?)\s*$`)

var (
	mu     sync.RWMutex
	table  = map[string]string{}
	loaded bool
)

// Count returns the number of loaded vendor prefixes.
func Count() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(table)
}

// Loaded reports whether a non-empty OUI table is in memory.
func Loaded() bool {
	mu.RLock()
	defer mu.RUnlock()
	return loaded
}

// VendorOf resolves a MAC address to a vendor name ("" if unknown).
func VendorOf(mac string) string {
	key := netutil.OUIKey(mac)
	if key == "" {
		return ""
	}
	mu.RLock()
	defer mu.RUnlock()
	return table[key]
}

// Load reads a previously saved cache file (oui_cache.json) into memory.
func Load(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		return 0, err
	}
	mu.Lock()
	table = m
	loaded = len(m) > 0
	mu.Unlock()
	return len(m), nil
}

// Update downloads the IEEE OUI list, parses it, swaps it into memory and
// writes the cache file. onProgress receives short human-readable status notes.
func Update(ctx context.Context, cachePath string, onProgress func(note string)) (int, error) {
	note := func(s string) {
		if onProgress != nil {
			onProgress(s)
		}
	}

	client := &http.Client{} // cancellation comes from ctx on the request
	var resp *http.Response
	var lastErr error
	for _, u := range SourceURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "NetWatch/1.0 (+https://local)")
		note("connecting…")
		r, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if r.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d from %s", r.StatusCode, u)
			_ = r.Body.Close()
			continue
		}
		resp = r
		break
	}
	if resp == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("download failed")
		}
		return 0, lastErr
	}
	defer resp.Body.Close()

	note("parsing…")
	m := make(map[string]string, 40000)
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		mt := ouiLineRe.FindStringSubmatch(sc.Text())
		if mt == nil {
			continue
		}
		key := strings.ToUpper(mt[1] + mt[2] + mt[3])
		vendor := strings.TrimSpace(mt[4])
		if vendor == "" {
			continue
		}
		m[key] = vendor
		n++
		if n%4000 == 0 {
			note(fmt.Sprintf("%d prefixes…", n))
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if len(m) == 0 {
		return 0, fmt.Errorf("no OUI entries parsed (unexpected format)")
	}

	out, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(cachePath, out, 0o644); err != nil {
		return 0, err
	}

	mu.Lock()
	table = m
	loaded = true
	mu.Unlock()
	return len(m), nil
}
