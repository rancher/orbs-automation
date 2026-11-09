package linesresolve

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// now is after v2.15's GA and before its EOM, and after v2.13's EOL.
var now = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

// docSet is the documents of the fixture, by name and in order, so a test can drop
// or change one.
type docSet struct {
	names []string
	docs  map[string]string
}

func (d *docSet) put(name, doc string) {
	if d.docs == nil {
		d.docs = map[string]string{}
	}
	if _, ok := d.docs[name]; !ok {
		d.names = append(d.names, name)
	}
	d.docs[name] = doc
}

func (d docSet) list() []string {
	var out []string
	for _, n := range d.names {
		if doc, ok := d.docs[n]; ok {
			out = append(out, doc)
		}
	}
	return out
}

func project(name, spec string) string { return chaintest.Doc("Project", name, "", spec) }

func producer(name, image, stream string) []string {
	return []string{
		chaintest.Doc("Stream", name, "{project: "+name+"}", stream),
		chaintest.Doc("Artifact", name+"-image", "{stream: "+name+"}", "  kind: image\n  name: "+image+"\n"),
	}
}

func holding(consumer, producer, key string) string {
	return chaintest.Doc("Holding", consumer+"-holds-"+producer, "{project: "+consumer+", artifact: "+producer+"-image}",
		"  at: {file: build.yaml, key: "+key+"}\n  holds: {kind: release, form: \"{image}:{tag}\", accept: all}\n")
}

// fixtureDocs is the config of the tests: rancher is the hub, a branch for each of
// four lines with v2.16 on main; shell has a series for each line but v2.13, and
// ships to rancher; scc-operator is universal and ships to rancher; docs is shipped
// to by the hub; charts has lines but is not connected to rancher, and obteam,
// universal, ships to it. The lines document is deliberately not newest first.
func fixtureDocs() *docSet {
	d := &docSet{}
	d.put("lines", chaintest.Doc("RancherLines", "rancher", "", `  lines:
    - id: v2.14
      dates: {ga: 2026-04-30, eom: 2026-10-30, eol: 2027-10-10}
      kubernetes: {min: "1.33", max: "1.35"}
    - id: v2.16
      kubernetes: {min: "1.35", max: "1.37"}
    - id: v2.13
      dates: {ga: 2025-12-17, eom: 2026-06-17, eol: 2026-11-01}
    - id: v2.15
      dates: {ga: 2026-08-27, eom: 2027-02-27, eol: 2028-02-27}
      kubernetes: {min: "1.34", max: "1.36"}
`))
	d.put("rancher", project("rancher", `  repo: {name: rancher/rancher}
  finalDestination: true
  branches:
    layout: perLine
    lines:
      - {line: v2.16}
      - {line: v2.15, branch: release/v2.15}
      - {line: v2.14, branch: release/v2.14}
      - {line: v2.13, branch: release/v2.13}
`))
	for i, doc := range producer("rancher", "rancher/rancher", "  lines:\n    - {line: v2.16, series: \"2.16.x\"}\n    - {line: v2.15, series: \"2.15.x\"}\n    - {line: v2.14, series: \"2.14.x\"}\n    - {line: v2.13, series: \"2.13.x\"}\n") {
		d.put("rancher-"+string(rune('a'+i)), doc)
	}
	d.put("shell", project("shell", `  repo: {name: rancher/shell, owned: true}
  branches:
    layout: perLine
    lines:
      - {line: v2.16}
      - {line: v2.15, branch: release/v2.15}
      - {line: v2.14, branch: release/v2.14}
`))
	for i, doc := range producer("shell", "rancher/shell", "  lines:\n    - {line: v2.16, series: \"0.9.x\"}\n    - {line: v2.15, series: \"0.8.x\"}\n    - {line: v2.14, series: \"0.7.x\"}\n") {
		d.put("shell-"+string(rune('a'+i)), doc)
	}
	d.put("scc", project("scc-operator", "  repo: {name: rancher/scc-operator, owned: true}\n"))
	for i, doc := range producer("scc-operator", "rancher/scc-operator", "  universal: true\n") {
		d.put("scc-"+string(rune('a'+i)), doc)
	}
	d.put("docs", project("docs", `  repo: {name: rancher/docs}
  branches:
    layout: perLine
    lines:
      - {line: v2.16}
      - {line: v2.15, branch: version-2.15}
`))
	d.put("charts", project("charts", `  repo: {name: rancher/charts, defaultBranch: dev-v2.16}
  branches:
    layout: perLine
    lines:
      - {line: v2.16}
      - {line: v2.15, branch: release-v2.15}
`))
	d.put("obteam", project("obteam", "  repo: {name: rancher/ob-team-charts, owned: true}\n"))
	for i, doc := range producer("obteam", "rancher/ob-team-charts", "  universal: true\n") {
		d.put("obteam-"+string(rune('a'+i)), doc)
	}
	d.put("rancher-holds-shell", holding("rancher", "shell", "defaultShellVersion"))
	d.put("rancher-holds-scc", holding("rancher", "scc-operator", "defaultSccOperatorImage"))
	d.put("docs-holds-rancher", holding("docs", "rancher", "rancherDocs"))
	d.put("charts-holds-obteam", holding("charts", "obteam", "obChart"))
	return d
}

func graphOf(t *testing.T, d *docSet) (*chain.Graph, *bump.Engine) {
	t.Helper()
	g := chaintest.Load(t, d.list()...)
	engine, err := bump.NewEngine(bump.Builtin(), g)
	require.NoError(t, err)
	return g, engine
}

func resolve(t *testing.T, opts Options) Result {
	t.Helper()
	g, engine := graphOf(t, fixtureDocs())
	got, err := Resolve(g, engine, opts, now)
	require.NoError(t, err)
	return got
}

func lineOf(t *testing.T, r Result, id string) Line {
	t.Helper()
	for _, l := range r.Lines {
		if l.ID == id {
			return l
		}
	}
	require.Failf(t, "no such line", "%s in %v", id, r.Lines)
	return Line{}
}

func names(links []Link) []string {
	out := make([]string, len(links))
	for i, l := range links {
		out[i] = l.Name
	}
	return out
}

func ids(r Result) []string {
	out := make([]string, len(r.Lines))
	for i, l := range r.Lines {
		out[i] = l.ID
	}
	return out
}

func TestResolveListsTheLinesNewestFirstWhateverOrderTheDocumentHasThem(t *testing.T) {
	got := resolve(t, Options{})

	assert.Equal(t, []string{"v2.16", "v2.15", "v2.14", "v2.13"}, ids(got))
}

func TestResolveHasTheFactsOfEachLineAndItsPhaseFromTheClock(t *testing.T) {
	got := resolve(t, Options{})

	v215 := lineOf(t, got, "v2.15")
	assert.Equal(t, "ga", v215.Phase)
	assert.Equal(t, Dates{GA: "2026-08-27", EOM: "2027-02-27", EOL: "2028-02-27"}, v215.Dates)
	assert.Equal(t, Kubernetes{Min: "1.34", Max: "1.36"}, v215.Kubernetes)
	assert.Equal(t, "development", lineOf(t, got, "v2.16").Phase)
	assert.Equal(t, "eom", lineOf(t, got, "v2.14").Phase)
	assert.Equal(t, "eol", lineOf(t, got, "v2.13").Phase)
}

func TestResolveTheHubIsTheFinalDestination(t *testing.T) {
	got := resolve(t, Options{})

	assert.Equal(t, "rancher/rancher", got.Hub)
	assert.Equal(t, "rancher", got.HubName)
	v215 := lineOf(t, got, "v2.15")
	require.NotNil(t, v215.Hub)
	assert.Equal(t, Node{Project: "rancher/rancher", Name: "rancher", Branch: "release/v2.15", Series: "2.15.x"}, *v215.Hub)
	dev := lineOf(t, got, "v2.16")
	require.NotNil(t, dev.Hub)
	assert.Equal(t, "main", dev.Hub.Branch)
	assert.True(t, dev.Hub.IsDefaultBranch, "a line with no branch is on the default branch")
}

func TestResolveAHubCanBeNamed(t *testing.T) {
	got := resolve(t, Options{Hub: "rancher/docs"})

	assert.Equal(t, "rancher/docs", got.Hub)
	v215 := lineOf(t, got, "v2.15")
	require.NotNil(t, v215.Hub)
	assert.Equal(t, "version-2.15", v215.Hub.Branch)
	assert.Nil(t, lineOf(t, got, "v2.14").Hub, "docs has no v2.14")
}

func TestResolveWithNoFinalDestinationAndNoHubNamedIsAnErrorThatSaysHow(t *testing.T) {
	d := fixtureDocs()
	d.put("rancher", strings.Replace(d.docs["rancher"], "  finalDestination: true\n", "", 1)) // the lines document is still named rancher, which is not used
	g, engine := graphOf(t, d)

	_, err := Resolve(g, engine, Options{}, now)

	require.ErrorIs(t, err, ErrNoHub)
	assert.ErrorContains(t, err, "finalDestination")
	assert.ErrorContains(t, err, "--hub")
}

func TestResolveAnUnknownHubIsAnError(t *testing.T) {
	g, engine := graphOf(t, fixtureDocs())

	_, err := Resolve(g, engine, Options{Hub: "nope"}, now)

	require.ErrorIs(t, err, ErrNoHub)
	assert.ErrorContains(t, err, "nope")
}

func TestResolveTakesFromTheProjectsThatShipToTheHubsBranchForTheLine(t *testing.T) {
	got := resolve(t, Options{})

	v215 := lineOf(t, got, "v2.15")
	assert.Equal(t, []string{"shell", "scc-operator"}, names(v215.TakesFrom), "in config order")
	shell := v215.TakesFrom[0]
	assert.Equal(t, Link{
		Project: "rancher/shell", Name: "shell", Holdings: []string{"rancher-holds-shell"}, Series: "0.8.x", Branch: "release/v2.15",
	}, shell)
	scc := v215.TakesFrom[1]
	assert.True(t, scc.Universal)
	assert.Empty(t, scc.Series, "it has no series and no branch for the line")
	assert.Equal(t, "rancher/scc-operator", scc.Project)
	assert.Equal(t, "0.9.x", lineOf(t, got, "v2.16").TakesFrom[0].Series)
}

func TestResolveAProducerWithNoSuchLineIsAGapAndNotALink(t *testing.T) {
	got := resolve(t, Options{})

	v213 := lineOf(t, got, "v2.13")
	assert.Equal(t, []string{"scc-operator"}, names(v213.TakesFrom), "shell has no v2.13")
	require.Len(t, v213.Gaps, 1)
	assert.Contains(t, v213.Gaps[0], "shell")
	assert.Contains(t, v213.Gaps[0], "v2.13")
	assert.Equal(t, v213.Gaps, got.Gaps, "the gaps of every line are collected")
	assert.Empty(t, lineOf(t, got, "v2.15").Gaps)
}

func TestResolveTheFinalDestinationHasNoShipsToEvenWhenItHasEdges(t *testing.T) {
	got := resolve(t, Options{})

	assert.True(t, got.HubIsFinalDestination)
	for _, l := range got.Lines {
		assert.Empty(t, l.ShipsTo, l.ID)
	}
	var docsOnTheLine bool
	for _, a := range lineOf(t, got, "v2.15").Also {
		docsOnTheLine = docsOnTheLine || a.Name == "docs"
	}
	assert.True(t, docsOnTheLine, "a project it ships to is then just on the line, with its own connections")
}

func TestResolveAHubThatIsNotTheFinalDestinationShipsToTheProjectsItShipsTo(t *testing.T) {
	d := fixtureDocs()
	d.put("rancher", strings.Replace(d.docs["rancher"], "  finalDestination: true\n", "", 1))
	g, engine := graphOf(t, d)
	got, err := Resolve(g, engine, Options{Hub: "rancher"}, now)
	require.NoError(t, err)
	assert.False(t, got.HubIsFinalDestination)

	v215 := lineOf(t, got, "v2.15")
	require.Len(t, v215.ShipsTo, 1)
	assert.Equal(t, "docs", v215.ShipsTo[0].Name)
	assert.Equal(t, []string{"version-2.15"}, v215.ShipsTo[0].Branches)
	assert.Equal(t, []string{"docs-holds-rancher"}, v215.ShipsTo[0].Holdings)
	assert.Empty(t, lineOf(t, got, "v2.14").ShipsTo, "docs has no v2.14 branch to follow it to")
}

func TestResolveProjectsOnALineThatAreNotConnectedToTheHubAreAlsoWithTheirOwnConnections(t *testing.T) {
	got := resolve(t, Options{})

	v215 := lineOf(t, got, "v2.15")
	var also []string
	for _, a := range v215.Also {
		also = append(also, a.Name)
	}
	assert.Equal(t, []string{"docs", "charts"}, also, "shell is connected to the hub; docs is shipped to by it, which a final destination does not show")
	require.Len(t, v215.Also, 2)
	charts := v215.Also[1]
	assert.Equal(t, "release-v2.15", charts.Branch)
	require.Len(t, charts.TakesFrom, 1)
	assert.Equal(t, "obteam", charts.TakesFrom[0].Name)
	assert.True(t, charts.TakesFrom[0].Universal)
	assert.Empty(t, charts.ShipsTo)
}

func TestResolveALineFilterKeepsOnlyThoseLinesWithOrWithoutTheV(t *testing.T) {
	got := resolve(t, Options{Lines: []string{"2.15", "v2.13"}})

	assert.Equal(t, []string{"v2.15", "v2.13"}, ids(got), "still newest first")
}

func TestResolveALineThatIsNotInTheLinesDocumentIsAnError(t *testing.T) {
	g, engine := graphOf(t, fixtureDocs())

	_, err := Resolve(g, engine, Options{Lines: []string{"v9.9"}}, now)

	require.Error(t, err)
	assert.ErrorContains(t, err, "v9.9")
	assert.ErrorContains(t, err, "v2.16, v2.15, v2.14, v2.13")
}

func TestResolveAPhaseFilterKeepsTheLinesInThosePhases(t *testing.T) {
	got := resolve(t, Options{Phases: []string{"ga", "eom"}})

	assert.Equal(t, []string{"v2.15", "v2.14"}, ids(got))
}

func TestResolveAnUnknownPhaseIsAnError(t *testing.T) {
	g, engine := graphOf(t, fixtureDocs())

	_, err := Resolve(g, engine, Options{Phases: []string{"soon"}}, now)

	require.Error(t, err)
	assert.ErrorContains(t, err, "soon")
}

func TestResolveWithNoLinesDocumentIsAnError(t *testing.T) {
	var d docSet
	d.put("rancher", project("rancher", "  repo: {name: rancher/rancher}\n  finalDestination: true\n"))
	g, engine := graphOf(t, &d)

	_, err := Resolve(g, engine, Options{}, now)

	require.Error(t, err)
	assert.ErrorContains(t, err, "RancherLines")
}

func TestResultIsJSONThatReadsBack(t *testing.T) {
	got := resolve(t, Options{})

	data, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	var back Result
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, got, back)
	assert.Contains(t, string(data), `"takesFrom"`)
}

type fakeWorkspace struct {
	graph     *chain.Graph
	cfgErr    error
	engine    *bump.Engine
	engineErr error
}

func (f fakeWorkspace) Chain() (*chain.Graph, error) { return f.graph, f.cfgErr }
func (f fakeWorkspace) Bump() (*bump.Engine, error)  { return f.engine, f.engineErr }

func workspaceOf(t *testing.T) fakeWorkspace {
	t.Helper()
	g, engine := graphOf(t, fixtureDocs())
	return fakeWorkspace{graph: g, engine: engine}
}

func rendered(t *testing.T, format string, opts Options) (string, error) {
	t.Helper()
	var b strings.Builder
	err := run(&b, workspaceOf(t), format, opts, now, nil)
	return b.String(), err
}

func TestRunTextIsTheLinesNewestFirstEachWithWhatIsConnectedToIt(t *testing.T) {
	out, err := rendered(t, "text", Options{})

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "Release lines, through rancher/rancher\n"), out)
	assert.Less(t, strings.Index(out, "\nv2.16"), strings.Index(out, "\nv2.15"))
	assert.Less(t, strings.Index(out, "\nv2.15"), strings.Index(out, "\nv2.13"))
	assert.NotContains(t, out, "\n    ships to\n", "nothing ships on from the final destination")
	for _, want := range []string{
		"v2.15  ga  GA 2026-08-27, EOM 2027-02-27, EOL 2028-02-27  Kubernetes 1.34-1.36",
		"v2.16  development", "v2.13  eol",
		"rancher/rancher  release/v2.15",
		"rancher/rancher  main (default branch)",
		"takes from", "also on this line",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRunTextSaysWhatFeedsAndWhatIsFedWithSeriesBranchAndMethod(t *testing.T) {
	out, err := rendered(t, "text", Options{Lines: []string{"v2.15"}})

	require.NoError(t, err)
	for _, want := range []string{
		"shell", "0.8.x", "release/v2.15", "rancher-holds-shell",
		"scc-operator", "any release",
		"docs", "version-2.15",
		"charts", "release-v2.15", "takes from obteam (any release)",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRunTextSaysWhichHoldingsMakeEachLink(t *testing.T) {
	out, err := rendered(t, "text", Options{Lines: []string{"v2.15"}})

	require.NoError(t, err)
	for _, want := range []string{"rancher-holds-shell", "rancher-holds-scc", "docs-holds-rancher", "via charts-holds-obteam"} {
		assert.Contains(t, out, want)
	}
}

func TestResolveLinksCarryTheHoldingsThatMakeThem(t *testing.T) {
	got := resolve(t, Options{Lines: []string{"v2.15"}})

	assert.Equal(t, []string{"rancher-holds-shell"}, got.Lines[0].TakesFrom[0].Holdings)
}

func TestRunTextMarksTheGapsAndCountsThem(t *testing.T) {
	out, err := rendered(t, "text", Options{})

	require.NoError(t, err)
	assert.Contains(t, out, "! shell has no line v2.13")
	assert.Contains(t, out, "Gaps: 1")

	clean, err := rendered(t, "text", Options{Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Contains(t, clean, "No gaps.")
}

func TestRunJSONIsTheResult(t *testing.T) {
	out, err := rendered(t, "json", Options{Phases: []string{"ga"}})

	require.NoError(t, err)
	var got Result
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	assert.Equal(t, []string{"v2.15"}, ids(got))
	assert.Equal(t, "rancher/rancher", got.Hub)
}

func TestRunErrorsComeBackBeforeAnythingIsWritten(t *testing.T) {
	var b strings.Builder
	ws := workspaceOf(t)
	assert.Error(t, run(&b, ws, "env", Options{}, now, nil), "an output format it does not support")
	ws.cfgErr = errors.New("no config directory")
	assert.ErrorContains(t, run(&b, ws, "text", Options{}, now, nil), "no config directory")
	ws = workspaceOf(t)
	ws.engine = nil
	ws.engineErr = errors.New("holding x is not usable")
	assert.ErrorContains(t, run(&b, ws, "text", Options{}, now, nil), "not usable")
	assert.Empty(t, b.String())
}

func TestRunAnEngineThatIsPartlyUsableIsStillUsed(t *testing.T) {
	ws := workspaceOf(t)
	ws.engineErr = errors.New("method y of charts is not usable")
	var b strings.Builder

	err := run(&b, ws, "text", Options{Lines: []string{"v2.15"}}, now, nil)

	require.NoError(t, err)
	assert.Contains(t, b.String(), "shell")
}

// resolveWithAssets adds rancher-assets: used for v2.16 only, listing only that
// line, its lines in its tags and one branch holding them all.
func resolveWithAssets(t *testing.T) Result {
	t.Helper()
	d := fixtureDocs()
	d.put("assets", project("rancher-assets", "  repo: {name: rancher/rancher-assets, managedBy: release-team}\n"))
	for i, doc := range producer("rancher-assets", "rancher/rancher-assets", "  scheme: calverLineStage\n  lines: [{line: v2.16}]\n") {
		d.put("assets-"+string(rune('a'+i)), doc)
	}
	d.put("rancher-holds-assets", holding("rancher", "rancher-assets", "defaultAssetsImage"))
	g, engine := graphOf(t, d)
	got, err := Resolve(g, engine, Options{}, now)
	require.NoError(t, err)
	return got
}

func TestAProjectUsedForOneLineTakesPartOnThatLineAndHasNoGapOnTheOthers(t *testing.T) {
	got := resolveWithAssets(t)

	var names []string
	for _, in := range lineOf(t, got, "v2.16").TakesFrom {
		names = append(names, in.Name)
	}
	assert.Contains(t, names, "rancher-assets")
	for _, id := range []string{"v2.15", "v2.14", "v2.13"} {
		l := lineOf(t, got, id)
		for _, in := range l.TakesFrom {
			assert.NotEqual(t, "rancher-assets", in.Name, id)
		}
		for _, gap := range l.Gaps {
			assert.NotContains(t, gap, "rancher-assets", "only the lines it declares are judged: %s", id)
		}
	}
	assert.Len(t, got.Gaps, 1, "shell's missing v2.13 is the only gap, as without Assets")
}

// moduleDocs adds a library with a stream for each of two minors, held by svc, a
// project on the lines that is not connected to the hub, for different lines.
func moduleDocs() *docSet {
	d := fixtureDocs()
	d.put("lib", project("lib", "  repo: {name: acme/lib, owned: true}\n"))
	d.put("lib-1", chaintest.Doc("Stream", "lib-1", "{project: lib}", "  tagPattern: \"v1.*\"\n  universal: true\n"))
	d.put("lib-2", chaintest.Doc("Stream", "lib-2", "{project: lib}", "  tagPattern: \"v2.*\"\n  universal: true\n"))
	d.put("lib-1-module", chaintest.Doc("Artifact", "lib-1-module", "{stream: lib-1}", "  kind: gomod\n  name: github.com/acme/lib\n"))
	d.put("lib-2-module", chaintest.Doc("Artifact", "lib-2-module", "{stream: lib-2}", "  kind: gomod\n  name: github.com/acme/lib\n"))
	d.put("svc", project("svc", "  repo: {name: acme/svc}\n  branches:\n    layout: perLine\n    lines:\n      - {line: v2.16}\n      - {line: v2.15, branch: release/v2.15}\n"))
	hold := func(name, artifact, lines string) string {
		return chaintest.Doc("Holding", name, "{project: svc, artifact: "+artifact+"}",
			"  at: {file: go.mod, key: github.com/acme/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", lines: "+lines+"}\n")
	}
	d.put("svc-holds-lib-1", hold("svc-holds-lib-1", "lib-1-module", "{since: v2.16}"))
	d.put("svc-holds-lib-2", hold("svc-holds-lib-2", "lib-2-module", "{until: v2.15}"))
	return d
}

func alsoOf(t *testing.T, r Result, line, name string) Also {
	t.Helper()
	for _, a := range lineOf(t, r, line).Also {
		if a.Name == name {
			return a
		}
	}
	require.Failf(t, "not also on the line", "%s on %s in %v", name, line, lineOf(t, r, line).Also)
	return Also{}
}

func TestResolveAProjectOnALineTakesFromOnlyTheStreamsItsHoldingsCoverForThatLine(t *testing.T) {
	g, engine := graphOf(t, moduleDocs())
	got, err := Resolve(g, engine, Options{}, now)
	require.NoError(t, err)

	newer := alsoOf(t, got, "v2.16", "svc")
	require.Len(t, newer.TakesFrom, 1, "only the holding that covers v2.16")
	assert.Equal(t, []string{"svc-holds-lib-1"}, newer.TakesFrom[0].Holdings)

	older := alsoOf(t, got, "v2.15", "svc")
	require.Len(t, older.TakesFrom, 1)
	assert.Equal(t, []string{"svc-holds-lib-2"}, older.TakesFrom[0].Holdings)
}
