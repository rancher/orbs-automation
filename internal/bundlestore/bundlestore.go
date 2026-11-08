// Package bundlestore finds the central config bundle for a workstation that has
// no local config: it chooses the release, downloads and verifies the bundle, keeps
// it in the user's cache, and falls back to the cache when the network fails
// (docs/specs/CENTRAL_BUNDLE.md). It never looks at local config, which always wins
// and is the workspace's business.
package bundlestore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/platform/assetfetch"
)

// ErrUnavailable is returned when there is no bundle to use: none in the cache and
// none fetched. The error wraps why.
var ErrUnavailable = errors.New("no config bundle is available")

// DefaultRefresh is how long a cached bundle is used before the release is
// looked at again.
const DefaultRefresh = 24 * time.Hour

// Releases is where releases come from. It is defined here, where it is consumed;
// assetfetch.Fetcher is the real one.
type Releases interface {
	Latest(ctx context.Context) (assetfetch.Release, error)
	Release(ctx context.Context, tag string) (assetfetch.Release, error)
	Asset(ctx context.Context, tag, name string) ([]byte, error)
}

// Resolver chooses and loads the bundle.
type Resolver struct {
	Releases Releases
	Cache    Cache
	Now      func() time.Time // nil for time.Now
	Refresh  time.Duration    // 0 for DefaultRefresh
}

// Request says who is asking.
type Request struct {
	// GeroVersion is the running gero's release, such as v0.5.0 or v0.6.0-rc.1, or
	// "dev" for a build that is not a release.
	GeroVersion string
	// Offline reads the cache and nothing else: no request is made.
	Offline bool
}

// Source says where a loaded bundle came from, for gero config show and doctor.
type Source struct {
	Tag       string        // the release the bundle is of
	FetchedAt time.Time     // when it was downloaded, or last confirmed current
	Age       time.Duration // how long ago that was
	FromCache bool          // it came from the cache and not from this run's download
	// Stale says the cached bundle is used because it could not be refreshed, or
	// because the request was offline and it is past its refresh time.
	Stale bool
	// Note says why, when there is something to say: that the network failed, or
	// that a newer bundle was not readable.
	Note string
}

// Loaded is a bundle ready for the config loader.
type Loaded struct {
	Header bundle.Header
	FS     fs.FS // holds one file, bundle.Name, the config chunk
	Source Source
}

// Load finds the bundle for the request. The rules are the spec's:
//
//   - A stable gero uses the newest stable release's bundle, and a build that is
//     not a release the same. A prerelease gero uses the bundle of its own
//     release, made with it, unless a newer stable one exists and is readable.
//   - A cached bundle under a day old is used with no request, unless it is older
//     than the gero's own release. An older one is checked against the newest
//     release, and downloaded only when that is a different one.
//   - A download is used only after it matches the published checksum and parses,
//     and only then replaces the cache.
//   - When the release cannot be reached or verified, the cache is used and the
//     source says so, with its release and age. With no cache that is an error.
//   - Offline reads the cache only.
func (r *Resolver) Load(ctx context.Context, req Request) (Loaded, error) {
	now := r.now()
	cached, have, _ := r.Cache.Read() // an unreadable cache is the same as none
	own := ownRelease(req.GeroVersion)

	if req.Offline {
		if !have {
			return Loaded{}, fmt.Errorf("%w: none is cached, and this run is offline", ErrUnavailable)
		}
		l, err := fromCache(cached, now)
		if err != nil {
			return Loaded{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		if l.Source.Age >= r.refresh() {
			l.Source.Stale = true
			l.Source.Note = fmt.Sprintf("the cached bundle of %s is %s old, and this run is offline", cached.Tag, ageString(l.Source.Age))
		}
		return l, nil
	}

	if have && now.Sub(cached.FetchedAt) < r.refresh() && !olderThan(cached.Tag, own) {
		if l, err := fromCache(cached, now); err == nil {
			return l, nil
		}
	}

	l, err := r.fetch(ctx, own, cached, have, now)
	if err == nil {
		return l, nil
	}
	if have {
		if l, cerr := fromCache(cached, now); cerr == nil {
			l.Source.Stale = true
			l.Source.Note = fmt.Sprintf("using the cached bundle of %s, %s old: %s", cached.Tag, ageString(l.Source.Age), reason(err))
			return l, nil
		}
	}
	return Loaded{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
}

func (r *Resolver) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Resolver) refresh() time.Duration {
	if r.Refresh > 0 {
		return r.Refresh
	}
	return DefaultRefresh
}

// fetch looks at the releases and takes the bundle that the rules choose.
func (r *Resolver) fetch(ctx context.Context, own string, cached Entry, have bool, now time.Time) (Loaded, error) {
	latest, err := r.Releases.Latest(ctx)
	if err != nil {
		return Loaded{}, err
	}
	target, fallback := latest.Tag, ""
	if own != "" && isPrerelease(own) {
		if cmp, ok := assetfetch.CompareVersions(latest.Tag, own); !ok || cmp <= 0 {
			target = own
		} else {
			fallback = own
		}
	}
	l, err := r.use(ctx, target, cached, have, now)
	if err != nil && fallback != "" && errors.Is(err, bundle.ErrUnsupported) {
		if l, ferr := r.use(ctx, fallback, cached, have, now); ferr == nil {
			l.Source.Note = fmt.Sprintf("the newest bundle, %s, is one this gero cannot read (%v); using the bundle of %s", target, err, fallback)
			return l, nil
		}
	}
	return l, err
}

// use is the bundle of the release: the cached one when it is of that release, else
// the download.
func (r *Resolver) use(ctx context.Context, tag string, cached Entry, have bool, now time.Time) (Loaded, error) {
	if have && cached.Tag == tag {
		if l, err := fromCache(cached, now); err == nil {
			if err := r.Cache.Confirm(now); err == nil {
				l.Source.FetchedAt, l.Source.Age = now, 0
			}
			return l, nil
		}
	}
	data, err := r.Releases.Asset(ctx, tag, bundle.Name)
	if err != nil {
		return Loaded{}, err
	}
	h, fsys, err := bundle.Parse(data)
	if err != nil {
		return Loaded{}, fmt.Errorf("the bundle of %s: %w", tag, err)
	}
	_ = r.Cache.Write(Entry{Tag: tag, Data: data, FetchedAt: now}) // a cache that cannot be written costs the next run a download
	return Loaded{Header: h, FS: fsys, Source: Source{Tag: tag, FetchedAt: now}}, nil
}

func fromCache(e Entry, now time.Time) (Loaded, error) {
	h, fsys, err := bundle.Parse(e.Data)
	if err != nil {
		return Loaded{}, fmt.Errorf("the cached bundle of %s: %w", e.Tag, err)
	}
	return Loaded{Header: h, FS: fsys, Source: Source{
		Tag: e.Tag, FetchedAt: e.FetchedAt, Age: now.Sub(e.FetchedAt), FromCache: true,
	}}, nil
}

// ownRelease is the release tag of the running gero, or "" for a build that is not
// one, such as "dev". GoReleaser stamps the version without the leading v that the
// tags of the releases have, so it is put back.
func ownRelease(version string) string {
	if _, ok := assetfetch.CompareVersions(version, version); !ok {
		return ""
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func isPrerelease(tag string) bool {
	tag, _, _ = strings.Cut(tag, "+")
	return strings.Contains(tag, "-")
}

// olderThan is whether the cached release is older than the gero's own, which a
// gero that has been upgraded must not run on.
func olderThan(tag, own string) bool {
	if own == "" {
		return false
	}
	cmp, ok := assetfetch.CompareVersions(tag, own)
	return ok && cmp < 0
}

// reason says in a sentence why a release could not be used.
func reason(err error) string {
	switch {
	case errors.Is(err, assetfetch.ErrRateLimited):
		return "the GitHub rate limit is used up"
	case errors.Is(err, assetfetch.ErrChecksumMismatch):
		return "the download did not match its published checksum"
	case errors.Is(err, assetfetch.ErrNoChecksum):
		return "the release publishes no checksum for the bundle"
	case errors.Is(err, bundle.ErrUnsupported):
		return "this gero cannot read the newest bundle (" + err.Error() + ")"
	}
	return err.Error()
}

// ageString says how old something is, roughly.
func ageString(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "under a minute"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d/time.Minute))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d hours", int(d/time.Hour))
	}
	return fmt.Sprintf("%d days", int(d/(24*time.Hour)))
}

// Describe says in one phrase what the config was read from, for gero config show
// and gero doctor: the release and how old the copy is, such as "bundle v0.5.0, 3
// hours old".
func (s Source) Describe() string {
	return ""
}
