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
	heldV9   = "prodVersion: v9 # keep this\nother: 1\n"
	heldV8   = "prodVersion: v8 # keep this\nother: 1\n"
	seriesBr = "bot/cons-holds-prod-v8-v7.x"
)

// seriesRepo is a holder on the host: main (v2.16) holds the series of its line, and
// release/v7.x (v2.15) holds the series of another, and each has the script that the
// Edit runs after the write.
func seriesRepo(t *testing.T) string {
	t.Helper()
	dir := remote(t)
	runGit(t, dir, "checkout", "--quiet", "main")
	put(t, dir, "build.yaml", heldV9)
	put(t, dir, "scripts/gen.sh", "#!/bin/sh\nmkdir -p generated && printf %s \"$GERO_TARGET_BRANCH\" > generated/branch.txt\n")
	require.NoError(t, os.Chmod(filepath.Join(dir, "scripts", "gen.sh"), 0o755))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "series and script")
	runGit(t, dir, "branch", "release/v7.x")
	return dir
}

// seriesWS is the holder served from a real repository, with the engineer's clone.
type seriesWS struct {
	t      *testing.T
	graph  *chain.Graph
	up     string
	clone  string
	push   PushSettings
	closed bool
}

func newSeriesWS(t *testing.T, edit bool) (*seriesWS, *Executor) {
	t.Helper()
	docs := []string{
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "prod", "", "  repo: {name: acme/prod, managedBy: orbs}\n"),
		chaintest.Doc("Stream", "prod", "{project: prod}", "  lines:\n    - {line: v2.16, series: \"9.x\"}\n    - {line: v2.15, series: \"8.x\"}\n"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: release/v7.x}]\n"),
		chaintest.Doc("Holding", "cons-holds-prod", "{project: cons, stream: prod}", "  at: {file: build.yaml, key: prodVersion}\n  holds: {kind: series, form: \"v{major}\"}\n"),
	}
	if edit {
		docs = append(docs, chaintest.Doc("Edit", "regen", "{holding: [cons-holds-prod]}", "  postEdit:\n    - {run: ./scripts/gen.sh}\n"))
	}
	up := seriesRepo(t)
	ws := &seriesWS{t: t, graph: chaintest.Load(t, docs...), up: up, clone: engineer(t, up)}
	return ws, &Executor{Git: GitCLI{}, Runner: steprun.Runner{}, TempDir: t.TempDir()}
}

func (w *seriesWS) Project(key string) (*chain.Project, error) { return w.graph.Project("cons"), nil }
func (w *seriesWS) Bump() (*bump.Engine, error)                { return bump.NewEngine(bump.Builtin(), w.graph) }
func (w *seriesWS) CloneDir(string) (string, error)            { return w.clone, nil }
func (w *seriesWS) Remote(string) (string, error)              { return "upstream", nil }
func (w *seriesWS) PushSettings(string) PushSettings           { return w.push }

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

// seriesTarget is the target of a line, which the test requires.
func seriesTarget(t *testing.T, p SeriesPlan, line string) SeriesTarget {
	t.Helper()
	for _, tg := range p.Targets {
		if tg.Line == line {
			return tg
		}
	}
	require.Failf(t, "no target", "line %s in %+v", line, p.Targets)
	return SeriesTarget{}
}

func TestFixSeriesWithoutAnExecutorOnlyChecks(t *testing.T) {
	ws, _ := newSeriesWS(t, true)

	plan, err := FixSeries(bg, ws, SeriesInput{NoPush: true}, nil)

	require.NoError(t, err)
	assert.Equal(t, "acme/cons", plan.Holder)
	require.Len(t, plan.Targets, 2)
	ok, behind := seriesTarget(t, plan, "v2.16"), seriesTarget(t, plan, "v2.15")
	assert.Equal(t, StatusUpToDate, ok.Status)
	assert.Nil(t, ok.Preview)
	assert.Equal(t, StatusReady, behind.Status)
	assert.Equal(t, "cons-holds-prod", behind.Holding)
	assert.Equal(t, "v9", behind.Current)
	assert.Equal(t, "v8", behind.Want)
	require.NotNil(t, behind.Preview)
	assert.Equal(t, seriesBr, behind.Preview.Branch)
	assert.Equal(t, "[v7.x] Track prod 8.x on v2.15", behind.Preview.Title)
	assert.Contains(t, behind.Preview.Body, "Set `prodVersion` in `build.yaml` from `v9` to `v8`")
	assert.Contains(t, behind.Preview.Body, "gero:ship")
	assert.Nil(t, behind.Result, "nothing was written")
	assert.False(t, branchExistsIn(t, ws.clone, seriesBr))
}

func TestFixSeriesDoesNotWriteAHoldingNoEditFollows(t *testing.T) {
	ws, ex := newSeriesWS(t, false)

	plan, err := FixSeries(bg, ws, SeriesInput{NoPush: true}, ex)

	require.NoError(t, err)
	behind := seriesTarget(t, plan, "v2.15")
	assert.Equal(t, StatusBlocked, behind.Status)
	assert.Contains(t, behind.Reason, "no Edit follows")
	assert.Nil(t, behind.Preview)
	assert.Nil(t, behind.Result)
	assert.False(t, branchExistsIn(t, ws.clone, seriesBr), "nothing is written until an Edit says what a write takes")
}

func TestFixSeriesCommitsTheWriteAndTheEditsStepOnANewBranch(t *testing.T) {
	ws, ex := newSeriesWS(t, true)

	plan, err := FixSeries(bg, ws, SeriesInput{NoPush: true}, ex)

	require.NoError(t, err)
	res := seriesTarget(t, plan, "v2.15").Result
	require.NotNil(t, res)
	assert.Equal(t, ResultCommitted, res.Status)
	assert.Equal(t, seriesBr, res.Branch)
	assert.Equal(t, heldV8, runGit(t, ws.clone, "show", seriesBr+":build.yaml")+"\n")
	assert.Equal(t, "release/v7.x", runGit(t, ws.clone, "show", seriesBr+":generated/branch.txt"), "the Edit's step ran on the branch")
	assert.Len(t, res.Commits, 2, "the write, then the step that changed files")
	assert.Contains(t, runGit(t, ws.clone, "log", "-2", "--format=%B", seriesBr), "Automation: gero ship")
	assert.Equal(t, "main", current(t, ws.clone))
	assert.Equal(t, heldV9, slurp(t, filepath.Join(ws.clone, "build.yaml")), "the engineer's checkout is untouched")
	assert.Nil(t, seriesTarget(t, plan, "v2.16").Result, "an up to date line has no work")
}

func TestFixSeriesDryRunLeavesNothingBehind(t *testing.T) {
	ws, ex := newSeriesWS(t, true)

	plan, err := FixSeries(bg, ws, SeriesInput{NoPush: true, DryRun: true}, ex)

	require.NoError(t, err)
	res := seriesTarget(t, plan, "v2.15").Result
	require.NotNil(t, res)
	assert.Equal(t, ResultWouldCommit, res.Status)
	assert.Equal(t, Worktree, res.Place)
	assert.False(t, branchExistsIn(t, ws.clone, seriesBr))
	entries, err := os.ReadDir(ex.TempDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestFixSeriesLimitsTheRunToTheLinesAndTheHoldingAsked(t *testing.T) {
	ws, _ := newSeriesWS(t, true)

	plan, err := FixSeries(bg, ws, SeriesInput{NoPush: true, Lines: []string{"2.16"}}, nil)
	require.NoError(t, err)
	require.Len(t, plan.Targets, 1)
	assert.Equal(t, "v2.16", plan.Targets[0].Line)

	_, err = FixSeries(bg, ws, SeriesInput{NoPush: true, Holding: "nope"}, nil)
	require.ErrorContains(t, err, "nope")
}

// seriesPublishing is a run that pushes to the clone's upstream and opens the PR, with
// the fake GitHub.
func seriesPublishing(t *testing.T) (*seriesWS, *Executor, *fakeRemote, *log) {
	t.Helper()
	ws, ex := newSeriesWS(t, true)
	lg := &log{}
	remote := &fakeRemote{log: lg, labels: []string{"status/auto-created"}}
	ws.push = PushSettings{Upstream: "upstream"}
	ex.Publish = &Publisher{Git: GitCLI{}, Remote: remote}
	return ws, ex, remote, lg
}

func TestFixSeriesPushesAndOpensAPRWithAMarker(t *testing.T) {
	ws, ex, _, lg := seriesPublishing(t)

	plan, err := FixSeries(bg, ws, SeriesInput{}, ex)

	require.NoError(t, err)
	res := seriesTarget(t, plan, "v2.15").Result
	require.NotNil(t, res)
	assert.Equal(t, ResultOpened, res.Status)
	require.NotNil(t, res.PR)
	assert.Equal(t, 104, res.PR.Number)
	assert.Equal(t, "upstream", res.Pushed)
	assert.Equal(t, res.Tip(), runGit(t, ws.up, "rev-parse", seriesBr), "the branch is on the remote")
	assert.Less(t, lg.index("open acme/cons release/v7.x"), lg.index("create "), "the open PRs are looked at first")
	assert.True(t, lg.has(`create acme/cons head=`+seriesBr+` base=release/v7.x title="[v7.x] Track prod 8.x on v2.15"`))
}

func TestFixSeriesAnAlreadyOpenPRMakesNothing(t *testing.T) {
	ws, ex, remote, lg := seriesPublishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{pr(77, "danpock", Marker{
		Source: "cons-holds-prod", Consumer: "acme/cons", Target: "release/v7.x", Version: "v8",
	})}}

	plan, err := FixSeries(bg, ws, SeriesInput{}, ex)

	require.NoError(t, err)
	res := seriesTarget(t, plan, "v2.15").Result
	require.NotNil(t, res)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	require.NotNil(t, res.PR)
	assert.Equal(t, 77, res.PR.Number)
	assert.False(t, lg.has("create"))
	assert.False(t, branchExistsIn(t, ws.clone, seriesBr), "no branch was made")
}
