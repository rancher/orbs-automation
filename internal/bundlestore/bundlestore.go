// Package bundlestore finds the central config bundle for a workstation that has
// no local config: it chooses the release, downloads and verifies the bundle, keeps
// it in the user's cache, and falls back to the cache when the network fails
// (docs/specs/CENTRAL_BUNDLE.md). It never looks at local config, which always wins
// and is the workspace's business.
package bundlestore

import (
	"context"
	"errors"
	"io/fs"
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

// Load finds the bundle for the request.
func (r *Resolver) Load(ctx context.Context, req Request) (Loaded, error) {
	return Loaded{}, nil
}
