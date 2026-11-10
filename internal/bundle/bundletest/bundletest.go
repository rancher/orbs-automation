// Package bundletest publishes imagined releases of gero for tests: it compiles a
// config into a bundle exactly as the release does, and puts it on a fakehub.Hub
// with the checksums.txt GoReleaser would write. It is test support, and no
// production code imports it.
package bundletest

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/platform/assetfetch/fakehub"
)

// CompiledAt is the time every test bundle says it was compiled at.
var CompiledAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// Commit is the commit every test bundle says its config came from.
const Commit = "0123abc"

// Config is a central config of one project, with a stream, for each name: the
// least that is valid, in a file of its own.
func Config(names ...string) fstest.MapFS {
	cfg := fstest.MapFS{}
	for _, n := range names {
		cfg[n+".yaml"] = &fstest.MapFile{Data: []byte(`apiVersion: gero/v1alpha2
kind: Project
name: ` + n + `
spec:
  repo: {name: rancher/` + n + `, managedBy: orbs}
  branches: {layout: single}
---
apiVersion: gero/v1alpha2
kind: Stream
name: ` + n + `
refs: {project: ` + n + `}
spec:
  scheme: semver
  requiresPrerelease: true
  universal: true
`)}
	}
	return cfg
}

// Compile is the bundle of cfg for the release tag.
func Compile(t testing.TB, tag string, cfg fs.FS) []byte {
	t.Helper()
	data, err := bundle.Compile(cfg, bundle.Header{GeroVersion: tag, SourceCommit: Commit, CompiledAt: CompiledAt})
	if err != nil {
		t.Fatalf("compile the test bundle: %v", err)
	}
	return data
}

// Publish compiles cfg and publishes it on the hub as the bundle asset of the
// release tag, and returns the bundle.
func Publish(t testing.TB, h *fakehub.Hub, tag string, cfg fs.FS, opts ...fakehub.Option) []byte {
	t.Helper()
	data := Compile(t, tag, cfg)
	PublishRaw(h, tag, data, opts...)
	return data
}

// PublishRaw publishes data, which need not be a bundle, as the bundle asset of the
// release tag.
func PublishRaw(h *fakehub.Hub, tag string, data []byte, opts ...fakehub.Option) {
	h.Publish(tag, append([]fakehub.Option{fakehub.Asset(bundle.Name, data)}, opts...)...)
}
