package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectWith is a project of the repo, with the pull request conventions given as
// the flow YAML of the spec's pullRequests, or none.
func projectWith(t *testing.T, name, repo, pullRequests string) *chain.Project {
	t.Helper()
	spec := "  repo: {name: " + repo + "}\n"
	if pullRequests != "" {
		spec += "  pullRequests: " + pullRequests + "\n"
	}
	return chaintest.Load(t, chaintest.Doc("Project", name, "", spec)).Project(name)
}

func previewFixture(t *testing.T) previewInput {
	t.Helper()
	return previewInput{
		producer: projectWith(t, "scc-operator", "rancher/scc-operator", ""),
		consumer: projectWith(t, "rancher", "rancher/rancher", ""),
		release:  Release{Tag: "v0.5.3", Commit: "sha", Stable: true},
		target:   Target{Branch: "release/v2.15", Commit: "sha-215"},
		checks:   []bump.HoldingCheck{{Check: oneLocation()}},
	}
}

func oneLocation() bump.Check {
	change := bump.Change{
		Path: "build.yaml", From: "rancher/scc-operator:v0.5.2", To: "rancher/scc-operator:v0.5.3",
		Description: "Updated `defaultSccOperatorImage` in `build.yaml` from `rancher/scc-operator:v0.5.2` to `rancher/scc-operator:v0.5.3`",
	}
	return bump.Check{
		Current: "rancher/scc-operator:v0.5.2", Want: "rancher/scc-operator:v0.5.3", Change: change,
		Locations: []bump.LocationCheck{{Current: "rancher/scc-operator:v0.5.2", Change: change}},
	}
}

func TestRenderPreviewDefaults(t *testing.T) {
	got, err := renderPreview(previewFixture(t))
	require.NoError(t, err)

	assert.Equal(t, "bot/scc-operator-v0.5.3-v2.15", got.Branch, "what the script made, without the epoch")
	assert.Equal(t, "[v2.15] Update scc-operator to v0.5.3", got.Title)
	assert.Empty(t, got.Labels)
	assert.Equal(t, "Update scc-operator to v0.5.3\n\nAutomated update from rancher/scc-operator release v0.5.3\n\nAutomation: gero ship\n",
		got.CommitMessage)
	assert.Contains(t, got.Body, "Update scc-operator to [`v0.5.3`](https://github.com/rancher/scc-operator/releases/tag/v0.5.3)")
	assert.Contains(t, got.Body, "- Updated `defaultSccOperatorImage` in `build.yaml` from `rancher/scc-operator:v0.5.2` to `rancher/scc-operator:v0.5.3`",
		"the changes come from what the strategy did, so the body cannot name a step that did not run")
}

func TestRenderPreviewListsEveryLocationThatChanges(t *testing.T) {
	in := previewFixture(t)
	first := bump.Change{Path: "go.mod", Description: "Updated `a` in `go.mod` from `v1` to `v2`"}
	second := bump.Change{Path: "pkg/apis/go.mod", Description: "Updated `a` in `pkg/apis/go.mod` from `v1` to `v2`"}
	upToDate := bump.Change{Path: "pkg/plan/go.mod", Description: "Updated `a` in `pkg/plan/go.mod` from `v2` to `v2`"}
	in.checks = []bump.HoldingCheck{{Check: bump.Check{
		Current: "v1", Want: "v2", Change: first,
		Locations: []bump.LocationCheck{
			{Current: "v1", Change: first},
			{Current: "v1", Change: second},
			{Current: "v2", UpToDate: true, Change: upToDate},
		},
	}}}

	got, err := renderPreview(in)

	require.NoError(t, err)
	assert.Contains(t, got.Body, "- "+first.Description)
	assert.Contains(t, got.Body, "- "+second.Description, "a second location is changed too")
	assert.NotContains(t, got.Body, upToDate.Description, "a location that already holds it is not a change")
}

func TestStrategyMessageNamesTheStrategyAndWhatItChanged(t *testing.T) {
	in := previewFixture(t)
	unit := bump.Unit{Kind: bump.UnitStrategy, Edit: "tidy", Strategy: "goBump", Changes: []bump.Change{
		{Path: "go.mod", Description: "Ran `go mod tidy`"},
		{Path: "pkg/apis/go.mod", Description: "Ran `go mod tidy` in `pkg/apis`"},
	}}

	got, err := strategyMessage(in)(unit)

	require.NoError(t, err)
	assert.Equal(t, "Run goBump\n\n- Ran `go mod tidy`\n- Ran `go mod tidy` in `pkg/apis`\n\n"+
		"Part of updating scc-operator to v0.5.3: the goBump strategy of edit tidy.\n\nAutomation: gero ship\n", got)
}

// moduleProducer is a project whose releases are tagged with a prefix, as a Go module
// in a subdirectory is, or with none.
func moduleProducer(t *testing.T, tagPattern string) *chain.Project {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Doc("Project", "ob-charts-tool", "", "  repo: {name: rancher/ob-charts-tool}\n"),
		chaintest.Doc("Stream", "helmtools", "{project: ob-charts-tool}", "  tagPattern: \""+tagPattern+"\"\n  universal: true\n"),
	).Project("ob-charts-tool")
}

func TestTheVersionOfAReleaseIsTheTagWithoutThePrefixOfItsStreamOrItsV(t *testing.T) {
	tests := []struct {
		name, pattern, tag, want string
	}{
		{"a prefixed tag", "helmtools/v*", "helmtools/v0.1.2", "0.1.2"},
		{"a tag with only a v", "v*", "v0.5.3", "0.5.3"},
		{"a prerelease", "helmtools/v*", "helmtools/v0.1.2-rc.1", "0.1.2-rc.1"},
		{"a tag the stream does not match", "pkg/v*", "v0.5.3", "0.5.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := previewFixture(t)
			in.producer = moduleProducer(t, tt.pattern)
			in.release = Release{Tag: tt.tag}

			assert.Equal(t, tt.want, textVars(in)["version"])
			assert.Equal(t, tt.tag, textVars(in)["tag"], "the tag is as published")
		})
	}
}

func TestAPullRequestTitleWithTheVersionOfAPrefixedReleaseHasNoPrefix(t *testing.T) {
	in := previewFixture(t)
	in.producer = moduleProducer(t, "helmtools/v*")
	in.consumer = projectWith(t, "rancher-assets", "rancher/rancher-assets", `{title: "Bump {source} to {version}"}`)
	in.release = Release{Tag: "helmtools/v0.1.2"}

	got, err := renderPreview(in)

	require.NoError(t, err)
	assert.Equal(t, "Bump ob-charts-tool to 0.1.2", got.Title)
}

func TestRenderPreviewBodyCarriesTheMarker(t *testing.T) {
	got, err := renderPreview(previewFixture(t))
	require.NoError(t, err)
	m, ok := ParseMarker(got.Body)
	require.True(t, ok)
	assert.Equal(t, Marker{Source: "rancher/scc-operator", Consumer: "rancher/rancher", Target: "release/v2.15", Version: "v0.5.3"}, m)
}

func TestRenderPreviewTheDevelopmentBranch(t *testing.T) {
	in := previewFixture(t)
	in.target = Target{Branch: "main"}
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "bot/scc-operator-v0.5.3-main", got.Branch)
	assert.Equal(t, "[main] Update scc-operator to v0.5.3", got.Title)
}

func TestRenderPreviewUsesTheProjectsTitleLabelsAndPrefix(t *testing.T) {
	in := previewFixture(t)
	in.consumer = projectWith(t, "rancher", "rancher/rancher",
		`{title: "Bump {source} ({version}) on {targetBranch}", branchPrefix: "auto/{project}/", labels: [status/auto-created, team/orbs]}`)
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "Bump scc-operator (0.5.3) on release/v2.15", got.Title)
	assert.Equal(t, "auto/scc-operator/scc-operator-v0.5.3-v2.15", got.Branch, "the key follows whatever the prefix says")
	assert.Equal(t, []string{"status/auto-created", "team/orbs"}, got.Labels)
}

func TestRenderPreviewTheUsersPrefixReplacesTheProjectsWhole(t *testing.T) {
	in := previewFixture(t)
	in.consumer = projectWith(t, "rancher", "rancher/rancher", `{branchPrefix: "auto/"}`)
	in.userPrefix = "dan/"
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "dan/scc-operator-v0.5.3-v2.15", got.Branch, "the prefix is theirs, and the key is gero's")
}

func TestRenderPreviewAPrefixThatLeavesOutTheSourceStillEndsInTheKey(t *testing.T) {
	in := previewFixture(t)
	in.userPrefix = "dan-"
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "dan-scc-operator-v0.5.3-v2.15", got.Branch)
}

func TestRenderPreviewTheKeyIsAVariableOfTheTitle(t *testing.T) {
	in := previewFixture(t)
	in.consumer = projectWith(t, "rancher", "rancher/rancher", `{title: "Bump {key}"}`)
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "Bump scc-operator-v0.5.3-v2.15", got.Title)
}

func TestRenderPreviewListsThePostEditCommands(t *testing.T) {
	in := previewFixture(t)
	in.steps = []bump.StepRef{
		{Phase: bump.PostEdit, Step: chain.Step{Run: v1alpha2.Command{"scripts/generate-buildconfig"}}},
		{Phase: bump.PostEdit, Index: 1, Step: chain.Step{Run: v1alpha2.Command{"go", "generate", "./..."}}},
	}
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Contains(t, got.Body, "- Ran `scripts/generate-buildconfig`")
	assert.Contains(t, got.Body, "- Ran `go generate ./...`")
}

func TestRenderPreviewAPrereleaseKeepsItsTag(t *testing.T) {
	in := previewFixture(t)
	in.release = Release{Tag: "v0.6.0-rc.1", Stable: false}
	in.checks[0].Want = "rancher/scc-operator:v0.6.0-rc.1"
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "bot/scc-operator-v0.6.0-rc.1-v2.15", got.Branch)
}

func TestRenderPreviewHasTheCommitEachStepWouldMakeInTheOrderTheyRun(t *testing.T) {
	in := previewFixture(t)
	in.steps = []bump.StepRef{
		{Phase: bump.PreEdit, Step: chain.Step{Run: v1alpha2.Command{"scripts/prepare"}}},
		{Phase: bump.PostEdit, Step: chain.Step{Run: v1alpha2.Command{"scripts/generate-buildconfig"}}},
		{Phase: bump.PostEdit, Index: 1, Step: chain.Step{Run: v1alpha2.Command{"go", "generate", "./..."}}},
	}

	got, err := renderPreview(in)

	require.NoError(t, err)
	require.Len(t, got.Steps, 3)
	assert.Equal(t, "preEdit", got.Steps[0].Phase)
	assert.True(t, got.Steps[0].BeforeEdit, "a pre-edit step's commit comes before the edit's")
	assert.Equal(t, "scripts/prepare", got.Steps[0].Run)
	assert.False(t, got.Steps[1].BeforeEdit)
	assert.Equal(t, "go generate ./...", got.Steps[2].Run)
	for _, s := range got.Steps {
		assert.Contains(t, s.CommitMessage, "Run `"+s.Run+"`")
		assert.Contains(t, s.CommitMessage, "the "+s.Phase+" step")
		assert.Contains(t, s.CommitMessage, "Automation: gero ship")
	}
}

func TestRenderPreviewWithNoStepsHasNoStepCommits(t *testing.T) {
	got, err := renderPreview(previewFixture(t))

	require.NoError(t, err)
	assert.Empty(t, got.Steps)
}

func TestRenderPreviewTheStepMessagesAreTheOnesARunCommits(t *testing.T) {
	in := previewFixture(t)
	in.steps = []bump.StepRef{{Phase: bump.PostEdit, Index: 2, Step: chain.Step{Run: v1alpha2.Command{"gen", "--all"}}}}
	want, err := stepMessage(in)(bump.Unit{Kind: bump.UnitStep, Phase: bump.PostEdit, Index: 2, Run: []string{"gen", "--all"}})
	require.NoError(t, err)

	got, err := renderPreview(in)

	require.NoError(t, err)
	require.Len(t, got.Steps, 1)
	assert.Equal(t, want, got.Steps[0].CommitMessage)
}
