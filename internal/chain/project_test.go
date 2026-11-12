package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func project(name, spec string) string { return chaintest.Doc("Project", name, "", spec) }

func TestTheGraphFindsAProjectByNameOrSlugWhateverTheCase(t *testing.T) {
	g := chaintest.Load(t,
		project("shell", "  repo: {name: rancher/shell}\n"),
		project("rancher", "  repo: {name: rancher/rancher}\n"),
	)

	require.Len(t, g.Projects(), 2)
	assert.Equal(t, "shell", g.Projects()[0].Name())
	assert.Equal(t, "rancher", g.Projects()[1].Name())
	assert.Equal(t, "rancher/shell", g.Project("shell").Slug())
	assert.Equal(t, "shell", g.Project("SHELL").Name())
	assert.Equal(t, "rancher", g.Project("Rancher/Rancher").Name())
	assert.Nil(t, g.Project("nope"))
	assert.Nil(t, g.Project("rancher/nope"))
	assert.Nil(t, g.Project(""))
}

func TestTheGraphFindsAProjectByAnAliasWhateverTheCase(t *testing.T) {
	g := chaintest.Load(t,
		project("kuberlr-kubectl", "  repo: {name: rancher/kuberlr-kubectl}\n  aliases: [rk, kubectl]\n"),
		project("rancher", "  repo: {name: rancher/rancher}\n"),
	)

	assert.Equal(t, []string{"rk", "kubectl"}, g.Project("kuberlr-kubectl").Aliases())
	assert.Empty(t, g.Project("rancher").Aliases())
	assert.Equal(t, "kuberlr-kubectl", g.Project("rk").Name())
	assert.Equal(t, "kuberlr-kubectl", g.Project("KubeCtl").Name())
	assert.Equal(t, "rancher", g.Project("rancher").Name(), "a name still finds its own project")
	assert.Nil(t, g.Project("rk/rk"), "an alias is never a slug")
}

func TestTheGraphHasTheLines(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15"))

	require.Len(t, g.Lines(), 2)
	assert.Equal(t, "v2.16", g.Lines()[0].ID)
	assert.Equal(t, "v2.15", g.Lines()[1].ID)
	assert.Empty(t, chaintest.Load(t, project("p", "  repo: {name: a/b}\n")).Lines())
}

func TestAProjectKnowsItsOwnFacts(t *testing.T) {
	g := chaintest.Load(t,
		project("a", "  repo: {name: acme/a}\n"),
		project("b", "  repo: {name: acme/b, defaultBranch: master}\n"),
	)

	assert.Equal(t, "main", g.Project("a").DefaultBranch())
	assert.Equal(t, "master", g.Project("b").DefaultBranch())
}

func TestTheManagerOfAProject(t *testing.T) {
	tests := []struct {
		name, repo, want string
	}{
		{"managedBy as given", "{name: acme/a, managedBy: release-team}", "release-team"},
		{"orbs is not assumed", "{name: acme/a}", ""},
		{"shared as given", "{name: acme/a, managedBy: shared}", "shared"},
		{"an external repo has none", "{name: acme/a, external: true}", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := chaintest.Load(t, project("a", "  repo: "+tt.repo+"\n"))

			assert.Equal(t, tt.want, g.Project("a").ManagedBy())
		})
	}
}

const perLine = `  repo: {name: acme/a}
  branches:
    layout: perLine
    stages: [dev, stable]
    lines:
      - {line: v2.16}
      - {line: v2.15, branch: release/v2.15}
      - {line: v2.14, branch: ""}
      - {line: v2.13, branch: {dev: dev-v2.13, stable: release-v2.13}}
`

func TestTheBranchOfALine(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))
	a, s := g.Project("a"), g.Project("s")

	tests := []struct {
		name, line, stage string
		want              string
		ok                bool
	}{
		{"a line in development with no branch written", "v2.16", "", "main", true},
		{"a branch", "v2.15", "", "release/v2.15", true},
		{"a line without its v", "2.15", "", "release/v2.15", true},
		{"a line in capitals", "V2.15", "", "release/v2.15", true},
		{"one branch serves every stage", "v2.15", "dev", "release/v2.15", true},
		{"no branch yet", "v2.14", "", "", false},
		{"a branch for each stage", "v2.13", "dev", "dev-v2.13", true},
		{"the other stage", "v2.13", "stable", "release-v2.13", true},
		{"a stage map with no stage asked", "v2.13", "", "", false},
		{"a stage that has no branch", "v2.13", "beta", "", false},
		{"a line the project does not list", "v2.99", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := a.BranchOfLine(tt.line, tt.stage)

			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.ok, ok)
		})
	}

	t.Run("a single layout has the default branch for every line", func(t *testing.T) {
		got, ok := s.BranchOfLine("v2.99", "")

		assert.True(t, ok)
		assert.Equal(t, "main", got)
	})
}

func TestTheLinesAProjectLists(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))

	assert.Equal(t, []string{"v2.16", "v2.15", "v2.14", "v2.13"}, g.Project("a").Lines())
	assert.Empty(t, g.Project("s").Lines())
}

func TestTheLineOfABranch(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))
	a := g.Project("a")

	assert.Equal(t, "v2.16", a.LineOfBranch("main"))
	assert.Equal(t, "v2.15", a.LineOfBranch("release/v2.15"))
	assert.Equal(t, "v2.13", a.LineOfBranch("dev-v2.13"))
	assert.Equal(t, "v2.13", a.LineOfBranch("release-v2.13"))
	assert.Empty(t, a.LineOfBranch("feature/x"))
	assert.Empty(t, a.LineOfBranch(""))
	assert.Empty(t, g.Project("s").LineOfBranch("main"), "a single layout's branch holds every line")
}

func TestTheReleaseBranchesOfAProject(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))

	assert.Equal(t, []string{"main", "release/v2.15", "dev-v2.13", "release-v2.13"}, g.Project("a").ReleaseBranches())
	assert.Equal(t, []string{"main"}, g.Project("s").ReleaseBranches())
}

func TestAProjectIsOwnedByTheTeamThatManagesIt(t *testing.T) {
	g := chaintest.Load(t,
		project("mine", "  repo: {name: acme/mine, managedBy: orbs}\n"),
		project("theirs", "  repo: {name: acme/theirs, managedBy: release-team}\n"),
		project("everyones", "  repo: {name: acme/everyones, managedBy: shared}\n"),
		project("outside", "  repo: {name: acme/outside, external: true}\n"),
		project("plain", "  repo: {name: acme/plain}\n"),
	)

	assert.True(t, g.Project("mine").Owned("orbs"))
	assert.False(t, g.Project("mine").Owned("release-team"), "ownership follows the active team")
	assert.True(t, g.Project("theirs").Owned("release-team"))
	assert.False(t, g.Project("theirs").Owned("orbs"))
	assert.False(t, g.Project("mine").Owned(""), "no active team owns nothing")
	assert.False(t, g.Project("plain").Owned("orbs"), "a project that names no team is no team's")
	assert.False(t, g.Project("outside").Owned("orbs"))
	assert.True(t, g.Project("outside").External())
	assert.False(t, g.Project("mine").External())
}

func TestASharedProjectIsNoTeamsButEveryones(t *testing.T) {
	g := chaintest.Load(t,
		project("everyones", "  repo: {name: acme/everyones, managedBy: shared}\n"),
		project("mine", "  repo: {name: acme/mine, managedBy: orbs}\n"),
	)

	assert.True(t, g.Project("everyones").Shared())
	assert.False(t, g.Project("everyones").Owned("shared"), "shared is not a team")
	assert.False(t, g.Project("everyones").Owned("orbs"))
	assert.False(t, g.Project("mine").Shared())
}

func TestTheLayoutAndStagesOfAProject(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))

	assert.Equal(t, "perLine", g.Project("a").Layout())
	assert.Equal(t, []string{"dev", "stable"}, g.Project("a").Stages())
	assert.Equal(t, "single", g.Project("s").Layout())
	assert.Empty(t, g.Project("s").Stages())
}

func TestTheLineBranchesOfAProject(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))

	assert.Equal(t, []chain.LineBranch{
		{Line: "v2.16", Branch: "main", IsDefault: true},
		{Line: "v2.15", Branch: "release/v2.15"},
		{Line: "v2.14", Pending: true},
		{Line: "v2.13", Stages: map[string]string{"dev": "dev-v2.13", "stable": "release-v2.13"}},
	}, g.Project("a").LineBranches())
	assert.Empty(t, g.Project("s").LineBranches())
}

func TestTheBranchesOfAStage(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))
	a := g.Project("a")

	assert.Equal(t, []string{"main", "release/v2.15", "dev-v2.13"}, a.BranchesOfStage("dev"))
	assert.Equal(t, []string{"main", "release/v2.15", "release-v2.13"}, a.BranchesOfStage("stable"))
	assert.Equal(t, a.ReleaseBranches(), a.BranchesOfStage(""))
	assert.Equal(t, []string{"main"}, g.Project("s").BranchesOfStage("dev"))
}

func TestAProjectKnowsItsPullRequestConventions(t *testing.T) {
	g := chaintest.Load(t,
		project("a", "  repo: {name: acme/a}\n  pullRequests: {title: \"[{branch}] Update\", labels: [x], botLabels: [bot], pushTo: upstream}\n"),
		project("b", "  repo: {name: acme/b}\n"),
	)

	pr := g.Project("a").PullRequests()

	assert.Equal(t, "[{branch}] Update", pr.Title)
	assert.Equal(t, []string{"x"}, pr.Labels)
	assert.Equal(t, []string{"bot"}, pr.BotLabels)
	assert.Equal(t, "upstream", pr.PushTo)
	assert.Empty(t, g.Project("b").PullRequests().Title)
}

func TestAProjectSaysWhetherItIsTheFinalDestination(t *testing.T) {
	g := chaintest.Load(t,
		project("end", "  repo: {name: acme/end}\n  finalDestination: true\n"),
		project("other", "  repo: {name: acme/other}\n"),
	)

	assert.True(t, g.Project("end").FinalDestination())
	assert.False(t, g.Project("other").FinalDestination())
}

func TestAStepIsFromTheProducerWhenItSaysSo(t *testing.T) {
	assert.True(t, chain.Step{Run: []string{"x"}, From: "producer"}.FromProducer())
	assert.False(t, chain.Step{Run: []string{"x"}, From: "consumer"}.FromProducer())
	assert.False(t, chain.Step{Run: []string{"x"}}.FromProducer(), "no from is the consumer")
}

func TestAProjectResolvesWhatIsTypedAsALineOrABranch(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"),
		project("a", perLine),
		project("s", "  repo: {name: acme/s}\n"),
		stream("s-one", "s", "  lines:\n    - {line: v2.15, series: \"0.9.x\"}\n"),
	)
	a, s := g.Project("a"), g.Project("s")

	tests := []struct {
		name         string
		p            *chain.Project
		arg          string
		branch, line string
		errContains  string
	}{
		{"a line with a branch of its own", a, "v2.15", "release/v2.15", "v2.15", ""},
		{"a line written without its v", a, "2.15", "release/v2.15", "2.15", ""},
		{"a line in development, on the default branch", a, "v2.16", "main", "v2.16", ""},
		{"a release branch is the branch of its line", a, "release/v2.15", "release/v2.15", "v2.15", ""},
		{"the default branch is the branch of the line on it", a, "main", "main", "v2.16", ""},
		{"a line with no branch yet", a, "v2.14", "", "", "has no release line or branch \"v2.14\": it has v2.16, v2.15, v2.14, v2.13"},
		{"a line the project does not list", a, "v2.99", "", "", "acme/a has no release line or branch \"v2.99\""},
		{"a line of the stream's, on the one branch of a single layout", s, "v2.15", "main", "v2.15", ""},
		{"the one branch of a single layout is in no line", s, "main", "main", "", ""},
		{"a line a single layout knows nothing of", s, "v2.99", "", "", "acme/s has no release line or branch \"v2.99\": it has main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch, line, err := tt.p.ResolveLineOrBranch(tt.arg)

			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.branch, branch)
			assert.Equal(t, tt.line, line)
		})
	}
}

func TestAProjectKnowsWhatItIsCalled(t *testing.T) {
	g := chaintest.Load(t,
		project("scc-operator", "  repo: {name: rancher/scc-operator}\n  aliases: [scc]\n"),
		project("shell", "  repo: {name: rancher/shell}\n"),
	)
	scc, shell := g.Project("scc-operator"), g.Project("shell")

	for _, key := range []string{"scc-operator", "SCC-Operator", "rancher/scc-operator", "Rancher/SCC-Operator", "scc", "SCC"} {
		assert.True(t, scc.Named(key), key)
		assert.False(t, shell.Named(key), key)
	}
	for _, key := range []string{"", "rancher", "scc-op", "rancher/shell"} {
		assert.False(t, scc.Named(key), "%q is not a name of it", key)
	}

	t.Run("it is what the graph finds a project by", func(t *testing.T) {
		for _, p := range g.Projects() {
			for _, key := range []string{p.Name(), p.Slug(), "shell", "scc", "SCC-OPERATOR", "rancher/shell", "nobody"} {
				assert.Equal(t, g.Project(key) == p, p.Named(key), "%s as %q", p.Name(), key)
			}
		}
	})
}

func TestAProjectKnowsWhichLinesItListsForItsBranches(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"),
		project("a", perLine),
		project("s", "  repo: {name: acme/s}\n"),
	)
	a, s := g.Project("a"), g.Project("s")

	for _, line := range []string{"v2.16", "v2.15", "2.15", "V2.15", "v2.14", "v2.13"} {
		assert.True(t, a.ListsLine(line), line)
	}
	assert.False(t, a.ListsLine("v2.99"))
	assert.False(t, s.ListsLine("v2.15"), "a single layout lists none, though it keeps them all")
}

func TestAProjectKnowsWhichLinesItsStreamsList(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		project("a", "  repo: {name: acme/a}\n"),
		project("u", "  repo: {name: acme/u}\n"),
		stream("a-one", "a", "  lines:\n    - {line: v2.15, series: \"0.9.x\"}\n"),
		stream("u-one", "u", "  universal: true\n"),
	)

	assert.True(t, g.Project("a").StreamListsLine("v2.15"))
	assert.True(t, g.Project("a").StreamListsLine("2.15"))
	assert.False(t, g.Project("a").StreamListsLine("v2.16"))
	assert.False(t, g.Project("u").StreamListsLine("v2.15"), "a universal stream serves every line but lists none")
}

func TestTheHoldingsOfAProjectInTheFinalDestination(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		project("prod", "  repo: {name: acme/prod}\n"),
		project("hold", "  repo: {name: acme/hold}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r/2.15}]\n"),
		project("end", "  repo: {name: acme/end}\n  finalDestination: true\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r/2.15}]\n"),
		stream("prod", "prod", "  lines:\n    - {line: v2.16, series: \"2.x\"}\n    - {line: v2.15, series: \"1.x\"}\n    - {line: v2.14, series: \"0.x\"}\n"),
		artifact("img", "prod", "  kind: image\n  name: acme/prod\n"),
		holding("hold-img", "{project: hold, artifact: img}", holdsImage),
		holding("end-img", "{project: end, artifact: img}", holdsImage),
		holding("end-commit", "{project: end, from: prod}", holdsFrom),
	)
	prod := g.Project("prod")

	got := prod.DestinationHoldings("v2.15")

	require.Len(t, got, 1, "not the other holder's, and not a commit")
	assert.Equal(t, "end-img", got[0].Name())
	assert.Empty(t, prod.DestinationHoldings("v2.14"), "the destination lists no such line")
	assert.Empty(t, g.Project("end").DestinationHoldings("v2.15"), "it produces nothing the destination holds")
}

func releaseTargetGraph(t *testing.T) *chain.Graph {
	return chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"),
		project("a", perLine),
		project("s", "  repo: {name: acme/s}\n"),
		project("t", "  repo: {name: acme/t}\n"),
		stream("a-one", "a", "  lines:\n    - {line: v2.15, series: \"0.9.x\"}\n    - {line: v2.13, series: \"0.7.x\"}\n"),
		stream("t-one", "t", "  lines:\n    - {line: v2.16, series: \"1.x\"}\n    - {line: v2.15, series: \"0.9.x\"}\n"),
	)
}

func TestTheReleaseTargetsOfAProject(t *testing.T) {
	g := releaseTargetGraph(t)

	t.Run("a branch for each line, and for each stage of a line that has one for each", func(t *testing.T) {
		assert.Equal(t, []chain.ReleaseTarget{
			{Line: "v2.16", Branch: "main", IsDefault: true},
			{Line: "v2.15", Series: "0.9.x", Branch: "release/v2.15"},
			{Line: "v2.14"},
			{Line: "v2.13", Series: "0.7.x", Stage: "dev", Branch: "dev-v2.13"},
			{Line: "v2.13", Series: "0.7.x", Stage: "stable", Branch: "release-v2.13"},
		}, g.Project("a").ReleaseTargets())
	})
	t.Run("a single layout has one branch, with a row for each line its streams list", func(t *testing.T) {
		assert.Equal(t, []chain.ReleaseTarget{
			{Line: "v2.16", Series: "1.x", Branch: "main", IsDefault: true},
			{Line: "v2.15", Series: "0.9.x", Branch: "main", IsDefault: true},
		}, g.Project("t").ReleaseTargets())
	})
	t.Run("a single layout whose streams list no line has the one row", func(t *testing.T) {
		assert.Equal(t, []chain.ReleaseTarget{{Branch: "main", IsDefault: true}}, g.Project("s").ReleaseTargets())
	})
}

func TestTheReleaseTargetsOfALine(t *testing.T) {
	g := releaseTargetGraph(t)
	a, s, tagged := g.Project("a"), g.Project("s"), g.Project("t")

	assert.Equal(t, []chain.ReleaseTarget{{Line: "v2.15", Series: "0.9.x", Branch: "release/v2.15"}}, a.ReleaseTargetsOf("v2.15"))
	assert.Equal(t, a.ReleaseTargetsOf("v2.15"), a.ReleaseTargetsOf("2.15"), "with or without the v")
	assert.Equal(t, a.ReleaseTargetsOf("v2.15"), a.ReleaseTargetsOf("V2.15"), "in any case")
	assert.Len(t, a.ReleaseTargetsOf("v2.13"), 2, "a row for each stage")
	assert.Equal(t, []chain.ReleaseTarget{{Line: "v2.14"}}, a.ReleaseTargetsOf("v2.14"), "a line with no branch yet")
	assert.Empty(t, a.ReleaseTargetsOf("v2.99"))
	assert.Equal(t, []chain.ReleaseTarget{{Line: "v2.16", Series: "1.x", Branch: "main", IsDefault: true}}, tagged.ReleaseTargetsOf("v2.16"))
	assert.Empty(t, s.ReleaseTargetsOf("v2.16"), "the one row of a single layout is of no line")
}

func TestTheSeriesAProjectsStreamsGiveALine(t *testing.T) {
	g := releaseTargetGraph(t)
	a := g.Project("a")

	assert.Equal(t, "0.9.x", a.SeriesOf("v2.15"))
	assert.Equal(t, "0.9.x", a.SeriesOf("2.15"))
	assert.Empty(t, a.SeriesOf("v2.16"), "no stream gives it one")
	assert.Empty(t, g.Project("s").SeriesOf("v2.15"), "a project with no stream")
}
