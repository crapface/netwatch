// Package oui downloads, parses and caches the IEEE OUI database, mapping the
// first 6 hex digits of a MAC address to a hardware vendor name.
//
// Robustness notes: IEEE's oui.txt endpoint frequently rejects non-browser
// User-Agents and is often blocked/SSL-inspected on corporate networks, which
// is the usual reason vendor names come back blank. We therefore try the CSV
// endpoint first, send a browser-like User-Agent, surface the real error when
// every source fails, and support loading a local oui.csv/oui.txt dropped next
// to the executable for locked-down networks.
package oui

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"netwatch/internal/netutil"
)

// Sources are tried in order. CSV first (most reliable + easiest to parse).
var Sources = []string{
	"https://standards-oui.ieee.org/oui/oui.csv",
	"https://standards-oui.ieee.org/oui/oui.txt",
	"http://standards-oui.ieee.org/oui/oui.txt",
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NetWatch/1.x OUI-updater"

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

func swap(m map[string]string) {
	mu.Lock()
	table = m
	loaded = len(m) > 0
	mu.Unlock()
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
	swap(m)
	return len(m), nil
}

// LoadFile parses a local oui.csv or oui.txt (chosen by extension) and swaps it
// into memory. Lets users on locked-down networks supply the file by hand.
func LoadFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	m, err := parse(path, f)
	if err != nil {
		return 0, err
	}
	if len(m) == 0 {
		return 0, fmt.Errorf("no OUI entries parsed from %s", path)
	}
	swap(m)
	return len(m), nil
}

// Update downloads the OUI list from the first working source, parses it, swaps
// it into memory and writes the JSON cache.
func Update(ctx context.Context, cachePath string, onProgress func(note string)) (int, error) {
	note := func(s string) {
		if onProgress != nil {
			onProgress(s)
		}
	}
	client := &http.Client{}
	var errs []string

	for _, u := range Sources {
		note("connecting…")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", u, err))
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "text/csv,text/plain,*/*")

		resp, err := client.Do(req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", u, err))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("%s: HTTP %d", u, resp.StatusCode))
			_ = resp.Body.Close()
			continue
		}

		note("parsing…")
		m, perr := parse(u, resp.Body)
		_ = resp.Body.Close()
		if perr != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", u, perr))
			continue
		}
		if len(m) == 0 {
			errs = append(errs, fmt.Sprintf("%s: no entries parsed", u))
			continue
		}

		if out, err := json.Marshal(m); err == nil {
			_ = os.WriteFile(cachePath, out, 0o644)
		}
		swap(m)
		return len(m), nil
	}

	return 0, fmt.Errorf("all OUI sources failed:\n%s\n\nTip: on a restricted network, download oui.csv from IEEE on another machine and place it next to NetWatch.exe", strings.Join(errs, "\n"))
}

// parse picks the CSV or text parser based on the source name/extension.
func parse(name string, r io.Reader) (map[string]string, error) {
	if strings.HasSuffix(strings.ToLower(name), ".csv") {
		return parseCSV(r)
	}
	return parseTXT(r)
}

// parseTXT handles the classic oui.txt "(hex)" lines:
//
//	28-6F-B9   (hex)    Nokia Shanghai Bell Co., Ltd.
func parseTXT(r io.Reader) (map[string]string, error) {
	m := make(map[string]string, 40000)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 256*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		i := strings.Index(line, "(hex)")
		if i < 0 {
			continue
		}
		key := hex6(line[:i])
		if key == "" {
			continue
		}
		vendor := strings.TrimSpace(line[i+len("(hex)"):])
		if vendor != "" {
			m[key] = vendor
		}
	}
	return m, sc.Err()
}

// parseCSV handles oui.csv with header Registry,Assignment,Organization Name,…
func parseCSV(r io.Reader) (map[string]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	m := make(map[string]string, 40000)
	first := true
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, err
		}
		if first {
			first = false
			continue // header row
		}
		if len(rec) < 3 {
			continue
		}
		key := hex6(rec[1])
		vendor := strings.TrimSpace(rec[2])
		if key != "" && vendor != "" {
			m[key] = vendor
		}
	}
	return m, nil
}

// hex6 extracts the first 6 hex digits (uppercase) from s, ignoring separators.
func hex6(s string) string {
	b := make([]byte, 0, 6)
	for i := 0; i < len(s) && len(b) < 6; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9', c >= 'A' && c <= 'F':
			b = append(b, c)
		case c >= 'a' && c <= 'f':
			b = append(b, c-'a'+'A')
		}
	}
	if len(b) < 6 {
		return ""
	}
	return string(b)
}
