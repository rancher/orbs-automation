package createtag

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/git/releasetag"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

// shell is the tag of shell's v2.15 to make in the clone at /work: 0.8.x is its
// series, and the commit to tag is head1234.
func shell(tag string, existing ...git.Tag) Input {
	return Input{CloneDir: "/work", Input: releasetag.Input{
		Repo: "rancher/shell", Tag: tag, Commit: "head1234", Tags: existing, Pattern: "v*", Series: "0.8.x",
	}}
}

func tagsAt(commit string, names ...string) []git.Tag {
	out := make([]git.Tag, len(names))
	for i, n := range names {
		out[i] = git.Tag{Name: n, Commit: commit}
	}
	return out
}

type fakeGit struct {
	made []string
	err  error
}

func (f *fakeGit) CreateTag(_ context.Context, dir, name, commit string) error {
	f.made = append(f.made, dir+" "+name+" "+commit)
	return f.err
}

func TestRunMakesTheTagOnTheCommit(t *testing.T) {
	r := &fakeGit{}

	got, err := Run(bg, r, shell("v0.8.4-rc.1", tagsAt("aaa", "v0.8.3")...))

	require.NoError(t, err)
	assert.Equal(t, releasetag.Create, got.Action)
	assert.Equal(t, []string{"/work v0.8.4-rc.1 head1234"}, r.made)
}

func TestRunMakesNothingForATagThatIsAlreadyThere(t *testing.T) {
	r := &fakeGit{}

	got, err := Run(bg, r, shell("v0.8.3", tagsAt("head1234", "v0.8.3")...))

	require.NoError(t, err)
	assert.Equal(t, releasetag.Already, got.Action)
	assert.Empty(t, r.made)
}

func TestRunMakesNothingForATagThatFailsTheCheck(t *testing.T) {
	r := &fakeGit{}

	_, err := Run(bg, r, shell("v0.9.0"))

	require.ErrorIs(t, err, releasetag.ErrNotInSeries)
	assert.Empty(t, r.made)
}

func TestRunReportsAFailureToMakeTheTag(t *testing.T) {
	r := &fakeGit{err: errors.New("fatal: tag already exists")}

	_, err := Run(bg, r, shell("v0.8.4-rc.1"))

	require.Error(t, err)
	assert.ErrorContains(t, err, "already exists")
}
