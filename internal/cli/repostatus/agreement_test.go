package repostatus

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/reposync"
	"github.com/rancher/orbs-automation/internal/ship"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// broWorld is what the lasso and wrangler config is for: a holder with a branch for each of three lines
// holds wrangler by minor, each minor pinning lasso, and a lasso of its own on the oldest line, which no
// wrangler holding covers. gero repo status and gero repo sync both read it, and must agree on it.
type broWorld struct {
	g        *chain.Graph
	tags     map[string][]git.Tag
	branches []git.Branch
	files    map[string]string // by "slug@ref:path"
}

func goMod(wr, la string) string {
	return "module acme/bro\n\nrequire (\n\tgithub.com/acme/la " + la + "\n\tgithub.com/acme/wr/v3 " + wr + "\n)\n"
}

func gitTags(names ...string) []git.Tag {
	out := make([]git.Tag, len(names))
	for i, n := range names {
		out[i] = git.Tag{Name: n, Commit: "sha-" + n}
	}
	return out
}

func newBroWorld(t *testing.T) *broWorld {
	t.Helper()
	const hold = "  at: {file: go.mod, key: %s, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all%s}\n"
	wrHold := func(lines string) string {
		return "  at: {file: go.mod, key: github.com/acme/wr/v3, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: " + lines + "}\n"
	}
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		chaintest.Doc("Project", "wr", "", "  repo: {name: acme/wr, external: true}\n"),
		chaintest.Doc("Project", "la", "", "  repo: {name: acme/la, external: true}\n"),
		chaintest.Doc("Stream", "wr-8", "{project: wr}", "  scheme: semver\n  tagPattern: \"v3.8.*\"\n  universal: true\n"),
		chaintest.Doc("Stream", "wr-7", "{project: wr}", "  scheme: semver\n  tagPattern: \"v3.7.*\"\n  universal: true\n"),
		chaintest.Doc("Stream", "la", "{project: la}", "  scheme: semver\n  tagPattern: \"v0.2.*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "la-mod", "{stream: la}", "  kind: gomod\n  name: github.com/acme/la\n"),
		chaintest.Doc("Artifact", "wr-8-mod", "{stream: wr-8, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr/v3\n"),
		chaintest.Doc("Artifact", "wr-7-mod", "{stream: wr-7, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr/v3\n"),
		chaintest.Doc("Project", "bro", "", "  repo: {name: acme/bro, managedBy: orbs}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r15}, {line: v2.14, branch: r14}]\n"),
		chaintest.Doc("Holding", "bro-holds-wr-8", "{project: bro, artifact: wr-8-mod}", wrHold("{since: v2.16}")),
		chaintest.Doc("Holding", "bro-holds-wr-7", "{project: bro, artifact: wr-7-mod}", wrHold("{since: v2.15, until: v2.15}")),
		chaintest.Doc("Holding", "bro-holds-la", "{project: bro, artifact: la-mod}", fmtHold(hold, "github.com/acme/la", ", lines: {until: v2.14}")),
		chaintest.Doc("Edit", "tidy", "{holding: [bro-holds-wr-8, bro-holds-wr-7, bro-holds-la]}", "  postEdit:\n    - {run: [go, generate]}\n"),
	)
	return &broWorld{
		g: g,
		tags: map[string][]git.Tag{
			"acme/wr": gitTags("v3.7.1", "v3.7.2", "v3.8.0", "v3.8.1-rc.1"),
			"acme/la": gitTags("v0.2.7", "v0.2.8", "v0.2.9", "v0.2.10"),
		},
		branches: []git.Branch{{Name: "main", Commit: "sha-main"}, {Name: "r15", Commit: "sha-r15"}, {Name: "r14", Commit: "sha-r14"}},
		files: map[string]string{
			"acme/wr@v3.7.2:go.mod":      "module github.com/acme/wr/v3\n\nrequire github.com/acme/la v0.2.9\n",
			"acme/wr@v3.8.0:go.mod":      "module github.com/acme/wr/v3\n\nrequire github.com/acme/la v0.2.10\n",
			"acme/wr@v3.8.1-rc.1:go.mod": "module github.com/acme/wr/v3\n\nrequire github.com/acme/la v0.2.10\n",
			"acme/bro@sha-main:go.mod":   goMod("v3.8.0", "v0.2.10"),
			"acme/bro@sha-r15:go.mod":    goMod("v3.7.2", "v0.2.8"),
			"acme/bro@sha-r14:go.mod":    goMod("v3.6.1", "v0.2.7"),
		},
	}
}

func fmtHold(format, key, lines string) string {
	return "  at: {file: go.mod, key: " + key + ", format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all" + lines + "}\n"
}

func (w *broWorld) Project(key string) (*chain.Project, error) {
	if key == "" {
		key = "bro"
	}
	return w.g.Project(key), nil
}
func (w *broWorld) Bump() (*bump.Engine, error) { return bump.NewEngine(bump.Builtin(), w.g) }
func (w *broWorld) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	if slug == "acme/bro" {
		return git.Refs{Branches: w.branches}, nil
	}
	return git.Refs{Tags: w.tags[slug]}, nil
}
func (w *broWorld) UpstreamFile(_ context.Context, slug, ref, path string) ([]byte, error) {
	if data, ok := w.files[slug+"@"+ref+":"+path]; ok {
		return []byte(data), nil
	}
	return nil, errors.New("file not found")
}
func (w *broWorld) CloneDir(string) (string, error)           { return "", nil }
func (w *broWorld) Remote(string) (string, error)             { return "upstream", nil }
func (w *broWorld) PushSettings(string) proposal.PushSettings { return proposal.PushSettings{} }

// cell is what one holding of one branch is expected to hold.
type cell struct{ holding, line, branch string }

// statusWants is what repo status says each holding of the branches is expected to hold: the floor a pin puts
// on a holding it makes, and for any other the newest release when the holding takes a prerelease, else the latest stable.
func statusWants(t *testing.T, w *broWorld) map[cell]string {
	t.Helper()
	srcs, err := ship.Sources(context.Background(), w, w.g.Project("bro"))
	require.NoError(t, err)
	acceptsAll := map[string]bool{}
	for _, h := range append(w.g.Holdings(), w.g.Virtual()...) {
		acceptsAll[h.Name()] = h.AcceptsAll()
	}
	out := map[cell]string{}
	for _, src := range srcs {
		require.Empty(t, src.Error, src.Producer)
		for _, tgt := range src.Targets {
			for _, held := range tgt.Held {
				k := cell{held.Name, tgt.Line, tgt.Branch}
				switch {
				case held.Pin != nil:
					require.NotNil(t, held.Pin.Wanted, k)
					require.NotNil(t, held.Pin.Wanted.Verdict, "%v: %s", k, held.Pin.Wanted.Error)
					out[k] = held.Pin.Wanted.Verdict.Want
				case acceptsAll[held.Name] && held.Newest != nil:
					out[k] = held.Newest.Want
				case held.Stable != nil:
					out[k] = held.Stable.Want
				}
			}
		}
	}
	return out
}

// syncWants is what repo sync says each holding of the branches is expected to hold.
func syncWants(t *testing.T, w *broWorld) map[cell]string {
	t.Helper()
	plan, err := reposync.Run(context.Background(), w, reposync.Input{NoPush: true}, nil)
	require.NoError(t, err)
	out := map[cell]string{}
	for _, b := range plan.Branches {
		for _, r := range b.Rows {
			require.Empty(t, r.Error, "%s on %s", r.Holding, b.Branch)
			out[cell{r.Holding, r.Line, r.Branch}] = r.Want
		}
	}
	return out
}

func TestStatusAndSyncAgreeOnWhatEveryHoldingOfABranchIsExpectedToHold(t *testing.T) {
	w := newBroWorld(t)

	status, sync := statusWants(t, w), syncWants(t, w)

	assert.NotEmpty(t, sync)
	assert.Equal(t, sync, status, "the same lines, branches and values: wrangler by minor, the lasso each requires, and the lasso of the oldest line")
	assert.Equal(t, "v0.2.9", sync[cell{"bro-holds-wr-7+la-mod", "v2.15", "r15"}], "lasso is what wrangler v3.7.2 requires, and not the newest, v0.2.10")
	assert.Equal(t, "v3.8.1-rc.1", sync[cell{"bro-holds-wr-8", "v2.16", "main"}], "a holding that takes a prerelease is expected to hold the newest release")
}
