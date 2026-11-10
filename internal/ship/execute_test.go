package ship

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/ociregistry"

	"github.com/rancher/orbs-automation/internal/bump"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/steprun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	oldImage = "defaultSccOperatorImage: rancher/scc-operator:v0.5.2\n"
	newImage = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n"
	botV215  = "bot/scc-operator-v0.5.3-v2.15"
	botV214  = "bot/scc-operator-v0.5.3-v2.14"
)

// shipRemote is rancher on the host: main and the two release branches, each
// holding the old image in build.yaml and a script that records the tag.
func shipRemote(t *testing.T) string {
	t.Helper()
	dir := remote(t)
	runGit(t, dir, "checkout", "--quiet", "main")
	put(t, dir, "build.yaml", oldImage)
	put(t, dir, "scripts/gen.sh", "#!/bin/sh\nmkdir -p generated && printf %s \"$GERO_TAG\" > generated/tag.txt\n")
	require.NoError(t, os.Chmod(filepath.Join(dir, "scripts", "gen.sh"), 0o755))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "old image and script")
	runGit(t, dir, "branch", "release/v2.15")
	runGit(t, dir, "branch", "release/v2.14")
	return dir
}

// realWorkspace is the fixture's workspace with rancher served from a real
// repository on disk, and the engineer's clone of it.
type realWorkspace struct {
	*fakeWorkspace
	t     *testing.T
	up    string            // the repository on the host
	stale map[string]string // branch to the commit the refs claim, for one that "moved"
}

func (w *realWorkspace) UpstreamRefs(c context.Context, slug string) (git.Refs, error) {
	if slug != "rancher/rancher" {
		return w.fakeWorkspace.UpstreamRefs(c, slug)
	}
	var refs git.Refs
	for _, line := range strings.Split(runGit(w.t, w.up, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads"), "\n") {
		name, commit, _ := strings.Cut(line, " ")
		if stale, ok := w.stale[name]; ok {
			commit = stale
		}
		refs.Branches = append(refs.Branches, git.Branch{Name: name, Commit: commit})
	}
	return refs, nil
}

func (w *realWorkspace) UpstreamFile(_ context.Context, _, ref, path string) ([]byte, error) {
	for branch, claimed := range w.stale {
		if ref == claimed {
			ref = branch // the file is read as it is now, whatever the refs claimed
		}
	}
	return []byte(runGit(w.t, w.up, "show", ref+":"+path) + "\n"), nil
}

func newShip(t *testing.T) (*realWorkspace, string, *Executor) {
	t.Helper()
	up := shipRemote(t)
	clone := engineer(t, up)
	ws := &realWorkspace{fakeWorkspace: fixture(t), t: t, up: up}
	ws.cloneDir = clone
	return ws, clone, &Executor{Git: GitCLI{}, Runner: steprun.Runner{}, TempDir: t.TempDir()}
}

func shipIn(line string) Input {
	return Input{Team: testTeam, Project: "scc-operator", Lines: lineArgs(line), NoPush: true}
}

// onlyResult is the result of the one target, which the test requires.
func onlyResult(t *testing.T, p Plan) *TargetResult {
	t.Helper()
	require.Len(t, p.Targets, 1)
	require.NotNil(t, p.Targets[0].Result, "the target has a result")
	return p.Targets[0].Result
}

func TestShipWithoutAnExecutorOnlyChecks(t *testing.T) {
	ws, clone, _ := newShip(t)

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), nil, nil)

	require.NoError(t, err)
	assert.Nil(t, plan.Targets[0].Result)
	assert.Equal(t, "main", current(t, clone))
	assert.False(t, branchExistsIn(t, clone, botV215))
}

func branchExistsIn(t *testing.T, dir, name string) bool {
	t.Helper()
	return runGit(t, dir, "branch", "--list", name) != ""
}

func TestShipNoPushCommitsOnANewBranchInTheCloneAndGoesBack(t *testing.T) {
	ws, clone, ex := newShip(t)

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultCommitted, res.Status)
	assert.Equal(t, botV215, res.Branch)
	assert.Equal(t, InPlace, res.Place)
	assert.Equal(t, res.Tip(), runGit(t, clone, "rev-parse", botV215))
	assert.Equal(t, newImage, runGit(t, clone, "show", botV215+":build.yaml")+"\n")
	assert.NotEmpty(t, res.Changes)
	assert.Equal(t, "main", current(t, clone), "the clone is back on the branch it started on")
	assert.Equal(t, oldImage, slurp(t, filepath.Join(clone, "build.yaml")))
	assert.Contains(t, runGit(t, clone, "log", "-1", "--format=%B", botV215), "v0.5.3", "the preview's commit message")
}

func TestShipDryRunCommitsInAWorktreeAndLeavesNothingBehind(t *testing.T) {
	ws, clone, ex := newShip(t)
	in := shipIn("v2.15")
	in.DryRun = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultWouldCommit, res.Status)
	assert.Equal(t, Worktree, res.Place)
	assert.Contains(t, res.Reason, "dry run")
	assert.NotEmpty(t, res.Tip())
	assert.NotEmpty(t, res.Changes)
	assert.False(t, branchExistsIn(t, clone, botV215), "the branch is removed")
	assert.Equal(t, "main", current(t, clone))
	entries, err := os.ReadDir(ex.TempDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the worktree is removed")
}

func TestShipUsesAWorktreeForADirtyCloneAndKeepsTheBranch(t *testing.T) {
	ws, clone, ex := newShip(t)
	put(t, clone, "build.yaml", "defaultSccOperatorImage: mine\n")

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultCommitted, res.Status)
	assert.Equal(t, Worktree, res.Place)
	assert.Contains(t, res.Reason, "uncommitted")
	assert.Equal(t, res.Tip(), runGit(t, clone, "rev-parse", botV215))
	assert.Equal(t, "defaultSccOperatorImage: mine\n", slurp(t, filepath.Join(clone, "build.yaml")), "the engineer's edit is untouched")
	assert.Equal(t, "main", current(t, clone))
}

func TestShipMakesACommitForEachTargetAndFailsOnlyTheOneThatFails(t *testing.T) {
	ws, clone, ex := newShip(t)
	// release/v2.14 has no script, so its post-edit step fails; release/v2.15 has it.
	runGit(t, ws.up, "checkout", "--quiet", "release/v2.14")
	runGit(t, ws.up, "rm", "--quiet", "scripts/gen.sh")
	runGit(t, ws.up, "commit", "--quiet", "-m", "no script here")
	runGit(t, ws.up, "checkout", "--quiet", "main")
	ws.links[0].Post = []stepSpec{{Run: "./scripts/gen.sh"}}

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn(""), ex, nil)

	require.NoError(t, err)
	results := map[string]*TargetResult{}
	for _, tp := range plan.Targets {
		require.NotNil(t, tp.Result, tp.Branch)
		results[tp.Branch] = tp.Result
	}
	ok, bad := results["release/v2.15"], results["release/v2.14"]
	assert.Equal(t, ResultCommitted, ok.Status)
	assert.Equal(t, "v0.5.3", runGit(t, clone, "show", botV215+":generated/tag.txt"), "the step ran and its output is committed")
	assert.Equal(t, ResultFailed, bad.Status)
	assert.Contains(t, bad.Error, "postEdit[0]")
	assert.False(t, branchExistsIn(t, clone, botV214), "the failed target leaves no branch")
	assert.Equal(t, "main", current(t, clone))
	assert.Equal(t, oldImage, slurp(t, filepath.Join(clone, "build.yaml")), "and nothing changed in the clone")
}

func TestShipSkipsATargetThatIsAlreadyUpToDate(t *testing.T) {
	ws, clone, ex := newShip(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v2.14")
	put(t, ws.up, "build.yaml", newImage)
	runGit(t, ws.up, "commit", "--quiet", "-am", "already there")
	runGit(t, ws.up, "checkout", "--quiet", "main")

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn(""), ex, nil)

	require.NoError(t, err)
	byBranch := map[string]TargetPlan{}
	for _, tp := range plan.Targets {
		byBranch[tp.Branch] = tp
	}
	assert.Equal(t, StatusUpToDate, byBranch["release/v2.14"].Status)
	assert.Nil(t, byBranch["release/v2.14"].Result, "the state check already stopped it")
	require.NotNil(t, byBranch["release/v2.15"].Result)
	assert.False(t, branchExistsIn(t, clone, botV214))
}

func TestShipAMovedBranchIsAFailureThatSaysToRunAgain(t *testing.T) {
	ws, clone, ex := newShip(t)
	ws.stale = map[string]string{"release/v2.15": strings.Repeat("a", 40)}

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultFailed, res.Status)
	assert.Contains(t, res.Error, "moved")
	assert.Contains(t, res.Error, "run it again")
	assert.False(t, branchExistsIn(t, clone, botV215))
}

func TestShipARerunWithCommitsOnTheBranchIsAFailureNamingIt(t *testing.T) {
	ws, clone, ex := newShip(t)
	runGit(t, clone, "fetch", "--quiet", "upstream", "release/v2.15")
	runGit(t, clone, "checkout", "--quiet", "-b", botV215, "FETCH_HEAD")
	put(t, clone, "mine.txt", "x")
	runGit(t, clone, "add", ".")
	runGit(t, clone, "commit", "--quiet", "-m", "mine")
	runGit(t, clone, "checkout", "--quiet", "main")

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultFailed, res.Status)
	assert.Contains(t, res.Error, botV215)
}

func TestShipMakesNoCommitWhenSomethingBlocksTheRelease(t *testing.T) {
	ws, clone, ex := newShip(t)
	p := &fakeProber{results: map[string]ociregistry.Result{
		"registry.rancher.com rancher/scc-operator:v0.5.3": {Status: ociregistry.NotFound},
	}}

	plan, err := Ship(bg, ws, p, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, StatusBlocked, plan.Status)
	assert.Nil(t, plan.Targets[0].Result)
	assert.False(t, branchExistsIn(t, clone, botV215))
}

func TestShipWithNoCloneIsAnErrorWhenThereIsAnythingToCommit(t *testing.T) {
	ws, _, ex := newShip(t)
	ws.cloneErr = ErrNoClone

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.ErrorIs(t, err, ErrNoClone)
	assert.Equal(t, StatusReady, plan.Status, "the plan is still returned")
	assert.Nil(t, plan.Targets[0].Result)
}

func TestShipWithNoCloneIsFineWhenEverythingIsUpToDate(t *testing.T) {
	ws, _, ex := newShip(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v2.15")
	put(t, ws.up, "build.yaml", newImage)
	runGit(t, ws.up, "commit", "--quiet", "-am", "already there")
	ws.cloneErr = errors.New("no clone")

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, plan.Status)
}

var _ bump.StepRunner = steprun.Runner{}

func TestShipRecordsTheCloneAndRemoteItMadeCommitsIn(t *testing.T) {
	ws, clone, ex := newShip(t)

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, clone, plan.Clone)
	assert.Equal(t, "upstream", plan.Remote)
}

func TestShipRecordsNoCloneWhenItMadeNoCommits(t *testing.T) {
	ws, _, _ := newShip(t)

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), nil, nil)

	require.NoError(t, err)
	assert.Empty(t, plan.Clone)
	assert.Empty(t, plan.Remote)
}

func TestShipCommitsTheEditAndEachStepSeparatelyWithTheirOwnMessages(t *testing.T) {
	ws, clone, ex := newShip(t)
	ws.links[0].Post = []stepSpec{{Run: "./scripts/gen.sh"}}

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	require.Equal(t, ResultCommitted, res.Status)
	require.Len(t, res.Commits, 2)
	assert.Equal(t, "Update scc-operator to v0.5.3", res.Commits[0].Subject, "the preview's commit message")
	assert.Equal(t, "Run `./scripts/gen.sh`", res.Commits[1].Subject)
	assert.Equal(t, res.Tip(), runGit(t, clone, "rev-parse", botV215))
	step := runGit(t, clone, "log", "-1", "--format=%B", botV215)
	assert.Contains(t, step, "Part of updating scc-operator to v0.5.3: the postEdit step.")
	assert.Contains(t, step, "Automation: gero ship")
	assert.Equal(t, "2", runGit(t, clone, "rev-list", "--count", "upstream/release/v2.15.."+botV215), "two commits on the base")
}

// lineArgs is a line as the lines of an Input: none for "".
func lineArgs(line string) []string {
	if line == "" {
		return nil
	}
	return []string{line}
}

// brokenSigning makes git's signing fail, as an engineer with no usable key has
// it, so a commit that is asked to be signed cannot be made.
func brokenSigning(t *testing.T, clone string) {
	t.Helper()
	runGit(t, clone, "config", "gpg.program", "false")
	runGit(t, clone, "config", "user.signingkey", "no-such-key")
}

func TestShipSignsTheCommitsWhenAskedAndFailsWhenItCannot(t *testing.T) {
	ws, clone, ex := newShip(t)
	brokenSigning(t, clone)
	ex.Sign = true

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultFailed, res.Status, "asked for signed, so never an unsigned one instead")
	assert.False(t, branchExistsIn(t, clone, botV215), "and nothing is left behind")
}

func TestShipDoesNotSignByDefault(t *testing.T) {
	ws, clone, ex := newShip(t)
	brokenSigning(t, clone)

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultCommitted, onlyResult(t, plan).Status)
}
