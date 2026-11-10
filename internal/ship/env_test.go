package ship

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// regenEdit is an Edit of bro's wrangler holding that runs go generate in the
// toolchain the branch pins.
var regenEdit = chaintest.Doc("Edit", "regen", "{holding: bro-holds-wr}", "  env:\n    GOTOOLCHAIN: \"{goToolchain}\"\n  postEdit:\n    - {run: [go, generate]}\n")

// goModOf is bro's go.mod with the directives given before its requires, wrangler at
// v1.7.1 and lasso at v0.2.3.
func goModOf(directives string) string {
	return "module acme/bro\n\n" + directives + "\nrequire (\n\tgithub.com/acme/la v0.2.3\n\tgithub.com/acme/wr v1.7.1\n)\n"
}

// envWorkspace is the world of pinWorkspace with regenEdit, whose branches pin
// different Go versions: r15 go1.26.10, and main go1.27.1.
func envWorkspace(t *testing.T) pinWorkspace {
	t.Helper()
	w := newPinWorkspace(t, regenEdit)
	w.host.files["acme/bro@sha-r15:go.mod"] = goModOf("go 1.26\n\ntoolchain go1.26.10\n")
	w.host.files["acme/bro@sha-main:go.mod"] = goModOf("go 1.27.1\n")
	return w
}

func TestACheckSaysTheToolchainEachBranchPins(t *testing.T) {
	got, err := shipWrangler(envWorkspace(t))

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"GOTOOLCHAIN": "go1.26.10"}, r15Target(t, got).Env, "the toolchain line")
	for _, tp := range got.Targets {
		if tp.Branch == "main" {
			assert.Equal(t, map[string]string{"GOTOOLCHAIN": "go1.27.1"}, tp.Env, "the go line has a patch")
		}
	}
}

func TestACheckHasNoEnvWhenNoEditWritesOne(t *testing.T) {
	w := newPinWorkspace(t)
	w.host.files["acme/bro@sha-r15:go.mod"] = goModOf("go 1.26\n\ntoolchain go1.26.10\n")

	got, err := shipWrangler(w)

	require.NoError(t, err)
	assert.Empty(t, r15Target(t, got).Env)
}

func TestThePRBodySaysWhatToolchainTheStepsRanWith(t *testing.T) {
	got, err := shipWrangler(envWorkspace(t))

	require.NoError(t, err)
	tp := r15Target(t, got)
	require.NotNil(t, tp.Preview)
	assert.Contains(t, tp.Preview.Body, "The steps ran with `GOTOOLCHAIN=go1.26.10`")
}

func TestATargetThatHoldsWhatItShouldHasNoEnvAndNoBlocker(t *testing.T) {
	w := envWorkspace(t)
	w.host.files["acme/bro@sha-r15:go.mod"] = "module acme/bro\n\ngo 1.26\n\nrequire (\n\tgithub.com/acme/la v0.2.5\n\tgithub.com/acme/wr v1.7.4\n)\n"

	got, err := shipWrangler(w)

	require.NoError(t, err)
	tp := r15Target(t, got)
	assert.Equal(t, StatusUpToDate, tp.Status, "nothing to run, so a go line that cannot be resolved does not matter")
	assert.Empty(t, tp.Env)
	assert.Empty(t, tp.Blockers)
}

func TestATargetBlocksWhenItsToolchainCannotBeNamed(t *testing.T) {
	w := envWorkspace(t)
	w.host.files["acme/bro@sha-r15:go.mod"] = goModOf("go 1.26\n") // no toolchain line, and no list of releases

	got, err := shipWrangler(w)

	require.NoError(t, err, "the plan says it, and does not stop")
	tp := r15Target(t, got)
	assert.Equal(t, StatusBlocked, tp.Status)
	require.Len(t, tp.Blockers, 1)
	assert.Contains(t, tp.Blockers[0], "toolchain:")
	assert.Contains(t, tp.Blockers[0], "toolchain line")
	assert.Equal(t, StatusBlocked, got.Status)
}

// releaseList is a prober that also names the newest patch of a Go minor.
type releaseList struct {
	*fakeProber
	toolchain string
	err       error
}

func (r releaseList) NewestPatch(_ context.Context, minor string) (string, error) {
	if minor != "1.26" {
		return "", errors.New("unexpected minor " + minor)
	}
	return r.toolchain, r.err
}

func TestACheckNamesAGoLineWithNoPatchFromTheListOfReleasesTheProberHas(t *testing.T) {
	w := envWorkspace(t)
	w.host.files["acme/bro@sha-r15:go.mod"] = goModOf("go 1.26\n")
	prober := releaseList{fakeProber: &fakeProber{}, toolchain: "go1.26.10"}

	got, err := Check(ctx, w, prober, Input{Team: testTeam, Project: "wr", To: "bro", Arg: "v1.7.4"})

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"GOTOOLCHAIN": "go1.26.10"}, r15Target(t, got).Env)
}

// envRecorder is a step runner that records the environment each step ran in.
type envRecorder struct {
	envs []map[string]string
	runs []string
}

func (r *envRecorder) Run(_ context.Context, step chain.Step, env bump.StepEnv) error {
	r.envs = append(r.envs, env.Env)
	r.runs = append(r.runs, strings.Join(step.Run, " "))
	return nil
}

func TestShipRunsTheStepsInTheToolchainTheBranchPins(t *testing.T) {
	up := shipRemote(t)
	runGit(t, up, "checkout", "--quiet", "release/v2.15")
	put(t, up, "go.mod", "module acme/rancher\n\ngo 1.26\n\ntoolchain go1.26.10\n")
	runGit(t, up, "add", ".")
	runGit(t, up, "commit", "--quiet", "-m", "pin go")
	runGit(t, up, "checkout", "--quiet", "main")
	clone := engineer(t, up)
	ws := &realWorkspace{fakeWorkspace: fixture(t), t: t, up: up}
	ws.cloneDir = clone
	ws.links[0].Post = []stepSpec{{Run: "regen"}}
	ws.links[0].Env = "    GOTOOLCHAIN: \"{goToolchain}\"\n"
	rec := &envRecorder{}
	ex := &Executor{Git: GitCLI{}, Runner: rec, TempDir: t.TempDir()}

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"GOTOOLCHAIN": "go1.26.10"}, plan.Targets[0].Env)
	assert.Equal(t, ResultCommitted, onlyResult(t, plan).Status)
	require.Len(t, rec.runs, 1)
	assert.Equal(t, "regen", rec.runs[0])
	assert.Equal(t, "go1.26.10", rec.envs[0]["GOTOOLCHAIN"], "the step ran in the toolchain the branch pins, whatever the engineer's Go is")
	assert.Equal(t, "v0.5.3", rec.envs[0]["GERO_TAG"], "beside the engine's own")
}
