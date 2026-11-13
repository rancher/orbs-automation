// Package bundlestore finds the central config bundle for a workstation that has
// no local config: the bundle compiled from the config embedded in the binary, or a
// newer release's that an opt-in fetch (internal/bundlefetch) downloaded, verified and
// kept in the user's cache, where several bundles are kept at once, each under an ID
// worked out from the files it was made from (docs/specs/CENTRAL_BUNDLE.md). It never looks at local config, which always wins
// and is the workspace's business.
package bundlestore

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundle/embedded"
	"github.com/rancher/orbs-automation/internal/gero/assetfetch"
)

// Resolver chooses the bundle to use, from the store and the config embedded in the
// binary. What fetches a newer one is internal/bundlefetch.
type Resolver struct {
	Store Store
	Now   func() time.Time // nil for time.Now
	// Embedded is the config compiled into the binary, as files, which the bundle of
	// the binary is made from. Nil is the real one, embedded.FS.
	Embedded fs.FS
	// EmbeddedHeader is the header that bundle is made with. The zero value is the
	// real one, embedded.Header.
	EmbeddedHeader bundle.Header
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

// Load is the bundle to use with no request made: the highest release in the store
// that is newer than the gero's own and on the channel, passing over one this gero
// cannot read for the next, and the embedded bundle when there is none. The store is
// the result of earlier Fetches, and is never refreshed here, so a bundle is used
// however old it is. A gero that is not a release has no release to be newer than,
// and takes the embedded bundle.
func (r *Resolver) Load(_ context.Context, req Request) (Loaded, error) {
	now := r.now()
	own := OwnRelease(req.GeroVersion)
	skipped := ""
	for _, cached := range r.Store.Releases() {
		if !NewerThan(cached.Tag, own) || !FitsChannel(cached.Tag, own, req.Channel) {
			continue
		}
		l, err := fromEntry(cached, now)
		if err != nil {
			if skipped == "" {
				skipped = fmt.Sprintf("the cached bundle of %s is not readable (%v)", cached.Tag, err)
			}
			continue
		}
		if skipped != "" {
			l.Source.Note = skipped + "; using the bundle of " + cached.Tag
		}
		return l, nil
	}
	l, err := r.embedded(own, now)
	if skipped != "" {
		l.Source.Note = skipped + "; using the embedded bundle"
	}
	return l, err
}

// KeepEntries is how many bundles the store keeps: the newest by when each was
// fetched or compiled.
const KeepEntries = 3

// embedded is the bundle of the config compiled into the binary, for the gero of
// release own. It is looked for in the store under the ID of the files and the
// compiler, which the files alone tell, and compiled and kept there when it is not.
// A store that cannot be read or written costs only the compile: the bundle made in
// memory is used either way.
func (r *Resolver) embedded(own string, now time.Time) (Loaded, error) {
	fsys, hdr := r.Embedded, r.EmbeddedHeader
	if fsys == nil {
		fsys = embedded.FS()
	}
	if hdr.GeroVersion == "" {
		hdr = embedded.Header
	}
	digest, err := bundle.Digest(fsys)
	if err != nil {
		return Loaded{}, fmt.Errorf("the embedded bundle: %w", err)
	}
	id := bundle.Header{InputsDigest: digest, CompilerRevision: bundle.CompilerRevision}.ID()
	data := r.storedBundle(id)
	if data == nil {
		if data, err = bundle.Compile(fsys, hdr); err != nil {
			return Loaded{}, fmt.Errorf("the embedded bundle: %w", err)
		}
		if _, err := r.Store.Put(Entry{Tag: own, Origin: OriginEmbedded, Data: data, FetchedAt: now}); err == nil {
			_ = r.Store.Prune(KeepEntries)
		}
	}
	h, bundleFS, err := bundle.Parse(data)
	if err != nil {
		return Loaded{}, fmt.Errorf("the embedded bundle: %w", err)
	}
	return Loaded{Header: h, FS: bundleFS, Source: Source{Embedded: true, Tag: own}}, nil
}

// storedBundle is the bytes of the entry with the ID, or nil when the store has none.
func (r *Resolver) storedBundle(id string) []byte {
	if e, ok := r.Store.Get(id); ok {
		return e.Data
	}
	return nil
}

// NewerThan is whether the release tag is newer than the gero's own. A gero with no
// release of its own has nothing a cached bundle could be newer than.
func NewerThan(tag, own string) bool {
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

// WantsPrereleases is whether pre-releases are candidates: as the channel says, or,
// left to auto, when the gero itself is a pre-release.
func WantsPrereleases(own, channel string) bool {
	switch channel {
	case ChannelStable:
		return false
	case ChannelPrerelease:
		return true
	}
	return own != "" && IsPrerelease(own)
}

// FitsChannel is whether a cached release is one the channel would choose: a
// pre-release is not, when pre-releases are not candidates.
func FitsChannel(tag, own, channel string) bool {
	return WantsPrereleases(own, channel) || !IsPrerelease(tag)
}

// FromEntry is a stored bundle ready for the config loader, as of the time.
func FromEntry(e Entry, now time.Time) (Loaded, error) {
	return fromEntry(e, now)
}

func fromEntry(e Entry, now time.Time) (Loaded, error) {
	h, fsys, err := bundle.Parse(e.Data)
	if err != nil {
		return Loaded{}, fmt.Errorf("the cached bundle of %s: %w", e.Tag, err)
	}
	return Loaded{Header: h, FS: fsys, Source: Source{
		Tag: e.Tag, FetchedAt: e.FetchedAt, Age: now.Sub(e.FetchedAt), FromCache: true,
	}}, nil
}

// OwnRelease is the release tag of the running gero, or "" for a build that is not
// one, such as "dev". GoReleaser stamps the version without the leading v that the
// tags of the releases have, so it is put back.
func OwnRelease(version string) string {
	if _, ok := assetfetch.CompareVersions(version, version); !ok {
		return ""
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func IsPrerelease(tag string) bool {
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
