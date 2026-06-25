// Package updates checks GitHub for a newer NetWatch release. It only reports
// availability and points to the download page — it never downloads or runs
// anything itself.
package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// LatestAPI is the GitHub "latest release" endpoint; ReleasesPage is the
// human-facing download page.
const (
	LatestAPI    = "https://api.github.com/repos/crapface/netwatch/releases/latest"
	ReleasesPage = "https://github.com/crapface/netwatch/releases/latest"
)

// Result is the outcome of a check.
type Result struct {
	Latest      string // newest version without leading "v", e.g. "1.1.2"
	URL         string // release page to open in a browser
	UpdateAvail bool   // true if Latest is newer than the current version
}

// Check queries GitHub for the latest release and compares it to current.
func Check(ctx context.Context, current string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LatestAPI, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "NetWatch")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Result{}, err
	}

	latest := strings.TrimPrefix(strings.TrimSpace(body.TagName), "v")
	url := strings.TrimSpace(body.HTMLURL)
	if url == "" {
		url = ReleasesPage
	}
	return Result{
		Latest:      latest,
		URL:         url,
		UpdateAvail: newer(latest, strings.TrimPrefix(current, "v")),
	}, nil
}

// newer reports whether semver-ish a is greater than b (major.minor.patch).
func newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parts(v string) [3]int {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i] // drop any pre-release / build metadata
	}
	var out [3]int
	for i, s := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		out[i], _ = strconv.Atoi(strings.TrimSpace(s))
	}
	return out
}
