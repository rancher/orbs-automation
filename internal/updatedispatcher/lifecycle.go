package updatedispatcher

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
)

// Phase is where in the lifecycle a step runs.
type Phase string

// The phases, in the order they run around the edit.
const (
	MethodPreEdit  Phase = "method preEdit"
	EdgePreEdit    Phase = "edge preEdit"
	EdgePostEdit   Phase = "edge postEdit"
	MethodPostEdit Phase = "method postEdit"
)

// StepEnv is what a step runs with: the directory of the clone, and the
// environment the dispatcher builds for it (GERO_TAG, GERO_VERSION,
// GERO_TARGET_BRANCH and GERO_PARAM_<KEY> for each input).
type StepEnv struct {
	Dir string
	Env map[string]string
}

// StepRunner runs one step. The dispatcher decides which steps run and in what
// order; a runner owns how: the process, its sandbox and where a step's
// program comes from (the step's From).
type StepRunner interface {
	Run(ctx context.Context, step config.Step, env StepEnv) error
}

// StepError is a step that failed, naming where it was.
type StepError struct {
	Phase Phase
	Index int // within the phase's list
	Run   []string
	Err   error
}

func (e *StepError) Error() string {
	return fmt.Sprintf("%s[%d] %s: %v", e.Phase, e.Index, strings.Join(e.Run, " "), e.Err)
}

func (e *StepError) Unwrap() error { return e.Err }

// Invocation is a route with its params bound: everything a call needs.
type Invocation struct {
	Route  Route
	Inputs Inputs
}

// Invoke binds the route's edge params against its method.
func (e *Engine) Invoke(route Route) (Invocation, error) {
	in, err := route.Method.Bind(route.Edge)
	if err != nil {
		return Invocation{}, err
	}
	return Invocation{Route: route, Inputs: in}, nil
}

// Check is the read side of the route's method: what the consumer holds in src
// and what the release wants. It never writes.
func (inv Invocation) Check(ctx context.Context, src Tree, rel Release) (Check, error) {
	return inv.Route.Method.Updater.Check(ctx, src, inv.Inputs, rel)
}

// StepRef is a step with the phase and position it has.
type StepRef struct {
	Phase Phase
	Index int
	Step  config.Step
}

// Steps returns the steps that run for a branch of the consumer, in the order
// they run: the method's pre-edit steps, the edge's, then (after the edit) the
// edge's post-edit steps and, last, the method's. The consumer's steps surround
// the producer's, so its post-edit regenerates what the producer's touched. Only
// steps whose range holds the branch's line run; a branch in no line gets every
// step that has no range.
func (inv Invocation) Steps(branch string) (pre, post []StepRef) {
	line := inv.Route.Consumer.LineOfBranch(branch)
	collect := func(phase Phase, steps []config.Step) []StepRef {
		var out []StepRef
		for i, s := range steps {
			if s.AppliesTo(line) {
				out = append(out, StepRef{Phase: phase, Index: i, Step: s})
			}
		}
		return out
	}
	m, e := inv.Route.Method, inv.Route.Edge
	pre = append(collect(MethodPreEdit, m.PreEdit), collect(EdgePreEdit, e.PreEdit)...)
	post = append(collect(EdgePostEdit, e.PostEdit), collect(MethodPostEdit, m.PostEdit)...)
	return pre, post
}

// Run runs the lifecycle for one branch in the clone: the pre-edit steps, Apply,
// then the post-edit steps. A step that fails stops the run, as a *StepError; if
// it is a post-edit step the edit has been made, and Result says what changed.
func (inv Invocation) Run(ctx context.Context, tree WritableTree, runner StepRunner, rel Release, branch string, dir string) (Result, error) {
	pre, post := inv.Steps(branch)
	env := StepEnv{Dir: dir, Env: inv.stepEnv(rel, branch)}
	for _, ref := range pre {
		if err := runner.Run(ctx, ref.Step, env); err != nil {
			return Result{}, &StepError{Phase: ref.Phase, Index: ref.Index, Run: ref.Step.Run, Err: err}
		}
	}
	res, err := inv.Route.Method.Updater.Apply(ctx, tree, inv.Inputs, rel)
	if err != nil {
		return Result{}, err
	}
	for _, ref := range post {
		if err := runner.Run(ctx, ref.Step, env); err != nil {
			return res, &StepError{Phase: ref.Phase, Index: ref.Index, Run: ref.Step.Run, Err: err}
		}
	}
	return res, nil
}

// stepEnv is the environment of a step.
func (inv Invocation) stepEnv(rel Release, branch string) map[string]string {
	env := map[string]string{
		"GERO_TAG": rel.Tag, "GERO_VERSION": rel.Version, "GERO_TARGET_BRANCH": branch,
		"GERO_SOURCE_REPO": rel.SourceRepo,
	}
	keys := make([]string, 0, len(inv.Inputs))
	for k := range inv.Inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env["GERO_PARAM_"+strings.ToUpper(k)] = inv.Inputs[k]
	}
	return env
}
