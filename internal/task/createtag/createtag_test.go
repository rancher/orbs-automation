package createtag

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

func tagsAt(commit string, names ...string) []git.Tag {
	out := make([]git.Tag, len(names))
	for i, n := range names {
		out[i] = git.Tag{Name: n, Commit: commit}
	}
	return out
}

// shell is the producer of the examples: 0.8.x is the series of v2.15.
func shell(tag string, existing ...git.Tag) Input {
	return Input{Repo: "rancher/shell", Tag: tag, Commit: "head1234", Tags: existing, Pattern: "v*", Series: "0.8.x"}
}

type fakeRemote struct {
	made []string
	err  error
}

func (f *fakeRemote) Create(_ context.Context, slug, name, commit string) error {
	f.made = append(f.made, slug+" "+name+" "+commit)
	return f.err
}

func TestCheckATagTheLineCanHaveIsCreatedAndSaysWhatItFollows(t *testing.T) {
	got, err := Check(shell("v0.8.4-rc.1", tagsAt("aaa", "v0.8.2", "v0.8.3", "v0.9.0")...))

	require.NoError(t, err)
	assert.Equal(t, Create, got.Action)
	require.NotNil(t, got.Newest)
	assert.Equal(t, "v0.8.3", got.Newest.Name, "the newest of the series, not of the repository")
}

func TestCheckTheFirstTagOfALineHasNothingToFollow(t *testing.T) {
	got, err := Check(shell("v0.8.0-rc.1", tagsAt("aaa", "v0.9.0")...))

	require.NoError(t, err)
	assert.Equal(t, Create, got.Action)
	assert.Nil(t, got.Newest)
}

func TestCheckATagThatIsAlreadyOnTheCommitIsAlreadyDone(t *testing.T) {
	got, err := Check(shell("v0.8.3", tagsAt("head1234", "v0.8.3")...))

	require.NoError(t, err)
	assert.Equal(t, Already, got.Action, "a rerun is safe, though v0.8.3 is not newer than itself")
}

func TestCheckATagThatIsOnAnotherCommitIsNeverMoved(t *testing.T) {
	_, err := Check(shell("v0.8.3", tagsAt("other999", "v0.8.3")...))

	require.ErrorIs(t, err, ErrTagExists)
	assert.ErrorContains(t, err, "v0.8.3")
	assert.ErrorContains(t, err, "other999")
}

func TestCheckRefusesWhatTheLineCannotHave(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		want error
	}{
		{"not a version", shell("v0.8.x"), ErrNotAVersion},
		{"not a version at all", shell("vlatest"), ErrNotAVersion},
		{"does not match the pattern", shell("0.8.4"), ErrPattern},
		{"another series", shell("v0.9.1", tagsAt("aaa", "v0.8.3")...), ErrNotInSeries},
		{"older than the newest", shell("v0.8.2", tagsAt("aaa", "v0.8.3")...), ErrNotNewer},
		{"a prerelease of a released version", shell("v0.8.3-rc.2", tagsAt("aaa", "v0.8.3")...), ErrNotNewer},
		{"the same version, differently written", shell("v0.8.3+build", tagsAt("aaa", "v0.8.3")...), ErrNotNewer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Check(tt.in)

			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestCheckAProjectWithNoSeriesIsNewerThanAnyTagOfItsPattern(t *testing.T) {
	in := Input{Repo: "rancher/scc-operator", Tag: "v0.5.4-rc.1", Commit: "h", Pattern: "v*",
		Tags: tagsAt("a", "v0.5.3", "main-abc1234", "v0.0.0-pr.112.b3")}

	got, err := Check(in)

	require.NoError(t, err)
	require.NotNil(t, got.Newest)
	assert.Equal(t, "v0.5.3", got.Newest.Name, "tags that are not releases of the pattern do not count")

	in.Tag = "v0.5.2"
	_, err = Check(in)
	require.ErrorIs(t, err, ErrNotNewer)
}

func TestCheckARcAfterTheRcItFollowsIsNewer(t *testing.T) {
	got, err := Check(shell("v0.8.4-rc.10", tagsAt("a", "v0.8.3", "v0.8.4-rc.9")...))

	require.NoError(t, err)
	require.NotNil(t, got.Newest)
	assert.Equal(t, "v0.8.4-rc.9", got.Newest.Name, "rc.10 follows rc.9 by number")
}

func TestRunMakesTheTagOnTheCommit(t *testing.T) {
	r := &fakeRemote{}

	got, err := Run(bg, r, shell("v0.8.4-rc.1", tagsAt("aaa", "v0.8.3")...))

	require.NoError(t, err)
	assert.Equal(t, Create, got.Action)
	assert.Equal(t, []string{"rancher/shell v0.8.4-rc.1 head1234"}, r.made)
}

func TestRunMakesNothingForATagThatIsAlreadyThere(t *testing.T) {
	r := &fakeRemote{}

	got, err := Run(bg, r, shell("v0.8.3", tagsAt("head1234", "v0.8.3")...))

	require.NoError(t, err)
	assert.Equal(t, Already, got.Action)
	assert.Empty(t, r.made)
}

func TestRunMakesNothingForATagThatFailsTheCheck(t *testing.T) {
	r := &fakeRemote{}

	_, err := Run(bg, r, shell("v0.9.0"))

	require.ErrorIs(t, err, ErrNotInSeries)
	assert.Empty(t, r.made)
}

func TestRunReportsAFailureToMakeTheTag(t *testing.T) {
	r := &fakeRemote{err: errors.New("403 Resource not accessible")}

	_, err := Run(bg, r, shell("v0.8.4-rc.1"))

	require.Error(t, err)
	assert.ErrorContains(t, err, "403")
}
