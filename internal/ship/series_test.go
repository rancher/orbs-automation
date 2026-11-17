package ship

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seriesWorld is a holder that holds the series of a producer's stream in build.yaml,
// one branch for each line, as the host serves it.
type seriesWorld struct {
	graph *chain.Graph
	refs  git.Refs
	files map[string]string // "ref:path"
}

func newSeriesWorld(t *testing.T, extra ...string) *seriesWorld {
	t.Helper()
	docs := append([]string{
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		chaintest.Doc("Project", "prod", "", "  repo: {name: acme/prod, managedBy: orbs}\n"),
		chaintest.Doc("Stream", "prod", "{project: prod}", "  lines:\n    - {line: v2.16, series: \"9.x\"}\n    - {line: v2.15, series: \"8.x\"}\n"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons, managedBy: orbs}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: rel-v2.15}, {line: v2.14, branch: rel-v2.14}]\n"),
		chaintest.Doc("Holding", "cons-holds-prod", "{project: cons, stream: prod}", "  at: {file: build.yaml, key: prodVersion}\n  holds: {kind: series, form: \"v{major}\"}\n"),
	}, extra...)
	return &seriesWorld{
		graph: chaintest.Load(t, docs...),
		refs: git.Refs{Branches: []git.Branch{
			{Name: "main", Commit: "sha-main"}, {Name: "rel-v2.15", Commit: "sha-215"}, {Name: "rel-v2.14", Commit: "sha-214"},
		}},
		files: map[string]string{
			"sha-main:build.yaml": "prodVersion: v9\n",
			"sha-215:build.yaml":  "prodVersion: v9\n",
		},
	}
}

func (w *seriesWorld) Project(key string) (*chain.Project, error)             { return w.graph.Project(key), nil }
func (w *seriesWorld) Bump() (*bump.Engine, error)                            { return bump.NewEngine(bump.Builtin(), w.graph) }
func (w *seriesWorld) UpstreamRefs(context.Context, string) (git.Refs, error) { return w.refs, nil }
func (w *seriesWorld) UpstreamFile(_ context.Context, _, ref, path string) ([]byte, error) {
	content, ok := w.files[ref+":"+path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return []byte(content), nil
}

func TestSeriesHoldingsSaysWhatEachBranchHoldsAgainstTheSeriesOfItsLine(t *testing.T) {
	w := newSeriesWorld(t)

	got, err := SeriesHoldings(ctx, w, w.graph.Project("cons"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "cons-holds-prod", got[0].Name)
	assert.Equal(t, "acme/prod", got[0].Producer)
	assert.Equal(t, "prod", got[0].Stream)
	require.Len(t, got[0].Lines, 2, "v2.14 has no series in the stream, so the holding is not for it")

	main, rel := got[0].Lines[0], got[0].Lines[1]
	assert.Equal(t, SeriesLine{Line: "v2.16", Branch: "main", Head: "sha-main", File: "build.yaml", Current: "v9", Want: "v9", UpToDate: true}, main)
	assert.Equal(t, "v2.15", rel.Line)
	assert.Equal(t, "rel-v2.15", rel.Branch)
	assert.Equal(t, "v9", rel.Current)
	assert.Equal(t, "v8", rel.Want)
	assert.False(t, rel.UpToDate, "v2.15 holds the series of v2.16")
	assert.Empty(t, rel.Suggest, "no edit follows the holding, so no pull request is suggested")
}

func TestSeriesHoldingsSuggestsAPullRequestWhenAnEditFollowsTheHolding(t *testing.T) {
	w := newSeriesWorld(t, chaintest.Doc("Edit", "cons-edit", "{holding: cons-holds-prod}", "  postEdit:\n    - {run: [make, generate]}\n"))

	got, err := SeriesHoldings(ctx, w, w.graph.Project("cons"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got[0].Lines, 2)
	assert.Equal(t, []string{"cons-edit"}, got[0].Edits)
	assert.Contains(t, got[0].Lines[1].Suggest, "rel-v2.15")
	assert.Contains(t, got[0].Lines[1].Suggest, "v8")
	assert.Contains(t, got[0].Lines[1].Suggest, "cons-edit")
	assert.Empty(t, got[0].Lines[0].Suggest, "a line that is held rightly needs none")
}

func TestSeriesHoldingsADifferenceTheDocumentAcknowledgesSaysWhy(t *testing.T) {
	w := newSeriesWorld(t)
	w.graph = chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		chaintest.Doc("Project", "prod", "", "  repo: {name: acme/prod, managedBy: orbs}\n"),
		chaintest.Doc("Stream", "prod", "{project: prod}", "  lines:\n    - {line: v2.16, series: \"9.x\"}\n    - {line: v2.15, series: \"8.x\"}\n"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons, managedBy: orbs}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: rel-v2.15}]\n"),
		"apiVersion: gero/v1alpha2\nkind: Holding\nname: cons-holds-prod\nrefs: {project: cons, stream: prod}\nacknowledge:\n  - {rule: series-differs, lines: {since: v2.15, until: v2.15}, reason: held back on purpose}\nspec:\n  at: {file: build.yaml, key: prodVersion}\n  holds: {kind: series, form: \"v{major}\"}\n",
	)

	got, err := SeriesHoldings(ctx, w, w.graph.Project("cons"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got[0].Lines, 2)
	assert.Equal(t, "held back on purpose", got[0].Lines[1].Acknowledged)
	assert.Empty(t, got[0].Lines[0].Acknowledged)
}

func TestSeriesHoldingsABranchThatCannotBeReadDoesNotStopTheOthers(t *testing.T) {
	w := newSeriesWorld(t)
	delete(w.files, "sha-215:build.yaml")

	got, err := SeriesHoldings(ctx, w, w.graph.Project("cons"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got[0].Lines, 2)
	assert.Contains(t, got[0].Lines[1].Error, "build.yaml")
	assert.True(t, got[0].Lines[0].UpToDate)
}

func TestAProjectWithNoSeriesHoldingHasNone(t *testing.T) {
	w := newSeriesWorld(t)

	got, err := SeriesHoldings(ctx, w, w.graph.Project("prod"))

	require.NoError(t, err)
	assert.Empty(t, got)
}
