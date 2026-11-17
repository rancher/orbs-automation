package ship

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/steprun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	heldV9 = "prodVersion: v9 # keep this\notherVersion: r4\nother: 1\n"
	heldV8 = "prodVersion: v8 # keep this\notherVersion: r3\nother: 1\n"
)

// seriesRepo is a holder on the host: main (v2.16) holds the series of its line, and
// release/v7.x (v2.15) holds the series of another, and each has the script that the
// Edit runs after the write.
func seriesRepo(t *testing.T) string {
	t.Helper()
	dir := remote(t)
	runGit(t, dir, "checkout", "--quiet", "main")
	put(t, dir, "build.yaml", "prodVersion: v9 # keep this\notherVersion: r4\nother: 1\n")
	put(t, dir, "scripts/gen.sh", "#!/bin/sh\nmkdir -p generated && printf %s \"$GERO_TARGET_BRANCH\" > generated/branch.txt\n")
	require.NoError(t, os.Chmod(filepath.Join(dir, "scripts", "gen.sh"), 0o755))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "series and script")
	runGit(t, dir, "branch", "release/v7.x")
	return dir
}

// seriesWS is the holder served from a real repository, with the engineer's clone.
type seriesWS struct {
	t     *testing.T
	graph *chain.Graph
	up    string
	clone string
	push  PushSettings
}

// seriesDocs is the holder and its two streams, with the Edit following the holdings
// named (every one by default).
func seriesDocs(edit bool, followed ...string) []string {
	docs := []string{
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "prod", "", "  repo: {name: acme/prod, managedBy: orbs}\n"),
		chaintest.Doc("Stream", "prod", "{project: prod}", "  lines:\n    - {line: v2.16, series: \"9.x\"}\n    - {line: v2.15, series: \"8.x\"}\n"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: release/v7.x}]\n"),
		chaintest.Doc("Holding", "cons-holds-prod", "{project: cons, stream: prod}", "  at: {file: build.yaml, key: prodVersion}\n  holds: {kind: series, form: \"v{major}\"}\n"),
		chaintest.Doc("Project", "side", "", "  repo: {name: acme/side, managedBy: orbs}\n"),
		chaintest.Doc("Stream", "side", "{project: side}", "  lines:\n    - {line: v2.16, series: \"4.x\"}\n    - {line: v2.15, series: \"3.x\"}\n"),
		chaintest.Doc("Holding", "cons-holds-side", "{project: cons, stream: side}", "  at: {file: build.yaml, key: otherVersion}\n  holds: {kind: series, form: \"r{major}\"}\n"),
	}
	if edit {
		if len(followed) == 0 {
			followed = []string{"cons-holds-prod", "cons-holds-side"}
		}
		docs = append(docs, chaintest.Doc("Edit", "regen", "{holding: ["+strings.Join(followed, ", ")+"]}", "  postEdit:\n    - {run: ./scripts/gen.sh}\n"))
	}
	return docs
}

func newSeriesWS(t *testing.T, edit bool) (*seriesWS, *Executor) {
	t.Helper()
	up := seriesRepo(t)
	ws := &seriesWS{t: t, graph: chaintest.Load(t, seriesDocs(edit)...), up: up, clone: engineer(t, up)}
	return ws, &Executor{Git: GitCLI{}, Runner: steprun.Runner{}, TempDir: t.TempDir()}
}

func (w *seriesWS) Project(string) (*chain.Project, error) { return w.graph.Project("cons"), nil }
func (w *seriesWS) Bump() (*bump.Engine, error)            { return bump.NewEngine(bump.Builtin(), w.graph) }
func (w *seriesWS) CloneDir(string) (string, error)        { return w.clone, nil }
func (w *seriesWS) Remote(string) (string, error)          { return "upstream", nil }
func (w *seriesWS) PushSettings(string) PushSettings       { return w.push }

func (w *seriesWS) UpstreamRefs(context.Context, string) (git.Refs, error) {
	var refs git.Refs
	for _, line := range strings.Split(runGit(w.t, w.up, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads"), "\n") {
		name, commit, _ := strings.Cut(line, " ")
		refs.Branches = append(refs.Branches, git.Branch{Name: name, Commit: commit})
	}
	return refs, nil
}

func (w *seriesWS) UpstreamFile(_ context.Context, _, ref, path string) ([]byte, error) {
	out, err := exec.Command("git", "-C", w.up, "show", ref+":"+path).Output()
	if err != nil {
		return nil, errors.New("file not found")
	}
	return out, nil
}

// branchOf is the branch of the plan, which the test requires.
func branchOf(t *testing.T, p SyncPlan, branch string) SyncBranch {
	t.Helper()
	for _, b := range p.Branches {
		if b.Branch == branch {
			return b
		}
	}
	require.Failf(t, "no branch", "%s in %+v", branch, p.Branches)
	return SyncBranch{}
}

const behind = "release/v7.x"

func TestSyncWithoutAnExecutorOnlyChecks(t *testing.T) {
	ws, _ := newSeriesWS(t, true)

	plan, err := Sync(bg, ws, SyncInput{NoPush: true}, nil)

	require.NoError(t, err)
	assert.Equal(t, "acme/cons", plan.Holder)
	require.Len(t, plan.Branches, 2, "a branch of the holder for each, and not a line for each holding")
	ok, b := branchOf(t, plan, "main"), branchOf(t, plan, behind)
	assert.Equal(t, StatusUpToDate, ok.Status)
	assert.Nil(t, ok.Preview)
	assert.Equal(t, StatusReady, b.Status)
	require.Len(t, b.Rows, 2, "both holdings of the branch are in the one PR")
	assert.Equal(t, "cons-holds-prod", b.Rows[0].Holding)
	assert.Equal(t, "v9", b.Rows[0].Current)
	assert.Equal(t, "v8", b.Rows[0].Want)
	assert.Equal(t, "r3", b.Rows[1].Want)
	require.NotNil(t, b.Preview)
	assert.True(t, strings.HasPrefix(b.Preview.Branch, "bot/sync-"), b.Preview.Branch)
	assert.True(t, strings.HasSuffix(b.Preview.Branch, "-v7.x"), b.Preview.Branch)
	assert.Equal(t, "[v7.x] Sync cons-holds-prod, cons-holds-side on v2.15", b.Preview.Title)
	assert.Contains(t, b.Preview.Body, "Updated `prodVersion` in `build.yaml` from `v9` to `v8` (cons-holds-prod: the series `8.x` of `prod` for v2.15)")
	assert.Contains(t, b.Preview.Body, "Updated `otherVersion` in `build.yaml` from `r4` to `r3`")
	assert.Contains(t, b.Preview.Body, "Ran `./scripts/gen.sh`")
	assert.Contains(t, b.Preview.Body, "gero:ship")
	assert.Nil(t, b.Result, "nothing was written")
	assert.False(t, branchExistsIn(t, ws.clone, b.Preview.Branch))
}

func TestSyncDoesNotWriteAHoldingNoEditFollows(t *testing.T) {
	ws, ex := newSeriesWS(t, false)

	plan, err := Sync(bg, ws, SyncInput{NoPush: true}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, behind)
	assert.Equal(t, StatusBlocked, b.Status)
	assert.Contains(t, b.Rows[0].Reason, "no Edit follows")
	assert.Nil(t, b.Preview)
	assert.Nil(t, b.Result)
	assert.Equal(t, "main", current(t, ws.clone))
	assert.Empty(t, runGit(t, ws.clone, "branch", "--list", "bot/*"), "nothing is written until an Edit says what a write takes")
}

func TestSyncWritesTheHoldingsThatHaveAnEditAndLeavesTheRest(t *testing.T) {
	ws, ex := newSeriesWS(t, true)
	// the Edit follows only the first holding: the second is only read
	ws.graph = chaintest.Load(t, seriesDocs(true, "cons-holds-prod")...)

	plan, err := Sync(bg, ws, SyncInput{NoPush: true}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, behind)
	assert.Equal(t, StatusReady, b.Status)
	assert.Equal(t, StatusReady, b.Rows[0].Status)
	assert.Equal(t, StatusBlocked, b.Rows[1].Status)
	assert.Contains(t, b.Rows[1].Reason, "no Edit follows")
	require.NotNil(t, b.Result)
	assert.Equal(t, "prodVersion: v8 # keep this\notherVersion: r4\nother: 1\n", runGit(t, ws.clone, "show", b.Preview.Branch+":build.yaml")+"\n")
}

func TestSyncCommitsEveryCorrectionAndTheEditsStepOnOneBranch(t *testing.T) {
	ws, ex := newSeriesWS(t, true)

	plan, err := Sync(bg, ws, SyncInput{NoPush: true}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, behind)
	res := b.Result
	require.NotNil(t, res)
	assert.Equal(t, ResultCommitted, res.Status)
	assert.Equal(t, b.Preview.Branch, res.Branch)
	assert.Equal(t, heldV8, runGit(t, ws.clone, "show", res.Branch+":build.yaml")+"\n", "both values are written")
	assert.Equal(t, "release/v7.x", runGit(t, ws.clone, "show", res.Branch+":generated/branch.txt"), "the Edit's step ran once, on the branch")
	assert.Len(t, res.Commits, 2, "the write of both, then the one step that changed files")
	assert.Contains(t, runGit(t, ws.clone, "log", "-2", "--format=%B", res.Branch), "Automation: gero ship")
	assert.Equal(t, "main", current(t, ws.clone))
	assert.Equal(t, heldV9, slurp(t, filepath.Join(ws.clone, "build.yaml")), "the engineer's checkout is untouched")
	assert.Nil(t, branchOf(t, plan, "main").Result, "an up to date branch has no work")
}

func TestSyncDryRunLeavesNothingBehind(t *testing.T) {
	ws, ex := newSeriesWS(t, true)

	plan, err := Sync(bg, ws, SyncInput{NoPush: true, DryRun: true}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, behind)
	require.NotNil(t, b.Result)
	assert.Equal(t, ResultWouldCommit, b.Result.Status)
	assert.Equal(t, Worktree, b.Result.Place)
	assert.False(t, branchExistsIn(t, ws.clone, b.Preview.Branch))
	entries, err := os.ReadDir(ex.TempDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestSyncLimitsTheRunToTheLinesAndTheHoldingAsked(t *testing.T) {
	ws, _ := newSeriesWS(t, true)

	plan, err := Sync(bg, ws, SyncInput{NoPush: true, Lines: []string{"2.16"}}, nil)
	require.NoError(t, err)
	require.Len(t, plan.Branches, 1)
	assert.Equal(t, "main", plan.Branches[0].Branch)

	plan, err = Sync(bg, ws, SyncInput{NoPush: true, Holding: "cons-holds-side"}, nil)
	require.NoError(t, err)
	b := branchOf(t, plan, behind)
	require.Len(t, b.Rows, 1)
	assert.Equal(t, "[v7.x] Sync cons-holds-side on v2.15", b.Preview.Title)

	_, err = Sync(bg, ws, SyncInput{NoPush: true, Holding: "nope"}, nil)
	require.ErrorContains(t, err, "nope")
}

// syncPublishing is a run that pushes to the clone's upstream and opens the PR, with
// the fake GitHub.
func syncPublishing(t *testing.T) (*seriesWS, *Executor, *fakeRemote, *log) {
	t.Helper()
	ws, ex := newSeriesWS(t, true)
	lg := &log{}
	remote := &fakeRemote{Log: lg, Labels: []string{"status/auto-created"}}
	ws.push = PushSettings{Upstream: "upstream"}
	ex.Publish = &Publisher{Git: GitCLI{}, Remote: remote}
	return ws, ex, remote, lg
}

func TestSyncPushesAndOpensOnePRForTheBranchWithAMarker(t *testing.T) {
	ws, ex, _, lg := syncPublishing(t)

	plan, err := Sync(bg, ws, SyncInput{}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, behind)
	res := b.Result
	require.NotNil(t, res)
	assert.Equal(t, ResultOpened, res.Status)
	require.NotNil(t, res.PR)
	assert.Equal(t, 104, res.PR.Number)
	assert.Equal(t, "upstream", res.Pushed)
	assert.Equal(t, res.Tip(), runGit(t, ws.up, "rev-parse", res.Branch), "the branch is on the remote")
	assert.Equal(t, 1, lg.Count("create "), "one PR for the two holdings")
	assert.Less(t, lg.Index("open acme/cons release/v7.x"), lg.Index("create "), "the open PRs are looked at first")
	assert.True(t, lg.Has(`create acme/cons head=`+res.Branch+` base=release/v7.x title="[v7.x] Sync cons-holds-prod, cons-holds-side on v2.15"`))
}

func TestSyncAnAlreadyOpenPRMakesNothing(t *testing.T) {
	ws, ex, remote, lg := syncPublishing(t)
	checked, err := Sync(bg, ws, SyncInput{NoPush: true}, nil)
	require.NoError(t, err)
	fp := branchOf(t, checked, behind).fingerprint
	remote.Open = pulls.Open{PRs: []pulls.PR{pr(77, "danpock", Marker{Source: "sync", Consumer: "acme/cons", Target: behind, Version: fp})}}

	plan, err := Sync(bg, ws, SyncInput{}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, behind)
	require.NotNil(t, b.Result)
	assert.Equal(t, ResultAlreadyOpen, b.Result.Status)
	assert.Equal(t, 77, b.Result.PR.Number)
	assert.False(t, lg.Has("create"))
	assert.False(t, branchExistsIn(t, ws.clone, b.Preview.Branch), "no branch was made")
}

func TestSyncClosesMyStalePRForTheBranchWhenWhatItWritesChanged(t *testing.T) {
	ws, ex, remote, lg := syncPublishing(t)
	remote.Open = pulls.Open{PRs: []pulls.PR{pr(61, "danpock", Marker{Source: "sync", Consumer: "acme/cons", Target: behind, Version: "0ld0ld00"})}}

	plan, err := Sync(bg, ws, SyncInput{}, ex)

	require.NoError(t, err)
	res := branchOf(t, plan, behind).Result
	require.NotNil(t, res)
	assert.Equal(t, ResultOpened, res.Status)
	require.Len(t, res.Superseded, 1)
	assert.Equal(t, 61, res.Superseded[0].Number)
	assert.True(t, lg.Has("close #61"), "an earlier sync of the branch is replaced by this one")
}

func TestSyncLeavesLinesThatWantDifferentValuesOfOnePlaceOnASingleBranch(t *testing.T) {
	ws, _ := newSeriesWS(t, true)
	docs := seriesDocs(true, "cons-holds-prod", "cons-holds-side")
	docs[3] = chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n")
	ws.graph = chaintest.Load(t, docs...)

	plan, err := Sync(bg, ws, SyncInput{NoPush: true}, nil)

	require.NoError(t, err)
	require.Len(t, plan.Branches, 1, "one branch holds every line")
	b := plan.Branches[0]
	assert.Equal(t, "main", b.Branch)
	var reasons []string
	for _, r := range b.Rows {
		if r.Reason != "" {
			reasons = append(reasons, r.Reason)
		}
	}
	require.NotEmpty(t, reasons)
	assert.Contains(t, reasons[0], "want different values of one place")
}
