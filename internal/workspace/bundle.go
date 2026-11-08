package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rancher/orbs-automation/internal/bundlestore"
	"github.com/rancher/orbs-automation/internal/platform/assetfetch"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
)

// BundleSource is where the central config comes from when no local source is
// named: the bundle of a gero release (docs/specs/CENTRAL_BUNDLE.md). It is
// defined here, where it is consumed; bundlestore.Resolver is the real one.
type BundleSource interface {
	Load(ctx context.Context) (bundlestore.Loaded, error)
}

// ConfigSource says what the central config was read from: a local directory,
// which always wins, or the bundle. Exactly one is set.
type ConfigSource struct {
	Dir    CentralConfigDir
	Bundle *bundlestore.Source
}

// WithBundle sets where the bundle comes from, in place of the default, and
// returns w. A nil source means there is none.
func (w *Workspace) WithBundle(b BundleSource) *Workspace {
	w.bundle = b
	return w
}

// ConfigSource resolves what the central config is read from, once for the
// invocation. A local source wins over the bundle, and when one is named but
// broken the problem is reported and the bundle is never tried. With none named
// the bundle is loaded, and ErrNotFound is returned, naming why, when it cannot be.
func (w *Workspace) ConfigSource() (ConfigSource, error) {
	w.sourceOnce.Do(func() { w.source, w.sourceErr = w.resolveSource() })
	return w.source, w.sourceErr
}

func (w *Workspace) resolveSource() (ConfigSource, error) {
	dir, err := w.CentralConfigDir()
	if err == nil {
		return ConfigSource{Dir: dir}, nil
	}
	if !errors.Is(err, ErrNotFound) || w.bundle == nil {
		return ConfigSource{}, err
	}
	l, berr := w.bundle.Load(context.Background())
	if berr != nil {
		return ConfigSource{}, fmt.Errorf("%w; the central config bundle is not available either: %w", ErrNotFound, berr)
	}
	w.bundleFiles = &l
	return ConfigSource{Bundle: &l.Source}, nil
}

// bundleFor is the default bundle source for the settings: the release's bundle,
// fetched, verified and cached. There is none without a gero version, which only
// cmd/ sets, so no test or library caller reaches the network by accident, and
// none in CI, which has a checkout of the central repo (docs/specs/CENTRAL_BUNDLE.md).
func bundleFor(s clicfg.Settings) BundleSource {
	if s.GeroVersion == "" || s.CI {
		return nil
	}
	return resolverSource{version: s.GeroVersion, offline: s.Offline}
}

// The repository whose releases carry the bundle. It is fixed in the binary: the
// bundle is the central config of this project, and a release is what vouches for it.
const (
	releaseOwner = "rancher"
	releaseRepo  = "orbs-automation"
	loadTimeout  = 60 * time.Second
)

// resolverSource is the BundleSource over the real cache and the real releases. The
// token is looked up only when a request is to be made: a public repository needs
// none, and the lookup can start gh.
type resolverSource struct {
	version string
	offline bool
}

func (r resolverSource) Load(ctx context.Context) (bundlestore.Loaded, error) {
	ctx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()
	home, _ := os.UserHomeDir()
	res := bundlestore.Resolver{Cache: bundlestore.Cache{Dir: clicfg.CacheDir(os.Getenv, home)}}
	if !r.offline {
		_, token := defaultCredentials()
		res.Releases = assetfetch.New(releaseOwner, releaseRepo, assetfetch.Options{Token: token})
	}
	return res.Load(ctx, bundlestore.Request{GeroVersion: r.version, Offline: r.offline})
}
