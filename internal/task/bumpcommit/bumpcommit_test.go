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

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
	"github.com/rancher/orbs-automation/internal/platform/workdir"
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

func (r *fakeRunner) Run(_ context.Context, step chain.Step, _ bump.StepEnv) error {
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

func step(cmd ...string) chain.Step { return chain.Step{Run: v1alpha2.Command(cmd)} }

// scene is scc-operator shipping its image to rancher, which holds it in build.yaml,
// with the given steps before and after the write.
type scene struct {
	route  bump.Route
	target bump.Target
}

func invocation(t *testing.T, pre, post []chain.Step) scene {
	t.Helper()
	spec := "  holds: {kind: release, form: \"{image}:{tag}\"}\n"
	docs := []string{
		chaintest.Doc("Project", "scc-operator", "", "  repo: {name: rancher/scc-operator, owned: true}\n"),
		chaintest.Doc("Stream", "scc-operator", "{project: scc-operator}", "  universal: true\n"),
		chaintest.Doc("Artifact", "scc-image", "{stream: scc-operator}", "  kind: image\n  name: rancher/scc-operator\n"),
		chaintest.Doc("Project", "rancher", "", "  repo: {name: rancher/rancher}\n"),
		chaintest.Doc("Holding", "rancher-holds-scc", "{project: rancher, artifact: scc-image}", "  at: {file: build.yaml, key: image}\n"+spec),
	}
	if len(pre)+len(post) > 0 {
		docs = append(docs, chaintest.Doc("Edit", "regen", "{holding: rancher-holds-scc}", steps("preEdit", pre)+steps("postEdit", post)))
	}
	g := chaintest.Load(t, docs...)
	engine, err := bump.NewEngine(bump.Builtin(), g)
	require.NoError(t, err)
	routes := engine.RoutesFrom(g.Project("scc-operator"))
	require.Len(t, routes, 1)
	targets, err := routes[0].TargetsFor("")
	require.NoError(t, err)
	require.Len(t, targets, 1)
	return scene{route: routes[0], target: targets[0]}
}

// steps writes the steps of a phase as the YAML of an edit's spec.
func steps(phase string, list []chain.Step) string {
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("  " + phase + ":\n")
	for _, st := range list {
		fmt.Fprintf(&b, "    - {run: [%s]}\n", strings.Join(st.Run, ", "))
	}
	return b.String()
}

func input(w *workdir.Work, sc scene) Input {
	return Input{Work: w, Route: sc.route, Target: sc.target, Message: "Update to v0.5.3\n\nbody", Tag: "v0.5.3", Commit: "abc"}
}

func stepMessage(u bump.Unit) (string, error) {
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
	in := input(work(t, g), invocation(t, nil, []chain.Step{step("go", "generate"), step("make", "docs")}))
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
	in := input(work(t, g), invocation(t, nil, []chain.Step{step("go", "generate")}))

	_, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	require.Len(t, g.commits, 2)
	assert.Equal(t, "Run `go generate`", g.commits[1].Message)
}

func TestRunAStepThatChangesNothingMakesNoCommit(t *testing.T) {
	g := &fakeGit{nothingFor: map[int]bool{2: true}}
	in := input(work(t, g), invocation(t, nil, []chain.Step{step("true")}))
	in.StepMessage = stepMessage

	out, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	assert.Equal(t, []string{"Update to v0.5.3"}, subjects(out.Commits))
}

func TestRunAPreEditStepGetsItsOwnCommitBeforeTheEdit(t *testing.T) {
	g := &fakeGit{}
	in := input(work(t, g), invocation(t, []chain.Step{step("prepare")}, nil))
	in.StepMessage = stepMessage

	out, err := Run(bg, &fakeRunner{}, in)

	require.NoError(t, err)
	assert.Equal(t, []string{"Run `prepare`", "Update to v0.5.3"}, subjects(out.Commits))
}

func TestRunAFailingStepIsAStepErrorAndTheCommitsMadeSoFarAreNotReported(t *testing.T) {
	g := &fakeGit{}
	in := input(work(t, g), invocation(t, nil, []chain.Step{step("false")}))

	out, err := Run(bg, &fakeRunner{failOn: "false"}, in)

	var stepErr *bump.StepError
	require.True(t, errors.As(err, &stepErr), "got %v", err)
	assert.Equal(t, bump.PostEdit, stepErr.Phase)
	assert.Equal(t, []string{"false"}, stepErr.Run)
	assert.Empty(t, out.Commits, "the caller aborts the work, which removes them")
}

func TestRunAFailureToMakeAStepsMessageIsAFailure(t *testing.T) {
	g := &fakeGit{}
	in := input(work(t, g), invocation(t, nil, []chain.Step{step("go", "generate")}))
	in.StepMessage = func(bump.Unit) (string, error) { return "", errors.New("bad placeholder") }

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
	in := input(work(t, g), invocation(t, nil, []chain.Step{step("go", "generate")}))
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
	inv := invocation(t, nil, []chain.Step{{Run: v1alpha2.Command{"gen"}}})

	out, err := Run(bg, &fakeRunner{}, Input{
		Work: w, Route: inv.route, Target: inv.target, Message: "Update to v0.5.3", Tag: "v0.5.3", Commit: "abc",
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

// goBumpScene is the same release held in a go.mod, with the goBump strategy.
func goBumpScene(t *testing.T) scene {
	t.Helper()
	docs := []string{
		chaintest.Doc("Project", "lib", "", "  repo: {name: acme/lib, owned: true}\n"),
		chaintest.Doc("Stream", "lib", "{project: lib}", "  universal: true\n"),
		chaintest.Doc("Artifact", "lib-image", "{stream: lib}", "  kind: image\n  name: acme/lib\n"),
		chaintest.Doc("Project", "svc", "", "  repo: {name: acme/svc}\n"),
		chaintest.Doc("Holding", "svc-holds-lib", "{project: svc, artifact: lib-image}", "  at: {file: go.mod, key: github.com/acme/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n"),
		chaintest.Doc("Edit", "tidy", "{holding: svc-holds-lib}", "  strategy: goBump\n"),
	}
	g := chaintest.Load(t, docs...)
	engine, err := bump.NewEngine(bump.Builtin(), g)
	require.NoError(t, err)
	routes := engine.RoutesFrom(g.Project("lib"))
	require.Len(t, routes, 1)
	targets, err := routes[0].TargetsFor("")
	require.NoError(t, err)
	require.Len(t, targets, 1)
	return scene{route: routes[0], target: targets[0]}
}

func goModWork(t *testing.T, g *fakeGit) *workdir.Work {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module acme.test/svc\n\nrequire github.com/acme/lib v0.5.2\n"), 0o644))
	return workdir.NewInWorktree(g, "/clone", dir, "bot/x-v0.5.3-main", "a test")
}

// goSumWriter is a runner whose go mod tidy writes a go.sum in the directory it runs in.
type goSumWriter struct{ ran []string }

func (r *goSumWriter) Run(_ context.Context, step chain.Step, env bump.StepEnv) error {
	r.ran = append(r.ran, strings.Join(step.Run, " "))
	return os.WriteFile(filepath.Join(env.Dir, "go.sum"), []byte("github.com/acme/lib v0.5.3 h1:x\n"), 0o644)
}

func TestRunAStrategyGetsItsOwnCommitWithTheMessageOfTheStrategy(t *testing.T) {
	g := &fakeGit{}
	in := input(goModWork(t, g), goBumpScene(t))
	var seen bump.Unit
	in.StrategyMessage = func(u bump.Unit) (string, error) {
		seen = u
		return "Run " + u.Strategy + "\n\nAutomation: gero ship", nil
	}

	out, err := Run(bg, &goSumWriter{}, in)

	require.NoError(t, err)
	require.Len(t, g.commits, 2)
	assert.Equal(t, "Update to v0.5.3\n\nbody", g.commits[0].Message, "the write keeps the message of the bump")
	assert.Equal(t, "Run goBump\n\nAutomation: gero ship", g.commits[1].Message)
	assert.Equal(t, bump.UnitStrategy, seen.Kind)
	assert.Equal(t, "tidy", seen.Edit)
	assert.Equal(t, "goBump", seen.Strategy)
	require.Len(t, seen.Changes, 1)
	assert.Equal(t, []string{"Update to v0.5.3", "Run goBump"}, subjects(out.Commits))
}

func TestRunWithoutAStrategyMessageItsCommitSaysWhichStrategyRanAndWhatItChanged(t *testing.T) {
	g := &fakeGit{}
	in := input(goModWork(t, g), goBumpScene(t))

	_, err := Run(bg, &goSumWriter{}, in)

	require.NoError(t, err)
	require.Len(t, g.commits, 2)
	assert.Equal(t, "Run goBump\n\n- Ran `go mod tidy`", g.commits[1].Message)
}

func TestRunAStrategyThatChangesNothingMakesNoCommit(t *testing.T) {
	g := &fakeGit{nothingFor: map[int]bool{2: true}}
	in := input(goModWork(t, g), goBumpScene(t))

	out, err := Run(bg, &goSumWriter{}, in)

	require.NoError(t, err)
	assert.Equal(t, []string{"Update to v0.5.3"}, subjects(out.Commits))
}

func TestRunAFailureToMakeAStrategysMessageIsAFailure(t *testing.T) {
	g := &fakeGit{}
	in := input(goModWork(t, g), goBumpScene(t))
	in.StrategyMessage = func(bump.Unit) (string, error) { return "", errors.New("no template") }

	_, err := Run(bg, &goSumWriter{}, in)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no template")
	assert.Contains(t, err.Error(), "goBump")
}
