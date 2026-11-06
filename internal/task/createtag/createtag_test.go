package createtag

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/semver"
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

func at(commit string, names ...string) []git.Tag { return tagsAt(commit, names...) }

// line is the Choice of shell's v2.15: the series 0.8.x, the head at head1234.
func line(tags ...git.Tag) Choice {
	return Choice{Tags: tags, Pattern: "v*", Series: "0.8.x", Commit: "head1234"}
}

func TestChooseAfterAStableTagTheDefaultIsThePrereleaseOfThenextPatch(t *testing.T) {
	got, err := Choose(line(at("old", "v0.8.2", "v0.8.3")...))

	require.NoError(t, err)
	assert.Equal(t, "v0.8.4-rc.1", got.Tag)
	assert.Nil(t, got.Promotes)
}

func TestChooseAfterAPrereleaseTheDefaultIsItsNextIteration(t *testing.T) {
	got, err := Choose(line(at("old", "v0.8.3", "v0.8.4-rc.1", "v0.8.4-rc.2")...))

	require.NoError(t, err)
	assert.Equal(t, "v0.8.4-rc.3", got.Tag, "the bump does not apply to a prerelease that is under way")
}

func TestChooseKeepsTheStyleOfAPrereleaseThatHasNoDot(t *testing.T) {
	got, err := Choose(line(at("old", "v0.8.3", "v0.8.4-rc2")...))

	require.NoError(t, err)
	assert.Equal(t, "v0.8.4-rc3", got.Tag, "rc.3 would sort below rc2")
}

func TestChooseAfterAStableTagTheBumpRaisesTheVersion(t *testing.T) {
	c := line(at("old", "v0.8.3")...)
	c.Series = ""
	c.Pattern = "v*"
	for bump, want := range map[semver.Bump]string{semver.Patch: "v0.8.4-rc.1", semver.Minor: "v0.9.0-rc.1", semver.Major: "v1.0.0-rc.1"} {
		c.Bump = bump

		got, err := Choose(c)

		require.NoError(t, err)
		assert.Equal(t, want, got.Tag)
	}
}

func TestChooseStablePromotesThePrereleaseOnTheHeadWithNoCommitsSince(t *testing.T) {
	c := line(at("old", "v0.8.3")...)
	c.Tags = append(c.Tags, git.Tag{Name: "v0.8.4-rc.2", Commit: "head1234"})
	c.Stable = true

	got, err := Choose(c)

	require.NoError(t, err)
	assert.Equal(t, "v0.8.4", got.Tag)
	require.NotNil(t, got.Promotes)
	assert.Equal(t, "v0.8.4-rc.2", got.Promotes.Name)
}

func TestChooseStablePromotesWhateverTheBumpIs(t *testing.T) {
	c := line(git.Tag{Name: "v0.8.4-rc.2", Commit: "head1234"})
	c.Stable, c.Bump = true, semver.Major

	got, err := Choose(c)

	require.NoError(t, err)
	assert.Equal(t, "v0.8.4", got.Tag)
}

func TestChooseStableRefusesAPromotionWhenTheBranchHasMovedOnFromThePrerelease(t *testing.T) {
	c := line(git.Tag{Name: "v0.8.4-rc.2", Commit: "rccommit99"})
	c.Stable = true

	_, err := Choose(c)

	require.ErrorIs(t, err, ErrMovedSincePrerelease)
	assert.ErrorContains(t, err, "v0.8.4-rc.2")
	assert.ErrorContains(t, err, "rccommit")
	assert.ErrorContains(t, err, "head1234")
}

func TestChooseStableAfterAStableTagIsTheNextStableUnlessAPrereleaseIsRequired(t *testing.T) {
	c := line(at("old", "v0.8.3")...)
	c.Stable = true

	got, err := Choose(c)
	require.NoError(t, err)
	assert.Equal(t, "v0.8.4", got.Tag)
	assert.Nil(t, got.Promotes)

	c.RequiresPrerelease = true
	_, err = Choose(c)
	require.ErrorIs(t, err, ErrNeedsPrerelease)
}

func TestChooseWillNotTagACommitThatAlreadyHasTheNewestTag(t *testing.T) {
	c := line(at("head1234", "v0.8.3")...)

	_, err := Choose(c)
	require.ErrorIs(t, err, ErrNothingNew)

	c = line(git.Tag{Name: "v0.8.4-rc.1", Commit: "head1234"})
	_, err = Choose(c)
	require.ErrorIs(t, err, ErrNothingNew, "another rc of the same commit says nothing new")
}

func TestChooseTheFirstTagOfALineStartsFromItsSeries(t *testing.T) {
	for series, want := range map[string]string{"0.8.x": "v0.8.0-rc.1", "9.x": "v9.0.0-rc.1", "v2.15": "v2.15.0-rc.1"} {
		c := line()
		c.Series = series

		got, err := Choose(c)

		require.NoError(t, err, series)
		assert.Equal(t, want, got.Tag, series)
	}
}

func TestChooseTheFirstStableOfALineIsRefusedWhenAPrereleaseIsRequired(t *testing.T) {
	c := line()
	c.Stable = true
	got, err := Choose(c)
	require.NoError(t, err)
	assert.Equal(t, "v0.8.0", got.Tag)

	c.RequiresPrerelease = true
	_, err = Choose(c)
	require.ErrorIs(t, err, ErrNeedsPrerelease)
}

func TestChooseWithNoTagAndNoSeriesHasNothingToStartFrom(t *testing.T) {
	c := line()
	c.Series = ""

	_, err := Choose(c)

	require.ErrorIs(t, err, ErrNoBase)
}

func TestChooseWritesTheVPrefixOnlyWhenTheTagsHaveIt(t *testing.T) {
	c := Choice{Tags: at("old", "0.8.3"), Pattern: "*", Series: "0.8.x", Commit: "head1234"}

	got, err := Choose(c)

	require.NoError(t, err)
	assert.Equal(t, "0.8.4-rc.1", got.Tag)

	c = Choice{Pattern: "v*", Series: "0.8.x", Commit: "head1234"}
	got, err = Choose(c)
	require.NoError(t, err)
	assert.Equal(t, "v0.8.0-rc.1", got.Tag, "with none to copy, the pattern says")
}

func TestChooseIgnoresTagsOfOtherSeriesAndTagsThatAreNotReleases(t *testing.T) {
	c := line(at("old", "v0.8.3", "v0.9.5", "main-abc1234", "v0.0.0-pr.112.b3")...)

	got, err := Choose(c)

	require.NoError(t, err)
	assert.Equal(t, "v0.8.4-rc.1", got.Tag)
}

func TestCheckAStableTagOfAProjectThatRequiresAPrereleaseNeedsOneOnTheCommit(t *testing.T) {
	in := shell("v0.8.4", git.Tag{Name: "v0.8.3", Commit: "old"}, git.Tag{Name: "v0.8.4-rc.2", Commit: "head1234"})
	in.RequiresPrerelease = true

	got, err := Check(in)
	require.NoError(t, err)
	assert.Equal(t, Create, got.Action)

	in.Tags = at("old", "v0.8.3", "v0.8.4-rc.2")
	_, err = Check(in)
	require.ErrorIs(t, err, ErrNeedsPrerelease, "the prerelease is on another commit")

	in.Tags = at("head1234", "v0.8.3-rc.1")
	_, err = Check(in)
	require.ErrorIs(t, err, ErrNeedsPrerelease, "and one of another version does not count")

	in.RequiresPrerelease = false
	in.Tags = at("old", "v0.8.3")
	_, err = Check(in)
	require.NoError(t, err, "a project that does not require one is not asked")
}

func TestCheckAPrereleaseIsNeverAskedForAPrerelease(t *testing.T) {
	in := shell("v0.8.4-rc.1", at("old", "v0.8.3")...)
	in.RequiresPrerelease = true

	_, err := Check(in)

	require.NoError(t, err)
}
