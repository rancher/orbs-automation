package tagscheme

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tags(names ...string) []git.Tag {
	out := make([]git.Tag, len(names))
	for i, n := range names {
		out[i] = git.Tag{Name: n, Commit: "sha-" + n}
	}
	return out
}

func TestSemverValid(t *testing.T) {
	s := streamScheme(t, "kuberlr")
	for tag, want := range map[string]bool{"v8.1.0": true, "v8.1.0-rc.1": true, "8.1.0": true, "main-abc1234": false, "latest": false, "": false} {
		assert.Equal(t, want, s.Valid(tag), tag)
	}
}

func TestSemverLinesAreTheOnesTheConfigListsWhateverTheTags(t *testing.T) {
	assert.Equal(t, []string{"v2.16", "v2.15"}, streamScheme(t, "kuberlr").Lines(tags("v8.1.0")))
	assert.Empty(t, streamScheme(t, "scc").Lines(tags("v0.5.3")), "a project with no lines has none")
}

func TestSemverLineOf(t *testing.T) {
	s := streamScheme(t, "kuberlr")
	assert.Equal(t, "v2.15", s.LineOf("v8.1.0"))
	assert.Equal(t, "v2.16", s.LineOf("v9.0.0-rc.1"))
	assert.Equal(t, "", s.LineOf("v7.0.0"), "a series no line has")
	assert.Equal(t, "", streamScheme(t, "scc").LineOf("v0.5.3"))
}

func TestSemverLatestOfALine(t *testing.T) {
	s := streamScheme(t, "kuberlr")
	all := tags("v8.0.9", "v8.1.0", "v8.2.0-rc.1", "v9.0.0", "v9.1.0-rc.1", "main-abc1234", "v7.9.9")

	got := s.Latest(all, "v2.15")
	require.NotNil(t, got.Stable)
	require.NotNil(t, got.Prerelease)
	assert.Equal(t, "v8.1.0", got.Stable.Name)
	assert.Equal(t, "v8.2.0-rc.1", got.Prerelease.Name, "a prerelease newer than the stable one")

	got = s.Latest(tags("v8.0.9", "v8.1.0", "v8.0.10-rc.1"), "v2.15")
	require.NotNil(t, got.Stable)
	assert.Equal(t, "v8.1.0", got.Stable.Name)
	assert.Nil(t, got.Prerelease, "an older prerelease is history")
}

func TestSemverLatestOfEveryLineIsTheHighestVersion(t *testing.T) {
	got := streamScheme(t, "kuberlr").Latest(tags("v8.1.0", "v9.0.0", "v9.1.0-rc.1"), "")

	require.NotNil(t, got.Stable)
	require.NotNil(t, got.Prerelease)
	assert.Equal(t, "v9.0.0", got.Stable.Name)
	assert.Equal(t, "v9.1.0-rc.1", got.Prerelease.Name)
}

func TestSemverLatestIgnoresTagsThePatternDoesNotMatch(t *testing.T) {
	g := chaintest.Load(t,
		proj("scc", "  repo: {name: rancher/scc-operator}\n"),
		strm("scc", "scc", "  universal: true\n  tagPattern: \"v0.*\"\n"),
	)
	scheme, err := ForProject(g.Project("scc"))
	require.NoError(t, err)

	got := scheme.Latest(tags("v0.5.3", "v1.0.0"), "")

	require.NotNil(t, got.Stable)
	assert.Equal(t, "v0.5.3", got.Stable.Name)
}

func TestSemverLatestOfALineWithNoTagsOrAnUnknownLineIsEmpty(t *testing.T) {
	s := streamScheme(t, "kuberlr")
	assert.Equal(t, git.Latest{}, s.Latest(tags("v9.0.0"), "v2.15"))
	assert.Equal(t, git.Latest{}, s.Latest(tags("v9.0.0"), "v2.99"))
}

func TestSemverIsStable(t *testing.T) {
	s := streamScheme(t, "kuberlr")
	assert.True(t, s.IsStable("v8.1.0"))
	assert.False(t, s.IsStable("v8.1.0-rc.1"))
}

func TestSemverCompare(t *testing.T) {
	s := streamScheme(t, "kuberlr")
	assert.Positive(t, s.Compare("v8.1.0", "v8.0.9"))
	assert.Negative(t, s.Compare("v8.1.0-rc.1", "v8.1.0"))
	assert.Zero(t, s.Compare("v8.1.0", "v8.1.0"))
	assert.Zero(t, s.Compare("v8.1.0", "main-abc1234"), "a name that is not a release has no order")
}

func TestSemverLatestOfALineWrittenWithoutTheV(t *testing.T) {
	s := streamScheme(t, "kuberlr")
	all := tags("v8.0.9", "v8.1.0", "v9.0.0")

	assert.Equal(t, s.Latest(all, "v2.15"), s.Latest(all, "2.15"), "a line is the same line with or without its v, as everywhere in config")
	assert.NotNil(t, s.Latest(all, "2.15").Stable)
}
