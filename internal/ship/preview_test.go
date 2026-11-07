package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func previewFixture() previewInput {
	return previewInput{
		producer: config.Project{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}},
		consumer: config.Project{Name: "rancher", Repo: config.Repo{Name: "rancher/rancher", DefaultBranch: "main"}},
		release:  Release{Tag: "v0.5.3", Commit: "sha", Stable: true},
		target:   Target{Branch: "release/v2.15", Commit: "sha-215"},
		check: updatedispatcher.Check{
			Current: "rancher/scc-operator:v0.5.2", Want: "rancher/scc-operator:v0.5.3",
			Change: updatedispatcher.Change{
				Path: "build.yaml", From: "rancher/scc-operator:v0.5.2", To: "rancher/scc-operator:v0.5.3",
				Description: "Updated `defaultSccOperatorImage` in `build.yaml` from `rancher/scc-operator:v0.5.2` to `rancher/scc-operator:v0.5.3`",
			},
		},
	}
}

func TestRenderPreviewDefaults(t *testing.T) {
	got, err := renderPreview(previewFixture())
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

func TestRenderPreviewBodyCarriesTheMarker(t *testing.T) {
	got, err := renderPreview(previewFixture())
	require.NoError(t, err)
	m, ok := ParseMarker(got.Body)
	require.True(t, ok)
	assert.Equal(t, Marker{Source: "rancher/scc-operator", Consumer: "rancher/rancher", Target: "release/v2.15", Version: "v0.5.3"}, m)
}

func TestRenderPreviewTheDevelopmentBranch(t *testing.T) {
	in := previewFixture()
	in.target = Target{Branch: "main"}
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "bot/scc-operator-v0.5.3-main", got.Branch)
	assert.Equal(t, "[main] Update scc-operator to v0.5.3", got.Title)
}

func TestRenderPreviewUsesTheProjectsTitleLabelsAndPrefix(t *testing.T) {
	in := previewFixture()
	in.consumer.PullRequests = config.PullRequests{
		Title:        "Bump {source} ({version}) on {targetBranch}",
		BranchPrefix: "auto/{project}/",
		Labels:       []string{"status/auto-created", "team/orbs"},
	}
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "Bump scc-operator (0.5.3) on release/v2.15", got.Title)
	assert.Equal(t, "auto/scc-operator/v0.5.3-v2.15", got.Branch)
	assert.Equal(t, []string{"status/auto-created", "team/orbs"}, got.Labels)
}

func TestRenderPreviewTheUsersPrefixReplacesTheProjectsWhole(t *testing.T) {
	in := previewFixture()
	in.consumer.PullRequests.BranchPrefix = "bot/{source}-"
	in.userPrefix = "dan/"
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "dan/v0.5.3-v2.15", got.Branch, "what the user leaves out is their call")
}

func TestRenderPreviewListsThePostEditCommands(t *testing.T) {
	in := previewFixture()
	in.steps = []updatedispatcher.StepRef{
		{Phase: updatedispatcher.MethodPostEdit, Step: config.Step{Run: config.Command{"scripts/generate-buildconfig"}}},
		{Phase: updatedispatcher.MethodPostEdit, Index: 1, Step: config.Step{Run: config.Command{"go", "generate", "./..."}}},
	}
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Contains(t, got.Body, "- Ran `scripts/generate-buildconfig`")
	assert.Contains(t, got.Body, "- Ran `go generate ./...`")
}

func TestRenderPreviewAPrereleaseKeepsItsTag(t *testing.T) {
	in := previewFixture()
	in.release = Release{Tag: "v0.6.0-rc.1", Stable: false}
	in.check.Want = "rancher/scc-operator:v0.6.0-rc.1"
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "bot/scc-operator-v0.6.0-rc.1-v2.15", got.Branch)
}

func TestRenderPreviewRejectsAnUnknownPlaceholder(t *testing.T) {
	in := previewFixture()
	in.consumer.PullRequests.Title = "Update {nope}"
	_, err := renderPreview(in)
	assert.ErrorContains(t, err, "nope")

	in = previewFixture()
	in.consumer.PullRequests.BranchPrefix = "{nope}/"
	_, err = renderPreview(in)
	assert.ErrorContains(t, err, "nope")
}

func TestRenderPreviewHasTheCommitEachStepWouldMakeInTheOrderTheyRun(t *testing.T) {
	in := previewFixture()
	in.steps = []updatedispatcher.StepRef{
		{Phase: updatedispatcher.MethodPreEdit, Step: config.Step{Run: config.Command{"scripts/prepare"}}},
		{Phase: updatedispatcher.EdgePostEdit, Step: config.Step{Run: config.Command{"scripts/generate-buildconfig"}}},
		{Phase: updatedispatcher.MethodPostEdit, Index: 1, Step: config.Step{Run: config.Command{"go", "generate", "./..."}}},
	}

	got, err := renderPreview(in)

	require.NoError(t, err)
	require.Len(t, got.Steps, 3)
	assert.Equal(t, "method preEdit", got.Steps[0].Phase)
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
	got, err := renderPreview(previewFixture())

	require.NoError(t, err)
	assert.Empty(t, got.Steps)
}

func TestRenderPreviewTheStepMessagesAreTheOnesARunCommits(t *testing.T) {
	in := previewFixture()
	in.steps = []updatedispatcher.StepRef{{Phase: updatedispatcher.EdgePostEdit, Index: 2, Step: config.Step{Run: config.Command{"gen", "--all"}}}}
	want, err := stepMessage(in)(updatedispatcher.Unit{Kind: updatedispatcher.UnitStep, Phase: updatedispatcher.EdgePostEdit, Index: 2, Run: []string{"gen", "--all"}})
	require.NoError(t, err)

	got, err := renderPreview(in)

	require.NoError(t, err)
	require.Len(t, got.Steps, 1)
	assert.Equal(t, want, got.Steps[0].CommitMessage)
}
