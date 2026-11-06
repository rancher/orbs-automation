package updatedispatcher

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorder is a StepRunner that records what it was asked to run, in order.
type recorder struct {
	ran   []string
	envs  []StepEnv
	fail  string // the program that fails
	trace *[]string
}

func (r *recorder) Run(_ context.Context, step config.Step, env StepEnv) error {
	r.ran = append(r.ran, step.Run[0])
	r.envs = append(r.envs, env)
	if r.trace != nil {
		*r.trace = append(*r.trace, step.Run[0])
	}
	if step.Run[0] == r.fail {
		return errors.New("exit status 1")
	}
	return nil
}

func stepConfig() config.Config {
	cfg := fixtureConfig()
	m := &cfg.Projects[1].UpdateMethods[0]
	m.PreEdit = []config.Step{{Run: config.Command{"pre-method"}}}
	m.PostEdit = []config.Step{
		{Run: config.Command{"post-method-new"}, Since: "v2.16"},
		{Run: config.Command{"post-method-old"}, Until: "v2.15"},
	}
	e := &cfg.Projects[0].Downstream[0]
	e.PreEdit = []config.Step{{Run: config.Command{"pre-edge"}, From: "producer"}}
	e.PostEdit = []config.Step{{Run: config.Command{"post-edge"}}}
	return cfg
}

func invocationFor(t *testing.T, cfg config.Config) Invocation {
	t.Helper()
	e, err := NewEngine(Builtin(), cfg)
	require.NoError(t, err)
	inv, err := e.Invoke(e.RoutesFrom(cfg.Projects[0])[0])
	require.NoError(t, err)
	return inv
}

func names(refs []StepRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Step.Run[0]
	}
	return out
}

func TestInvokeBindsTheInputs(t *testing.T) {
	inv := invocationFor(t, fixtureConfig())
	assert.Equal(t, Inputs{"updateKey": "defaultSccOperatorImage", "value": "{image}:{tag}"}, inv.Inputs)
}

func TestInvokeWithBadParamsIsAnError(t *testing.T) {
	cfg := fixtureConfig()
	e, _ := NewEngine(Builtin(), cfg)
	route := e.RoutesFrom(cfg.Projects[0])[0]
	route.Edge.Params = map[string]string{}
	_, err := e.Invoke(route)
	assert.ErrorContains(t, err, "updateKey")
}

func TestStepsRunOutermostLast(t *testing.T) {
	inv := invocationFor(t, stepConfig())

	pre, post := inv.Steps("main")
	assert.Equal(t, []string{"pre-method", "pre-edge"}, names(pre))
	assert.Equal(t, []string{"post-edge", "post-method-new"}, names(post), "the development line takes the new post-edit step")

	pre, post = inv.Steps("release/v2.15")
	assert.Equal(t, []string{"pre-method", "pre-edge"}, names(pre))
	assert.Equal(t, []string{"post-edge", "post-method-old"}, names(post), "the older line takes the old one")
}

func TestStepsCarryTheirPhaseAndPosition(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	_, post := inv.Steps("release/v2.15")
	assert.Equal(t, MethodPostEdit, post[1].Phase)
	assert.Equal(t, 1, post[1].Index, "the position in the method's own list, so an error can point at the config")
	assert.Equal(t, EdgePostEdit, post[0].Phase)
}

func TestStepsOfABranchInNoLineRunEverythingNotRanged(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	_, post := inv.Steps("feature/x")
	assert.Equal(t, []string{"post-edge", "post-method-new", "post-method-old"}, names(post), "nothing is skipped on a guess")
}

func TestRunWrapsTheEditInTheSteps(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	var trace []string
	runner := &recorder{trace: &trace}
	tracing := &tracingTree{memTree: tree, trace: &trace}

	res, err := inv.Run(context.Background(), tracing, runner, rel, "release/v2.15", "/work/rancher", nil)

	require.NoError(t, err)
	assert.True(t, res.Changed)
	assert.Equal(t, []string{"pre-method", "pre-edge", "WRITE", "post-edge", "post-method-old"}, trace,
		"the consumer's steps outermost: its pre-edit first and its post-edit last")
}

type tracingTree struct {
	*memTree
	trace *[]string
}

func (t *tracingTree) WriteFile(ctx context.Context, path string, data []byte) error {
	*t.trace = append(*t.trace, "WRITE")
	return t.memTree.WriteFile(ctx, path, data)
}

func TestRunGivesEachStepTheDirectoryAndTheEnvironment(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	runner := &recorder{}

	_, err := inv.Run(context.Background(), tree, runner, rel, "release/v2.15", "/work/rancher", nil)

	require.NoError(t, err)
	env := runner.envs[0]
	assert.Equal(t, "/work/rancher", env.Dir)
	assert.Equal(t, "v0.5.3", env.Env["GERO_TAG"])
	assert.Equal(t, "0.5.3", env.Env["GERO_VERSION"])
	assert.Equal(t, "release/v2.15", env.Env["GERO_TARGET_BRANCH"])
	assert.Equal(t, "defaultSccOperatorImage", env.Env["GERO_PARAM_UPDATEKEY"], "each input reaches the steps")
	assert.Equal(t, "{image}:{tag}", env.Env["GERO_PARAM_VALUE"])
}

func TestRunAPreEditFailureStopsBeforeTheEdit(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	runner := &recorder{fail: "pre-edge"}

	_, err := inv.Run(context.Background(), tree, runner, rel, "main", "/w", nil)

	var stepErr *StepError
	require.ErrorAs(t, err, &stepErr)
	assert.Equal(t, EdgePreEdit, stepErr.Phase)
	assert.Equal(t, 0, stepErr.Index)
	assert.Equal(t, []string{"pre-edge"}, stepErr.Run)
	assert.Zero(t, tree.writes, "no edit was made")
	assert.Equal(t, []string{"pre-method", "pre-edge"}, runner.ran, "and nothing after it ran")
	assert.Contains(t, err.Error(), "edge preEdit[0] pre-edge")
}

func TestRunAPostEditFailureKeepsWhatWasChanged(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	runner := &recorder{fail: "post-method-new"}

	res, err := inv.Run(context.Background(), tree, runner, rel, "main", "/w", nil)

	var stepErr *StepError
	require.ErrorAs(t, err, &stepErr)
	assert.Equal(t, MethodPostEdit, stepErr.Phase)
	assert.True(t, res.Changed, "the edit was made, and the result says so")
	assert.ErrorContains(t, err, "exit status 1")
}

func TestRunAnEditFailureRunsNoPostEditStep(t *testing.T) {
	cfg := stepConfig()
	cfg.Projects[0].Downstream[0].Params["updateKey"] = "missing"
	inv := invocationFor(t, cfg)
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	runner := &recorder{}

	_, err := inv.Run(context.Background(), tree, runner, rel, "main", "/w", nil)

	require.Error(t, err)
	var stepErr *StepError
	assert.False(t, errors.As(err, &stepErr), "it is the edit that failed, not a step")
	assert.Equal(t, []string{"pre-method", "pre-edge"}, runner.ran, "the post-edit steps do not run after a failed edit")
}

func TestRunWhenAlreadyUpToDateStillRunsTheSteps(t *testing.T) {
	// The steps are the consumer's and the producer's to decide; whether to skip a
	// no-op is the orchestrator's, which checks first.
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n"}}
	runner := &recorder{}
	res, err := inv.Run(context.Background(), tree, runner, rel, "main", "/w", nil)
	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.NotEmpty(t, runner.ran)
}

func TestInvocationCheckIsTheReadSide(t *testing.T) {
	inv := invocationFor(t, fixtureConfig())
	tree := &memTree{files: map[string]string{"build.yaml": "defaultSccOperatorImage: rancher/scc-operator:v0.5.2\n"}}

	got, err := inv.Check(context.Background(), tree, rel)

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", got.Want)
	assert.Equal(t, RelationNewer, got.Relation)
	assert.Zero(t, tree.writes)
}

// units records the checkpoints.
type units struct{ got []Unit }

func (u *units) after(_ context.Context, unit Unit) error {
	u.got = append(u.got, unit)
	return nil
}

func TestRunCallsTheCheckpointAfterEachUnitInOrder(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	u := &units{}

	_, err := inv.Run(context.Background(), tree, &recorder{}, rel, "release/v2.15", "/w", u.after)

	require.NoError(t, err)
	require.Len(t, u.got, 5, "two pre steps, the edit, two post steps")
	kinds := make([]UnitKind, len(u.got))
	for i, unit := range u.got {
		kinds[i] = unit.Kind
	}
	assert.Equal(t, []UnitKind{UnitStep, UnitStep, UnitEdit, UnitStep, UnitStep}, kinds)
	assert.Equal(t, MethodPreEdit, u.got[0].Phase)
	assert.Equal(t, []string{"pre-method"}, u.got[0].Run)
	assert.Equal(t, EdgePreEdit, u.got[1].Phase)
	assert.Equal(t, EdgePostEdit, u.got[3].Phase)
	assert.Equal(t, MethodPostEdit, u.got[4].Phase)
	assert.Equal(t, []string{"post-method-old"}, u.got[4].Run)
	assert.Equal(t, 1, u.got[4].Index, "the position in the phase's own list")
}

func TestRunTheEditUnitSaysWhatTheStrategyChanged(t *testing.T) {
	inv := invocationFor(t, fixtureConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	u := &units{}

	_, err := inv.Run(context.Background(), tree, &recorder{}, rel, "main", "/w", u.after)

	require.NoError(t, err)
	require.Len(t, u.got, 1)
	assert.Equal(t, UnitEdit, u.got[0].Kind)
	require.Len(t, u.got[0].Changes, 1)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", u.got[0].Changes[0].To)
}

func TestRunCallsTheCheckpointAfterTheUnitHasDoneItsWork(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	runner := &recorder{}
	var seen []string
	after := func(_ context.Context, u Unit) error {
		if u.Kind == UnitStep {
			seen = append(seen, u.Run[0]+" after "+runner.ran[len(runner.ran)-1])
		}
		return nil
	}

	_, err := inv.Run(context.Background(), tree, runner, rel, "release/v2.15", "/w", after)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"pre-method after pre-method", "pre-edge after pre-edge", "post-edge after post-edge", "post-method-old after post-method-old",
	}, seen, "each step had run when its checkpoint was called")
}

func TestRunAFailingCheckpointStopsTheRun(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	runner := &recorder{}
	boom := errors.New("commit failed")
	after := func(_ context.Context, u Unit) error {
		if u.Kind == UnitEdit {
			return boom
		}
		return nil
	}

	_, err := inv.Run(context.Background(), tree, runner, rel, "release/v2.15", "/w", after)

	require.ErrorIs(t, err, boom)
	assert.Equal(t, []string{"pre-method", "pre-edge"}, runner.ran, "no post-edit step ran")
}

func TestRunAFailingStepHasNoCheckpointOfItsOwn(t *testing.T) {
	inv := invocationFor(t, stepConfig())
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	runner := &recorder{fail: "post-edge"}
	u := &units{}

	_, err := inv.Run(context.Background(), tree, runner, rel, "release/v2.15", "/w", u.after)

	var stepErr *StepError
	require.True(t, errors.As(err, &stepErr))
	require.Len(t, u.got, 3, "the two pre steps and the edit, and nothing for the step that failed")
}
