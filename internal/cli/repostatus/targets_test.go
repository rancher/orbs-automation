package repostatus

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
)

const perLineStages = `  repo: {name: acme/a}
  branches:
    layout: perLine
    stages: [dev, stable]
    lines:
      - {line: v2.16}
      - {line: v2.15, branch: release/v2.15}
      - {line: v2.14, branch: ""}
      - {line: v2.13, branch: {dev: dev-v2.13, stable: release-v2.13}}
`

func TestTargetsAreTheReleaseBranchesOfAProject(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"),
		chaintest.Doc("Project", "a", "", perLineStages),
		chaintest.Doc("Project", "s", "", "  repo: {name: acme/s}\n"),
		chaintest.Doc("Project", "t", "", "  repo: {name: acme/t}\n"),
		chaintest.Doc("Stream", "a-one", "{project: a}", "  lines:\n    - {line: v2.15, series: \"0.9.x\"}\n    - {line: v2.13, series: \"0.7.x\"}\n"),
		chaintest.Doc("Stream", "t-one", "{project: t}", "  lines:\n    - {line: v2.16, series: \"1.x\"}\n    - {line: v2.15, series: \"0.9.x\"}\n"),
	)

	t.Run("a branch for each line, and for each stage of a line that has one for each", func(t *testing.T) {
		assert.Equal(t, []Target{
			{Line: "v2.16", Branch: "main", IsDefaultBranch: true},
			{Line: "v2.15", Series: "0.9.x", Branch: "release/v2.15"},
			{Line: "v2.14"},
			{Line: "v2.13", Series: "0.7.x", Stage: "dev", Branch: "dev-v2.13"},
			{Line: "v2.13", Series: "0.7.x", Stage: "stable", Branch: "release-v2.13"},
		}, targets(g.Project("a")))
	})
	t.Run("one branch with a row for each line its streams list", func(t *testing.T) {
		assert.Equal(t, []Target{
			{Line: "v2.16", Series: "1.x", Branch: "main", IsDefaultBranch: true},
			{Line: "v2.15", Series: "0.9.x", Branch: "main", IsDefaultBranch: true},
		}, targets(g.Project("t")))
	})
	t.Run("one branch and no line", func(t *testing.T) {
		assert.Equal(t, []Target{{Branch: "main", IsDefaultBranch: true}}, targets(g.Project("s")))
	})
}
