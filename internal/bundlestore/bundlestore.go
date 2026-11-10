// Package bundlestore finds the central config bundle for a workstation that has
// no local config: the bundle compiled into the binary, or a newer release's that an
// opt-in fetch downloaded, verified and kept in the user's cache
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
	"github.com/rancher/orbs-automation/internal/bundle/embedded"
	"github.com/rancher/orbs-automation/internal/platform/assetfetch"
)

// ErrUnavailable is returned by Fetch when the release's bundle could not be got:
// the releases were not reached, or what was downloaded was not verified. The error
// wraps why.
var ErrUnavailable = errors.New("no config bundle could be fetched")

// Releases is where releases come from. It is defined here, where it is consumed;
// assetfetch.Fetcher is the real one.
type Releases interface {
	Latest(ctx context.Context, prereleases bool) (assetfetch.Release, error)
	Release(ctx context.Context, tag string) (assetfetch.Release, error)
	Asset(ctx context.Context, tag, name string) ([]byte, error)
}

// Resolver chooses and loads the bundle.
type Resolver struct {
	Releases Releases
	Cache    Cache
	Now      func() time.Time // nil for time.Now
	// Embedded is the bundle compiled into the binary, in the format bundle.Parse
	// reads. Nil is the real one, embedded.Bytes.
	Embedded []byte
}

// Request says who is asking.
type Request struct {
	// GeroVersion is the running gero's release, such as v0.5.0 or v0.6.0-rc.1, or
	// "dev" for a build that is not a release.
	GeroVersion string
	// Channel is which releases may be chosen: ChannelAuto, ChannelStable or
	// ChannelPrerelease. Empty is ChannelAuto.
	Channel string
}

// The channels of Request.
const (
	ChannelAuto       = "auto"       // follows the running gero
	ChannelStable     = "stable"     // stable releases only
	ChannelPrerelease = "prerelease" // the newest release, a pre-release or not
)

// Source says where a loaded bundle came from, for gero config show and doctor.
type Source struct {
	// File is the path of a bundle file given by hand, when that is the source:
	// there is then no release fetched, no cache and no age.
	File string
	// Embedded says the bundle is the one compiled into the binary: there is no
	// download, no cache and no age.
	Embedded  bool
	Tag       string        // the release the bundle is of
	FetchedAt time.Time     // when it was downloaded, or last confirmed current
	Age       time.Duration // how long ago that was
	FromCache bool          // it came from the cache and not from this run's download
	// Note says why, when there is something to say: that nothing newer is published,
	// or that a newer bundle was not readable.
	Note string
}

// Loaded is a bundle ready for the config loader.
type Loaded struct {
	Header bundle.Header
	FS     fs.FS // holds one file, bundle.Name, the config chunk
	Source Source
}

// Fetch is the opt-in network path: it looks at the releases and, when the newest on
// the channel is newer than the gero's own, downloads its bundle into the cache,
// where Load finds it from then on. The rules are the spec's:
//
//   - The channel says which releases are candidates. Auto follows the gero: a
//     stable gero looks at stable releases, and a prerelease gero at the newest
//     release, a prerelease or not. Stable looks at stable releases only for any
//     gero, and prerelease at the newest of all.
//   - Only a release newer than the gero's own is downloaded. Its own bundle is the
//     embedded one, so when nothing is newer the result is the embedded bundle, with
//     a note that says so. A gero that is not a release has none to be newer than.
//   - The release is always asked for, however fresh the cache is. A cached bundle of
//     the same release is confirmed and not downloaded again.
//   - A download is used only after it matches the published checksum, the
//     fetcher has verified the signatures, and it parses, and only then replaces
//     the cache.
//   - A newest bundle in a format this gero cannot read is passed over: for the
//     newest stable release when the channel looked at pre-releases and that one is
//     newer than the gero, and otherwise for what Load gives. The note names why.
//   - When the release cannot be reached or verified that is an error, and the cache
//     is as it was.
func (r *Resolver) Fetch(ctx context.Context, req Request) (Loaded, error) {
	own := ownRelease(req.GeroVersion)
	if own == "" {
		return r.loadNoting(ctx, req, "this build is not a release, so there is no newer release to fetch a bundle of")
	}
	cached, have, _ := r.Cache.Read() // an unreadable cache is the same as none
	prereleases := wantsPrereleases(own, req.Channel)
	latest, err := r.Releases.Latest(ctx, prereleases)
	if err != nil {
		return Loaded{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if !newerThan(latest.Tag, own) {
		return r.loadNoting(ctx, req, fmt.Sprintf("the newest release on this channel, %s, is not newer than this gero's own, %s", latest.Tag, own))
	}
	now := r.now()
	l, err := r.use(ctx, latest.Tag, cached, have, now)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, bundle.ErrUnsupported) {
		return Loaded{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	note := fmt.Sprintf("the newest bundle, %s, is one this gero cannot read (%v)", latest.Tag, err)
	if prereleases && isPrerelease(latest.Tag) {
		if stable, serr := r.Releases.Latest(ctx, false); serr == nil && newerThan(stable.Tag, own) {
			if l, ferr := r.use(ctx, stable.Tag, cached, have, now); ferr == nil {
				l.Source.Note = note + "; using the bundle of " + stable.Tag
				return l, nil
			}
		}
	}
	return r.loadNoting(ctx, req, note)
}

// loadNoting is what Load gives, with the note said before any it has of its own.
func (r *Resolver) loadNoting(ctx context.Context, req Request, note string) (Loaded, error) {
	l, err := r.Load(ctx, req)
	if l.Source.Note != "" {
		note += "; " + l.Source.Note
	}
	l.Source.Note = note
	return l, err
}

// Load is the bundle to use with no request made: a cached bundle when it is of a
// newer release than the gero's own on the channel, and the embedded bundle in every
// other case. The cache is the result of an earlier Fetch, and is never refreshed
// here, so it is used however old it is. A gero that is not a release has no release
// to be newer than, and takes the embedded bundle.
func (r *Resolver) Load(ctx context.Context, req Request) (Loaded, error) {
	now := r.now()
	own := ownRelease(req.GeroVersion)
	note := ""
	if cached, have, _ := r.Cache.Read(); have && newerThan(cached.Tag, own) && fitsChannel(cached.Tag, own, req.Channel) {
		l, err := fromCache(cached, now)
		if err == nil {
			return l, nil
		}
		note = fmt.Sprintf("the cached bundle of %s is not readable (%v); using the embedded bundle", cached.Tag, err)
	}
	l, err := r.embedded(own)
	l.Source.Note = note
	return l, err
}

// embedded is the bundle compiled into the binary, for the gero of release own.
func (r *Resolver) embedded(own string) (Loaded, error) {
	data := r.Embedded
	if data == nil {
		data = embedded.Bytes()
	}
	h, fsys, err := bundle.Parse(data)
	if err != nil {
		return Loaded{}, fmt.Errorf("the embedded bundle: %w", err)
	}
	return Loaded{Header: h, FS: fsys, Source: Source{Embedded: true, Tag: own}}, nil
}

// newerThan is whether the release tag is newer than the gero's own. A gero with no
// release of its own has nothing a cached bundle could be newer than.
func newerThan(tag, own string) bool {
	if own == "" {
		return false
	}
	cmp, ok := assetfetch.CompareVersions(tag, own)
	return ok && cmp > 0
}

func (r *Resolver) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// wantsPrereleases is whether pre-releases are candidates: as the channel says, or,
// left to auto, when the gero itself is a pre-release.
func wantsPrereleases(own, channel string) bool {
	switch channel {
	case ChannelStable:
		return false
	case ChannelPrerelease:
		return true
	}
	return own != "" && isPrerelease(own)
}

// fitsChannel is whether a cached release is one the channel would choose: a
// pre-release is not, when pre-releases are not candidates.
func fitsChannel(tag, own, channel string) bool {
	return wantsPrereleases(own, channel) || !isPrerelease(tag)
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
	if s.Embedded {
		if s.Tag == "" {
			return "embedded bundle (a build that is not a release)"
		}
		return "embedded bundle " + s.Tag
	}
	if s.File != "" {
		return fmt.Sprintf("bundle file %s (%s)", s.File, s.Tag)
	}
	return fmt.Sprintf("bundle %s, %s old", s.Tag, ageString(s.Age))
}
