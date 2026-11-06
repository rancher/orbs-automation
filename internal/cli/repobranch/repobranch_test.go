package repobranch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/task/createbranch"
	"github.com/rancher/orbs-automation/internal/workdir"
	"github.com/rancher/orbs-automation/internal/workspace"
)

var bg = context.Background()

const (
	slug = "rancher/rancher"
	head = "1b3c9e07aaaa"
)

type fakeWorkspace struct {
	project    config.Project
	projectErr error
	path       workspace.RepoPath
	pathErr    error
	upstream   string
	asked      string // the project key it was asked for
}

func (f *fakeWorkspace) Project(key string) (config.Project, error) {
	f.asked = key
	return f.project, f.projectErr
}

func (f *fakeWorkspace) RepoPath(string) (workspace.RepoPath, error) { return f.path, f.pathErr }
func (f *fakeWorkspace) UpstreamFor(string) string                   { return f.upstream }

func newWorkspace() *fakeWorkspace {
	return &fakeWorkspace{
		project: config.Project{
			Name: "rancher",
			Repo: config.Repo{Name: slug, DefaultBranch: "main"},
			Release: config.Release{Branching: config.BranchingPerLine, Lines: []config.ProjectLine{
				{Line: "v2.15", Branch: "release/v2.15"}, {Line: "v2.14", Branch: "release/v2.14"}, {Line: "v2.16"},
			}},
		},
		path:     workspace.RepoPath{Slug: slug, Path: "/src/rancher"},
		upstream: "upstream",
	}
}

// fakeGit is git for the task: a clone on main, and the target at head, unless
// set otherwise. It records every change in order.
type fakeGit struct {
	state    workdir.GitState
	existing string
	mergeB   string
	commits  []git.BranchCommit
	calls    []string
}

func (f *fakeGit) State(context.Context, string) (workdir.GitState, error) { return f.state, nil }

func (f *fakeGit) Fetch(_ context.Context, dir, remote, branch string) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("fetch %s %s %s", dir, remote, branch))
	return head, nil
}

func (f *fakeGit) Collisions(context.Context, string, string, []string) ([]string, error) {
	return nil, nil
}

func (f *fakeGit) BranchCommit(context.Context, string, string) (string, bool, error) {
	return f.existing, f.existing != "", nil
}

func (f *fakeGit) StartBranch(_ context.Context, _, branch, commit string) error {
	f.calls = append(f.calls, "start "+branch+" "+commit)
	return nil
}

func (f *fakeGit) AddWorktree(_ context.Context, _, path, branch, commit string) error {
	f.calls = append(f.calls, "worktree "+path+" "+branch+" "+commit)
	return nil
}

func (f *fakeGit) DeleteBranch(_ context.Context, _, branch string) error {
	f.calls = append(f.calls, "delete "+branch)
	return nil
}

func (f *fakeGit) MergeBase(context.Context, string, string, string) (string, error) {
	return f.mergeB, nil
}

func (f *fakeGit) CommitsSince(context.Context, string, string, string) ([]git.BranchCommit, error) {
	return f.commits, nil
}

func (f *fakeGit) Config(context.Context, string, string) (string, error) {
	return "dev@example.com", nil
}

func (f *fakeGit) Commit(context.Context, string, workdir.GitCommit) (string, error) { return "", nil }

func (f *fakeGit) Checkout(_ context.Context, _, branch string) error {
	f.calls = append(f.calls, "checkout "+branch)
	return nil
}

func (f *fakeGit) RemoveWorktree(_ context.Context, _, path string) error {
	f.calls = append(f.calls, "remove worktree "+path)
	return nil
}

func (f *fakeGit) Discard(context.Context, string, []string) error { return nil }

func cleanGit() *fakeGit { return &fakeGit{state: workdir.GitState{Branch: "main"}} }

func opts() Options { return Options{Line: "v2.15", Name: "my-fix"} }

func do(t *testing.T, ws *fakeWorkspace, g *fakeGit, format string, dry bool, o Options) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(bg, &out, ws, g, "/tmp/gero", format, dry, o)
	return out.String(), err
}

func TestParseArgsTwoAreTheLineAndTheNameAndThreeStartWithTheProject(t *testing.T) {
	project, line, name := ParseArgs([]string{"v2.15", "my-fix"})
	assert.Equal(t, []string{"", "v2.15", "my-fix"}, []string{project, line, name})

	project, line, name = ParseArgs([]string{"scc-operator", "main", "bump-deps"})
	assert.Equal(t, []string{"scc-operator", "main", "bump-deps"}, []string{project, line, name})
}

func TestRunAskedForTheProjectTheUserNamed(t *testing.T) {
	ws := newWorkspace()
	o := opts()
	o.Project = "scc-operator"

	_, err := do(t, ws, cleanGit(), clicfg.OutputText, false, o)

	require.NoError(t, err)
	assert.Equal(t, "scc-operator", ws.asked)
}

func TestRunMakesTheBranchAtTheHeadOfTheLinesUpstreamBranchAndLeavesTheCheckout(t *testing.T) {
	g := cleanGit()

	out, err := do(t, newWorkspace(), g, clicfg.OutputText, false, opts())

	require.NoError(t, err)
	assert.Equal(t, []string{
		"fetch /src/rancher upstream release/v2.15", // the head, to read it
		"fetch /src/rancher upstream release/v2.15", // the task's own check that it did not move
		"start my-fix " + head,
		"checkout main", // back to where the engineer was
	}, g.calls)
	assert.Contains(t, out, "Created my-fix at upstream/release/v2.15 (1b3c9e07) in rancher/rancher")
	assert.Contains(t, out, "git switch my-fix")
}

func TestRunADirtyCloneIsLeftAloneAndTheBranchIsStillMade(t *testing.T) {
	g := cleanGit()
	g.state.Tracked = []string{"go.mod"}

	out, err := do(t, newWorkspace(), g, clicfg.OutputText, false, opts())

	require.NoError(t, err)
	joined := strings.Join(g.calls, "\n")
	assert.Contains(t, joined, "worktree /tmp/gero/my-fix my-fix "+head)
	assert.Contains(t, joined, "remove worktree /tmp/gero/my-fix")
	assert.NotContains(t, joined, "delete my-fix", "the branch is kept")
	assert.NotContains(t, joined, "checkout")
	assert.Contains(t, out, "Created my-fix")
}

func TestRunSwitchLeavesTheCheckoutOnTheNewBranch(t *testing.T) {
	g := cleanGit()
	o := opts()
	o.Switch = true

	out, err := do(t, newWorkspace(), g, clicfg.OutputText, false, o)

	require.NoError(t, err)
	require.NotEmpty(t, g.calls)
	assert.Equal(t, "checkout my-fix", g.calls[len(g.calls)-1])
	assert.Contains(t, out, "and switched to it")
	assert.NotContains(t, out, "git switch", "it already did")
}

func TestRunADryRunMakesNothingThatStays(t *testing.T) {
	g := cleanGit()

	out, err := do(t, newWorkspace(), g, clicfg.OutputText, true, opts())

	require.NoError(t, err)
	joined := strings.Join(g.calls, "\n")
	assert.Contains(t, joined, "delete my-fix", "the branch made to find out is removed")
	assert.NotContains(t, joined, "checkout my-fix")
	assert.Contains(t, out, "Would create my-fix at upstream/release/v2.15")
	assert.NotContains(t, out, "git switch")
}

func TestRunAnEarlierBranchOfOursOnTheCurrentHeadIsKept(t *testing.T) {
	g := cleanGit()
	g.existing = "tip22222"
	g.mergeB = head
	g.commits = []git.BranchCommit{{Hash: "aaaa", AuthorEmail: "dev@example.com", Message: "Update\n\n" + git.OurTrailer}}

	out, err := do(t, newWorkspace(), g, clicfg.OutputText, false, opts())

	require.NoError(t, err)
	assert.Contains(t, out, "my-fix already exists")
	assert.Contains(t, out, "1 commit of ours")
	for _, c := range g.calls {
		assert.False(t, strings.HasPrefix(c, "start") || strings.HasPrefix(c, "delete"), c)
	}
}

func TestRunABranchWithCommitsThatAreNotOursIsRefusedAndNothingIsDeleted(t *testing.T) {
	g := cleanGit()
	g.existing = "tip22222"
	g.mergeB = head
	g.commits = []git.BranchCommit{{Hash: "aaaa1111bbbb", AuthorEmail: "someone@example.com", Message: "Mine"}}

	_, err := do(t, newWorkspace(), g, clicfg.OutputText, false, opts())

	require.ErrorIs(t, err, createbranch.ErrBranchExists)
	for _, c := range g.calls {
		assert.False(t, strings.HasPrefix(c, "delete"), c)
	}
}

func TestRunTheCheckedOutBranchSaysToChooseAnotherName(t *testing.T) {
	g := cleanGit()
	g.state.Branch = "my-fix"

	_, err := do(t, newWorkspace(), g, clicfg.OutputText, false, opts())

	require.ErrorIs(t, err, createbranch.ErrBranchCheckedOut)
	assert.Contains(t, err.Error(), "choose another name")
}

func TestRunALineIsFoundByItsNameOrAsABranchOfTheProject(t *testing.T) {
	cases := map[string]string{
		"v2.14":         "release/v2.14",
		"release/v2.15": "release/v2.15",
		"main":          "main",
		"v2.16":         "main", // a line with no branch of its own is on the default branch
		"V2.15":         "release/v2.15",
	}
	for line, branch := range cases {
		t.Run(line, func(t *testing.T) {
			g := cleanGit()
			o := opts()
			o.Line = line

			_, err := do(t, newWorkspace(), g, clicfg.OutputText, false, o)

			require.NoError(t, err)
			require.NotEmpty(t, g.calls)
			assert.Equal(t, "fetch /src/rancher upstream "+branch, g.calls[0])
		})
	}
}

func TestRunAnUnknownLineNamesTheOnesTheProjectHasAndTouchesNothing(t *testing.T) {
	g := cleanGit()
	o := opts()
	o.Line = "v9.9"

	_, err := do(t, newWorkspace(), g, clicfg.OutputText, false, o)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "v9.9")
	assert.Contains(t, err.Error(), "v2.15")
	assert.Contains(t, err.Error(), "v2.14")
	assert.Empty(t, g.calls)
}

func TestRunWithNoUpstreamRemoteSaysWhatToSet(t *testing.T) {
	ws := newWorkspace()
	ws.upstream = ""

	_, err := do(t, ws, cleanGit(), clicfg.OutputText, false, opts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream")
	assert.Contains(t, err.Error(), slug)
}

func TestRunWithNoCloneSaysHowToGetOne(t *testing.T) {
	ws := newWorkspace()
	ws.pathErr = errors.New("not found")

	_, err := do(t, ws, cleanGit(), clicfg.OutputText, false, opts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repos.paths."+slug)
}

func TestRunAProjectThatIsNotFoundIsItsError(t *testing.T) {
	ws := newWorkspace()
	ws.projectErr = errors.New("no such project")

	_, err := do(t, ws, cleanGit(), clicfg.OutputText, false, opts())

	require.ErrorContains(t, err, "no such project")
}

func TestRunJSONIsTheResult(t *testing.T) {
	out, err := do(t, newWorkspace(), cleanGit(), clicfg.OutputJSON, false, opts())

	require.NoError(t, err)
	var got Result
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, Result{Project: slug, Branch: "my-fix", From: "upstream/release/v2.15", Base: head}, got)
}

func TestRunAFormatThatIsNeitherTextNorJSONIsAnError(t *testing.T) {
	g := cleanGit()

	_, err := do(t, newWorkspace(), g, clicfg.OutputEnv, false, opts())

	require.ErrorContains(t, err, "text or json")
	assert.Empty(t, g.calls)
}
