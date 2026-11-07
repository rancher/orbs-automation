package bumpcommit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git/gitcli"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/rancher/orbs-automation/internal/workdir"
)

var bg = context.Background()

// fakeGit is the git of a work directory: it records the commits asked for, and
// says there was nothing to commit for the calls it is told to.
type fakeGit struct {
	commits    []workdir.GitCommit
	nothingFor map[int]bool // the 1-based call numbers with nothing to commit
	commitErr  error
	files      map[string][]string // what each commit changed, by hash
}

func (f *fakeGit) CommitFiles(_ context.Context, _, commit string) ([]string, error) {
	return f.files[commit], nil
}

func (f *fakeGit) State(context.Context, string) (workdir.GitState, error) {
	return workdir.GitState{}, nil
}

func (f *fakeGit) Commit(_ context.Context, _ string, c workdir.GitCommit) (string, error) {
	f.commits = append(f.commits, c)
	n := len(f.commits)
	if f.commitErr != nil {
		return "", f.commitErr
	}
	if f.nothingFor[n] {
		return "", gitcli.ErrNothingToCommit
	}
	return fmt.Sprintf("commit%d", n), nil
}

func (f *fakeGit) Checkout(context.Context, string, string) error       { return nil }
func (f *fakeGit) DeleteBranch(context.Context, string, string) error   { return nil }
func (f *fakeGit) RemoveWorktree(context.Context, string, string) error { return nil }
func (f *fakeGit) Discard(context.Context, string, []string) error      { return nil }

// fakeRunner records the steps it was asked to run, and fails the one it is told to.
type fakeRunner struct {
	ran    []string
	failOn string
}

func (r *fakeRunner) Run(_ context.Context, step config.Step, _ updatedispatcher.StepEnv) error {
	cmd := strings.Join(step.Run, " ")
	r.ran = append(r.ran, cmd)
	if cmd == r.failOn {
		return errors.New("exit status 1")
	}
	return nil
}

// work is a branch in a directory that holds build.yaml at the old image.
func work(t *testing.T, g *fakeGit) *workdir.Work {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.yaml"), []byte("image: rancher/scc-operator:v0.5.2\n"), 0o644))
	return workdir.NewInWorktree(g, "/clone", dir, "bot/x-v0.5.3-main", "a test")
}

func step(cmd ...string) config.Step { return config.Step{Run: config.Command(cmd)} }

// invocation is scc-operator shipping to rancher through yamlBump, with the given
// steps before and after the edit.
func invocation(t *testing.T, pre, post []config.Step) updatedispatcher.Invocation {
	t.Helper()
	producer := config.Project{
		Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"},
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

func input(w *workdir.Work, inv updatedispatcher.Invocation) Input {
	return Input{
		Work: w, Invocation: inv, Branch: "main", Message: "Update to v0.5.3\n\nbody",
		Release: updatedispatcher.Release{Tag: "v0.5.3", Version: "0.5.3", Image: "rancher/scc-operator", SourceRepo: "rancher/scc-operator"},
	}
}

func stepMessage(u updatedispatcher.Unit) (string, error) {
	return "Run `" + strings.Join(u.Run, " ") + "`\n\nAutomation: gero ship", nil
}

func TestRunEditsTheFileAndCommitsItWithTheMessage(t *testing.T) {
	g := &fakeGit{}
	w := work(t, g)

	out, err := Run(bg, &fakeRunner{}, input(w, invocation(t, nil, nil)))

	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(w.Dir, "build.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "image: rancher/scc-operator:v0.5.3\n", string(body))
	require.Len(t, g.commits, 1)
	assert.Equal(t, "Update to v0.5.3\n\nbody", g.commits[0].Message)
	assert.Equal(t, []workdir.CommitInfo{{Hash: "commit1", Subject: "Update to v0.5.3"}}, out.Commits)
	assert.Equal(t, "commit1", out.Tip())
	require.Len(t, out.Changes, 1)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", out.Changes[0].From)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", out.Changes[0].To)
}

func TestRunCommitsAsTheGivenIdentityAndSigning(t *testing.T) {
	g := &fakeGit{}
	in := input(work(t, g), invocation(t, nil, nil))
	in.Name, in.Email, in.Sign = "gero bot", "bot@example.com", true

	_, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	require.Len(t, g.commits, 1)
	assert.Equal(t, "gero bot", g.commits[0].Name)
	assert.Equal(t, "bot@example.com", g.commits[0].Email)
	assert.True(t, g.commits[0].Sign)
}

func TestRunWithoutAnIdentityLeavesItToTheClonesGitConfig(t *testing.T) {
	g := &fakeGit{}

	_, err := Run(bg, &fakeRunner{}, input(work(t, g), invocation(t, nil, nil)))

	require.NoError(t, err)
	require.Len(t, g.commits, 1)
	assert.Empty(t, g.commits[0].Name)
	assert.Empty(t, g.commits[0].Email)
	assert.False(t, g.commits[0].Sign)
}

func TestRunNothingToCommitAnywhereIsErrNothingChanged(t *testing.T) {
	g := &fakeGit{nothingFor: map[int]bool{1: true}}

	out, err := Run(bg, &fakeRunner{}, input(work(t, g), invocation(t, nil, nil)))

	require.ErrorIs(t, err, ErrNothingChanged)
	assert.Empty(t, out.Commits)
}

func TestRunCommitsTheEditAndThenWhatEachStepMade(t *testing.T) {
	g := &fakeGit{}
	r := &fakeRunner{}
	in := input(work(t, g), invocation(t, nil, []config.Step{step("go", "generate"), step("make", "docs")}))
	in.StepMessage = stepMessage

	out, err := Run(bg, r, in)

	require.NoError(t, err)
	assert.Equal(t, []string{"go generate", "make docs"}, r.ran)
	require.Len(t, g.commits, 3)
	assert.Equal(t, "Update to v0.5.3\n\nbody", g.commits[0].Message)
	assert.Equal(t, "Run `go generate`\n\nAutomation: gero ship", g.commits[1].Message)
	assert.Equal(t, "Run `make docs`\n\nAutomation: gero ship", g.commits[2].Message)
	assert.Equal(t, []string{"Update to v0.5.3", "Run `go generate`", "Run `make docs`"}, subjects(out.Commits))
	assert.Equal(t, "commit3", out.Tip())
}

func TestRunWithoutAStepMessageAStepsCommitSaysWhatRan(t *testing.T) {
	g := &fakeGit{}
	in := input(work(t, g), invocation(t, nil, []config.Step{step("go", "generate")}))

	_, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	require.Len(t, g.commits, 2)
	assert.Equal(t, "Run `go generate`", g.commits[1].Message)
}

func TestRunAStepThatChangesNothingMakesNoCommit(t *testing.T) {
	g := &fakeGit{nothingFor: map[int]bool{2: true}}
	in := input(work(t, g), invocation(t, nil, []config.Step{step("true")}))
	in.StepMessage = stepMessage

	out, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	assert.Equal(t, []string{"Update to v0.5.3"}, subjects(out.Commits))
}

func TestRunAPreEditStepGetsItsOwnCommitBeforeTheEdit(t *testing.T) {
	g := &fakeGit{}
	in := input(work(t, g), invocation(t, []config.Step{step("prepare")}, nil))
	in.StepMessage = stepMessage

	out, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	assert.Equal(t, []string{"Run `prepare`", "Update to v0.5.3"}, subjects(out.Commits))
}

func TestRunAFailingStepIsAStepErrorAndTheCommitsMadeSoFarAreNotReported(t *testing.T) {
	g := &fakeGit{}
	in := input(work(t, g), invocation(t, nil, []config.Step{step("false")}))

	out, err := Run(bg, &fakeRunner{failOn: "false"}, in)

	var stepErr *updatedispatcher.StepError
	require.True(t, errors.As(err, &stepErr), "got %v", err)
	assert.Equal(t, updatedispatcher.MethodPostEdit, stepErr.Phase)
	assert.Equal(t, []string{"false"}, stepErr.Run)
	assert.Empty(t, out.Commits, "the caller aborts the work, which removes them")
}

func TestRunAFailureToMakeAStepsMessageIsAFailure(t *testing.T) {
	g := &fakeGit{}
	in := input(work(t, g), invocation(t, nil, []config.Step{step("go", "generate")}))
	in.StepMessage = func(updatedispatcher.Unit) (string, error) { return "", errors.New("bad placeholder") }

	_, err := Run(bg, &fakeRunner{}, in)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "the commit message for go generate")
	assert.Contains(t, err.Error(), "bad placeholder")
}

func TestRunACommitThatFailsForAnotherReasonStopsTheRun(t *testing.T) {
	boom := errors.New("hook rejected")
	g := &fakeGit{commitErr: boom}

	_, err := Run(bg, &fakeRunner{}, input(work(t, g), invocation(t, nil, nil)))

	require.ErrorIs(t, err, boom)
}

func TestRunTellsOnCommitOfEachCommitAsItIsMade(t *testing.T) {
	g := &fakeGit{}
	var seen []workdir.CommitInfo
	in := input(work(t, g), invocation(t, nil, []config.Step{step("go", "generate")}))
	in.StepMessage = stepMessage
	in.OnCommit = func(c workdir.CommitInfo) { seen = append(seen, c) }

	out, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	require.Len(t, seen, 2, "the edit and the step")
	assert.Equal(t, out.Commits, seen)
}

func subjects(commits []workdir.CommitInfo) []string {
	out := make([]string, len(commits))
	for i, c := range commits {
		out[i] = c.Subject
	}
	return out
}

func TestRunEachCommitHasTheFilesItChanged(t *testing.T) {
	g := &fakeGit{files: map[string][]string{"commit1": {"build.yaml"}, "commit2": {"generated/a.go", "generated/b.go"}}}
	w := work(t, g)
	inv := invocation(t, nil, []config.Step{{Run: config.Command{"gen"}}})

	out, err := Run(bg, &fakeRunner{}, Input{
		Work: w, Invocation: inv, Branch: "main", Message: "Update to v0.5.3",
		Release:     updatedispatcher.Release{Tag: "v0.5.3", Version: "0.5.3", Image: "rancher/scc-operator", SourceRepo: "rancher/scc-operator"},
		StepMessage: stepMessage,
	})

	require.NoError(t, err)
	require.Len(t, out.Commits, 2)
	assert.Equal(t, []string{"build.yaml"}, out.Commits[0].Files)
	assert.Equal(t, []string{"generated/a.go", "generated/b.go"}, out.Commits[1].Files)
}

func TestRunTheCommitCallbackGetsTheFilesToo(t *testing.T) {
	g := &fakeGit{files: map[string][]string{"commit1": {"build.yaml"}}}
	w := work(t, g)
	var seen []workdir.CommitInfo
	in := input(w, invocation(t, nil, nil))
	in.OnCommit = func(c workdir.CommitInfo) { seen = append(seen, c) }

	_, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	require.Len(t, seen, 1)
	assert.Equal(t, []string{"build.yaml"}, seen[0].Files)
}
