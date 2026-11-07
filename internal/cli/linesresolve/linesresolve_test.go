package linesresolve

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// now is after v2.15's GA and before its EOM, and after v2.13's EOL.
var now = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// linesDoc is deliberately not newest first.
func linesDoc() *config.RancherLines {
	return &config.RancherLines{Name: "rancher", Lines: []config.Line{
		{ID: "v2.14", Dates: config.Dates{GA: day(2026, 4, 30), EOM: day(2026, 10, 30), EOL: day(2027, 10, 10)}, Kubernetes: config.KubernetesRange{Min: "1.33", Max: "1.35"}},
		{ID: "v2.16", Kubernetes: config.KubernetesRange{Min: "1.35", Max: "1.37"}},
		{ID: "v2.13", Dates: config.Dates{GA: day(2025, 12, 17), EOM: day(2026, 6, 17), EOL: day(2026, 11, 1)}},
		{ID: "v2.15", Dates: config.Dates{GA: day(2026, 8, 27), EOM: day(2027, 2, 27), EOL: day(2028, 2, 27)}, Kubernetes: config.KubernetesRange{Min: "1.34", Max: "1.36"}},
	}}
}

func method() config.UpdateMethod {
	return config.UpdateMethod{
		Name: "build-yaml-bump", Strategy: "yamlBump", With: map[string]string{"file": "build.yaml"},
		AllowedParams: []config.AllowedParam{{Key: "updateKey"}, {Key: "value", Default: "{tag}"}},
	}
}

func edge(to, key string) config.Edge {
	return config.Edge{To: to, Method: "build-yaml-bump", Params: map[string]string{"updateKey": key, "value": "{image}:{tag}"}}
}

func ln(id, branch, version string) config.ProjectLine {
	return config.ProjectLine{Line: id, Branch: branch, Version: version}
}

// rancher is the hub: a branch for each of four lines, with v2.16 on main.
func rancher() config.Project {
	return config.Project{
		Name: "rancher", Repo: config.Repo{Name: "rancher/rancher", DefaultBranch: "main"},
		Release: config.Release{Branching: config.BranchingPerLine, Lines: []config.ProjectLine{
			ln("v2.16", "", ""), ln("v2.15", "release/v2.15", ""), ln("v2.14", "release/v2.14", ""), ln("v2.13", "release/v2.13", ""),
		}},
		UpdateMethods: []config.UpdateMethod{method()},
		Downstream:    []config.Edge{edge("docs", "rancherDocs")},
	}
}

// shell has a series for each line but v2.13, and ships to rancher.
func shell() config.Project {
	return config.Project{
		Name: "shell", Repo: config.Repo{Name: "rancher/shell", DefaultBranch: "main", Owned: true},
		Release: config.Release{Branching: config.BranchingPerLine, Lines: []config.ProjectLine{
			ln("v2.16", "", "0.9.x"), ln("v2.15", "release/v2.15", "0.8.x"), ln("v2.14", "release/v2.14", "0.7.x"),
		}},
		Downstream: []config.Edge{edge("rancher", "defaultShellVersion")},
	}
}

// scc is universal: no lines of its own, and it ships to rancher.
func scc() config.Project {
	return config.Project{
		Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator", DefaultBranch: "main", Owned: true},
		Release:    config.Release{Branching: config.BranchingSingle},
		Downstream: []config.Edge{edge("rancher", "defaultSccOperatorImage")},
	}
}

// docs is shipped to by the hub.
func docs() config.Project {
	return config.Project{
		Name: "docs", Repo: config.Repo{Name: "rancher/docs", DefaultBranch: "main"},
		Release: config.Release{Branching: config.BranchingPerLine, Lines: []config.ProjectLine{
			ln("v2.16", "", ""), ln("v2.15", "version-2.15", ""),
		}},
		UpdateMethods: []config.UpdateMethod{method()},
	}
}

// charts has lines but is not connected to rancher: obteam, universal, ships to it.
func charts() config.Project {
	return config.Project{
		Name: "charts", Repo: config.Repo{Name: "rancher/charts", DefaultBranch: "dev-v2.16"},
		Release: config.Release{Branching: config.BranchingPerLine, Lines: []config.ProjectLine{
			ln("v2.16", "", ""), ln("v2.15", "release-v2.15", ""),
		}},
		UpdateMethods: []config.UpdateMethod{method()},
	}
}

func obteam() config.Project {
	return config.Project{
		Name: "obteam", Repo: config.Repo{Name: "rancher/ob-team-charts", DefaultBranch: "main"},
		Release:    config.Release{Branching: config.BranchingSingle},
		Downstream: []config.Edge{edge("charts", "obChart")},
	}
}

func fixtureConfig() config.Config {
	return config.Config{
		Projects: []config.Project{rancher(), shell(), scc(), docs(), charts(), obteam()},
		Lines:    linesDoc(),
	}
}

func engineOf(t *testing.T, cfg config.Config) *updatedispatcher.Engine {
	t.Helper()
	engine, err := updatedispatcher.NewEngine(updatedispatcher.Builtin(), cfg)
	require.NoError(t, err)
	return engine
}

func resolve(t *testing.T, opts Options) Result {
	t.Helper()
	cfg := fixtureConfig()
	got, err := Resolve(cfg, engineOf(t, cfg), opts, now)
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

func TestResolveTheHubIsTheProjectTheLinesDocumentIsNamedFor(t *testing.T) {
	got := resolve(t, Options{})

	assert.Equal(t, "rancher/rancher", got.Hub)
	assert.Equal(t, "rancher", got.HubName)
	v215 := lineOf(t, got, "v2.15")
	require.NotNil(t, v215.Hub)
	assert.Equal(t, Node{Project: "rancher/rancher", Name: "rancher", Branch: "release/v2.15"}, *v215.Hub)
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

func TestResolveWithNoHubToBeFoundAndNoneNamedIsAnError(t *testing.T) {
	cfg := fixtureConfig()
	cfg.Lines.Name = "nothing-like-it"
	cfg.Projects = cfg.Projects[1:] // rancher is gone

	_, err := Resolve(cfg, engineOf(t, cfg), Options{}, now)

	require.ErrorIs(t, err, ErrNoHub)
	assert.ErrorContains(t, err, "--hub")
}

func TestResolveAnUnknownHubIsAnError(t *testing.T) {
	cfg := fixtureConfig()

	_, err := Resolve(cfg, engineOf(t, cfg), Options{Hub: "nope"}, now)

	require.ErrorIs(t, err, ErrNoHub)
	assert.ErrorContains(t, err, "nope")
}

func TestResolveTakesFromTheProjectsThatShipToTheHubsBranchForTheLine(t *testing.T) {
	got := resolve(t, Options{})

	v215 := lineOf(t, got, "v2.15")
	assert.Equal(t, []string{"shell", "scc-operator"}, names(v215.TakesFrom), "in config order")
	shell := v215.TakesFrom[0]
	assert.Equal(t, Link{Project: "rancher/shell", Name: "shell", Method: "build-yaml-bump", Series: "0.8.x", Branch: "release/v2.15"}, shell)
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

func TestResolveShipsToTheProjectsTheHubShipsTo(t *testing.T) {
	got := resolve(t, Options{})

	v215 := lineOf(t, got, "v2.15")
	require.Len(t, v215.ShipsTo, 1)
	assert.Equal(t, "docs", v215.ShipsTo[0].Name)
	assert.Equal(t, []string{"version-2.15"}, v215.ShipsTo[0].Branches)
	assert.Equal(t, "build-yaml-bump", v215.ShipsTo[0].Method)
	assert.Empty(t, lineOf(t, got, "v2.14").ShipsTo, "docs has no v2.14 branch to follow it to")
}

func TestResolveProjectsOnALineThatAreNotConnectedToTheHubAreAlsoWithTheirOwnConnections(t *testing.T) {
	got := resolve(t, Options{})

	v215 := lineOf(t, got, "v2.15")
	var also []string
	for _, a := range v215.Also {
		also = append(also, a.Name)
	}
	assert.Equal(t, []string{"charts"}, also, "shell and docs are connected to the hub, charts is not")
	charts := v215.Also[0]
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
	cfg := fixtureConfig()

	_, err := Resolve(cfg, engineOf(t, cfg), Options{Lines: []string{"v9.9"}}, now)

	require.Error(t, err)
	assert.ErrorContains(t, err, "v9.9")
	assert.ErrorContains(t, err, "v2.16, v2.15, v2.14, v2.13")
}

func TestResolveAPhaseFilterKeepsTheLinesInThosePhases(t *testing.T) {
	got := resolve(t, Options{Phases: []string{"ga", "eom"}})

	assert.Equal(t, []string{"v2.15", "v2.14"}, ids(got))
}

func TestResolveAnUnknownPhaseIsAnError(t *testing.T) {
	cfg := fixtureConfig()

	_, err := Resolve(cfg, engineOf(t, cfg), Options{Phases: []string{"soon"}}, now)

	require.Error(t, err)
	assert.ErrorContains(t, err, "soon")
}

func TestResolveWithNoLinesDocumentIsAnError(t *testing.T) {
	cfg := fixtureConfig()
	cfg.Lines = nil

	_, err := Resolve(cfg, engineOf(t, cfg), Options{}, now)

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
	cfg       config.Config
	cfgErr    error
	engine    *updatedispatcher.Engine
	engineErr error
}

func (f fakeWorkspace) Config() (config.Config, error) { return f.cfg, f.cfgErr }
func (f fakeWorkspace) Updates() (*updatedispatcher.Engine, error) {
	return f.engine, f.engineErr
}

func workspaceOf(t *testing.T) fakeWorkspace {
	t.Helper()
	cfg := fixtureConfig()
	return fakeWorkspace{cfg: cfg, engine: engineOf(t, cfg)}
}

func rendered(t *testing.T, format string, opts Options) (string, error) {
	t.Helper()
	var b strings.Builder
	err := run(&b, workspaceOf(t), format, opts, now)
	return b.String(), err
}

func TestRunTextIsTheLinesNewestFirstEachWithWhatIsConnectedToIt(t *testing.T) {
	out, err := rendered(t, "text", Options{})

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "Release lines, through rancher/rancher\n"), out)
	assert.Less(t, strings.Index(out, "\nv2.16"), strings.Index(out, "\nv2.15"))
	assert.Less(t, strings.Index(out, "\nv2.15"), strings.Index(out, "\nv2.13"))
	for _, want := range []string{
		"v2.15  ga  GA 2026-08-27, EOM 2027-02-27, EOL 2028-02-27  Kubernetes 1.34-1.36",
		"v2.16  development", "v2.13  eol",
		"rancher/rancher  release/v2.15",
		"rancher/rancher  main (default branch)",
		"takes from", "ships to", "also on this line",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRunTextSaysWhatFeedsAndWhatIsFedWithSeriesBranchAndMethod(t *testing.T) {
	out, err := rendered(t, "text", Options{Lines: []string{"v2.15"}})

	require.NoError(t, err)
	for _, want := range []string{
		"shell", "0.8.x", "release/v2.15", "build-yaml-bump",
		"scc-operator", "any release",
		"docs", "version-2.15",
		"charts", "release-v2.15", "takes from obteam (any release)",
	} {
		assert.Contains(t, out, want)
	}
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
	assert.Error(t, run(&b, ws, "env", Options{}, now), "an output format it does not support")
	ws.cfgErr = errors.New("no config directory")
	assert.ErrorContains(t, run(&b, ws, "text", Options{}, now), "no config directory")
	ws = workspaceOf(t)
	ws.engine = nil
	ws.engineErr = errors.New("method x is not usable")
	assert.ErrorContains(t, run(&b, ws, "text", Options{}, now), "not usable")
	assert.Empty(t, b.String())
}

func TestRunAnEngineThatIsPartlyUsableIsStillUsed(t *testing.T) {
	ws := workspaceOf(t)
	ws.engineErr = errors.New("method y of charts is not usable")
	var b strings.Builder

	err := run(&b, ws, "text", Options{Lines: []string{"v2.15"}}, now)

	require.NoError(t, err)
	assert.Contains(t, b.String(), "shell")
}
