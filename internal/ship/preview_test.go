package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func previewFixture() previewInput {
	strategy, _ := LookupStrategy("yamlBump")
	c := riLink("yamlBump")
	return previewInput{
		producer: config.Project{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}},
		link:     c,
		params:   bindParams(c),
		release:  Release{Tag: "v0.5.3", Commit: "sha", Stable: true},
		target:   Target{Branch: "release/v2.15", Commit: "sha-215"},
		strategy: strategy,
		file:     "build.yaml",
		current:  "rancher/scc-operator:v0.5.2",
		want:     "rancher/scc-operator:v0.5.3",
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
	in.link.Consumer.PullRequests = config.PullRequests{
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
	in.link.Consumer.PullRequests.BranchPrefix = "bot/{source}-"
	in.userPrefix = "dan/"
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "dan/v0.5.3-v2.15", got.Branch, "what the user leaves out is their call")
}

func TestRenderPreviewListsThePostEditCommands(t *testing.T) {
	in := previewFixture()
	in.steps = []config.Step{{Run: config.Command{"scripts/generate-buildconfig"}}, {Run: config.Command{"go", "generate", "./..."}}}
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Contains(t, got.Body, "- Ran `scripts/generate-buildconfig`")
	assert.Contains(t, got.Body, "- Ran `go generate ./...`")
}

func TestRenderPreviewAPrereleaseKeepsItsTag(t *testing.T) {
	in := previewFixture()
	in.release = Release{Tag: "v0.6.0-rc.1", Stable: false}
	in.want = "rancher/scc-operator:v0.6.0-rc.1"
	got, err := renderPreview(in)
	require.NoError(t, err)
	assert.Equal(t, "bot/scc-operator-v0.6.0-rc.1-v2.15", got.Branch)
}

func TestRenderPreviewRejectsAnUnknownPlaceholder(t *testing.T) {
	in := previewFixture()
	in.link.Consumer.PullRequests.Title = "Update {nope}"
	_, err := renderPreview(in)
	assert.ErrorContains(t, err, "nope")

	in = previewFixture()
	in.link.Consumer.PullRequests.BranchPrefix = "{nope}/"
	_, err = renderPreview(in)
	assert.ErrorContains(t, err, "nope")
}
