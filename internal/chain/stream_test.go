package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stream(name, proj, spec string) string {
	return chaintest.Doc("Stream", name, "{project: "+proj+"}", spec)
}

func TestAProjectKnowsItsStreams(t *testing.T) {
	g := chaintest.Load(t,
		project("a", "  repo: {name: acme/a}\n"),
		project("b", "  repo: {name: acme/b}\n"),
		stream("a-one", "a", "  universal: true\n"),
		stream("b-one", "b", "  universal: true\n"),
		stream("a-two", "a", "  universal: true\n  tagPattern: \"pkg/v*\"\n"),
	)

	streams := g.Project("a").Streams()
	require.Len(t, streams, 2)
	assert.Equal(t, "a-one", streams[0].Name())
	assert.Equal(t, "a-two", streams[1].Name())
	assert.Equal(t, "a", streams[0].Project().Name())
	assert.Equal(t, "semver", streams[0].Scheme())
	assert.Equal(t, "v*", streams[0].TagPattern())
	assert.Equal(t, "pkg/v*", streams[1].TagPattern())
	require.Len(t, g.Project("b").Streams(), 1)
}

func TestAStreamKnowsWhichTagsAreItsReleases(t *testing.T) {
	g := chaintest.Load(t,
		project("a", "  repo: {name: acme/a}\n"),
		stream("s", "a", "  universal: true\n  tagPattern: \"v*\"\n  tagExclude: [\"*-hotfix*\"]\n"),
	)
	s := g.Project("a").Streams()[0]

	assert.True(t, s.TagMatches("v1.2.3"))
	assert.True(t, s.TagMatches("v1.2.3-rc.1"))
	assert.False(t, s.TagMatches("v1.2.3-hotfix1"))
	assert.False(t, s.TagMatches("pkg/v1.2.3"))
}

func TestTheSeriesAStreamServesForALine(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		project("a", "  repo: {name: acme/a}\n"),
		stream("s", "a", "  lines:\n    - {line: v2.16, series: \"0.10.x\"}\n    - {line: v2.15, series: \"0.9.x\"}\n"),
		stream("u", "a", "  universal: true\n"),
	)
	s, u := g.Project("a").Streams()[0], g.Project("a").Streams()[1]

	series, ok := s.Series("v2.15")
	assert.True(t, ok)
	assert.Equal(t, "0.9.x", series)
	series, ok = s.Series("2.16")
	assert.True(t, ok)
	assert.Equal(t, "0.10.x", series)
	_, ok = s.Series("v2.14")
	assert.False(t, ok)
	_, ok = u.Series("v2.16")
	assert.False(t, ok)

	assert.True(t, s.Serves("v2.16"))
	assert.True(t, s.Serves("V2.15"))
	assert.False(t, s.Serves("v2.14"))
	assert.True(t, u.Serves("v2.14"))
	assert.True(t, u.Serves("v9.99"))
}

func TestTheStagesOfAStream(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16"),
		project("assets", "  repo: {name: acme/assets}\n"),
		project("charts", "  repo: {name: acme/charts}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines: [{line: v2.16, branch: {dev: d, stable: s}}]\n"),
		stream("calver", "assets", "  scheme: calverLineStage\n  lines: [{line: v2.16}]\n"),
		stream("chart", "charts", "  scheme: chartVersion\n  lines: [{line: v2.16, series: \"111.x\"}]\n"),
		stream("semver", "assets", "  universal: true\n"),
		stream("written", "assets", "  universal: true\n  stages: [{name: nightly, prerelease: true}]\n"),
	)
	byName := func(project, name string) *chain.Stream {
		for _, s := range g.Project(project).Streams() {
			if s.Name() == name {
				return s
			}
		}
		t.Fatalf("no stream %s", name)
		return nil
	}

	assert.Equal(t, []v1alpha2.Stage{{Name: "dev", Prerelease: true}, {Name: "stable"}}, byName("assets", "calver").Stages())
	assert.Equal(t, []v1alpha2.Stage{{Name: "dev", Prerelease: true}, {Name: "stable"}}, byName("charts", "chart").Stages())
	assert.Empty(t, byName("assets", "semver").Stages())
	assert.Equal(t, []v1alpha2.Stage{{Name: "nightly", Prerelease: true}}, byName("assets", "written").Stages())
}

func TestTheLineOfATag(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		project("a", "  repo: {name: acme/a}\n"),
		project("charts", "  repo: {name: acme/charts}\n"),
		project("assets", "  repo: {name: acme/assets}\n"),
		stream("shell", "a", "  tagPattern: \"v*\"\n  lines:\n    - {line: v2.16, series: \"0.10.x\"}\n    - {line: v2.15, series: \"0.9.x\"}\n"),
		stream("chart", "charts", "  scheme: chartVersion\n  tagPattern: \"*\"\n  lines: [{line: v2.16, series: \"111.x\"}]\n"),
		stream("calver", "assets", "  scheme: calverLineStage\n  lines: [{line: v2.16}]\n"),
	)
	shell := g.Project("a").Streams()[0]

	for tag, want := range map[string]string{"v0.10.3": "v2.16", "0.10.0": "v2.16", "v0.9.1-rc.1": "v2.15"} {
		line, ok := shell.LineOf(tag)
		assert.True(t, ok, tag)
		assert.Equal(t, want, line, tag)
	}
	for _, tag := range []string{"v1.0.0", "v0.11.0", "nonsense", ""} {
		_, ok := shell.LineOf(tag)
		assert.False(t, ok, tag)
	}
	line, ok := g.Project("charts").Streams()[0].LineOf("111.0.2+up1.2.3")
	assert.True(t, ok)
	assert.Equal(t, "v2.16", line)
	_, ok = g.Project("assets").Streams()[0].LineOf("v2.16-20261030T1111Z")
	assert.False(t, ok, "a calver tag carries its line, which the tag scheme reads")
}

func TestAProjectFindsTheStreamAndLineOfATag(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16"),
		project("mono", "  repo: {name: acme/mono}\n"),
		stream("pkg-a", "mono", "  universal: true\n  tagPattern: \"pkg-a/v*\"\n"),
		stream("pkg-b", "mono", "  tagPattern: \"v*\"\n  lines: [{line: v2.16, series: \"3.x\"}]\n"),
	)
	mono := g.Project("mono")

	require.NotNil(t, mono.StreamOf("pkg-a/v1.2.3"))
	assert.Equal(t, "pkg-a", mono.StreamOf("pkg-a/v1.2.3").Name())
	assert.Equal(t, "pkg-b", mono.StreamOf("v3.1.0").Name())
	assert.Nil(t, mono.StreamOf("other/v1.0.0"))

	line, ok := mono.LineOf("v3.1.0")
	assert.True(t, ok)
	assert.Equal(t, "v2.16", line)
	_, ok = mono.LineOf("pkg-a/v1.2.3")
	assert.False(t, ok, "a universal stream has no line of its own")
	_, ok = mono.LineOf("other/v1.0.0")
	assert.False(t, ok)
}
