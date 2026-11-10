// Package goreleases reads the list of Go releases, to name the newest patch of a
// minor: a go.mod that says go 1.26 and no more is run by the newest go1.26.x, as
// the branch's own CI would be. It is one plain HTTP read of JSON, with no Go
// toolchain, cached on disk because the list changes a few times a year.
package goreleases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultURL is the public list of Go releases, with the ones that were withdrawn
// or are old.
const DefaultURL = "https://go.dev/dl/?mode=json&include=all"

// ErrNoRelease is returned when the list has no stable release of the minor.
var ErrNoRelease = errors.New("no stable release")

// Client reads the list. The zero value works: it uses the public list, a client
// with a timeout and no cache.
type Client struct {
	HTTP *http.Client // nil means a client with a 20 second timeout
	URL  string       // "" means DefaultURL; or a test server
	// CacheDir is where the list is kept, and "" keeps none. A file younger than TTL
	// is used with no request.
	CacheDir string
	TTL      time.Duration    // 0 means a day
	Now      func() time.Time // nil means time.Now
}

type release struct {
	Version string `json:"version"` // go1.26.10
	Stable  bool   `json:"stable"`
}

// NewestPatch returns the toolchain of the newest stable patch of a minor given as
// 1.26 (or go1.26): go1.26.10. A release candidate is not one.
func (c Client) NewestPatch(ctx context.Context, minor string) (string, error) {
	minor = strings.TrimPrefix(minor, "go")
	list, err := c.list(ctx)
	if err != nil {
		return "", err
	}
	best, bestPatch := "", -1
	for _, r := range list {
		patch, ok := patchOf(r.Version, minor)
		if !ok || !r.Stable || patch <= bestPatch {
			continue
		}
		best, bestPatch = r.Version, patch
	}
	if best == "" {
		return "", fmt.Errorf("%w: the list of Go releases has none of go%s", ErrNoRelease, minor)
	}
	return best, nil
}

// patchOf is the patch number of a version such as go1.26.10 in the minor 1.26. The
// first release of an old minor, named go1.20 and not go1.20.0, is patch 0.
func patchOf(version, minor string) (int, bool) {
	rest, ok := strings.CutPrefix(version, "go"+minor)
	switch {
	case !ok:
		return 0, false
	case rest == "":
		return 0, true
	}
	rest, ok = strings.CutPrefix(rest, ".")
	if !ok {
		return 0, false // go1.260, or a release candidate such as go1.26rc1
	}
	patch, err := strconv.Atoi(rest)
	return patch, err == nil
}

func (c Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// list is the releases, from the cache when it is fresh, else from the network,
// which refreshes the cache.
func (c Client) list(ctx context.Context) ([]release, error) {
	ttl := c.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	var path string
	if c.CacheDir != "" {
		path = filepath.Join(c.CacheDir, "go-releases.json")
		if info, err := os.Stat(path); err == nil && c.now().Sub(info.ModTime()) < ttl {
			if data, err := os.ReadFile(path); err == nil {
				if list, err := parse(data); err == nil {
					return list, nil
				}
			}
		}
	}
	data, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}
	list, err := parse(data)
	if err != nil {
		return nil, err
	}
	if path != "" { // the cache is only a saving: a failure to write it is not one to report
		if os.MkdirAll(c.CacheDir, 0o755) == nil {
			_ = os.WriteFile(path, data, 0o600)
		}
	}
	return list, nil
}

func (c Client) fetch(ctx context.Context) ([]byte, error) {
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	url := c.URL
	if url == "" {
		url = DefaultURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gero")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("read the list of Go releases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read the list of Go releases: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func parse(data []byte) ([]release, error) {
	var list []release
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("the list of Go releases is not JSON: %w", err)
	}
	return list, nil
}
