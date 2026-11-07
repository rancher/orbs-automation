package ship

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git/gitcli"
	"github.com/rancher/orbs-automation/internal/steprun"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const applyBranch = "bot/x-v0.5.3-main"

// applyRemote is a consumer whose main holds build.yaml at the old image and a
// script that records the tag it ran for.
func applyRemote(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	dir := remote(t)
	runGit(t, dir, "checkout", "--quiet", "main")
	put(t, dir, "build.yaml", "image: rancher/scc-operator:v0.5.2\n")
	put(t, dir, "scripts/gen.sh", "#!/bin/sh\nmkdir -p generated && printf %s \"$GERO_TAG\" > generated/tag.txt\n")
	put(t, dir, "scripts/pre.sh", "#!/bin/sh\nprintf prepared > prepared.txt\n")
	require.NoError(t, os.Chmod(filepath.Join(dir, "scripts", "gen.sh"), 0o755))
	require.NoError(t, os.Chmod(filepath.Join(dir, "scripts", "pre.sh"), 0o755))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "script and old image")
	return dir
}

// invocationFor is scc-operator shipping to rancher through yamlBump, with the
// given steps after the edit.
func invocationFor(t *testing.T, post []config.Step) updatedispatcher.Invocation {
	t.Helper()
	return invocationSteps(t, nil, post)
}

// invocationSteps is invocationFor with steps before the edit as well.
func invocationSteps(t *testing.T, pre, post []config.Step) updatedispatcher.Invocation {
	t.Helper()
	producer := config.Project{
		Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator", Owned: true},
		Downstream: []config.Edge{{To: "rancher", Method: "build-yaml-bump",
			Params: map[string]string{"updateKey": "image", "value": "{image}:{tag}"}}},
	}
	consumer := config.Project{
		Name: "rancher", Repo: config.Repo{Name: "rancher/rancher", DefaultBranch: "main"},
		UpdateMethods: []config.UpdateMethod{{
			Name: "build-yaml-bump", Strategy: "yamlBump", With: map[string]string{"file": "build.yaml"},
			AllowedParams: []config.AllowedParam{{Key: "updateKey"}, {Key: "value", Default: "{tag}"}},
			PreEdit:       pre,
			PostEdit:      post,
		}},
	}
	engine, err := updatedispatcher.NewEngine(updatedispatcher.Builtin(), config.Config{Projects: []config.Project{producer, consumer}})
	require.NoError(t, err)
	routes := engine.RoutesFrom(producer)
	require.Len(t, routes, 1)
	inv, err := engine.Invoke(routes[0])
	require.NoError(t, err)
	return inv
}

var applyRelease = updatedispatcher.Release{
	Tag: "v0.5.3", Version: "0.5.3", Image: "rancher/scc-operator", SourceRepo: "rancher/scc-operator",
}

func applyFor(inv updatedispatcher.Invocation) Apply {
	return Apply{
		Invocation: inv, Release: applyRelease, Branch: "main", Message: "Update to v0.5.3\n\nbody",
		Runner: steprun.Runner{}, StepMessage: testStepMessage,
	}
}

// testStepMessage is the commit message of a step, as the real one is made.
func testStepMessage(u updatedispatcher.Unit) (string, error) {
	return "Run `" + strings.Join(u.Run, " ") + "`\n\nAutomation: gero ship", nil
}

// subjects lists the subjects of the commits on the branch since base, oldest
// first.
func subjects(t *testing.T, dir, base string) []string {
	t.Helper()
	out := runGit(t, dir, "log", "--reverse", "--format=%s", base+"..HEAD")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// filesOf lists the files a commit changed.
func filesOf(t *testing.T, dir, commit string) []string {
	t.Helper()
	return strings.Fields(runGit(t, dir, "show", "--name-only", "--format=", commit))
}

func prepareMain(t *testing.T, up, clone string, dry bool) *Work {
	t.Helper()
	return mustPrepare(t, PrepareRequest{
		CloneDir: clone, Remote: "upstream", Target: "main", Base: runGit(t, up, "rev-parse", "main"),
		Branch: applyBranch, DryRun: dry, TempDir: t.TempDir(),
	})
}

func TestApplyEditsTheFileAndCommitsItWithTheMessage(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)

	res, err := applyFor(invocationFor(t, nil)).Run(bg, w)

	require.NoError(t, err)
	assert.Equal(t, res.Tip(), runGit(t, w.Dir, "rev-parse", "HEAD"))
	assert.Equal(t, "image: rancher/scc-operator:v0.5.3\n", slurp(t, filepath.Join(w.Dir, "build.yaml")))
	assert.Equal(t, "Update to v0.5.3\n\nbody", runGit(t, w.Dir, "log", "-1", "--format=%B"))
	assert.Equal(t, []string{"build.yaml"}, strings.Fields(runGit(t, w.Dir, "show", "--name-only", "--format=", "HEAD")))
	require.Len(t, res.Changes, 1)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", res.Changes[0].From)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", res.Changes[0].To)
}

func TestApplyRunsThePostEditStepsInTheWorkDirectoryAndCommitsWhatTheyMake(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	put(t, clone, "notes.txt", "engineer's own")
	w := prepareMain(t, up, clone, false)
	require.Equal(t, InPlace, w.Place)
	inv := invocationFor(t, []config.Step{{Run: config.Command{"./scripts/gen.sh"}}})

	res, err := applyFor(inv).Run(bg, w)

	require.NoError(t, err)
	require.Len(t, res.Commits, 2)
	assert.Equal(t, []string{"build.yaml"}, filesOf(t, w.Dir, res.Commits[0].Hash))
	assert.Equal(t, []string{"generated/tag.txt"}, filesOf(t, w.Dir, res.Commits[1].Hash), "what the step made is its own commit")
	assert.Equal(t, "v0.5.3", slurp(t, filepath.Join(w.Dir, "generated", "tag.txt")), "the step saw GERO_TAG")
	st, err := gitcli.Status(bg, clone)
	require.NoError(t, err)
	assert.Equal(t, []string{"notes.txt"}, st.Untracked, "the engineer's file is not in the commit")
}

func TestApplyInAWorktreeLeavesTheEngineersCheckoutAlone(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, true)
	require.Equal(t, Worktree, w.Place)

	_, err := applyFor(invocationFor(t, nil)).Run(bg, w)

	require.NoError(t, err)
	assert.Equal(t, "image: rancher/scc-operator:v0.5.2\n", slurp(t, filepath.Join(clone, "build.yaml")))
	assert.Equal(t, "image: rancher/scc-operator:v0.5.3\n", slurp(t, filepath.Join(w.Dir, "build.yaml")))
}

func TestApplyWhenTheFileAlreadyHoldsTheValueChangesNothing(t *testing.T) {
	up := applyRemote(t)
	runGit(t, up, "checkout", "--quiet", "main")
	put(t, up, "build.yaml", "image: rancher/scc-operator:v0.5.3\n")
	runGit(t, up, "commit", "--quiet", "-am", "already there")
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)
	before := runGit(t, w.Dir, "rev-parse", "HEAD")

	res, err := applyFor(invocationFor(t, nil)).Run(bg, w)

	require.ErrorIs(t, err, ErrNothingChanged)
	assert.Empty(t, res.Commits)
	assert.Equal(t, before, runGit(t, w.Dir, "rev-parse", "HEAD"), "no commit was made")
}

func TestApplyAFailingStepIsAStepErrorAndAbortUndoesTheRun(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)
	inv := invocationFor(t, []config.Step{{Run: config.Command{"false"}}})

	_, err := applyFor(inv).Run(bg, w)

	var stepErr *updatedispatcher.StepError
	require.True(t, errors.As(err, &stepErr), "got %v", err)
	assert.Equal(t, updatedispatcher.MethodPostEdit, stepErr.Phase)
	assert.Equal(t, []string{"false"}, stepErr.Run)

	require.NoError(t, w.Abort(bg))
	assert.Equal(t, "main", current(t, clone))
	assert.False(t, branchExistsIn(t, clone, applyBranch), "the edit's commit is gone with the branch")
	assert.Equal(t, "image: rancher/scc-operator:v0.5.2\n", slurp(t, filepath.Join(clone, "build.yaml")), "the edit was undone")
}

func TestApplyCommitsAsTheGivenIdentity(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)
	a := applyFor(invocationFor(t, nil))
	a.Name, a.Email = "gero", "gero@example.com"

	_, err := a.Run(bg, w)

	require.NoError(t, err)
	assert.Equal(t, "gero <gero@example.com>", runGit(t, w.Dir, "log", "-1", "--format=%an <%ae>"))
}

func TestApplyWithoutAnIdentityUsesTheClonesOwn(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)

	_, err := applyFor(invocationFor(t, nil)).Run(bg, w)

	require.NoError(t, err)
	assert.Equal(t, "Dev <dev@example.com>", runGit(t, w.Dir, "log", "-1", "--format=%an <%ae>"))
}

func TestApplyCommitsTheEditAndThenWhatEachStepMade(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	put(t, clone, "notes.txt", "engineer's own")
	w := prepareMain(t, up, clone, false)
	base := runGit(t, w.Dir, "rev-parse", "HEAD")
	inv := invocationFor(t, []config.Step{{Run: config.Command{"./scripts/gen.sh"}}})

	res, err := applyFor(inv).Run(bg, w)

	require.NoError(t, err)
	require.Len(t, res.Commits, 2, "the edit, and what generate made")
	assert.Equal(t, []string{"Update to v0.5.3", "Run `./scripts/gen.sh`"}, subjects(t, w.Dir, base))
	assert.Equal(t, []string{"Update to v0.5.3", "Run `./scripts/gen.sh`"}, []string{res.Commits[0].Subject, res.Commits[1].Subject})
	assert.Equal(t, []string{"build.yaml"}, filesOf(t, w.Dir, res.Commits[0].Hash), "the edit's commit has only the edit")
	assert.Equal(t, []string{"generated/tag.txt"}, filesOf(t, w.Dir, res.Commits[1].Hash), "the step's commit has only what it made")
	assert.Equal(t, res.Tip(), runGit(t, w.Dir, "rev-parse", "HEAD"))
	assert.Contains(t, runGit(t, w.Dir, "log", "-1", "--format=%B"), "Automation: gero ship")
	st, err := gitcli.Status(bg, clone)
	require.NoError(t, err)
	assert.Equal(t, []string{"notes.txt"}, st.Untracked, "the engineer's file is in neither commit")
}

func TestApplyAStepThatChangesNothingMakesNoCommit(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)
	base := runGit(t, w.Dir, "rev-parse", "HEAD")
	inv := invocationFor(t, []config.Step{{Run: config.Command{"true"}}})

	res, err := applyFor(inv).Run(bg, w)

	require.NoError(t, err)
	assert.Len(t, res.Commits, 1)
	assert.Equal(t, []string{"Update to v0.5.3"}, subjects(t, w.Dir, base))
}

func TestApplyAPreEditStepGetsItsOwnCommitBeforeTheEdit(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)
	base := runGit(t, w.Dir, "rev-parse", "HEAD")
	inv := invocationSteps(t, []config.Step{{Run: config.Command{"./scripts/pre.sh"}}}, nil)

	res, err := applyFor(inv).Run(bg, w)

	require.NoError(t, err)
	require.Len(t, res.Commits, 2)
	assert.Equal(t, []string{"Run `./scripts/pre.sh`", "Update to v0.5.3"}, subjects(t, w.Dir, base))
	assert.Equal(t, []string{"prepared.txt"}, filesOf(t, w.Dir, res.Commits[0].Hash))
	assert.Equal(t, []string{"build.yaml"}, filesOf(t, w.Dir, res.Commits[1].Hash))
}

func TestApplyAFailingStepKeepsTheEarlierCommitsUntilTheWorkIsAborted(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)
	base := runGit(t, w.Dir, "rev-parse", "HEAD")
	inv := invocationFor(t, []config.Step{
		{Run: config.Command{"./scripts/gen.sh"}}, {Run: config.Command{"false"}},
	})

	_, err := applyFor(inv).Run(bg, w)

	var stepErr *updatedispatcher.StepError
	require.True(t, errors.As(err, &stepErr))
	assert.Equal(t, []string{"Update to v0.5.3", "Run `./scripts/gen.sh`"}, subjects(t, w.Dir, base), "what was done before the failure is committed")

	require.NoError(t, w.Abort(bg))
	assert.False(t, branchExistsIn(t, clone, applyBranch), "and the abort removes it all")
	assert.Equal(t, "main", current(t, clone))
}

func TestApplyAFailureToMakeAStepsMessageIsAFailure(t *testing.T) {
	up := applyRemote(t)
	clone := engineer(t, up)
	w := prepareMain(t, up, clone, false)
	a := applyFor(invocationFor(t, []config.Step{{Run: config.Command{"./scripts/gen.sh"}}}))
	a.StepMessage = func(updatedispatcher.Unit) (string, error) { return "", errors.New("bad placeholder") }

	_, err := a.Run(bg, w)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad placeholder")
}
