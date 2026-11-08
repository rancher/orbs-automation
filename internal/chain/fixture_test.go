package chain_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/rancher/orbs-automation/internal/appconfig"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureGraph joins the design fixture, the seven projects of the release chain.
func fixtureGraph(t *testing.T) *chain.Graph {
	t.Helper()
	dir := "../../docs/design/fixture"
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	fsys := fstest.MapFS{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		fsys[e.Name()] = &fstest.MapFile{Data: raw}
	}
	cfg, rep := appconfig.Load(fsys, config.Context{Now: chaintest.Now})
	require.NoError(t, rep.Err())
	return chain.Build(cfg)
}

func TestTheFixtureJoinsIntoTheReleaseChain(t *testing.T) {
	g := fixtureGraph(t)

	require.Len(t, g.Projects(), 7)
	rancher := g.Project("rancher")
	require.NotNil(t, rancher)
	assert.Equal(t, "shared", rancher.Steward())
	assert.Equal(t, "release/v2.15", mustBranch(t, rancher, "v2.15", ""))
	assert.Equal(t, "dev-v2.16", mustBranch(t, g.Project("rancher-charts"), "v2.16", "dev"))
	assert.Equal(t, "release-v2.16", mustBranch(t, g.Project("rancher-charts"), "v2.16", "stable"))

	var held []string
	for _, h := range rancher.Held() {
		held = append(held, h.Name())
	}
	assert.Contains(t, held, "rancher-holds-shell")
	assert.Contains(t, held, "rancher-follows-charts-branch")
	assert.NotEmpty(t, g.Project("shell").Sourced())
	assert.NotEmpty(t, g.Project("backup-restore-operator").Sourced(), "charts holds its chart files")
}

func mustBranch(t *testing.T, p *chain.Project, line, stage string) string {
	t.Helper()
	b, ok := p.BranchOfLine(line, stage)
	require.True(t, ok, "%s %s %s", p.Name(), line, stage)
	return b
}
