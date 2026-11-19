package ship

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/require"
)

// pinGraph is wrangler (wr), whose module pins lasso (la), and bro, which holds
// wrangler for v2.15 and newer. Lasso's tags are v0.2.*. The virtual Holding of
// lasso in bro is the first the graph makes.
func pinGraph(t *testing.T, laStream string, extra ...string) (*chain.Graph, *chain.Holding) {
	t.Helper()
	g := chaintest.Load(t, append([]string{
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "wr", "", "  repo: {name: acme/wr, managedBy: frameworks}\n"),
		chaintest.Doc("Project", "la", "", "  repo: {name: acme/la, managedBy: frameworks}\n"),
		chaintest.Doc("Project", "bro", "", "  repo: {name: acme/bro, managedBy: orbs}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r15}]\n"),
		chaintest.Doc("Stream", "wr", "{project: wr}", "  universal: true\n  tagPattern: \"v1.*\"\n"),
		chaintest.Doc("Stream", "la", "{project: la}", laStream),
		chaintest.Doc("Artifact", "la-mod", "{stream: la}", "  kind: gomod\n  name: github.com/acme/la\n"),
		chaintest.Doc("Artifact", "wr-mod", "{stream: wr, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr\n"),
		chaintest.Doc("Holding", "bro-holds-wr", "{project: bro, artifact: wr-mod}", "  at: {file: go.mod, key: github.com/acme/wr, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: {since: v2.15}}\n"),
	}, extra...)...)
	require.NotEmpty(t, g.Virtual())
	return g, g.Virtual()[0]
}

const laStream = "  universal: true\n  tagPattern: \"v0.2.*\"\n"

// pinHost is the host of the tests: files by "slug@ref:path" and the tags of each repository.
type pinHost struct {
	files    map[string]string
	tags     map[string][]git.Tag
	branches map[string][]git.Branch
	err      error
}

func (h pinHost) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	return git.Refs{Tags: h.tags[slug], Branches: h.branches[slug]}, h.err
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
