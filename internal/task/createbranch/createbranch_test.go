package createbranch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/workdir"
)

var bg = context.Background()

const (
	base   = "base1111"
	branch = "bot/scc-operator-v0.5.3-v2.15"
)

// fakeGit answers what making the branch asks, and records what it was asked to
// change, in order.
type fakeGit struct {
	state      workdir.GitState
	stateErr   error
	head       string // what the target is at on the remote
	fetchErr   error
	existing   string // the commit the branch of that name is at; empty is none
	collisions []string
	mergeBase  string
	commits    []git.BranchCommit
	email      string
	startErr   error
	worktreeEr error

	changes []string
	reads   []string
}

func (f *fakeGit) State(context.Context, string) (workdir.GitState, error) {
	return f.state, f.stateErr
}

func (f *fakeGit) Fetch(_ context.Context, dir, remote, b string) (string, error) {
	f.reads = append(f.reads, fmt.Sprintf("fetch %s %s %s", dir, remote, b))
	return f.head, f.fetchErr
}

func (f *fakeGit) Collisions(_ context.Context, _, commit string, untracked []string) ([]string, error) {
	f.reads = append(f.reads, fmt.Sprintf("collisions %s %s", commit, strings.Join(untracked, ",")))
	return f.collisions, nil
}

func (f *fakeGit) BranchCommit(context.Context, string, string) (string, bool, error) {
	return f.existing, f.existing != "", nil
}

func (f *fakeGit) StartBranch(_ context.Context, dir, b, commit string) error {
	f.changes = append(f.changes, fmt.Sprintf("start %s %s %s", dir, b, commit))
	return f.startErr
}

func (f *fakeGit) AddWorktree(_ context.Context, dir, path, b, commit string) error {
	f.changes = append(f.changes, fmt.Sprintf("worktree %s %s %s %s", dir, path, b, commit))
	return f.worktreeEr
}

func (f *fakeGit) DeleteBranch(_ context.Context, dir, b string) error {
	f.changes = append(f.changes, fmt.Sprintf("delete %s %s", dir, b))
	return nil
}

func (f *fakeGit) MergeBase(context.Context, string, string, string) (string, error) {
	return f.mergeBase, nil
}

func (f *fakeGit) CommitsSince(context.Context, string, string, string) ([]git.BranchCommit, error) {
	return f.commits, nil
}

func (f *fakeGit) Config(context.Context, string, string) (string, error) { return f.email, nil }

func (f *fakeGit) Commit(context.Context, string, workdir.GitCommit) (string, error) { return "", nil }
func (f *fakeGit) CommitFiles(context.Context, string, string) ([]string, error)     { return nil, nil }
func (f *fakeGit) Checkout(context.Context, string, string) error                    { return nil }
func (f *fakeGit) RemoveWorktree(context.Context, string, string) error              { return nil }
func (f *fakeGit) Discard(context.Context, string, []string) error                   { return nil }

// clean is a clone on main with nothing in the way, and the target where the
// caller read it.
func clean() *fakeGit {
	return &fakeGit{state: workdir.GitState{Branch: "main"}, head: base, email: "dev@example.com"}
}

func input() Input {
	return Input{CloneDir: "/src/rancher", Remote: "upstream", Target: "release/v2.15", Base: base, Branch: branch, TempDir: "/tmp/gero"}
}

func ours(hash string) git.BranchCommit {
	return git.BranchCommit{Hash: hash, AuthorEmail: "dev@example.com", Message: "Update scc-operator\n\n" + git.OurTrailer}
}

func TestRunACleanCloneIsWorkedInPlaceFromTheBase(t *testing.T) {
	g := clean()

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	require.NotNil(t, out.Work)
	assert.Equal(t, workdir.InPlace, out.Work.Place)
	assert.Equal(t, "/src/rancher", out.Work.Dir)
	assert.Empty(t, out.Work.Reason)
	assert.Equal(t, []string{"start /src/rancher " + branch + " " + base}, g.changes)
	assert.Contains(t, g.reads, "fetch /src/rancher upstream release/v2.15")
}

func TestRunADirtyCloneGetsAWorktreeNamedForTheBranch(t *testing.T) {
	g := clean()
	g.state.Tracked = []string{"go.mod"}

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	require.NotNil(t, out.Work)
	assert.Equal(t, workdir.Worktree, out.Work.Place)
	assert.Contains(t, out.Work.Reason, "uncommitted")
	assert.Equal(t, "/tmp/gero/bot-scc-operator-v0.5.3-v2.15", out.Work.Dir, "a slash in the branch name is not a directory")
	assert.Equal(t, []string{"worktree /src/rancher /tmp/gero/bot-scc-operator-v0.5.3-v2.15 " + branch + " " + base}, g.changes)
}

func TestRunADryRunNeverMovesTheCheckout(t *testing.T) {
	g := clean()
	in := input()
	in.DryRun = true

	out, err := Run(bg, g, in)

	require.NoError(t, err)
	require.NotNil(t, out.Work)
	assert.Equal(t, workdir.Worktree, out.Work.Place)
	assert.Contains(t, out.Work.Reason, "dry run")
	assert.Len(t, g.changes, 1)
	assert.True(t, strings.HasPrefix(g.changes[0], "worktree "))
}

func TestRunAnUntrackedFileInTheWayIsFoundAgainstTheBaseAndSendsItToAWorktree(t *testing.T) {
	g := clean()
	g.state.Untracked = []string{"gen/x.go", "notes.txt"}
	g.collisions = []string{"gen/x.go"}

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	require.NotNil(t, out.Work)
	assert.Equal(t, workdir.Worktree, out.Work.Place)
	assert.Contains(t, out.Work.Reason, "gen/x.go")
	assert.Contains(t, g.reads, "collisions "+base+" gen/x.go,notes.txt")
}

func TestRunFailuresBeforeAnythingIsWrittenCreateNothing(t *testing.T) {
	boom := errors.New("boom")
	cases := map[string]struct {
		set  func(*fakeGit)
		want error
	}{
		"the state cannot be read":     {func(g *fakeGit) { g.stateErr = boom }, boom},
		"the target cannot be fetched": {func(g *fakeGit) { g.fetchErr = boom }, boom},
		"the target moved":             {func(g *fakeGit) { g.head = "moved999" }, ErrBaseMoved},
		"the branch is checked out":    {func(g *fakeGit) { g.state.Branch = branch }, ErrBranchCheckedOut},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			g := clean()
			c.set(g)

			out, err := Run(bg, g, input())

			require.ErrorIs(t, err, c.want)
			assert.Nil(t, out.Work)
			assert.Empty(t, g.changes)
		})
	}
}

func TestRunABranchThatIsAtTheBaseHoldsNothingAndIsReplaced(t *testing.T) {
	g := clean()
	g.existing = base

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	require.NotNil(t, out.Work)
	assert.False(t, out.Work.Reused)
	assert.Equal(t, []string{"delete /src/rancher " + branch, "start /src/rancher " + branch + " " + base}, g.changes)
}

func TestRunABranchOfOursThatIsCurrentIsReusedAsItIs(t *testing.T) {
	g := clean()
	g.existing = "tip22222"
	g.mergeBase = base
	g.commits = []git.BranchCommit{
		{Hash: "aaaa", AuthorEmail: "dev@example.com", Message: "Update scc-operator\n\nbody\n\n" + git.OurTrailer},
		{Hash: "bbbb", AuthorEmail: "dev@example.com", Message: "Run generate\n\n" + git.OurTrailer},
	}

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	require.NotNil(t, out.Work)
	assert.True(t, out.Work.Reused)
	assert.Equal(t, workdir.Existing, out.Work.Place)
	assert.Equal(t, []workdir.CommitInfo{{Hash: "aaaa", Subject: "Update scc-operator"}, {Hash: "bbbb", Subject: "Run generate"}}, out.Work.Existing)
	assert.Empty(t, g.changes, "nothing is deleted, started or checked out")
}

func TestRunABranchOfOursThatTheTargetHasMovedOnFromIsBuiltAgain(t *testing.T) {
	g := clean()
	g.existing = "tip22222"
	g.mergeBase = "older000"
	g.commits = []git.BranchCommit{ours("aaaa")}

	out, err := Run(bg, g, input())

	require.NoError(t, err)
	require.NotNil(t, out.Work)
	assert.False(t, out.Work.Reused)
	assert.Equal(t, []string{"delete /src/rancher " + branch, "start /src/rancher " + branch + " " + base}, g.changes)
}

func TestRunRebuildBuildsAgainEvenWhenTheBranchIsCurrent(t *testing.T) {
	g := clean()
	g.existing = "tip22222"
	g.mergeBase = base
	g.commits = []git.BranchCommit{ours("aaaa")}
	in := input()
	in.Rebuild = true

	out, err := Run(bg, g, in)

	require.NoError(t, err)
	require.NotNil(t, out.Work)
	assert.False(t, out.Work.Reused)
	assert.Equal(t, []string{"delete /src/rancher " + branch, "start /src/rancher " + branch + " " + base}, g.changes)
}

func TestRunABranchWithACommitThatIsNotOursIsNeverTouched(t *testing.T) {
	g := clean()
	g.existing = "tip22222"
	g.mergeBase = base
	g.commits = []git.BranchCommit{{Hash: "aaaa1111bbbb", AuthorEmail: "someone@example.com", Message: "Mine\n\n" + git.OurTrailer}}
	in := input()
	in.Rebuild = true

	out, err := Run(bg, g, in)

	require.ErrorIs(t, err, ErrBranchExists)
	assert.Contains(t, err.Error(), "someone@example.com", "whose commit it is")
	assert.Nil(t, out.Work)
	assert.Empty(t, g.changes, "not even a rebuild deletes it")
}

func TestRunAFailureToStartTheBranchCreatesNothing(t *testing.T) {
	boom := errors.New("cannot start")
	g := clean()
	g.startErr = boom

	out, err := Run(bg, g, input())

	require.ErrorIs(t, err, boom)
	assert.Nil(t, out.Work)
}

func TestRunAFailureToAddTheWorktreeCreatesNothing(t *testing.T) {
	boom := errors.New("cannot add")
	g := clean()
	g.state.InProgress = "rebase"
	g.worktreeEr = boom

	out, err := Run(bg, g, input())

	require.ErrorIs(t, err, boom)
	assert.Nil(t, out.Work)
}
