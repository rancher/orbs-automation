// Package bundlefetch is the network side of the central config bundle: it asks the
// releases of this repository for a bundle newer than the gero's own, checks what it
// downloads, and keeps it in the store (internal/bundlestore), where every later run
// finds it. It never loads a bundle for a command: that is the store's resolver, which
// it falls back on when nothing newer is published (docs/specs/CENTRAL_BUNDLE.md).
package bundlefetch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundlestore"
	"github.com/rancher/orbs-automation/internal/gero/assetfetch"
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

// Loader is what gives the bundle to use when nothing newer was fetched:
// bundlestore.Resolver is the real one.
type Loader interface {
	Load(ctx context.Context, req bundlestore.Request) (bundlestore.Loaded, error)
}

// Fetcher fetches bundles into a store.
type Fetcher struct {
	Releases Releases
	Store    bundlestore.Store
	Local    Loader
	Now      func() time.Time // nil for time.Now
}

func (f *Fetcher) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
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
func (f *Fetcher) Fetch(ctx context.Context, req bundlestore.Request) (bundlestore.Loaded, error) {
	own := bundlestore.OwnRelease(req.GeroVersion)
	if own == "" {
		return f.loadNoting(ctx, req, "this build is not a release, so there is no newer release to fetch a bundle of")
	}
	prereleases := bundlestore.WantsPrereleases(own, req.Channel)
	latest, err := f.Releases.Latest(ctx, prereleases)
	if err != nil {
		return bundlestore.Loaded{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if !bundlestore.NewerThan(latest.Tag, own) {
		return f.loadNoting(ctx, req, fmt.Sprintf("the newest release on this channel, %s, is not newer than this gero's own, %s", latest.Tag, own))
	}
	now := f.now()
	l, err := f.use(ctx, latest.Tag, now)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, bundle.ErrUnsupported) {
		return bundlestore.Loaded{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	note := fmt.Sprintf("the newest bundle, %s, is one this gero cannot read (%v)", latest.Tag, err)
	if prereleases && bundlestore.IsPrerelease(latest.Tag) {
		if stable, serr := f.Releases.Latest(ctx, false); serr == nil && bundlestore.NewerThan(stable.Tag, own) {
			if l, ferr := f.use(ctx, stable.Tag, now); ferr == nil {
				l.Source.Note = note + "; using the bundle of " + stable.Tag
				return l, nil
			}
		}
	}
	return f.loadNoting(ctx, req, note)
}

// loadNoting is what Load gives, with the note said before any it has of its own.
func (f *Fetcher) loadNoting(ctx context.Context, req bundlestore.Request, note string) (bundlestore.Loaded, error) {
	l, err := f.Local.Load(ctx, req)
	if l.Source.Note != "" {
		note += "; " + l.Source.Note
	}
	l.Source.Note = note
	return l, err
}

// use is the bundle of the release: the stored one when it is of that release, else
// the download, which is kept, and the store is pruned.
func (f *Fetcher) use(ctx context.Context, tag string, now time.Time) (bundlestore.Loaded, error) {
	for _, cached := range f.Store.Releases() {
		if cached.Tag != tag {
			continue
		}
		if l, err := bundlestore.FromEntry(cached, now); err == nil {
			if err := f.Store.Touch(cached.ID, now); err == nil {
				l.Source.FetchedAt, l.Source.Age = now, 0
			}
			return l, nil
		}
		break
	}
	data, err := f.Releases.Asset(ctx, tag, bundle.Name)
	if err != nil {
		return bundlestore.Loaded{}, err
	}
	h, fsys, err := bundle.Parse(data)
	if err != nil {
		return bundlestore.Loaded{}, fmt.Errorf("the bundle of %s: %w", tag, err)
	}
	// A store that cannot be written costs the next run a download.
	if _, err := f.Store.Put(bundlestore.Entry{Tag: tag, Data: data, Origin: bundlestore.OriginRelease, FetchedAt: now}); err == nil {
		_ = f.Store.Prune(bundlestore.KeepEntries)
	}
	return bundlestore.Loaded{Header: h, FS: fsys, Source: bundlestore.Source{Tag: tag, FetchedAt: now}}, nil
}
