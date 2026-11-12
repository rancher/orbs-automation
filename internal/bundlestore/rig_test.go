package bundlestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundle/bundletest"
	"github.com/rancher/orbs-automation/internal/chainload"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

var bg = context.Background()

var t0 = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

// rig is a workstation with an empty cache, a clock, and the config in its binary of
// one project named "embedded".
type rig struct {
	t     *testing.T
	store Store
	now   time.Time
	r     *Resolver
}

func newRig(t *testing.T) *rig {
	t.Helper()
	g := &rig{t: t, store: Store{Dir: t.TempDir()}, now: t0}
	g.r = &Resolver{
		Store:          g.store,
		Now:            func() time.Time { return g.now },
		Embedded:       bundletest.Config("embedded"),
		EmbeddedHeader: bundle.Header{GeroVersion: "embedded", SourceCommit: bundletest.Commit, CompiledAt: bundletest.CompiledAt},
	}
	return g
}

// cached puts a bundle of the release in the cache, fetched at the time.
func (g *rig) cached(tag string, at time.Time, names ...string) {
	g.t.Helper()
	data := bundletest.Compile(g.t, tag, bundletest.Config(names...))
	_, err := g.store.Put(Entry{Tag: tag, Data: data, FetchedAt: at})
	require.NoError(g.t, err)
}

func projects(t *testing.T, l Loaded) int {
	t.Helper()
	require.NotNil(t, l.FS)
	g, rep := chainload.Load(l.FS, config.Context{Now: bundletest.CompiledAt})
	require.NoError(t, rep.Err())
	return len(g.Projects())
}
