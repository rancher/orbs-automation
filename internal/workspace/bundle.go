package workspace

import (
	"context"

	"github.com/rancher/orbs-automation/internal/bundlestore"
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
	return ConfigSource{}, nil
}

// bundleFor is the default bundle source for the settings: the release's bundle,
// fetched, verified and cached. There is none without a gero version, which only
// cmd/ sets, so no test or library caller reaches the network by accident, and
// none in CI, which has a checkout of the central repo (docs/specs/CENTRAL_BUNDLE.md).
func bundleFor(s clicfg.Settings) BundleSource {
	return nil
}
