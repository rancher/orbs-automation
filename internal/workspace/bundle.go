package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundlestore"
	"github.com/rancher/orbs-automation/internal/platform/assetfetch"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/releasesign"
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
	mode := w.settings.ConfigSource
	if file := w.settings.BundleFile; file != "" {
		if mode == clicfg.ConfigSourceDir {
			return ConfigSource{}, errors.New("--bundle-file names a bundle and --config-source=dir allows none: use one of them")
		}
		return w.loadBundleFile(file)
	}
	if mode == clicfg.ConfigSourceBundle {
		return w.loadBundle(errors.New("--config-source=bundle: the central config bundle is not available"))
	}
	dir, err := w.CentralConfigDir()
	if err == nil {
		return ConfigSource{Dir: dir}, nil
	}
	if mode == clicfg.ConfigSourceDir || !errors.Is(err, ErrNotFound) || w.bundle == nil {
		return ConfigSource{}, err
	}
	return w.loadBundle(ErrNotFound)
}

// loadBundleFile reads a bundle file the user downloaded, with no fetch and no
// cache. Its format and versions are checked as for any bundle; its signature and
// checksum are not, since the user chose the file.
func (w *Workspace) loadBundleFile(path string) (ConfigSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ConfigSource{}, fmt.Errorf("--bundle-file %s: %w", path, err)
	}
	h, fsys, err := bundle.Parse(data)
	if err != nil {
		return ConfigSource{}, fmt.Errorf("--bundle-file %s: %w", path, err)
	}
	l := bundlestore.Loaded{Header: h, FS: fsys, Source: bundlestore.Source{File: path, Tag: h.GeroVersion}}
	w.bundleFiles = &l
	return ConfigSource{Bundle: &l.Source}, nil
}

// loadBundle reads the bundle. When there is none, or it cannot be loaded, the
// error wraps lead, which says what was being looked for, and the cause.
func (w *Workspace) loadBundle(lead error) (ConfigSource, error) {
	if w.bundle == nil {
		return ConfigSource{}, fmt.Errorf("%w: there is no release to take it from (a development build, or no gero version)", lead)
	}
	l, err := w.bundle.Load(context.Background())
	if err != nil {
		return ConfigSource{}, fmt.Errorf("%w; the central config bundle is not available either: %w", lead, err)
	}
	w.bundleFiles = &l
	return ConfigSource{Bundle: &l.Source}, nil
}

// bundleFor is the default bundle source for the settings: the release's bundle,
// fetched, verified and cached. There is none without a gero version, which only
// cmd/ sets, so no test or library caller reaches the network by accident, and
// none in CI, which has a checkout of the central repo (docs/specs/CENTRAL_BUNDLE.md).
func bundleFor(s clicfg.Settings) BundleSource {
	if s.GeroVersion == "" || s.ConfigSource == clicfg.ConfigSourceDir {
		return nil
	}
	// CI has a checkout, unless the bundle is asked for by name.
	if s.CI && s.ConfigSource != clicfg.ConfigSourceBundle {
		return nil
	}
	return resolverSource{version: s.GeroVersion, offline: s.Offline, channel: channelOf(s)}
}

// channelOf is the bundle channel of the settings, as the resolver names it.
func channelOf(s clicfg.Settings) string {
	switch s.BundleChannel {
	case clicfg.BundleChannelStable:
		return bundlestore.ChannelStable
	case clicfg.BundleChannelPrerelease:
		return bundlestore.ChannelPrerelease
	}
	return bundlestore.ChannelAuto
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
	channel string
}

func (r resolverSource) Load(ctx context.Context) (bundlestore.Loaded, error) {
	ctx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()
	home, _ := os.UserHomeDir()
	res := bundlestore.Resolver{Cache: bundlestore.Cache{Dir: clicfg.CacheDir(os.Getenv, home)}}
	if !r.offline {
		// The signatures are checked against this repository's release workflow, so
		// a bundle another repository, or a stolen token, published is refused.
		verifier, err := releasesign.New(nil)
		if err != nil {
			return bundlestore.Loaded{}, err
		}
		_, token := defaultCredentials()
		res.Releases = assetfetch.New(releaseOwner, releaseRepo, assetfetch.Options{Token: token, Verifier: verifier})
	}
	return res.Fetch(ctx, bundlestore.Request{GeroVersion: r.version, Offline: r.offline, Channel: r.channel})
}
