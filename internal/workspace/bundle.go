package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundlefetch"
	"github.com/rancher/orbs-automation/internal/bundlestore"
	"github.com/rancher/orbs-automation/internal/gero/assetfetch"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/gero/releasesign"
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

// bundleFor is the default bundle source for the settings: a cached bundle newer than
// this gero, else the bundle compiled into it, and never a request. There is none
// without a gero version, which only cmd/ sets, so a test or library caller reads the
// config it was given and nothing else; and none when the config is to come from a
// directory. CI is no exception: a job with only the binary reads the bundle in it
// (docs/specs/CENTRAL_BUNDLE.md).
func bundleFor(s clicfg.Settings) BundleSource {
	if s.GeroVersion == "" || s.ConfigSource == clicfg.ConfigSourceDir {
		return nil
	}
	return resolverSource{version: s.GeroVersion, channel: channelOf(s)}
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
	fetchTimeout = 60 * time.Second
)

// resolverSource is the BundleSource over the real cache and the bundle in the
// binary. It makes no request: a newer bundle gets into the cache by FetchBundle.
type resolverSource struct {
	version string
	channel string
}

func (r resolverSource) Load(ctx context.Context) (bundlestore.Loaded, error) {
	return resolverFor().Load(ctx, bundlestore.Request{GeroVersion: r.version, Channel: r.channel})
}

// storeFor is the user's cache of bundles.
func storeFor() bundlestore.Store {
	home, _ := os.UserHomeDir()
	return bundlestore.Store{Dir: clicfg.CacheDir(os.Getenv, home)}
}

// resolverFor is the resolver over the user's cache and the config in the binary.
func resolverFor() *bundlestore.Resolver {
	return &bundlestore.Resolver{Store: storeFor()}
}

// ErrOffline is returned by FetchBundle when the settings say no request is to be
// made.
var ErrOffline = errors.New("fetching the config bundle needs the network, and this run is offline")

// FetchBundle is the one place the config bundle is looked for on the network, for
// gero config fetch: it asks the releases of this repository for a bundle newer than
// this gero's own on its channel, and keeps a verified one in the user's cache, where
// every later run finds it. The result says what is now in use. The token is looked
// up only here, since a public repository needs none and the lookup can start gh.
func FetchBundle(ctx context.Context, s clicfg.Settings) (bundlestore.Loaded, error) {
	if s.Offline {
		return bundlestore.Loaded{}, ErrOffline
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	// The signatures are checked against this repository's release workflow, so
	// a bundle another repository, or a stolen token, published is refused.
	verifier, err := releasesign.New(nil)
	if err != nil {
		return bundlestore.Loaded{}, err
	}
	_, token := defaultCredentials()
	releases := assetfetch.New(releaseOwner, releaseRepo, assetfetch.Options{Token: token, Verifier: verifier})
	fetcher := &bundlefetch.Fetcher{Releases: releases, Store: storeFor(), Local: resolverFor()}
	return fetcher.Fetch(ctx, bundlestore.Request{GeroVersion: s.GeroVersion, Channel: channelOf(s)})
}
