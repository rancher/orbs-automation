package linesresolve

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
)

func TestNodeOfIsAProjectsOwnPlaceOnALine(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"),
		chaintest.Doc("Project", "a", "", "  repo: {name: acme/a}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines:\n      - {line: v2.16}\n      - {line: v2.15, branch: release/v2.15}\n      - {line: v2.14, branch: \"\"}\n      - {line: v2.13, branch: {dev: dev-v2.13, stable: release-v2.13}}\n"),
		chaintest.Doc("Project", "t", "", "  repo: {name: acme/t}\n"),
		chaintest.Doc("Stream", "a-one", "{project: a}", "  lines:\n    - {line: v2.15, series: \"0.9.x\"}\n"),
		chaintest.Doc("Stream", "t-one", "{project: t}", "  lines:\n    - {line: v2.16, series: \"1.x\"}\n"),
	)
	a, tagged := g.Project("a"), g.Project("t")

	tests := []struct {
		name string
		node Node
		line string
		ok   bool
		of   string
	}{
		{"a line on a branch of its own, with its series", Node{Project: "acme/a", Name: "a", Branch: "release/v2.15", Series: "0.9.x"}, "v2.15", true, "a"},
		{"a line on the default branch", Node{Project: "acme/a", Name: "a", Branch: "main", IsDefaultBranch: true}, "v2.16", true, "a"},
		{"a line with no branch yet has none", Node{Project: "acme/a", Name: "a"}, "v2.14", true, "a"},
		{"a line with a branch for each stage has no one branch", Node{Project: "acme/a", Name: "a"}, "v2.13", true, "a"},
		{"a line the project does not list", Node{}, "v2.99", false, "a"},
		{"a line found in the tags of a single layout", Node{Project: "acme/t", Name: "t", Branch: "main", IsDefaultBranch: true, Series: "1.x"}, "v2.16", true, "t"},
		{"a line no stream of a single layout lists", Node{}, "v2.15", false, "t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := a
			if tt.of == "t" {
				p = tagged
			}

			node, ok := nodeOf(p, tt.line)

			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.node, node)
		})
	}
}
