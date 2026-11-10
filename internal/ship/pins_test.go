package ship

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

// pinGraph is wrangler (wr), whose module pins lasso (la), and bro, which holds
// wrangler for v2.15 and newer. Lasso's tags are v0.2.*. The virtual Holding of
// lasso in bro is the first the graph makes.
func pinGraph(t *testing.T, laStream string) (*chain.Graph, *chain.Holding) {
	t.Helper()
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "wr", "", "  repo: {name: acme/wr, managedBy: frameworks}\n"),
		chaintest.Doc("Project", "la", "", "  repo: {name: acme/la, managedBy: frameworks}\n"),
		chaintest.Doc("Project", "bro", "", "  repo: {name: acme/bro, managedBy: orbs}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r15}]\n"),
		chaintest.Doc("Stream", "wr", "{project: wr}", "  universal: true\n  tagPattern: \"v1.*\"\n"),
		chaintest.Doc("Stream", "la", "{project: la}", laStream),
		chaintest.Doc("Artifact", "la-mod", "{stream: la}", "  kind: gomod\n  name: github.com/acme/la\n"),
		chaintest.Doc("Artifact", "wr-mod", "{stream: wr, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr\n"),
		chaintest.Doc("Holding", "bro-holds-wr", "{project: bro, artifact: wr-mod}", "  at: {file: go.mod, key: github.com/acme/wr, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: {since: v2.15}}\n"),
	)
	require.Len(t, g.Virtual(), 1)
	return g, g.Virtual()[0]
}

const laStream = "  universal: true\n  tagPattern: \"v0.2.*\"\n"

// pinHost is the host of the tests: files by "slug@ref:path" and the tags of each repository.
type pinHost struct {
	files map[string]string
	tags  map[string][]git.Tag
	err   error
}

func (h pinHost) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	return git.Refs{Tags: h.tags[slug]}, h.err
}

func (h pinHost) UpstreamFile(_ context.Context, slug, ref, path string) ([]byte, error) {
	if data, ok := h.files[slug+"@"+ref+":"+path]; ok {
		return []byte(data), nil
	}
	return nil, errors.New("file not found")
}

func lassoHost(requires string) pinHost {
	return pinHost{
		files: map[string]string{"acme/wr@v1.7.4:go.mod": "module github.com/acme/wr\n\nrequire github.com/acme/la " + requires + "\n"},
		tags:  map[string][]git.Tag{"acme/la": tags("v0.2.4", "v0.2.5", "v0.2.6")},
	}
}

func TestDeriveTakesTheReleaseOfThePinnedModuleFromTheGoModOfTheTag(t *testing.T) {
	_, v := pinGraph(t, laStream)

	got := derive(ctx, lassoHost("v0.2.5"), v, "v1.7.4")

	assert.Empty(t, got.Error)
	assert.Equal(t, "v0.2.5", got.Tag)
	assert.Equal(t, "v0.2.5", got.Version)
	assert.Equal(t, "acme/la", got.Producer)
	assert.Equal(t, "github.com/acme/la", got.Module)
	assert.Equal(t, "acme/wr v1.7.4", got.RequiredBy)
	assert.Equal(t, "go.mod", got.File)
}

func TestDeriveReadsTheFileThePinNames(t *testing.T) {
	g, v := pinGraph(t, laStream)
	require.Equal(t, "go.mod", g.Project("wr").Artifacts()[0].PinFile())
	host := lassoHost("v0.2.5")
	host.files["acme/wr@v1.7.5:go.mod"] = "module github.com/acme/wr\n\nrequire github.com/acme/la v0.2.6\n"

	assert.Equal(t, "v0.2.6", derive(ctx, host, v, "v1.7.5").Tag, "the file of the tag asked for")
}

func TestDeriveRefusesWhatNoReleaseCanBeMadeOf(t *testing.T) {
	tests := []struct {
		name     string
		host     func() pinHost
		stream   string
		pinner   string
		contains string
	}{
		{"a tag whose go.mod is not there", func() pinHost { return lassoHost("v0.2.5") }, laStream, "v1.9.9", "read go.mod"},
		{"a go.mod that does not require the module", func() pinHost {
			h := lassoHost("v0.2.5")
			h.files["acme/wr@v1.7.4:go.mod"] = "module github.com/acme/wr\n"
			return h
		}, laStream, "v1.7.4", "does not require"},
		{"a pseudo-version, which has no tag", func() pinHost { return lassoHost("v0.2.6-0.20260102030405-abcdef123456") }, laStream, "v1.7.4", "pseudo-version"},
		{"a version of another stream", func() pinHost { return lassoHost("v0.3.0") }, laStream, "v1.7.4", "not a release of the stream"},
		{"a version the repository has no tag for", func() pinHost { return lassoHost("v0.2.9") }, laStream, "v1.7.4", "no tag v0.2.9"},
		{"a host that cannot be asked for tags", func() pinHost { h := lassoHost("v0.2.5"); h.err = errors.New("rate limited"); return h }, laStream, "v1.7.4", "rate limited"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := pinGraph(t, tt.stream)

			got := derive(ctx, tt.host(), v, tt.pinner)

			assert.Contains(t, got.Error, tt.contains)
			assert.Empty(t, got.Tag, "no release was derived")
		})
	}
}

func TestDeriveMakesATagWithThePrefixOfTheStream(t *testing.T) {
	_, v := pinGraph(t, "  universal: true\n  tagPattern: \"sub/v0.2.*\"\n")
	host := lassoHost("v0.2.5")
	host.tags["acme/la"] = tags("sub/v0.2.4", "sub/v0.2.5")

	got := derive(ctx, host, v, "v1.7.4")

	assert.Empty(t, got.Error)
	assert.Equal(t, "sub/v0.2.5", got.Tag, "the version is the bare tag, and the stream's prefix goes before it")
	assert.Equal(t, "v0.2.5", got.Version)
}
