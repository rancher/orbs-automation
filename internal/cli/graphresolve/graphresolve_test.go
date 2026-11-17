package graphresolve

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
)

type fakeWorkspace struct {
	g   *chain.Graph
	err error
}

func (f fakeWorkspace) Chain() (*chain.Graph, error) { return f.g, f.err }

// all is every project drawn, as a config with no external one would be.
var all = Options{External: true}

func project(name, spec string) string { return chaintest.Doc("Project", name, "", spec) }

// shipping is a config where shell produces an image that rancher, the final
// destination, holds, and scc-operator is a shared project outside any flow.
func shipping(t *testing.T) *chain.Graph {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Lines("v2.13", "v2.12"),
		project("rancher", "  repo: {name: rancher/rancher, managedBy: rancher}\n  finalDestination: true\n"),
		project("shell", "  repo: {name: rancher/shell, managedBy: rancher}\n"),
		project("scc", "  repo: {name: rancher/scc-operator, managedBy: shared}\n"),
		project("upstream", "  repo: {name: kubernetes/upstream, external: true}\n"),
		chaintest.Doc("Stream", "shell", "{project: shell}", "  universal: true\n"),
		chaintest.Doc("Artifact", "shell-image", "{stream: shell}", "  kind: image\n  name: rancher/shell\n"),
		chaintest.Doc("Holding", "rancher-holds-shell", "{project: rancher, artifact: shell-image}",
			"  at: {file: build.yaml, key: shell}\n  holds: {kind: release, form: \"{image}:{tag}\", accept: all}\n"),
	)
}

func TestGraphDrawsEachProjectAsANode(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)}, all)
	require.NoError(t, err)
	for _, slug := range []string{"rancher/rancher", "rancher/shell", "rancher/scc-operator", "kubernetes/upstream"} {
		assert.Contains(t, out, `"`+slug+`": {`+"\n", slug)
	}
}

func TestGraphFlowsFromProducerThroughHoldingToHolder(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)}, all)
	require.NoError(t, err)
	assert.Contains(t, out, `"rancher/shell"."stream:shell"."artifact:shell-image" -> "holding:rancher-holds-shell"`)
	assert.Contains(t, out, `"holding:rancher-holds-shell" -> "rancher/rancher"`)
	assert.NotContains(t, out, `-> "rancher/shell"`, "the flow runs one way")
}

func TestGraphMarksStewardshipAndTheFinalDestination(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)}, all)
	require.NoError(t, err)
	assert.Contains(t, out, "\"rancher/shell\": {\n  class: [managed]")
	assert.Contains(t, out, "\"rancher/rancher\": {\n  class: [managed; hub]")
	assert.Contains(t, out, "\"rancher/scc-operator\": {\n  class: [shared]")
	assert.Contains(t, out, "\"kubernetes/upstream\": {\n  class: [external]")
}

func TestGraphLabelsTheHolderEdgeWithTheLinesTheHoldingServes(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)}, all)
	require.NoError(t, err)
	assert.Contains(t, out, `"holding:rancher-holds-shell" -> "rancher/rancher": "v2.13, v2.12"`)
}

func TestGraphIsTheSameEachTime(t *testing.T) {
	g := shipping(t)
	first, err := graph(fakeWorkspace{g: g}, all)
	require.NoError(t, err)
	second, err := graph(fakeWorkspace{g: g}, all)
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestGraphReturnsTheWorkspaceError(t *testing.T) {
	boom := errors.New("no config")
	_, err := graph(fakeWorkspace{err: boom}, all)
	assert.ErrorIs(t, err, boom)
}

func TestGraphAsksForTheELKLayout(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)}, all)
	require.NoError(t, err)
	assert.Contains(t, out, "layout-engine: elk")
}

func TestGraphNestsTheStreamsAndArtifactsInTheirProject(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)}, all)
	require.NoError(t, err)
	shell := out[strings.Index(out, `"rancher/shell": {`):]
	shell = shell[:strings.Index(shell, "\n}\n")]
	assert.Contains(t, shell, `"stream:shell": {`)
	assert.Contains(t, shell, `"artifact:shell-image": {class: [artifact]; label: "shell-image\nrancher/shell"}`)
	assert.NotContains(t, out[:strings.Index(out, `"rancher/shell": {`)], "stream:shell")
}

// twoSeries is a producer with a stream for each of two series whose artifacts
// publish under the same name, as a Go module does, held by one project twice.
func twoSeries(t *testing.T) string {
	t.Helper()
	g := chaintest.Load(t,
		project("lib", "  repo: {name: rancher/lib, managedBy: rancher}\n"),
		project("app", "  repo: {name: rancher/app, managedBy: rancher}\n"),
		chaintest.Doc("Stream", "lib-v1-1", "{project: lib}", "  tagPattern: \"v1.1.*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "lib-v1-1-module", "{stream: lib-v1-1}", "  kind: gomod\n  name: github.com/rancher/lib\n"),
		chaintest.Doc("Stream", "lib-v1-2", "{project: lib}", "  tagPattern: \"v1.2.*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "lib-v1-2-module", "{stream: lib-v1-2}", "  kind: gomod\n  name: github.com/rancher/lib\n"),
		chaintest.Doc("Holding", "app-holds-lib-1-1", "{project: app, artifact: lib-v1-1-module}",
			"  at: {file: go.mod, key: github.com/rancher/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all}\n"),
		chaintest.Doc("Holding", "app-holds-lib-1-2", "{project: app, artifact: lib-v1-2-module}",
			"  at: {file: go.mod, key: github.com/rancher/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all}\n"),
	)
	out, err := graph(fakeWorkspace{g: g}, all)
	require.NoError(t, err)
	return out
}

func TestGraphLabelsEachNodeApartFromOthersOfTheSameArtifactName(t *testing.T) {
	out := twoSeries(t)
	labels := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, "label: "); i >= 0 && (strings.Contains(line, "artifact") || strings.Contains(line, "holding") || strings.Contains(line, "stream")) {
			label := line[i:]
			assert.False(t, labels[label], "two nodes are labelled %s", label)
			labels[label] = true
		}
	}
	assert.Contains(t, out, `label: "stream lib-v1-1\nv1.1.*"`)
	assert.Contains(t, out, `label: "lib-v1-2-module\ngithub.com/rancher/lib"`)
	assert.Contains(t, out, `label: "app-holds-lib-1-1\nrelease"`)
}

// withEdits is a project that holds two releases of one producer, with an edit
// that follows both and a strategy edit that follows one.
func withEdits(t *testing.T) string {
	t.Helper()
	hold := func(name, artifact string) string {
		return chaintest.Doc("Holding", name, "{project: app, artifact: "+artifact+"}",
			"  at: {file: go.mod, key: github.com/rancher/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all}\n")
	}
	g := chaintest.Load(t,
		project("lib", "  repo: {name: rancher/lib, managedBy: rancher}\n"),
		project("app", "  repo: {name: rancher/app, managedBy: rancher}\n"),
		chaintest.Doc("Stream", "lib", "{project: lib}", "  universal: true\n"),
		chaintest.Doc("Artifact", "lib-module", "{stream: lib}", "  kind: gomod\n  name: github.com/rancher/lib\n"),
		chaintest.Doc("Artifact", "lib-image", "{stream: lib}", "  kind: image\n  name: rancher/lib\n"),
		hold("app-holds-lib-module", "lib-module"),
		hold("app-holds-lib-image", "lib-image"),
		chaintest.Doc("Edit", "app-steps", "{holding: [app-holds-lib-module, app-holds-lib-image]}",
			"  preEdit:\n    - {run: scripts/prepare}\n  postEdit:\n    - {run: scripts/generate}\n    - {run: scripts/tidy}\n"),
		chaintest.Doc("Edit", "app-bump", "{holding: [app-holds-lib-module]}",
			"  strategy: chartsPackageBump\n  with: {package: lib}\n"),
	)
	out, err := graph(fakeWorkspace{g: g}, all)
	require.NoError(t, err)
	return out
}

func TestGraphDrawsAnEditOnceWithAnEdgeFromEachHoldingItFollows(t *testing.T) {
	out := withEdits(t)
	assert.Equal(t, 1, strings.Count(out, `"edit:app-steps": {class:`))
	assert.Contains(t, out, `"holding:app-holds-lib-module" -> "edit:app-steps"`)
	assert.Contains(t, out, `"holding:app-holds-lib-image" -> "edit:app-steps"`)
	assert.Contains(t, out, `"holding:app-holds-lib-module" -> "edit:app-bump"`)
	assert.NotContains(t, out, `"holding:app-holds-lib-image" -> "edit:app-bump"`)
}

func TestGraphLabelsAnEditWithItsStrategyAndStepCounts(t *testing.T) {
	out := withEdits(t)
	assert.Contains(t, out, `"edit:app-steps": {class: [edit]; label: "app-steps\npre 1 step, post 2 steps"}`)
	assert.Contains(t, out, `"edit:app-bump": {class: [edit]; label: "app-bump\nstrategy chartsPackageBump"}`)
}

func TestRunWritesD2ForTheTextFormat(t *testing.T) {
	out, err := run(fakeWorkspace{g: shipping(t)}, clicfg.OutputText, KindD2, all)
	require.NoError(t, err)
	assert.Contains(t, out, "classes: {")
}

func TestRunRefusesTheOtherFormatsAndWritesNothing(t *testing.T) {
	for _, format := range []string{clicfg.OutputJSON, clicfg.OutputEnv} {
		out, err := run(fakeWorkspace{g: shipping(t)}, format, KindD2, all)
		require.Error(t, err, format)
		assert.Contains(t, err.Error(), "config resolve graph", format)
		assert.Contains(t, err.Error(), "D2", format)
		assert.Empty(t, out, format)
	}
}

func TestGraphFlowsDownTheDiagram(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)}, all)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "direction: down\n"))
}

// pinned is wrangler, whose Go module pins lasso's, and rancher, the final
// destination, which holds wrangler and so lasso, though it says nothing of lasso.
func pinned(t *testing.T) *chain.Graph {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		project("rancher", "  repo: {name: rancher/rancher, managedBy: rancher}\n  finalDestination: true\n"),
		project("wrangler", "  repo: {name: rancher/wrangler, managedBy: frameworks}\n"),
		project("lasso", "  repo: {name: rancher/lasso, managedBy: frameworks}\n"),
		chaintest.Doc("Stream", "wrangler", "{project: wrangler}", "  universal: true\n"),
		chaintest.Doc("Stream", "lasso", "{project: lasso}", "  universal: true\n"),
		chaintest.Doc("Artifact", "lasso-module", "{stream: lasso}", "  kind: gomod\n  name: github.com/rancher/lasso\n"),
		chaintest.Doc("Artifact", "wrangler-module", "{stream: wrangler, pins: lasso-module}", "  kind: gomod\n  name: github.com/rancher/wrangler/v3\n"),
		chaintest.Doc("Holding", "rancher-holds-wrangler", "{project: rancher, artifact: wrangler-module}",
			"  at: {file: go.mod, key: github.com/rancher/wrangler/v3, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: {since: v2.15}}\n"),
		chaintest.Doc("Edit", "tidy", "{holding: rancher-holds-wrangler}", "  strategy: goBump\n"),
	)
}

func TestGraphDrawsWhatAPinMakesAsAHoldingToo(t *testing.T) {
	out, err := graph(fakeWorkspace{g: pinned(t)}, all)
	require.NoError(t, err)

	assert.Contains(t, out, `"holding:rancher-holds-wrangler+lasso-module": {class: [holding]; label: "rancher-holds-wrangler+lasso-module\nrelease, pinned"}`)
	assert.Contains(t, out, `"rancher/lasso"."stream:lasso"."artifact:lasso-module" -> "holding:rancher-holds-wrangler+lasso-module"`)
	assert.Contains(t, out, `"holding:rancher-holds-wrangler+lasso-module" -> "rancher/rancher": "v2.16, v2.15"`, "lasso reaches rancher, for the lines the wrangler holding is for")
}

func TestGraphDrawsAnEditOnceWhenItFollowsAHoldingAndWhatItsPinMakes(t *testing.T) {
	out, err := graph(fakeWorkspace{g: pinned(t)}, all)
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(out, `"edit:tidy": {class:`), "the edit is one node")
	assert.Contains(t, out, `"holding:rancher-holds-wrangler" -> "edit:tidy"`)
	assert.Contains(t, out, `"holding:rancher-holds-wrangler+lasso-module" -> "edit:tidy"`)
}

func mindmapOf(t *testing.T, g *chain.Graph) string {
	t.Helper()
	out, err := run(fakeWorkspace{g: g}, clicfg.OutputText, KindMindmap, all)
	require.NoError(t, err)
	return out
}

func regexpQuote(s string) string { return strings.NewReplacer("(", `\(`, ")", `\)`).Replace(s) }

func TestMindmapStartsAsAMermaidMindmapWithOneRoot(t *testing.T) {
	out := mindmapOf(t, shipping(t))
	assert.True(t, strings.HasPrefix(out, "mindmap\n  root((\""), out)
	assert.Equal(t, 1, strings.Count(out, "\n  root(("), "one root")
}

func TestMindmapHangsEachProjectOffTheRoot(t *testing.T) {
	out := mindmapOf(t, shipping(t))
	for _, text := range []string{
		"rancher (managed by rancher, hub)", "shell (managed by rancher)", "scc (shared)", "upstream (external)",
	} {
		assert.Regexp(t, `\n    n\d+\["`+regexpQuote(text)+`"\]\n`, out, text)
	}
}

func TestMindmapNestsStreamsAndArtifactsInTheirProject(t *testing.T) {
	out := mindmapOf(t, shipping(t))
	assert.Regexp(t, `\n    n\d+\["shell \(managed by rancher\)"\]\n      n\d+\["stream shell v\*"\]\n        n\d+\["artifact shell-image: rancher/shell"\]\n`, out)
}

func TestMindmapShowsWhereAnArtifactReachesAndWhatAProjectHolds(t *testing.T) {
	out := mindmapOf(t, shipping(t))
	assert.Contains(t, out, `["to rancher via rancher-holds-shell (v2.13, v2.12)"]`)
	assert.Regexp(t, `\n      n\d+\["holds"\]\n        n\d+\["rancher-holds-shell: release from shell"\]\n`, out)
}

func TestMindmapDrawsWhatAPinMakes(t *testing.T) {
	out := mindmapOf(t, pinned(t))
	assert.Contains(t, out, "release, pinned")
}

func TestMindmapKeepsQuotesOutOfLabels(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.13"),
		project("odd", "  repo: {name: rancher/odd, managedBy: rancher}\n"),
	)
	assert.NotContains(t, strings.ReplaceAll(mindmapOf(t, g), `["`, ""), `""`)
}

func TestMindmapIsTheSameEachTime(t *testing.T) {
	g := shipping(t)
	assert.Equal(t, mindmapOf(t, g), mindmapOf(t, g))
}

func TestMindmapReturnsTheWorkspaceError(t *testing.T) {
	_, err := run(fakeWorkspace{err: errors.New("boom")}, clicfg.OutputText, KindMindmap, all)
	assert.EqualError(t, err, "boom")
}

func TestRunRefusesAnUnknownKind(t *testing.T) {
	out, err := run(fakeWorkspace{g: shipping(t)}, clicfg.OutputText, Kind("pie"), all)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"pie"`)
	assert.Empty(t, out)
}

// filtered is app, which holds lib and k3s (external), other, which holds k3s, and
// an edit on each of app's holdings of lib and other's of k3s.
func filtered(t *testing.T) *chain.Graph {
	t.Helper()
	hold := func(name, holder, artifact string) string {
		return chaintest.Doc("Holding", name, "{project: "+holder+", artifact: "+artifact+"}",
			"  at: {file: build.yaml, key: "+artifact+"}\n  holds: {kind: release, form: \"{image}:{tag}\", accept: all}\n")
	}
	return chaintest.Load(t,
		chaintest.Lines("v2.13"),
		project("app", "  repo: {name: acme/app, managedBy: rancher}\n"),
		project("lib", "  repo: {name: acme/lib, managedBy: rancher}\n  aliases: [library]\n"),
		project("other", "  repo: {name: acme/other, managedBy: shared}\n"),
		project("k3s", "  repo: {name: k3s-io/k3s, external: true}\n  aliases: [vanilla]\n"),
		chaintest.Doc("Stream", "lib", "{project: lib}", "  universal: true\n"),
		chaintest.Doc("Stream", "k3s", "{project: k3s}", "  universal: true\n"),
		chaintest.Doc("Artifact", "lib-image", "{stream: lib}", "  kind: image\n  name: acme/lib\n"),
		chaintest.Doc("Artifact", "k3s-image", "{stream: k3s}", "  kind: image\n  name: rancher/k3s\n"),
		hold("app-holds-lib", "app", "lib-image"),
		hold("app-holds-k3s", "app", "k3s-image"),
		hold("other-holds-k3s", "other", "k3s-image"),
		chaintest.Doc("Edit", "lib-steps", "{holding: app-holds-lib}", "  preEdit:\n    - {run: scripts/prepare}\n"),
		chaintest.Doc("Edit", "k3s-steps", "{holding: other-holds-k3s}", "  preEdit:\n    - {run: scripts/prepare}\n"),
	)
}

func drawn(t *testing.T, opts Options) string {
	t.Helper()
	out, err := graph(fakeWorkspace{g: filtered(t)}, opts)
	require.NoError(t, err)
	return out
}

func TestGraphLeavesOutExternalProjectsButDrawsThemAsABoundaryWhereOneIsHeld(t *testing.T) {
	out := drawn(t, Options{})

	assert.Contains(t, out, `"k3s-io/k3s": {class: [boundary]; label: "k3s"}`)
	assert.NotContains(t, out, `"stream:k3s"`, "a boundary has nothing inside it")
	assert.Contains(t, out, `"k3s-io/k3s" -> "holding:app-holds-k3s"`, "the flow starts at the boundary itself")
	assert.Contains(t, out, `"holding:app-holds-k3s" -> "acme/app"`)
	assert.Contains(t, out, "\"acme/app\": {\n  class: [managed]")
}

func TestGraphWithExternalDrawsThemInFull(t *testing.T) {
	out := drawn(t, Options{External: true})

	assert.Contains(t, out, "\"k3s-io/k3s\": {\n  class: [external]")
	assert.Contains(t, out, `"k3s-io/k3s"."stream:k3s"."artifact:k3s-image" -> "holding:app-holds-k3s"`)
	assert.NotContains(t, out, "[boundary]")
}

func TestGraphProjectDrawsTheNamedProjectsAndTheBoundaryOfWhatTheyTouch(t *testing.T) {
	out := drawn(t, Options{Projects: []string{"app"}})

	assert.Contains(t, out, "\"acme/app\": {\n  class: [managed]")
	assert.Contains(t, out, `"acme/lib": {class: [boundary]; label: "lib"}`)
	assert.Contains(t, out, `"k3s-io/k3s": {class: [boundary]; label: "k3s"}`)
	assert.NotContains(t, out, `"acme/other"`, "it touches nothing that is drawn")
	assert.Contains(t, out, `"holding:app-holds-lib"`)
	assert.Contains(t, out, `"holding:app-holds-k3s"`)
	assert.NotContains(t, out, `"holding:other-holds-k3s"`, "both its ends are left out")
}

func TestGraphProjectMatchesNameSlugAliasAndGlobWhateverTheCase(t *testing.T) {
	for _, pattern := range []string{"lib", "ACME/lib", "library", "Li*", "acme/li?", "acme/lib*"} {
		out := drawn(t, Options{Projects: []string{pattern}})
		assert.Contains(t, out, "\"acme/lib\": {\n  class: [managed]", pattern)
		assert.Contains(t, out, `"acme/app": {class: [boundary]`, pattern)
	}
	out := drawn(t, Options{Projects: []string{"acme/*"}})
	for _, slug := range []string{"acme/app", "acme/lib", "acme/other"} {
		assert.Contains(t, out, `"`+slug+"\": {\n  class:", slug)
	}
	assert.Contains(t, out, `"k3s-io/k3s": {class: [boundary]`)
}

func TestGraphProjectNamingAnExternalProjectDrawsItWithoutExternal(t *testing.T) {
	for _, pattern := range []string{"k3s", "vanilla", "k3s-io/k3s"} {
		out := drawn(t, Options{Projects: []string{pattern}})
		assert.Contains(t, out, "\"k3s-io/k3s\": {\n  class: [external]", pattern)
		assert.Contains(t, out, `"holding:app-holds-k3s"`, pattern)
	}
}

func TestGraphProjectBangLeavesOutWhatItMatches(t *testing.T) {
	out := drawn(t, Options{Projects: []string{"!acme/other"}})
	assert.NotContains(t, out, `"acme/other"`)
	assert.Contains(t, out, "\"acme/app\": {\n  class: [managed]")
	assert.Contains(t, out, `[boundary]`, "k3s is external, so it is the default's boundary")

	out = drawn(t, Options{External: true, Projects: []string{"!k3s"}})
	assert.NotContains(t, out, "class: [external]")
	assert.Contains(t, out, `"k3s-io/k3s": {class: [boundary]`)

	out = drawn(t, Options{Projects: []string{"acme/*", "!acme/other"}})
	assert.NotContains(t, out, `"acme/other"`)
	assert.Contains(t, out, "\"acme/lib\": {\n  class: [managed]")
}

func TestGraphDrawsAnEditOnlyWhenAHoldingItFollowsIsDrawn(t *testing.T) {
	out := drawn(t, Options{Projects: []string{"app"}})

	assert.Contains(t, out, `"holding:app-holds-lib" -> "edit:lib-steps"`)
	assert.NotContains(t, out, `"edit:k3s-steps"`)
}

func TestGraphRefusesAPatternThatMatchesNothingAndAFilterThatLeavesNothing(t *testing.T) {
	for _, opts := range []Options{
		{Projects: []string{"nope"}},
		{Projects: []string{"app", "!nope"}},
		{Projects: []string{"app", "!app"}},
		{Projects: []string{"[bad"}},
	} {
		out, err := graph(fakeWorkspace{g: filtered(t)}, opts)
		require.Error(t, err, "%v", opts.Projects)
		assert.Empty(t, out)
	}
	_, err := graph(fakeWorkspace{g: filtered(t)}, Options{Projects: []string{"nope"}})
	assert.Contains(t, err.Error(), `"nope"`)
}

func TestFiltersAreTheSameEachTime(t *testing.T) {
	opts := Options{Projects: []string{"app", "!k3s"}}
	assert.Equal(t, drawn(t, opts), drawn(t, opts))
}

func TestMindmapDrawsABranchForEachProjectDrawnAndNoBoundaryNode(t *testing.T) {
	out, err := mindmap(fakeWorkspace{g: filtered(t)}, Options{Projects: []string{"app"}})
	require.NoError(t, err)

	assert.Regexp(t, `\n    n\d+\["app \(managed by rancher\)"\]\n`, out)
	assert.NotRegexp(t, `\["lib \(`, out)
	assert.NotRegexp(t, `\["k3s `, out)
	assert.NotContains(t, out, "boundary")
	assert.Contains(t, out, `["app-holds-lib: release from lib"]`)
	assert.Contains(t, out, `["app-holds-k3s: release from k3s"]`)
	assert.NotContains(t, out, "other-holds-k3s")
}

func TestMindmapLeavesOutExternalProjectsUnlessAsked(t *testing.T) {
	out, err := mindmap(fakeWorkspace{g: filtered(t)}, Options{})
	require.NoError(t, err)
	assert.NotRegexp(t, `\["k3s `, out)

	out, err = mindmap(fakeWorkspace{g: filtered(t)}, Options{External: true})
	require.NoError(t, err)
	assert.Regexp(t, `\["k3s \(external\)"\]`, out)
}

func TestGraphManagedByKeepsTheProjectsATeamManagesAndTheBoundaryOfWhatTheyTouch(t *testing.T) {
	out := drawn(t, Options{ManagedBy: []string{"rancher"}})

	assert.Contains(t, out, "\"acme/app\": {\n  class: [managed]")
	assert.Contains(t, out, "\"acme/lib\": {\n  class: [managed]")
	assert.NotContains(t, out, "\"acme/other\": {\n  class", "it is shared")
	assert.Contains(t, out, `"k3s-io/k3s": {class: [boundary]`)
	assert.NotContains(t, out, `"holding:other-holds-k3s"`)
}

func TestGraphManagedByIsAnyOfTheNamesWhateverTheCaseAndSharedIsOne(t *testing.T) {
	out := drawn(t, Options{ManagedBy: []string{"RANCHER", "shared"}})
	for _, slug := range []string{"acme/app", "acme/lib", "acme/other"} {
		assert.Contains(t, out, `"`+slug+"\": {\n  class:", slug)
	}

	out = drawn(t, Options{ManagedBy: []string{"shared"}})
	assert.Contains(t, out, "\"acme/other\": {\n  class: [shared]")
	assert.NotContains(t, out, `"acme/app"`, "it touches nothing that is drawn")
}

func TestGraphManagedByNarrowsProjectAndNeverAddsToIt(t *testing.T) {
	out := drawn(t, Options{ManagedBy: []string{"rancher"}, Projects: []string{"acme/*"}})
	assert.Contains(t, out, "\"acme/app\": {\n  class: [managed]")
	assert.NotContains(t, out, "\"acme/other\": {\n  class", "it matches the pattern and not the team")

	_, err := graph(fakeWorkspace{g: filtered(t)}, Options{ManagedBy: []string{"rancher"}, Projects: []string{"k3s"}})
	assert.Error(t, err, "an external project has no team")
}

func TestGraphManagedByWithExternalStillKeepsNoExternalProject(t *testing.T) {
	out := drawn(t, Options{ManagedBy: []string{"rancher"}, External: true})
	assert.NotContains(t, out, "class: [external]")
	assert.Contains(t, out, `"k3s-io/k3s": {class: [boundary]`)
}

func TestGraphRefusesATeamNoProjectIsManagedBy(t *testing.T) {
	out, err := graph(fakeWorkspace{g: filtered(t)}, Options{ManagedBy: []string{"rancher", "nobody"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nobody"`)
	assert.Empty(t, out)
}

func TestMindmapKeepsOnlyTheBranchesOfTheTeam(t *testing.T) {
	out, err := mindmap(fakeWorkspace{g: filtered(t)}, Options{ManagedBy: []string{"shared"}})
	require.NoError(t, err)
	assert.Regexp(t, `\["other \(shared\)"\]`, out)
	assert.NotRegexp(t, `\["app `, out)
}

var day = time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)

// lined is rancher, with a branch per line, a series for each and an image that
// exists from v2.16; shell, which holds from v2.16 and lists no line; charts, with a
// branch for each stage of v2.16 and none yet for v2.15; and the lines v2.17 in
// development, v2.16 ga, v2.15 eol and v2.14 by its dates.
func lined(t *testing.T) *chain.Graph {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Doc("RancherLines", "rancher", "", `  lines:
    - {id: v2.17, phase: development}
    - {id: v2.16, phase: ga}
    - {id: v2.15, phase: eol}
    - {id: v2.14, dates: {ga: 2026-01-01, eom: 2026-06-01, eol: 2027-01-01}}
`),
		project("rancher", `  repo: {name: rancher/rancher, managedBy: shared, defaultBranch: main}
  branches:
    layout: perLine
    lines:
      - {line: v2.17}
      - {line: v2.16, branch: release/v2.16}
      - {line: v2.15, branch: release/v2.15}
`),
		project("shell", "  repo: {name: rancher/shell, managedBy: rancher}\n"),
		project("charts", `  repo: {name: rancher/charts, managedBy: rancher}
  branches:
    layout: perLine
    stages: [dev, stable]
    lines:
      - {line: v2.16, branch: {dev: dev-v2.16, stable: release-v2.16}}
      - {line: v2.15, branch: ""}
`),
		chaintest.Doc("Stream", "rancher", "{project: rancher}", "  lines:\n    - {line: v2.17, series: \"2.17.x\"}\n    - {line: v2.16, series: \"2.16.x\"}\n    - {line: v2.15, series: \"2.15.x\"}\n"),
		chaintest.Doc("Stream", "shell", "{project: shell}", "  universal: true\n"),
		chaintest.Doc("Artifact", "rancher-image", "{stream: rancher}", "  kind: image\n  name: rancher/rancher\n"),
		chaintest.Doc("Artifact", "rancher-server-image", "{stream: rancher}", "  kind: image\n  name: rancher/rancher-server\n  lines: {since: v2.16}\n"),
		chaintest.Doc("Artifact", "shell-image", "{stream: shell}", "  kind: image\n  name: rancher/shell\n"),
		chaintest.Doc("Holding", "rancher-holds-shell", "{project: rancher, artifact: shell-image}",
			"  at: {file: build.yaml, key: shell}\n  holds: {kind: release, form: \"{image}:{tag}\", accept: all, lines: {since: v2.16}}\n"),
	)
}

func linedOut(t *testing.T, opts Options) string {
	t.Helper()
	opts.Now = day
	out, err := graph(fakeWorkspace{g: lined(t)}, opts)
	require.NoError(t, err)
	return out
}

func projectBlock(out, slug string) string {
	block := out[strings.Index(out, `"`+slug+`": {`):]
	return block[:strings.Index(block, "\n}\n")]
}

func TestGraphDrawsTheLinesOfAProjectInsideIt(t *testing.T) {
	rancher := projectBlock(linedOut(t, Options{}), "rancher/rancher")

	assert.Contains(t, rancher, `"lines": {`)
	assert.Contains(t, rancher, `"line:v2.17": {class: [line; development]; label: "v2.17\nmain\n2.17.x"}`, "the default branch by its name")
	assert.Contains(t, rancher, `"line:v2.16": {class: [line; ga]; label: "v2.16\nrelease/v2.16\n2.16.x"}`)
	assert.Contains(t, rancher, `"line:v2.15": {class: [line; eol]; label: "v2.15\nrelease/v2.15\n2.15.x"}`)
	assert.Less(t, strings.Index(rancher, `"line:v2.17"`), strings.Index(rancher, `"line:v2.16"`), "newest first")
	assert.Less(t, strings.Index(rancher, `"line:v2.16"`), strings.Index(rancher, `"line:v2.15"`))
}

func TestGraphDrawsAStageOfALineAndALineWithNoBranchYet(t *testing.T) {
	charts := projectBlock(linedOut(t, Options{}), "rancher/charts")

	assert.Contains(t, charts, `"line:v2.16:dev": {class: [line; ga]; label: "v2.16 dev\ndev-v2.16"}`)
	assert.Contains(t, charts, `"line:v2.16:stable": {class: [line; ga]; label: "v2.16 stable\nrelease-v2.16"}`)
	assert.Contains(t, charts, `"line:v2.15": {class: [line; eol]; label: "v2.15\nno branch yet"}`)
	assert.Less(t, strings.Index(charts, `"line:v2.16:dev"`), strings.Index(charts, `"line:v2.16:stable"`), "the stages in order")
}

func TestGraphHasNoLinesContainerForAProjectThatListsNoLineOrForABoundary(t *testing.T) {
	out := linedOut(t, Options{})
	assert.NotContains(t, projectBlock(out, "rancher/shell"), `"lines"`)

	out = linedOut(t, Options{Projects: []string{"rancher"}})
	assert.Contains(t, out, `"rancher/shell": {class: [boundary]`)
	assert.NotContains(t, out, `"line:v2.16:dev"`)
}

func TestGraphReadsThePhaseOfALineByTheDayItRuns(t *testing.T) {
	lines := func(now time.Time) string {
		g := chaintest.Load(t,
			chaintest.Doc("RancherLines", "rancher", "", "  lines:\n    - {id: v2.14, dates: {ga: 2026-01-01, eom: 2026-06-01, eol: 2027-01-01}}\n"),
			project("old", "  repo: {name: rancher/old}\n  branches:\n    layout: perLine\n    lines:\n      - {line: v2.14, branch: r14}\n"),
		)
		out, err := graph(fakeWorkspace{g: g}, Options{Now: now})
		require.NoError(t, err)
		return out
	}
	assert.Contains(t, lines(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)), "class: [line; ga]")
	assert.Contains(t, lines(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)), "class: [line; eom]")
	assert.Contains(t, lines(time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)), "class: [line; eol]")
}

func TestGraphLinesAreTheSameOnTheSameDay(t *testing.T) {
	assert.Equal(t, linedOut(t, Options{}), linedOut(t, Options{}))
}

func TestGraphLineDrawsOnlyThatLine(t *testing.T) {
	for _, line := range []string{"v2.16", "2.16", "V2.16"} {
		out := linedOut(t, Options{Line: line})
		rancher := projectBlock(out, "rancher/rancher")
		assert.Contains(t, rancher, `"line:v2.16"`, line)
		assert.NotContains(t, rancher, `"line:v2.17"`, line)
		assert.NotContains(t, rancher, `"line:v2.15"`, line)
		assert.Contains(t, rancher, `"artifact:rancher-server-image"`, line)
		assert.Contains(t, out, `"holding:rancher-holds-shell"`, line)
		assert.Contains(t, projectBlock(out, "rancher/charts"), `"line:v2.16:dev"`, line)
		assert.NotContains(t, out, `"line:v2.15"`, line)
	}
}

func TestGraphLineLeavesOutWhatIsNotForIt(t *testing.T) {
	out := linedOut(t, Options{Line: "v2.15"})

	rancher := projectBlock(out, "rancher/rancher")
	assert.Contains(t, rancher, `"artifact:rancher-image"`)
	assert.NotContains(t, rancher, `"artifact:rancher-server-image"`, "it exists from v2.16")
	assert.NotContains(t, out, `"holding:rancher-holds-shell"`, "it is held from v2.16")
	assert.NotContains(t, out, `"rancher/shell"`, "it has no row for the line and no holding drawn at it, and is no boundary")
	assert.Contains(t, projectBlock(out, "rancher/charts"), `"line:v2.15": {class: [line; eol]; label: "v2.15\nno branch yet"}`)
}

func TestGraphLineHasNoLineLabelOnTheHolderEdge(t *testing.T) {
	out := linedOut(t, Options{Line: "v2.16"})

	assert.Contains(t, out, "\"holding:rancher-holds-shell\" -> \"rancher/rancher\"\n")
	assert.NotContains(t, out, `-> "rancher/rancher": "`)
}

func TestGraphLineRefusesALineTheConfigDoesNotHave(t *testing.T) {
	out, err := graph(fakeWorkspace{g: lined(t)}, Options{Line: "v9.9", Now: day})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"v9.9"`)
	assert.Contains(t, err.Error(), "v2.17")
	assert.Empty(t, out)
}

func TestGraphLineNarrowsProjectAndManagedBy(t *testing.T) {
	out := linedOut(t, Options{Line: "v2.16", ManagedBy: []string{"rancher"}})
	assert.NotContains(t, out, "\"rancher/rancher\": {\n  class", "it is shared")
	assert.Contains(t, out, "\"rancher/charts\": {\n  class")
}

func mindmapLined(t *testing.T, opts Options) string {
	t.Helper()
	opts.Now = day
	out, err := mindmap(fakeWorkspace{g: lined(t)}, opts)
	require.NoError(t, err)
	return out
}

func TestMindmapListsTheLinesOfAProject(t *testing.T) {
	out := mindmapLined(t, Options{})

	assert.Regexp(t, `\n      n\d+\["lines"\]\n        n\d+\["v2.17: main, 2.17.x \(development\)"\]\n        n\d+\["v2.16: release/v2.16, 2.16.x \(ga\)"\]\n        n\d+\["v2.15: release/v2.15, 2.15.x \(eol\)"\]\n`, out)
	assert.Contains(t, out, `["v2.16 dev: dev-v2.16 (ga)"]`)
	assert.Contains(t, out, `["v2.15: no branch yet (eol)"]`)
}

func TestMindmapLineNarrowsTheLinesOfAProject(t *testing.T) {
	out := mindmapLined(t, Options{Line: "v2.16"})

	assert.Contains(t, out, `["v2.16: release/v2.16, 2.16.x (ga)"]`)
	assert.NotContains(t, out, "v2.17:")
	assert.NotContains(t, out, "v2.15:")
}
