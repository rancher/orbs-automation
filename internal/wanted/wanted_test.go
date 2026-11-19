package wanted

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

// graph is wrangler (wr), whose module pins lasso (la), a producer of series (prod), and the
// holder (cons): it holds wrangler in go.mod (taking a prerelease too), lasso directly on the
// older line, and a series of prod.
func graph(t *testing.T) *chain.Graph {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "wr", "", "  repo: {name: acme/wr, external: true}\n"),
		chaintest.Doc("Project", "la", "", "  repo: {name: acme/la, external: true}\n"),
		chaintest.Doc("Project", "prod", "", "  repo: {name: acme/prod, managedBy: orbs}\n"),
		chaintest.Doc("Stream", "wr", "{project: wr}", "  scheme: semver\n  tagPattern: \"v1.*\"\n  universal: true\n"),
		chaintest.Doc("Stream", "la", "{project: la}", "  scheme: semver\n  tagPattern: \"v0.2.*\"\n  universal: true\n"),
		chaintest.Doc("Stream", "prod", "{project: prod}", "  lines:\n    - {line: v2.16, series: \"9.x\"}\n    - {line: v2.15, series: \"8.x\"}\n"),
		chaintest.Doc("Artifact", "la-mod", "{stream: la}", "  kind: gomod\n  name: github.com/acme/la\n"),
		chaintest.Doc("Artifact", "wr-mod", "{stream: wr, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr\n"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: release/v7.x}]\n"),
		chaintest.Doc("Holding", "cons-holds-wr", "{project: cons, artifact: wr-mod}", "  at: {file: go.mod, key: github.com/acme/wr, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all}\n"),
		chaintest.Doc("Holding", "cons-holds-la", "{project: cons, artifact: la-mod}", "  at: {file: go.mod, key: github.com/acme/la, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", lines: {until: v2.15}}\n"),
		chaintest.Doc("Holding", "cons-holds-prod", "{project: cons, stream: prod}", "  at: {file: build.yaml, key: prod}\n  holds: {kind: series, form: \"v{major}\"}\n"),
	)
}

func holding(t *testing.T, g *chain.Graph, name string) *chain.Holding {
	t.Helper()
	for _, h := range append(g.Holdings(), g.Virtual()...) {
		if h.Name() == name {
			return h
		}
	}
	require.Failf(t, "no holding", "%s", name)
	return nil
}

func tags(names ...string) []git.Tag {
	out := make([]git.Tag, len(names))
	for i, n := range names {
		out[i] = git.Tag{Name: n, Commit: "sha-" + n}
	}
	return out
}

// host is the world of the tests: the tags of each repository and files by "slug@ref:path".
type host struct {
	tags  map[string][]git.Tag
	files map[string]string
	reads map[string]int // calls of UpstreamRefs, by repository
}

func (h *host) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	h.reads[slug]++
	return git.Refs{Tags: h.tags[slug]}, nil
}

func (h *host) UpstreamFile(_ context.Context, slug, ref, path string) ([]byte, error) {
	if data, ok := h.files[slug+"@"+ref+":"+path]; ok {
		return []byte(data), nil
	}
	return nil, errors.New("file not found")
}

func requires(lasso string) string {
	return "module github.com/acme/wr\n\nrequire github.com/acme/la " + lasso + "\n"
}

func newHost() *host {
	return &host{
		tags: map[string][]git.Tag{
			"acme/wr": tags("v1.7.4", "v1.7.5"),
			"acme/la": tags("v0.2.4", "v0.2.5", "v0.2.6", "v0.2.7", "v0.2.8-rc1"),
		},
		files: map[string]string{
			"acme/wr@v1.7.4:go.mod": requires("v0.2.5"),
			"acme/wr@v1.7.5:go.mod": requires("v0.2.6"),
		},
		reads: map[string]int{},
	}
}

func TestForAReleaseHoldingIsTheNewestStableReleaseOfItsStream(t *testing.T) {
	g := graph(t)

	got := New(newHost()).For(bg, holding(t, g, "cons-holds-la"), "v2.15")

	require.NoError(t, got.Err)
	assert.Equal(t, "v0.2.7", got.Release.Tag, "the release candidate v0.2.8-rc1 is newer and never taken")
	assert.Equal(t, "acme/la", got.Release.SourceRepo)
	assert.Contains(t, got.Why, "newest")
	assert.False(t, got.Floor)
}

func TestForAReleaseHoldingThatAcceptsAllTakesAPrerelease(t *testing.T) {
	g := graph(t)
	h := newHost()
	h.tags["acme/wr"] = tags("v1.7.4", "v1.7.5", "v1.8.0-rc1")

	got := New(h).For(bg, holding(t, g, "cons-holds-wr"), "v2.16")

	require.NoError(t, got.Err)
	assert.Equal(t, "v1.8.0-rc1", got.Release.Tag)
}

func TestForAReleaseHoldingWithNoStableReleaseHasNoValue(t *testing.T) {
	g := graph(t)
	h := newHost()
	h.tags["acme/la"] = tags("v0.2.8-rc1")

	got := New(h).For(bg, holding(t, g, "cons-holds-la"), "v2.15")

	require.Error(t, got.Err)
	assert.Contains(t, got.Err.Error(), "no stable release")
	assert.Empty(t, got.Release.Tag)
}

func TestForASeriesHoldingIsTheSeriesItsStreamHasForTheLine(t *testing.T) {
	g := graph(t)

	got := New(newHost()).For(bg, holding(t, g, "cons-holds-prod"), "v2.15")

	require.NoError(t, got.Err)
	assert.Equal(t, "8.x", got.Series)
	assert.Contains(t, got.Why, "series")
	assert.Empty(t, got.Release.Tag, "a series holds no release")
}

func TestForTheVirtualHoldingAPinMakesIsTheReleaseTheNewestPinnerRequires(t *testing.T) {
	g := graph(t)
	require.Len(t, g.Virtual(), 1)

	got := New(newHost()).For(bg, g.Virtual()[0], "v2.16")

	require.NoError(t, got.Err)
	assert.Equal(t, "v0.2.6", got.Release.Tag, "what the go.mod of wrangler v1.7.5 requires, and not the newest lasso, v0.2.7")
	assert.Equal(t, "acme/wr v1.7.5", got.RequiredBy)
	assert.Contains(t, got.Why, "pin")
	assert.True(t, got.Floor, "a pin is a floor")
}

func TestForPinnerTagIsTheReleaseThatTagRequires(t *testing.T) {
	g := graph(t)

	got := New(newHost()).ForPinnerTag(bg, g.Virtual()[0], "v1.7.4")

	require.NoError(t, got.Err)
	assert.Equal(t, "v0.2.5", got.Release.Tag)
	assert.Equal(t, "acme/wr v1.7.4", got.RequiredBy)
	assert.True(t, got.Floor)
}

func TestForAPinNoReleaseCanBeMadeOfHasNoValue(t *testing.T) {
	g := graph(t)
	h := newHost()
	h.files["acme/wr@v1.7.5:go.mod"] = requires("v0.2.99") // a lasso no tag exists for

	got := New(h).For(bg, g.Virtual()[0], "v2.16")

	require.Error(t, got.Err)
	assert.Contains(t, got.Err.Error(), "no tag v0.2.99")
	assert.Empty(t, got.Release.Tag)
}

func TestTheTagsOfARepositoryAreReadOnce(t *testing.T) {
	g := graph(t)
	h := newHost()
	r := New(h)

	r.For(bg, holding(t, g, "cons-holds-la"), "v2.15")
	r.For(bg, holding(t, g, "cons-holds-la"), "v2.15")
	r.For(bg, g.Virtual()[0], "v2.16")
	r.ForPinnerTag(bg, g.Virtual()[0], "v1.7.4")

	assert.Equal(t, 1, h.reads["acme/la"])
	assert.Equal(t, 1, h.reads["acme/wr"])
}
