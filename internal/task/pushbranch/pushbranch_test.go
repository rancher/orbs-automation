package pushbranch

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/platform/git"
)

var bg = context.Background()

// fakeGit answers what the push asks about the branch and the remote's copy of
// it, and records the pushes.
type fakeGit struct {
	tip        string // the remote's copy; fetchErr says there is none
	fetchErr   error
	local      string // the local branch; empty is not found
	mergeBases map[[2]string]string
	mergeErr   error
	commits    []git.BranchCommit
	commitsErr error
	email      string
	emailErr   error
	pushErr    error
	replaceErr error

	pushed   []string // "remote/branch"
	replaced []string // "remote/branch@expected"
}

func (f *fakeGit) Fetch(context.Context, string, string, string) (string, error) {
	return f.tip, f.fetchErr
}

func (f *fakeGit) BranchCommit(context.Context, string, string) (string, bool, error) {
	return f.local, f.local != "", nil
}

func (f *fakeGit) MergeBase(_ context.Context, _, a, b string) (string, error) {
	if f.mergeErr != nil {
		return "", f.mergeErr
	}
	return f.mergeBases[[2]string{a, b}], nil
}

func (f *fakeGit) CommitsSince(context.Context, string, string, string) ([]git.BranchCommit, error) {
	return f.commits, f.commitsErr
}

func (f *fakeGit) Config(context.Context, string, string) (string, error) {
	return f.email, f.emailErr
}

func (f *fakeGit) Push(_ context.Context, _, remote, branch string) error {
	f.pushed = append(f.pushed, remote+"/"+branch)
	return f.pushErr
}

func (f *fakeGit) PushReplacing(_ context.Context, _, remote, branch, expected string) error {
	f.replaced = append(f.replaced, remote+"/"+branch+"@"+expected)
	return f.replaceErr
}

func input() Input {
	return Input{Dir: "/clone", Remote: "origin", Branch: "bot/x-v1", Base: "base1"}
}

func ours(hash string) git.BranchCommit {
	return git.BranchCommit{Hash: hash, AuthorEmail: "dev@example.com", Message: "Update\n\n" + git.OurTrailer}
}

// diverged is a remote copy that the local branch did not grow from, so only
// replacing it can put the branch there.
func diverged() *fakeGit {
	return &fakeGit{
		tip: "remote1", local: "local1", email: "dev@example.com",
		mergeBases: map[[2]string]string{{"local1", "remote1"}: "base1", {"base1", "remote1"}: "base1"},
		commits:    []git.BranchCommit{ours("remote1")},
	}
}

func TestRunABranchThatIsNotOnTheRemoteIsPushedPlainly(t *testing.T) {
	g := &fakeGit{fetchErr: errors.New("no such ref"), local: "local1"}

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	assert.Equal(t, Output{Remote: "origin", Branch: "bot/x-v1", Action: ActionPushed}, out)
	assert.Equal(t, []string{"origin/bot/x-v1"}, g.pushed)
	assert.Empty(t, g.replaced)
}

func TestRunABranchThatIsNotLocalIsLeftToThePushToReport(t *testing.T) {
	g := &fakeGit{tip: "remote1"}

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	assert.Equal(t, ActionPushed, out.Action)
	assert.Equal(t, []string{"origin/bot/x-v1"}, g.pushed)
}

func TestRunTheSameCommitOnTheRemoteSendsNothing(t *testing.T) {
	g := &fakeGit{tip: "same", local: "same"}

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	assert.Equal(t, Output{Remote: "origin", Branch: "bot/x-v1", Action: ActionUpToDate}, out)
	assert.Empty(t, g.pushed)
	assert.Empty(t, g.replaced)
}

func TestRunABranchThatGrewFromTheRemotesCopyIsAnOrdinaryPush(t *testing.T) {
	g := &fakeGit{tip: "remote1", local: "local2", mergeBases: map[[2]string]string{{"local2", "remote1"}: "remote1"}}

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	assert.Equal(t, ActionPushed, out.Action)
	assert.Equal(t, []string{"origin/bot/x-v1"}, g.pushed)
	assert.Empty(t, g.replaced)
}

func TestRunWithAnyQuestionUnanswerableTheOrdinaryPushIsLeftToSayWhatIsWrong(t *testing.T) {
	boom := errors.New("boom")
	cases := map[string]func(*fakeGit){
		"the merge base":    func(g *fakeGit) { g.mergeErr = boom },
		"the commits":       func(g *fakeGit) { g.commitsErr = boom },
		"the actor's email": func(g *fakeGit) { g.emailErr = boom },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			g := diverged()
			breakIt(g)

			out, err := Run(bg, g, input())

			require.NoError(t, err)
			assert.Equal(t, ActionPushed, out.Action)
			assert.Equal(t, []string{"origin/bot/x-v1"}, g.pushed)
			assert.Empty(t, g.replaced)
		})
	}
}

func TestRunAnEarlierAttemptOfOursIsReplacedWhenAskedAndOnlyOverWhatWasSeen(t *testing.T) {
	g := diverged()
	in := input()
	in.Replace = true

	out, err := Run(bg, g, in)

	require.NoError(t, err)
	assert.Equal(t, Output{Remote: "origin", Branch: "bot/x-v1", Action: ActionReplaced}, out)
	assert.Equal(t, []string{"origin/bot/x-v1@remote1"}, g.replaced, "with a lease on the commit that was seen")
	assert.Empty(t, g.pushed)
}

func TestRunAnEarlierAttemptOfOursIsNotReplacedUnlessAsked(t *testing.T) {
	g := diverged()

	_, err := Run(bg, g, input())

	require.ErrorIs(t, err, ErrNeedsReplace)
	assert.Empty(t, g.pushed)
	assert.Empty(t, g.replaced)
}

func TestRunABranchWithACommitThatIsNotOursIsNeverReplacedAndSaysWhose(t *testing.T) {
	g := diverged()
	g.commits = []git.BranchCommit{{Hash: "remote1abc", AuthorEmail: "someone@example.com", Message: "Update\n\n" + git.OurTrailer}}
	in := input()
	in.Replace = true

	_, err := Run(bg, g, in)

	require.ErrorIs(t, err, ErrNotOurs)
	assert.Contains(t, err.Error(), "someone@example.com")
	assert.Contains(t, err.Error(), "bot/x-v1")
	assert.Empty(t, g.pushed)
	assert.Empty(t, g.replaced)
}

func TestRunAFailedPushIsAnErrorNamingTheBranchAndTheRemote(t *testing.T) {
	boom := errors.New("rejected")
	g := &fakeGit{fetchErr: errors.New("no such ref"), local: "local1", pushErr: boom}

	_, err := Run(bg, g, input())

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "bot/x-v1")
	assert.Contains(t, err.Error(), "origin")
}

func TestRunAFailedReplacementIsAnErrorNamingTheBranchAndTheRemote(t *testing.T) {
	boom := errors.New("lease lost")
	g := diverged()
	g.replaceErr = boom
	in := input()
	in.Replace = true

	_, err := Run(bg, g, in)

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "bot/x-v1")
	assert.Contains(t, err.Error(), "origin")
}
