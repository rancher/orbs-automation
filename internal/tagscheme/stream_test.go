package tagscheme

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func proj(name, spec string) string { return chaintest.Doc("Project", name, "", spec) }

func strm(name, project, spec string) string {
	return chaintest.Doc("Stream", name, "{project: "+project+"}", spec)
}

// graph has kuberlr-kubectl with lines of its own, scc-operator that is universal,
// rancher-assets that finds its lines in its tags, rancher-charts with a chart
// stream, and mono with two streams.
func graph(t *testing.T) *chain.Graph {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		proj("kuberlr", "  repo: {name: rancher/kuberlr-kubectl}\n"),
		strm("kuberlr", "kuberlr", "  tagExclude: [\"*-hotfix*\"]\n  lines:\n    - {line: v2.16, series: \"9.x\"}\n    - {line: v2.15, series: \"8.x\"}\n"),
		proj("scc", "  repo: {name: rancher/scc-operator}\n"),
		strm("scc", "scc", "  universal: true\n"),
		proj("assets", "  repo: {name: rancher/rancher-assets}\n"),
		strm("assets", "assets", "  scheme: calverLineStage\n  lines: [{line: v2.16}, {line: v2.15}]\n"),
		proj("charts", "  repo: {name: rancher/charts}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines: [{line: v2.16, branch: {dev: d, stable: s}}]\n"),
		strm("chart", "charts", "  scheme: chartVersion\n  lines: [{line: v2.16, series: \"111.x\"}]\n"),
		proj("mono", "  repo: {name: acme/mono}\n"),
		strm("a", "mono", "  universal: true\n  tagPattern: \"a/v*\"\n"),
		strm("b", "mono", "  universal: true\n  tagPattern: \"b/v*\"\n"),
		proj("bare", "  repo: {name: acme/bare}\n"),
	)
}

func streamScheme(t *testing.T, project string) Scheme {
	t.Helper()
	s, err := ForProject(graph(t).Project(project))
	require.NoError(t, err)
	require.NotNil(t, s)
	return s
}

func TestSemverSchemeOfAStream(t *testing.T) {
	s := streamScheme(t, "kuberlr")

	assert.Equal(t, "semver", s.Name())
	assert.True(t, s.Valid("v8.1.0"))
	assert.False(t, s.Valid("main-abc1234"))
	assert.Equal(t, []string{"v2.16", "v2.15"}, s.Lines(nil))
	assert.Equal(t, "v2.15", s.LineOf("v8.1.0"))
	assert.Equal(t, "v2.16", s.LineOf("v9.0.0-rc.1"))
	assert.Equal(t, "", s.LineOf("v7.0.0"))
	assert.Equal(t, "", s.Stage("v8.1.0"))
	assert.True(t, s.IsStable("v8.1.0"))
	assert.False(t, s.IsStable("v8.1.0-rc.1"))
	assert.Negative(t, s.Compare("v8.0.0", "v8.1.0"))
}

func TestSemverSchemeOfAUniversalStream(t *testing.T) {
	s := streamScheme(t, "scc")

	assert.Empty(t, s.Lines(nil))
	assert.Equal(t, "", s.LineOf("v0.5.3"))
}

func TestSemverLatestOfAStreamByLineAndAcrossLines(t *testing.T) {
	s := streamScheme(t, "kuberlr")
	all := tags("v9.0.0", "v9.1.0-rc.1", "v8.1.0", "v8.0.0", "v9.0.1-hotfix1")

	got := s.Latest(all, "v2.16")
	require.NotNil(t, got.Stable)
	assert.Equal(t, "v9.0.0", got.Stable.Name, "a tag the stream excludes is not a release")
	require.NotNil(t, got.Prerelease)
	assert.Equal(t, "v9.1.0-rc.1", got.Prerelease.Name)

	got = s.Latest(all, "v2.15")
	require.NotNil(t, got.Stable)
	assert.Equal(t, "v8.1.0", got.Stable.Name)
	assert.Nil(t, got.Prerelease)

	got = s.Latest(all, "")
	require.NotNil(t, got.Stable)
	assert.Equal(t, "v9.0.0", got.Stable.Name)
	assert.Equal(t, git.Latest{}, s.Latest(all, "v2.99"))
}

func TestCalverSchemeOfAStream(t *testing.T) {
	s := streamScheme(t, "assets")
	assetsTags := tags(
		"v2.16-20261030T1111Z-dev", "v2.16-20261101T0900Z-dev", "v2.16-20261029T0000Z",
		"v2.15-20260915T1243Z", "v2.15-20260915T1243Z-dev", "v2.14-20260401T1000Z", "v2.16", "main-abc1234",
	)

	assert.Equal(t, "calverLineStage", s.Name())
	assert.True(t, s.Valid("v2.16-20261029T0000Z"))
	assert.False(t, s.Valid("v2.16"))
	assert.Equal(t, []string{"v2.16", "v2.15"}, s.Lines(assetsTags), "the lines the config lists, whatever the tags")
	assert.Equal(t, "v2.15", s.LineOf("v2.15-20260915T1243Z"))
	assert.Equal(t, "dev", s.Stage("v2.16-20261030T1111Z-dev"))
	assert.Equal(t, "stable", s.Stage("v2.16-20261029T0000Z"))
	assert.True(t, s.IsStable("v2.16-20261029T0000Z"))
	assert.False(t, s.IsStable("v2.16-20261030T1111Z-dev"))
	assert.Negative(t, s.Compare("v2.16-20261029T0000Z", "v2.16-20261030T1111Z-dev"))

	got := s.Latest(assetsTags, "v2.16")
	require.NotNil(t, got.Stable)
	assert.Equal(t, "v2.16-20261029T0000Z", got.Stable.Name)
	require.NotNil(t, got.Prerelease)
	assert.Equal(t, "v2.16-20261101T0900Z-dev", got.Prerelease.Name)
	assert.Equal(t, git.Latest{}, s.Latest(assetsTags, "v2.14"), "a line the config does not list")
}

func TestForStreamTakesTheSchemeOfTheStream(t *testing.T) {
	g := graph(t)

	s, err := ForStream(g.Project("assets").Streams()[0])
	require.NoError(t, err)
	assert.Equal(t, "calverLineStage", s.Name())

	s, err = ForStream(g.Project("scc").Streams()[0])
	require.NoError(t, err)
	assert.Equal(t, "semver", s.Name())
}

func TestAChartStreamHasNoSchemeYet(t *testing.T) {
	_, err := ForProject(graph(t).Project("charts"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "chartVersion")
}

func TestForProjectNeedsExactlyOneStream(t *testing.T) {
	g := graph(t)

	_, err := ForProject(g.Project("bare"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no stream")

	_, err = ForProject(g.Project("mono"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 streams")
}
