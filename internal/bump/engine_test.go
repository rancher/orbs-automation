package bump

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var d = chaintest.Doc

// world has two producers and two consumers:
//
//	prod (owned, universal) makes an image and a chart file
//	lined (owned, lines v2.16 and v2.15) makes an image
//	cons holds prod's image and chart file (stage dev), and lined's image
//	other holds prod's image
func world(extra ...string) []string {
	docs := []string{
		chaintest.Lines("v2.16", "v2.15"),
		d("Project", "prod", "", "  repo: {name: acme/prod, owned: true}\n"),
		d("Stream", "prod", "{project: prod}", "  universal: true\n"),
		d("Artifact", "prod-image", "{stream: prod}", "  kind: image\n  name: acme/prod\n"),
		d("Artifact", "prod-chart", "{stream: prod}", "  kind: file\n  name: prod-{version}.tgz\n"),
		d("Project", "lined", "", "  repo: {name: acme/lined, owned: true}\n"),
		d("Stream", "lined", "{project: lined}", "  lines:\n    - {line: v2.16, series: \"2.x\"}\n    - {line: v2.15, series: \"1.x\"}\n"),
		d("Artifact", "lined-image", "{stream: lined}", "  kind: image\n  name: acme/lined\n"),
		d("Project", "cons", "", "  repo: {name: acme/cons}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines:\n      - {line: v2.16, branch: {dev: dev-v2.16, stable: rel-v2.16}}\n      - {line: v2.15, branch: {dev: dev-v2.15, stable: rel-v2.15}}\n"),
		d("Project", "other", "", "  repo: {name: acme/other}\n"),
		d("Holding", "cons-image", "{project: cons, artifact: prod-image}", "  at: {file: build.yaml, key: image}\n  holds: {kind: release, form: \"{image}:{tag}\", stage: dev}\n"),
		d("Holding", "cons-chart", "{project: cons, artifact: prod-chart}", "  at: {file: pkg.yaml, key: url}\n  holds: {kind: release, form: \"{url}\", stage: dev}\n"),
		d("Holding", "cons-lined", "{project: cons, artifact: lined-image}", "  at: {file: build.yaml, key: lined}\n  holds: {kind: release, form: \"{image}:{tag}\", stage: stable}\n"),
		d("Holding", "other-image", "{project: other, artifact: prod-image}", "  at: {file: other.yaml, key: image}\n  holds: {kind: release, form: \"{image}:{tag}\"}\n"),
		d("Holding", "cons-commit", "{project: cons, from: prod}", "  at: {file: lock.yaml, key: .commit}\n  holds: {kind: commit}\n"),
	}
	return append(docs, extra...)
}

func buildEngine(t *testing.T, extra ...string) (*Engine, *chain.Graph, error) {
	t.Helper()
	g := chaintest.Load(t, world(extra...)...)
	e, err := NewEngine(Builtin(), g)
	return e, g, err
}

func names(routes []Route) []string {
	var out []string
	for _, r := range routes {
		var hs []string
		for _, h := range r.Holdings {
			hs = append(hs, h.Name())
		}
		out = append(out, r.Producer.Name()+" > "+r.Consumer.Name()+": "+strings.Join(hs, ", "))
	}
	return out
}

func TestRoutesFromAProducerAreOneForEachConsumerWithAllItsReleaseHoldings(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"prod > cons: cons-image, cons-chart",
		"prod > other: other-image",
	}, names(e.RoutesFrom(g.Project("prod"))))
	assert.Equal(t, []string{"lined > cons: cons-lined"}, names(e.RoutesFrom(g.Project("lined"))))
	assert.Empty(t, e.RoutesFrom(g.Project("cons")), "a commit holding is not a route, and cons makes nothing")
}

func TestRoutesIntoAConsumerAreOneForEachProducer(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"prod > cons: cons-image, cons-chart",
		"lined > cons: cons-lined",
	}, names(e.RoutesInto(g.Project("cons"))))
	assert.Equal(t, []string{"prod > other: other-image"}, names(e.RoutesInto(g.Project("other"))))
	assert.Empty(t, e.RoutesInto(g.Project("prod")))
}

func TestARouteKnowsTheEditsThatFollowItsHoldingsOnce(t *testing.T) {
	e, g, err := buildEngine(t,
		d("Edit", "regen", "{holding: [cons-image, cons-chart]}", "  postEdit:\n    - {run: gen}\n"),
		d("Edit", "prep", "{holding: cons-chart}", "  preEdit:\n    - {run: prep}\n"),
	)
	require.NoError(t, err)

	route := e.RoutesFrom(g.Project("prod"))[0]

	var got []string
	for _, ed := range route.Edits() {
		got = append(got, ed.Name())
	}
	assert.Equal(t, []string{"regen", "prep"}, got)
}

func TestNewEngineConfiguresTheStrategyOfEachEdit(t *testing.T) {
	_, _, err := buildEngine(t, d("Edit", "bump", "{holding: cons-chart}", "  strategy: chartsPackageBump\n  with: {package: rancher-backup}\n"))

	require.NoError(t, err, "chartsPackageBump is named in the config and registered, though not built")
}

func TestNewEngineReportsEachMistakeOfAnEditNamingIt(t *testing.T) {
	e, g, err := buildEngine(t,
		d("Edit", "unknown", "{holding: cons-image}", "  strategy: nope\n"),
		d("Edit", "no-package", "{holding: cons-chart}", "  strategy: chartsPackageBump\n"),
		d("Edit", "stray", "{holding: other-image}", "  strategy: chartsPackageBump\n  with: {package: x, colour: red}\n"),
	)

	require.Error(t, err)
	text := err.Error()
	assert.Contains(t, text, `Edit "unknown": strategy "nope" is not known: use one of chartsPackageBump`)
	assert.Contains(t, text, `Edit "no-package"`)
	assert.Contains(t, text, "package")
	assert.Contains(t, text, `Edit "stray"`)
	assert.Contains(t, text, "colour")
	assert.Empty(t, e.RoutesFrom(g.Project("prod")), "a route whose edit cannot be built is left out")
	assert.NotEmpty(t, e.RoutesFrom(g.Project("lined")), "the others are fine")
}

func TestNewEngineReportsAReleaseHoldingInAFormatThatIsNotBuilt(t *testing.T) {
	e, g, err := buildEngine(t, d("Holding", "dockerfile", "{project: other, artifact: prod-image}", "  at: {file: Dockerfile, key: IMAGE, format: dockerfile}\n  holds: {kind: release}\n"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), `Holding "dockerfile"`)
	assert.Contains(t, err.Error(), "dockerfile")
	assert.Equal(t, []string{"prod > cons: cons-image, cons-chart"}, names(e.RoutesFrom(g.Project("prod"))), "the consumer whose holding cannot be written is left out")
}

func TestNewEngineAcceptsAGoModHolding(t *testing.T) {
	e, g, err := buildEngine(t, d("Holding", "gomod", "{project: other, artifact: prod-chart}", "  at: {file: go.mod, key: github.com/acme/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n"))

	require.NoError(t, err)
	assert.Contains(t, strings.Join(names(e.RoutesFrom(g.Project("prod"))), ";"), "gomod")
}

func TestNewEngineAcceptsARegexHolding(t *testing.T) {
	e, g, err := buildEngine(t, d("Holding", "rx", "{project: other, artifact: prod-chart}", "  at: {file: v.txt, key: 'v=(.*)', format: regex}\n  holds: {kind: release, form: \"{tag}\"}\n"))

	require.NoError(t, err)
	assert.Contains(t, strings.Join(names(e.RoutesFrom(g.Project("prod"))), ";"), "rx")
}

func TestNewEngineAcceptsAHoldingWithSeveralLocationsInABuiltFormat(t *testing.T) {
	e, g, err := buildEngine(t, d("Holding", "two", "{project: other, artifact: prod-chart}", "  at:\n    - {file: a.yaml, key: .url}\n    - {file: b/a.yaml, key: .url}\n  holds: {kind: release, form: \"{url}\"}\n"))

	require.NoError(t, err)
	assert.Contains(t, strings.Join(names(e.RoutesFrom(g.Project("prod"))), ";"), "two")
}

func TestNewEngineReportsALocationInAFormatNotBuiltAmongSeveral(t *testing.T) {
	e, g, err := buildEngine(t, d("Holding", "two", "{project: other, artifact: prod-chart}", "  at:\n    - {file: a.yaml, key: .url}\n    - {file: Dockerfile, key: U, format: dockerfile}\n  holds: {kind: release, form: \"{url}\"}\n"))

	require.ErrorIs(t, err, ErrFormatNotBuilt)
	assert.Contains(t, err.Error(), `Holding "two"`)
	assert.NotContains(t, strings.Join(names(e.RoutesFrom(g.Project("prod"))), ";"), "two")
}

func branches(ts []Target) []string {
	var out []string
	for _, t := range ts {
		var hs []string
		for _, h := range t.Holdings {
			hs = append(hs, h.Name())
		}
		out = append(out, t.Branch+" ("+t.Line+"): "+strings.Join(hs, ", "))
	}
	return out
}

func TestTargetsForAUniversalProducerAreTheBranchesOfEachStage(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]

	got, err := route.TargetsFor("")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"dev-v2.16 (v2.16): cons-image, cons-chart",
		"dev-v2.15 (v2.15): cons-image, cons-chart",
	}, branches(got))
}

func TestTargetsForAProducerWithLinesAreTheBranchOfTheLineAndStage(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("lined"))[0]

	got, err := route.TargetsFor("v2.15")

	require.NoError(t, err)
	assert.Equal(t, []string{"rel-v2.15 (v2.15): cons-lined"}, branches(got))

	_, err = route.TargetsFor("")
	require.ErrorIs(t, err, ErrNoBranches)
	assert.Contains(t, err.Error(), "none of the release lines")

	_, err = route.TargetsFor("v2.99")
	require.ErrorIs(t, err, ErrNoBranches)
}

func TestTargetsForASingleBranchConsumerIsItsDefaultBranch(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[1] // to other, which has one branch

	got, err := route.TargetsFor("")

	require.NoError(t, err)
	assert.Equal(t, []string{"main (): other-image"}, branches(got))
}

func TestTargetsForHoldingsOfOneBranchAreOneTarget(t *testing.T) {
	e, g, err := buildEngine(t, d("Holding", "cons-extra", "{project: cons, artifact: lined-image}", "  at: {file: x.yaml, key: img}\n  holds: {kind: release, stage: stable}\n"))
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("lined"))[0]

	got, err := route.TargetsFor("v2.16")

	require.NoError(t, err)
	assert.Equal(t, []string{"rel-v2.16 (v2.16): cons-lined, cons-extra"}, branches(got))
}

func TestTargetsLeaveOutALineAHoldingDoesNotApplyTo(t *testing.T) {
	e, g, err := buildEngine(t, d("Holding", "late", "{project: cons, artifact: lined-image}", "  at: {file: x.yaml, key: img}\n  holds: {kind: release, stage: dev, lines: {since: v2.16}}\n"))
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("lined"))[0]

	got, err := route.TargetsFor("v2.15")

	require.NoError(t, err)
	assert.Equal(t, []string{"rel-v2.15 (v2.15): cons-lined"}, branches(got), "late is since v2.16")
}

type recorder struct {
	steps []string
	fail  string
	order *[]string
}

func (r *recorder) Run(_ context.Context, step chain.Step, env StepEnv) error {
	cmd := strings.Join(step.Run, " ")
	r.steps = append(r.steps, cmd+" "+env.Env["GERO_TAG"]+" "+env.Env["GERO_TARGET_BRANCH"])
	*r.order = append(*r.order, "run "+cmd)
	if cmd == r.fail {
		return errors.New("exit 1")
	}
	return nil
}

const consFiles = "image: acme/prod:v1.0.0\nlined: acme/lined:v2.0.0\nurl: https://x/y\n"

func TestStepsOfATargetAreTheEditsStepsForItsLine(t *testing.T) {
	e, g, err := buildEngine(t,
		d("Edit", "regen", "{holding: [cons-image, cons-chart]}", "  preEdit:\n    - {run: pre-all}\n    - {run: pre-new, since: v2.16}\n  postEdit:\n    - {run: gen}\n    - {run: old-only, until: v2.15}\n"),
		d("Edit", "second", "{holding: cons-chart}", "  postEdit:\n    - {run: second-post}\n"),
	)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]

	pre, post := route.Steps(Target{Branch: "dev-v2.16", Line: "v2.16"})

	require.Len(t, pre, 2)
	assert.Equal(t, StepRef{Phase: PreEdit, Edit: "regen", Index: 0, Step: chain.Step{Run: v1alpha2.Command{"pre-all"}}}, pre[0])
	assert.Equal(t, "pre-new", pre[1].Step.Run[0])
	require.Len(t, post, 2)
	assert.Equal(t, []string{"gen", "second-post"}, []string{post[0].Step.Run[0], post[1].Step.Run[0]})

	_, post = route.Steps(Target{Branch: "dev-v2.15", Line: "v2.15"})
	require.Len(t, post, 3)
	assert.Equal(t, "old-only", post[1].Step.Run[0])
	assert.Equal(t, 1, post[1].Index)
}

func TestRunWritesAroundTheStepsAndCheckpointsEachUnit(t *testing.T) {
	e, g, err := buildEngine(t,
		d("Edit", "regen", "{holding: [cons-image, cons-chart]}", "  preEdit:\n    - {run: prep}\n  postEdit:\n    - {run: gen}\n"),
	)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]
	tree := files{"build.yaml": consFiles, "pkg.yaml": "url: https://x/y\n"}
	var order []string
	rec := &recorder{order: &order}
	target := Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: route.Holdings}

	res, err := route.Run(bg, tree, rec, target, "v1.2.0", "abc", "/work", func(_ context.Context, u Unit) error {
		order = append(order, fmt.Sprintf("unit %s %d", u.Kind, len(u.Changes)))
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"run prep", "unit step 0", "unit edit 2", "run gen", "unit step 0"}, order)
	assert.True(t, res.Changed)
	assert.Len(t, res.Changes, 2)
	assert.Contains(t, tree["build.yaml"], "image: acme/prod:v1.2.0")
	assert.Equal(t, "url: https://github.com/acme/prod/releases/download/v1.2.0/prod-1.2.0.tgz\n", tree["pkg.yaml"])
	assert.Equal(t, []string{"prep v1.2.0 dev-v2.16", "gen v1.2.0 dev-v2.16"}, rec.steps)
}

// envRunner records the environment each step it runs is given.
type envRunner struct{ envs []map[string]string }

func (r *envRunner) Run(_ context.Context, _ chain.Step, env StepEnv) error {
	r.envs = append(r.envs, env.Env)
	return nil
}

func TestRunGivesAStepTheVersionWithoutThePrefixOfTheStream(t *testing.T) {
	tests := []struct {
		name, pattern, tag, wantVersion string
	}{
		{"a prefixed tag", "mod/v*", "mod/v1.2.0", "1.2.0"},
		{"a tag with only a v", "v*", "v1.2.0", "1.2.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, g, err := buildEngine(t,
				d("Project", "mod", "", "  repo: {name: acme/mod, owned: true}\n"),
				d("Stream", "mod", "{project: mod}", "  tagPattern: \""+tt.pattern+"\"\n  universal: true\n"),
				d("Artifact", "mod-image", "{stream: mod}", "  kind: image\n  name: acme/mod\n"),
				d("Holding", "cons-mod", "{project: cons, artifact: mod-image}", "  at: {file: mod.yaml, key: image}\n  holds: {kind: release, form: \"{image}:{version}\"}\n"),
				d("Edit", "regen", "{holding: cons-mod}", "  postEdit:\n    - {run: gen}\n"),
			)
			require.NoError(t, err)
			route := e.RoutesFrom(g.Project("mod"))[0]
			rec := &envRunner{}
			tree := files{"mod.yaml": "image: acme/mod:0.0.1\n"}

			_, err = route.Run(bg, tree, rec, Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: route.Holdings}, tt.tag, "abc", "/work", nil)

			require.NoError(t, err)
			require.Len(t, rec.envs, 1)
			assert.Equal(t, tt.wantVersion, rec.envs[0]["GERO_VERSION"])
			assert.Equal(t, tt.tag, rec.envs[0]["GERO_TAG"], "the tag is as published")
			assert.Contains(t, tree["mod.yaml"], "acme/mod:"+tt.wantVersion, "the holding's form already has the same version")
		})
	}
}

func TestRunAFailedStepStopsItNamingWhereItWas(t *testing.T) {
	e, g, err := buildEngine(t, d("Edit", "regen", "{holding: cons-image}", "  postEdit:\n    - {run: ok}\n    - {run: boom, from: producer}\n    - {run: never}\n"))
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]
	var order []string
	rec := &recorder{order: &order, fail: "boom"}
	tree := files{"build.yaml": consFiles, "pkg.yaml": "url: x\n"}

	res, err := route.Run(bg, tree, rec, Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: route.Holdings[:1]}, "v1.2.0", "abc", "/work", nil)

	var stepErr *StepError
	require.ErrorAs(t, err, &stepErr)
	assert.Equal(t, PostEdit, stepErr.Phase)
	assert.Equal(t, "regen", stepErr.Edit)
	assert.Equal(t, 1, stepErr.Index)
	assert.Equal(t, []string{"boom"}, stepErr.Run)
	assert.Contains(t, err.Error(), "postEdit[1]")
	assert.Contains(t, err.Error(), "regen")
	assert.True(t, res.Changed, "the write was made before the failing step")
	assert.Equal(t, []string{"run ok", "run boom"}, order)
}

func TestACheckpointErrorStopsTheRun(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[1]
	stop := errors.New("stop")

	_, err = route.Run(bg, files{"other.yaml": "image: a:v1.0.0\n"}, &recorder{order: new([]string)}, Target{Branch: "main", Holdings: route.Holdings}, "v1.2.0", "abc", "/w",
		func(context.Context, Unit) error { return stop })

	assert.ErrorIs(t, err, stop)
}

func TestCheckSaysWhatEachHoldingOfTheTargetHolds(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]

	got, err := route.Check(bg, files{"build.yaml": consFiles, "pkg.yaml": "url: https://github.com/acme/prod/releases/download/v1.2.0/prod-1.2.0.tgz\n"},
		Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: route.Holdings}, "v1.2.0", "abc")

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "cons-image", got[0].Holding.Name())
	assert.False(t, got[0].UpToDate)
	assert.Equal(t, RelationNewer, got[0].Relation)
	assert.Equal(t, "cons-chart", got[1].Holding.Name())
	assert.True(t, got[1].UpToDate)

	_, err = route.Check(bg, files{"build.yaml": consFiles}, Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: route.Holdings}, "v1.2.0", "abc")
	require.ErrorContains(t, err, "cons-chart", "an unreadable holding names itself")
}

func TestAnUnbuiltStrategyStopsARunBeforeAnythingIsWritten(t *testing.T) {
	e, g, err := buildEngine(t, d("Edit", "bump", "{holding: cons-chart}", "  strategy: chartsPackageBump\n  with: {package: rancher-backup}\n"))
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]
	tree := files{"build.yaml": consFiles, "pkg.yaml": "url: x\n"}
	var order []string

	require.ErrorIs(t, route.Ready(), ErrNotBuilt)
	assert.ErrorContains(t, route.Ready(), "chartsPackageBump")
	_, err = route.Run(bg, tree, &recorder{order: &order}, Target{Branch: "dev-v2.16", Line: "v2.16", Holdings: route.Holdings}, "v1.2.0", "abc", "/w", nil)

	require.ErrorIs(t, err, ErrNotBuilt)
	assert.Equal(t, consFiles, tree["build.yaml"], "nothing was written")
	assert.Empty(t, order)
}

func TestAReadyRouteHasNoError(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)

	assert.NoError(t, e.RoutesFrom(g.Project("prod"))[0].Ready())
}

type namedStrategy struct{ name string }

func (s namedStrategy) Name() string { return s.name }
func (s namedStrategy) Configure(map[string]string) (Applier, error) {
	return nil, nil
}

func TestTheRegistryFindsStrategiesByName(t *testing.T) {
	r := NewStrategyRegistry()
	r.Register(namedStrategy{"b"})
	r.Register(namedStrategy{"a"})

	got, err := r.Get("a")
	require.NoError(t, err)
	assert.Equal(t, "a", got.Name())
	assert.Equal(t, []string{"a", "b"}, r.Names())
	_, err = r.Get("c")
	assert.ErrorContains(t, err, `strategy "c" is not known: use one of a, b`)
	assert.Panics(t, func() { r.Register(namedStrategy{"a"}) })
	assert.Equal(t, []string{"chartsPackageBump", "goBump"}, Builtin().Names())
}

func TestTargetsOnALineOfTheConsumerAreItsBranchesForEachStage(t *testing.T) {
	e, g, err := buildEngine(t)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("prod"))[0]

	got, err := route.TargetsOn("v2.15")

	require.NoError(t, err)
	assert.Equal(t, []string{"dev-v2.15 (v2.15): cons-image, cons-chart"}, branches(got))

	got, err = e.RoutesFrom(g.Project("lined"))[0].TargetsOn("2.16")
	require.NoError(t, err)
	assert.Equal(t, []string{"rel-v2.16 (v2.16): cons-lined"}, branches(got), "a line without its v")

	_, err = route.TargetsOn("v2.99")
	require.ErrorIs(t, err, ErrNoBranches)
	assert.Contains(t, err.Error(), "v2.99")
}

func TestTargetsOnALineLeavesOutAHoldingThatDoesNotApplyToIt(t *testing.T) {
	e, g, err := buildEngine(t, d("Holding", "late", "{project: cons, artifact: lined-image}", "  at: {file: x.yaml, key: img}\n  holds: {kind: release, stage: dev, lines: {since: v2.16}}\n"))
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("lined"))[0]

	got, err := route.TargetsOn("v2.15")
	require.NoError(t, err)
	assert.Equal(t, []string{"rel-v2.15 (v2.15): cons-lined"}, branches(got))

	got, err = route.TargetsOn("v2.16")
	require.NoError(t, err)
	assert.Equal(t, []string{"rel-v2.16 (v2.16): cons-lined", "dev-v2.16 (v2.16): late"}, branches(got))
}
