package bump

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goTree is a tree of files that says a file is not there as fs.ErrNotExist, as the
// trees of a clone and of the host do, and counts the files it reads.
type goTree struct {
	files map[string]string
	err   error // what every read gives instead, when set
	reads []string
}

func (g *goTree) ReadFile(_ context.Context, path string) ([]byte, error) {
	g.reads = append(g.reads, path)
	if g.err != nil {
		return nil, g.err
	}
	data, ok := g.files[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(data), nil
}

func (g *goTree) WriteFile(_ context.Context, path string, data []byte) error {
	g.files[path] = string(data)
	return nil
}

// releases is a list of Go releases that answers with one toolchain and says what
// minor it was asked for.
type releases struct {
	got       string
	toolchain string
	err       error
}

func (r *releases) NewestPatch(_ context.Context, minor string) (string, error) {
	r.got = minor
	return r.toolchain, r.err
}

// envRoute is the route wrangler to bro and a target on r15, whose edits are the
// documents given, which follow the holding.
func envRoute(t *testing.T, edits ...string) (Route, Target) {
	t.Helper()
	g := pinGraph(t, edits...)
	e, err := NewEngine(Builtin(), g)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("wr"))[0]
	return route, Target{Branch: "r15", Line: "v2.15", Holdings: route.Holdings}
}

func goEdit(name, env string) string {
	return chaintest.Doc("Edit", name, "{holding: bro-holds-wr}", "  env:\n"+env+"  postEdit:\n    - {run: [go, generate]}\n")
}

const toolchainEnv = "    GOTOOLCHAIN: \"{goToolchain}\"\n"

func TestEnvOfTheToolchainLineOfGoMod(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", toolchainEnv))
	tree := &goTree{files: map[string]string{"go.mod": "module m\n\ngo 1.26\n\ntoolchain go1.26.10\n"}}

	got, err := route.EnvOf(bg, tree, target)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"GOTOOLCHAIN": "go1.26.10"}, got)
}

func TestEnvOfAGoLineWithAPatchWhenThereIsNoToolchainLine(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", toolchainEnv))
	tree := &goTree{files: map[string]string{"go.mod": "module m\n\ngo 1.26.3\n"}}

	got, err := route.EnvOf(bg, tree, target)

	require.NoError(t, err)
	assert.Equal(t, "go1.26.3", got["GOTOOLCHAIN"], "no release list was needed")
}

func TestEnvOfAGoLineWithNoPatchIsTheNewestPatchOfTheMinor(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", toolchainEnv))
	list := &releases{toolchain: "go1.26.10"}
	route.GoReleases = list
	tree := &goTree{files: map[string]string{"go.mod": "module m\n\ngo 1.26\n"}}

	got, err := route.EnvOf(bg, tree, target)

	require.NoError(t, err)
	assert.Equal(t, "go1.26.10", got["GOTOOLCHAIN"])
	assert.Equal(t, "1.26", list.got, "it asked for the minor")
}

func TestEnvOfAGoLineWithNoPatchAndNoReleaseListSaysToNameAToolchain(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", toolchainEnv))
	tree := &goTree{files: map[string]string{"go.mod": "module m\n\ngo 1.26\n"}}

	_, err := route.EnvOf(bg, tree, target)

	require.Error(t, err)
	assert.ErrorContains(t, err, "go 1.26")
	assert.ErrorContains(t, err, "toolchain line")

	route.GoReleases = &releases{err: errors.New("offline")}
	_, err = route.EnvOf(bg, tree, target)
	require.Error(t, err)
	assert.ErrorContains(t, err, "offline", "and why the list could not say")
	assert.ErrorContains(t, err, "toolchain line")
}

func TestEnvOfABranchWithNoGoModLeavesTheEntryOut(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", toolchainEnv+"    GOFLAGS: -mod=mod\n"))

	got, err := route.EnvOf(bg, &goTree{files: map[string]string{}}, target)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"GOFLAGS": "-mod=mod"}, got, "a value that renders empty is not set, so the local Go is not told it is the empty string")
}

func TestEnvOfAGoModThatCannotBeReadOrParsedIsAnError(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", toolchainEnv))

	_, err := route.EnvOf(bg, &goTree{err: errors.New("rate limited")}, target)
	require.Error(t, err, "a read that failed is not a file that is not there")
	assert.ErrorContains(t, err, "rate limited")

	_, err = route.EnvOf(bg, &goTree{files: map[string]string{"go.mod": "module m\n\ngo\n"}}, target)
	require.Error(t, err)
	assert.ErrorContains(t, err, "go.mod")
}

func TestEnvOfReadsGoModOnlyWhenAValueUsesIt(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", "    GOFLAGS: -mod=mod\n    NOTE: \"{line} on {branch}\"\n"))
	tree := &goTree{files: map[string]string{"go.mod": "module m\n\ngo 1.26.3\n"}}

	got, err := route.EnvOf(bg, tree, target)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"GOFLAGS": "-mod=mod", "NOTE": "v2.15 on r15"}, got)
	assert.Empty(t, tree.reads, "nothing here needs the file")
}

func TestEnvOfReadsGoModOnceForAllTheValuesThatUseIt(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", toolchainEnv+"    ALSO: \"{goToolchain}\"\n"))
	tree := &goTree{files: map[string]string{"go.mod": "module m\n\ngo 1.26.3\n"}}

	got, err := route.EnvOf(bg, tree, target)

	require.NoError(t, err)
	assert.Equal(t, "go1.26.3", got["ALSO"])
	assert.Equal(t, []string{"go.mod"}, tree.reads)
}

func TestEnvOfTwoEditsThatAgreeIsOneEntryAndThatDisagreeIsAnError(t *testing.T) {
	tree := &goTree{files: map[string]string{"go.mod": "module m\n\ngo 1.26.3\n"}}

	route, target := envRoute(t, goEdit("a", toolchainEnv), goEdit("b", toolchainEnv+"    GOFLAGS: -mod=mod\n"))
	got, err := route.EnvOf(bg, tree, target)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"GOTOOLCHAIN": "go1.26.3", "GOFLAGS": "-mod=mod"}, got)

	route, target = envRoute(t, goEdit("a", "    GOFLAGS: -mod=mod\n"), goEdit("b", "    GOFLAGS: -mod=vendor\n"))
	_, err = route.EnvOf(bg, tree, target)
	require.Error(t, err)
	assert.ErrorContains(t, err, "GOFLAGS")
	assert.ErrorContains(t, err, `"a"`)
	assert.ErrorContains(t, err, `"b"`)
}

func TestEnvOfAnEditWithNoEnvIsEmpty(t *testing.T) {
	route, target := envRoute(t, chaintest.Doc("Edit", "plain", "{holding: bro-holds-wr}", "  postEdit:\n    - {run: x}\n"))

	got, err := route.EnvOf(bg, &goTree{files: map[string]string{}}, target)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRunGivesEveryStepAndStrategyTheEnvOfTheTarget(t *testing.T) {
	route, target := envRoute(t, chaintest.Doc("Edit", "regen", "{holding: bro-holds-wr}",
		"  env:\n"+toolchainEnv+"  strategy: goBump\n  preEdit:\n    - {run: [go, vet]}\n  postEdit:\n    - {run: [go, generate]}\n"))
	tree := &goTree{files: map[string]string{
		"go.mod": "module m\n\ngo 1.26\n\ntoolchain go1.26.10\n\nrequire github.com/acme/wr v1.7.1\n",
	}}
	rec := &envRunner{}

	_, err := route.Run(bg, tree, rec, target, "v1.7.4", "sha", "/work", nil)

	require.NoError(t, err)
	require.Len(t, rec.envs, 3, "the step before, go mod tidy of goBump, and the step after")
	for i, env := range rec.envs {
		assert.Equal(t, "go1.26.10", env["GOTOOLCHAIN"], "run %d", i)
	}
	assert.Equal(t, "v1.7.4", rec.envs[0]["GERO_TAG"], "the engine's own variables stay")
	assert.Empty(t, rec.envs[1]["GERO_TAG"], "goBump sets none of its own, and has the Edit's only")
}

func TestRunStopsBeforeAnythingIsWrittenWhenTheEnvCannotBeWorkedOut(t *testing.T) {
	route, target := envRoute(t, goEdit("regen", toolchainEnv))
	tree := &goTree{files: map[string]string{"go.mod": "module m\n\ngo 1.26\n\nrequire github.com/acme/wr v1.7.1\n"}}
	rec := &envRunner{}

	_, err := route.Run(bg, tree, rec, target, "v1.7.4", "sha", "/work", nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "toolchain line")
	assert.Empty(t, rec.envs, "no step ran")
	assert.Contains(t, tree.files["go.mod"], "v1.7.1", "and nothing was written")
}
