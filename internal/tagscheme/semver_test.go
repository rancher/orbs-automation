package tagscheme

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
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

// lined is a project with release lines of its own, as kuberlr-kubectl has.
func lined() config.Project {
	return config.Project{
		Name: "kuberlr-kubectl", Repo: config.Repo{Name: "rancher/kuberlr-kubectl"},
		Release: config.Release{Branching: config.BranchingPerLine, TagPattern: "v*", Lines: []config.ProjectLine{
			{Line: "v2.16", Version: "9.x"}, {Line: "v2.15", Version: "8.x"},
		}},
	}
}

func universal() config.Project {
	return config.Project{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}, Release: config.Release{TagPattern: "v*"}}
}

func semverFor(t *testing.T, p config.Project) Scheme {
	t.Helper()
	s, err := For(p)
	require.NoError(t, err)
	require.Equal(t, "semver", s.Name())
	return s
}

func TestSemverValid(t *testing.T) {
	s := semverFor(t, lined())
	for tag, want := range map[string]bool{"v8.1.0": true, "v8.1.0-rc.1": true, "8.1.0": true, "main-abc1234": false, "latest": false, "": false} {
		assert.Equal(t, want, s.Valid(tag), tag)
	}
}

func TestSemverLinesAreTheOnesTheConfigListsWhateverTheTags(t *testing.T) {
	assert.Equal(t, []string{"v2.16", "v2.15"}, semverFor(t, lined()).Lines(tags("v8.1.0")))
	assert.Empty(t, semverFor(t, universal()).Lines(tags("v0.5.3")), "a project with no lines has none")
}

func TestSemverLineOf(t *testing.T) {
	s := semverFor(t, lined())
	assert.Equal(t, "v2.15", s.LineOf("v8.1.0"))
	assert.Equal(t, "v2.16", s.LineOf("v9.0.0-rc.1"))
	assert.Equal(t, "", s.LineOf("v7.0.0"), "a series no line has")
	assert.Equal(t, "", semverFor(t, universal()).LineOf("v0.5.3"))
}

func TestSemverLatestOfALine(t *testing.T) {
	s := semverFor(t, lined())
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
	got := semverFor(t, lined()).Latest(tags("v8.1.0", "v9.0.0", "v9.1.0-rc.1"), "")

	require.NotNil(t, got.Stable)
	require.NotNil(t, got.Prerelease)
	assert.Equal(t, "v9.0.0", got.Stable.Name)
	assert.Equal(t, "v9.1.0-rc.1", got.Prerelease.Name)
}

func TestSemverLatestIgnoresTagsThePatternDoesNotMatch(t *testing.T) {
	p := universal()
	p.Release.TagPattern = "v0.*"

	got := semverFor(t, p).Latest(tags("v0.5.3", "v1.0.0"), "")

	require.NotNil(t, got.Stable)
	assert.Equal(t, "v0.5.3", got.Stable.Name)
}

func TestSemverLatestOfALineWithNoTagsOrAnUnknownLineIsEmpty(t *testing.T) {
	s := semverFor(t, lined())
	assert.Equal(t, git.Latest{}, s.Latest(tags("v9.0.0"), "v2.15"))
	assert.Equal(t, git.Latest{}, s.Latest(tags("v9.0.0"), "v2.99"))
}

func TestSemverIsStable(t *testing.T) {
	s := semverFor(t, lined())
	assert.True(t, s.IsStable("v8.1.0"))
	assert.False(t, s.IsStable("v8.1.0-rc.1"))
}

func TestSemverCompare(t *testing.T) {
	s := semverFor(t, lined())
	assert.Positive(t, s.Compare("v8.1.0", "v8.0.9"))
	assert.Negative(t, s.Compare("v8.1.0-rc.1", "v8.1.0"))
	assert.Zero(t, s.Compare("v8.1.0", "v8.1.0"))
	assert.Zero(t, s.Compare("v8.1.0", "main-abc1234"), "a name that is not a release has no order")
}
