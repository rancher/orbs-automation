package bump

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tidyRunner is a StepRunner that records the programs it is asked to run and the
// directory of each, changes files when told to, and fails in one directory.
type tidyRunner struct {
	ran   []string
	order *[]string
	onRun func(dir string)
	fail  string
}

func (r *tidyRunner) Run(_ context.Context, step chain.Step, env StepEnv) error {
	cmd := strings.Join(step.Run, " ") + "@" + env.Dir
	r.ran = append(r.ran, cmd)
	if r.order != nil {
		*r.order = append(*r.order, "run "+cmd)
	}
	if env.Dir == r.fail {
		return errors.New("exit 1")
	}
	if r.onRun != nil {
		r.onRun(env.Dir)
	}
	return nil
}

const (
	rootMod = "module acme.test/svc\n\nrequire github.com/acme/lib v1.0.0\n"
	apisMod = "module acme.test/svc/pkg/apis\n\nrequire github.com/acme/lib v1.0.0 // indirect\n"
)

// goBumpRoute is the consumer's holding of the library in two go.mod files, and an
// edit with the strategy over it.
func goBumpRoute(t *testing.T) (Route, Target) {
	t.Helper()
	e, g, err := buildEngine(t,
		d("Holding", "cons-mod", "{project: cons, artifact: prod-image}", "  at:\n    - {file: go.mod, key: github.com/acme/lib, format: gomod}\n    - {file: pkg/apis/go.mod, key: github.com/acme/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n"),
		d("Edit", "tidy", "{holding: cons-mod}", "  strategy: goBump\n"),
	)
	require.NoError(t, err)
	var mod *chain.Holding
	route := e.RoutesFrom(g.Project("prod"))[0]
	for _, h := range route.Holdings {
		if h.Name() == "cons-mod" {
			mod = h
		}
	}
	require.NotNil(t, mod)
	return route, Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: []*chain.Holding{mod}}
}

func TestGoBumpTidiesEachModuleTheHoldingChanged(t *testing.T) {
	route, target := goBumpRoute(t)
	tree := files{"go.mod": rootMod, "pkg/apis/go.mod": apisMod}
	var order []string
	run := &tidyRunner{order: &order}

	res, err := route.Run(bg, tree, run, target, "v1.2.0", "abc", "/work", func(_ context.Context, u Unit) error {
		order = append(order, fmt.Sprintf("unit %s", u.Kind))
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"go mod tidy@/work", "go mod tidy@/work/pkg/apis"}, run.ran, "in each module, the root first")
	assert.Equal(t, []string{"unit edit", "run go mod tidy@/work", "run go mod tidy@/work/pkg/apis", "unit strategy"}, order,
		"the holdings are written and committed, then tidy runs, then its own commit")
	assert.Contains(t, tree["go.mod"], "github.com/acme/lib v1.2.0")
	assert.True(t, res.Changed)
}

func TestGoBumpReportsOnlyTheModulesTidyChanged(t *testing.T) {
	route, target := goBumpRoute(t)
	tree := files{"go.mod": rootMod, "pkg/apis/go.mod": apisMod, "pkg/apis/go.sum": "old\n"}
	run := &tidyRunner{onRun: func(dir string) {
		if dir == "/work/pkg/apis" {
			tree["pkg/apis/go.sum"] = "new\n" // tidy changed go.sum only, in the nested module
		}
	}}
	var strategy []Change
	_, err := route.Run(bg, tree, run, target, "v1.2.0", "abc", "/work", func(_ context.Context, u Unit) error {
		if u.Kind == UnitStrategy {
			strategy = u.Changes
		}
		return nil
	})

	require.NoError(t, err)
	require.Len(t, strategy, 1, "the root module was not changed by tidy")
	assert.Equal(t, "pkg/apis/go.mod", strategy[0].Path)
	assert.Contains(t, strategy[0].Description, "go mod tidy")
	assert.Contains(t, strategy[0].Description, "pkg/apis")
}

func TestGoBumpWhenTidyChangesNothingReportsNothing(t *testing.T) {
	route, target := goBumpRoute(t)
	tree := files{"go.mod": rootMod, "pkg/apis/go.mod": apisMod}
	var strategy []Change
	called := false

	_, err := route.Run(bg, tree, &tidyRunner{}, target, "v1.2.0", "abc", "/work", func(_ context.Context, u Unit) error {
		if u.Kind == UnitStrategy {
			called, strategy = true, u.Changes
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called, "the strategy still has its checkpoint")
	assert.Empty(t, strategy)
}

func TestGoBumpDoesNotRunWhenTheHoldingChangedNothing(t *testing.T) {
	route, target := goBumpRoute(t)
	tree := files{"go.mod": strings.Replace(rootMod, "v1.0.0", "v1.2.0", 1), "pkg/apis/go.mod": strings.Replace(apisMod, "v1.0.0", "v1.2.0", 1)}
	run := &tidyRunner{}

	_, err := route.Run(bg, tree, run, target, "v1.2.0", "abc", "/work", nil)

	require.NoError(t, err)
	assert.Empty(t, run.ran, "already held, so nothing was written and nothing needs tidying")
}

func TestGoBumpTidiesAModuleOnceWhateverTheNumberOfLocationsInIt(t *testing.T) {
	e, g, err := buildEngine(t,
		d("Holding", "cons-two", "{project: cons, artifact: prod-image}", "  at:\n    - {file: go.mod, key: github.com/acme/lib, format: gomod}\n    - {file: go.mod, key: github.com/acme/other, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n"),
		d("Edit", "tidy", "{holding: cons-two}", "  strategy: goBump\n"),
	)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]
	var two *chain.Holding
	for _, h := range route.Holdings {
		if h.Name() == "cons-two" {
			two = h
		}
	}
	tree := files{"go.mod": "module m\n\nrequire (\n\tgithub.com/acme/lib v1.0.0\n\tgithub.com/acme/other v1.0.0\n)\n"}
	run := &tidyRunner{}

	_, err = route.Run(bg, tree, run, Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: []*chain.Holding{two}}, "v1.2.0", "abc", "/work", nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"go mod tidy@/work"}, run.ran)
}

func TestGoBumpOnlyTidiesGoModFilesNotOtherFilesTheEditsHoldingsWrite(t *testing.T) {
	e, g, err := buildEngine(t,
		d("Holding", "cons-mixed", "{project: cons, artifact: prod-image}", "  at:\n    - {file: go.mod, key: github.com/acme/lib, format: gomod}\n    - {file: charts/values.yaml, key: .image.tag}\n  holds: {kind: release, form: \"{bareTag}\"}\n"),
		d("Edit", "tidy", "{holding: cons-mixed}", "  strategy: goBump\n"),
	)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]
	var mixed *chain.Holding
	for _, h := range route.Holdings {
		if h.Name() == "cons-mixed" {
			mixed = h
		}
	}
	tree := files{"go.mod": rootMod, "charts/values.yaml": "image:\n  tag: v1.0.0\n"}
	run := &tidyRunner{}

	_, err = route.Run(bg, tree, run, Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: []*chain.Holding{mixed}}, "v1.2.0", "abc", "/work", nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"go mod tidy@/work"}, run.ran, "charts/ is not a Go module")
}

func TestGoBumpAFailedTidyStopsTheRunNamingTheEditAndTheModule(t *testing.T) {
	route, target := goBumpRoute(t)
	tree := files{"go.mod": rootMod, "pkg/apis/go.mod": apisMod}
	run := &tidyRunner{fail: "/work/pkg/apis"}

	res, err := route.Run(bg, tree, run, target, "v1.2.0", "abc", "/work", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `Edit "tidy"`)
	assert.Contains(t, err.Error(), "pkg/apis")
	assert.Contains(t, err.Error(), "go mod tidy")
	assert.True(t, res.Changed, "the require lines were written before tidy failed")
}

func TestGoBumpTakesNoValues(t *testing.T) {
	_, err := goBump{}.Configure(map[string]string{"package": "x"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "goBump")
	assert.Contains(t, err.Error(), "no with")
	_, err = goBump{}.Configure(nil)
	assert.NoError(t, err)
}

func TestARouteWithGoBumpIsReady(t *testing.T) {
	route, _ := goBumpRoute(t)

	assert.NoError(t, route.Ready())
}
