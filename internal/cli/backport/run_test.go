package backport

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	domain "github.com/rancher/orbs-automation/internal/backport"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWS is a project with two release branches and a PR on its host, and no clone: a run that
// goes no further than the check.
type fakeWS struct {
	graph  *chain.Graph
	merged bool
}

type host struct{ merged bool }

func (h host) Get(context.Context, string, int) (pulls.Detail, error) {
	return pulls.Detail{PR: pulls.PR{Number: 5, Title: "Fix", URL: "https://github.com/acme/cons/pull/5", Base: "main"}, Merged: h.merged, MergeCommit: "abc"}, nil
}
func (host) Commits(context.Context, string, int) ([]pulls.Commit, error) { return nil, nil }

func newFakeWS(t *testing.T, merged bool) fakeWS {
	t.Helper()
	return fakeWS{merged: merged, graph: chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: release/v1}]\n"))}
}

func (w fakeWS) Project(string) (*chain.Project, error) { return w.graph.Project("cons"), nil }
func (w fakeWS) PullRequests() domain.Host              { return host{w.merged} }
func (fakeWS) UpstreamRefs(context.Context, string) (git.Refs, error) {
	return git.Refs{Branches: []git.Branch{{Name: "main", Commit: "m1"}, {Name: "release/v1", Commit: "r1"}}}, nil
}
func (fakeWS) CloneDir(string) (string, error)           { return "", nil }
func (fakeWS) Remote(string) (string, error)             { return "upstream", nil }
func (fakeWS) PushSettings(string) proposal.PushSettings { return proposal.PushSettings{} }

func TestRunToWritesTheTextReportOfACheck(t *testing.T) {
	var out bytes.Buffer

	err := runTo(context.Background(), &out, newFakeWS(t, true), nil, clicfg.OutputText, domain.Input{Lines: []string{"v2.15"}, PR: 5}, false, style.Plain())

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Backport: acme/cons#5")
	assert.Contains(t, out.String(), "release/v1")
}

func TestRunToWritesTheJSONReportOfACheck(t *testing.T) {
	var out bytes.Buffer

	err := runTo(context.Background(), &out, newFakeWS(t, true), nil, clicfg.OutputJSON, domain.Input{Lines: []string{"v2.15"}, PR: 5}, false, style.Plain())

	require.NoError(t, err)
	var got struct {
		Repo    string
		Targets []struct{ Branch string }
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, "acme/cons", got.Repo)
	require.Len(t, got.Targets, 1)
	assert.Equal(t, "release/v1", got.Targets[0].Branch)
}

func TestRunToOfAPRThatIsNotMergedIsTheErrorAndNoReport(t *testing.T) {
	var out bytes.Buffer

	err := runTo(context.Background(), &out, newFakeWS(t, false), nil, clicfg.OutputText, domain.Input{Lines: []string{"v2.15"}, PR: 5}, false, style.Plain())

	require.ErrorContains(t, err, "not merged")
	assert.Empty(t, out.String(), "nothing was found, so there is no report to give")
}

func TestRunToRefusesAFormatItDoesNotWrite(t *testing.T) {
	err := runTo(context.Background(), &bytes.Buffer{}, newFakeWS(t, true), nil, "yaml", domain.Input{Lines: []string{"v2.15"}, PR: 5}, false, style.Plain())

	require.ErrorContains(t, err, "yaml")
}
