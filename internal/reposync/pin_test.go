package reposync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/steprun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// consGoMod holds wrangler v1.7.4 and the lasso under what v1.7.5 requires.
	consGoMod = "module acme/cons\n\nrequire (\n\tgithub.com/acme/la v0.2.3\n\tgithub.com/acme/wr v1.7.4\n)\n"
	// wrRequires is the go.mod of a wrangler tag, which requires the lasso given.
	wrRequires = "module github.com/acme/wr\n\nrequire github.com/acme/la %s\n"
)

// pinDocs is wrangler (wr), whose module pins lasso (la), and the holder, which holds
// wrangler in its go.mod on every line, with the Edit after it.
func pinDocs(edit bool) []string {
	docs := []string{
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "wr", "", "  repo: {name: acme/wr, external: true}\n"),
		chaintest.Doc("Project", "la", "", "  repo: {name: acme/la, external: true}\n"),
		chaintest.Doc("Stream", "wr", "{project: wr}", "  scheme: semver\n  tagPattern: \"v1.*\"\n  universal: true\n"),
		chaintest.Doc("Stream", "la", "{project: la}", "  scheme: semver\n  tagPattern: \"v0.2.*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "la-mod", "{stream: la}", "  kind: gomod\n  name: github.com/acme/la\n"),
		chaintest.Doc("Artifact", "wr-mod", "{stream: wr, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr\n"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: release/v7.x}]\n"),
		chaintest.Doc("Holding", "cons-holds-wr", "{project: cons, artifact: wr-mod}", "  at: {file: go.mod, key: github.com/acme/wr, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all}\n"),
	}
	if edit {
		docs = append(docs, chaintest.Doc("Edit", "regen", "{holding: [cons-holds-wr]}", "  postEdit:\n    - {run: ./scripts/gen.sh}\n"))
	}
	return docs
}

// pinWS is the holder served from a real repository, and the producers' tags and files.
type pinWS struct {
	*seriesWS
	files map[string]string // by "slug@ref:path"
}

func (w pinWS) UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error) {
	if data, ok := w.files[slug+"@"+ref+":"+path]; ok {
		return []byte(data), nil
	}
	return w.seriesWS.UpstreamFile(ctx, slug, ref, path)
}

// newPinWS is the holder holding goMod on main and on release/v7.x, wrangler v1.7.5 the newest
// release on the host and the lasso it requires, v0.2.6, the newest of lasso.
func newPinWS(t *testing.T, goMod string, edit bool) (pinWS, *proposal.Executor) {
	t.Helper()
	up := remote(t)
	runGit(t, up, "checkout", "--quiet", "main")
	put(t, up, "go.mod", goMod)
	put(t, up, "scripts/gen.sh", "#!/bin/sh\nmkdir -p generated && printf %s \"$GERO_TARGET_BRANCH\" > generated/branch.txt\n")
	require.NoError(t, os.Chmod(filepath.Join(up, "scripts", "gen.sh"), 0o755))
	runGit(t, up, "add", ".")
	runGit(t, up, "commit", "--quiet", "-m", "go.mod and script")
	runGit(t, up, "branch", "release/v7.x")
	ws := pinWS{
		seriesWS: &seriesWS{
			t: t, graph: chaintest.Load(t, pinDocs(edit)...), up: up, clone: engineer(t, up),
			tags: map[string][]git.Tag{
				"acme/wr": tagsOf("v1.7.4", "v1.7.5"),
				"acme/la": tagsOf("v0.2.3", "v0.2.5", "v0.2.6", "v0.2.7"),
			},
		},
		files: map[string]string{
			"acme/wr@v1.7.5:go.mod": wrRequiresLasso("v0.2.6"),
			"acme/wr@v1.7.4:go.mod": wrRequiresLasso("v0.2.5"),
		},
	}
	return ws, &proposal.Executor{Git: proposal.GitCLI{}, Runner: steprun.Runner{}, TempDir: t.TempDir()}
}

func wrRequiresLasso(version string) string { return fmt.Sprintf(wrRequires, version) }

func rowNamed(t *testing.T, b Branch, name string) Row {
	t.Helper()
	for _, r := range b.Rows {
		if r.Holding == name {
			return r
		}
	}
	require.Failf(t, "no row", "%s in %+v", name, b.Rows)
	return Row{}
}

const (
	wrHolding = "cons-holds-wr"
	laHolding = "cons-holds-wr+la-mod"
)

func TestSyncPullsTheCompanionAHoldingThatPinsAnotherArtifactRequires(t *testing.T) {
	ws, _ := newPinWS(t, consGoMod, true)

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	b := branchOf(t, plan, "main")
	assert.Equal(t, StatusReady, b.Status)
	wr, la := rowNamed(t, b, wrHolding), rowNamed(t, b, laHolding)
	assert.Equal(t, StatusReady, wr.Status, "wrangler is not left to ship")
	assert.Equal(t, "v1.7.5", wr.Want)
	assert.Equal(t, StatusReady, la.Status)
	assert.Equal(t, "v0.2.3", la.Current)
	assert.Equal(t, "v0.2.6", la.Want, "what the go.mod of wrangler v1.7.5 requires, and not the newest lasso, v0.2.7")
	assert.Equal(t, "main", la.Branch)
	assert.Contains(t, b.Preview.Body, "acme/wr")
}

func TestSyncWritesAHoldingAndItsCompanionInOneCommit(t *testing.T) {
	ws, ex := newPinWS(t, consGoMod, true)

	plan, err := Run(bg, ws, Input{NoPush: true}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, "main")
	require.NotNil(t, b.Result)
	assert.Equal(t, proposal.ResultCommitted, b.Result.Status)
	got := runGit(t, ws.clone, "show", b.Result.Branch+":go.mod")
	assert.Contains(t, got, "github.com/acme/wr v1.7.5")
	assert.Contains(t, got, "github.com/acme/la v0.2.6")
	assert.Equal(t, consGoMod, slurp(t, filepath.Join(ws.clone, "go.mod")), "the engineer's checkout is untouched")
}

func TestSyncRaisesACompanionBelowWhatTheHeldHoldingRequires(t *testing.T) {
	held := "module acme/cons\n\nrequire (\n\tgithub.com/acme/la v0.2.3\n\tgithub.com/acme/wr v1.7.5\n)\n"
	ws, _ := newPinWS(t, held, true)

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	b := branchOf(t, plan, "main")
	assert.Equal(t, StatusUpToDate, rowNamed(t, b, wrHolding).Status)
	la := rowNamed(t, b, laHolding)
	assert.Equal(t, StatusReady, la.Status, "wrangler is current, and the lasso below what it requires is the whole PR")
	assert.Equal(t, "v0.2.6", la.Want)
	assert.Equal(t, StatusReady, b.Status)
}

func TestSyncTakesACompanionNewerThanTheHeldHoldingRequiresAsCurrent(t *testing.T) {
	held := "module acme/cons\n\nrequire (\n\tgithub.com/acme/la v0.2.7\n\tgithub.com/acme/wr v1.7.5\n)\n"
	ws, _ := newPinWS(t, held, true)

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	b := branchOf(t, plan, "main")
	assert.Equal(t, StatusUpToDate, rowNamed(t, b, laHolding).Status, "a pin is a floor: a newer lasso holds it")
	assert.Equal(t, StatusUpToDate, b.Status)
}

func TestSyncDoesNotWriteWranglerWhenItsCompanionCannotBeDerived(t *testing.T) {
	ws, _ := newPinWS(t, consGoMod, true)
	ws.files["acme/wr@v1.7.5:go.mod"] = wrRequiresLasso("v0.2.99") // a lasso no tag exists for

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	b := branchOf(t, plan, "main")
	la := rowNamed(t, b, laHolding)
	assert.Equal(t, StatusFailed, la.Status)
	assert.Contains(t, la.Reason, "no tag v0.2.99")
	wr := rowNamed(t, b, wrHolding)
	assert.Equal(t, StatusBlocked, wr.Status, "wrangler alone would put go.mod out of step with what it requires")
	assert.Contains(t, wr.Reason, "companion")
	assert.Nil(t, b.Preview)
}

func TestSyncDoesNotWriteAPinnedHoldingNoEditFollows(t *testing.T) {
	ws, ex := newPinWS(t, consGoMod, false)

	plan, err := Run(bg, ws, Input{NoPush: true}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, "main")
	for _, name := range []string{wrHolding, laHolding} {
		r := rowNamed(t, b, name)
		assert.Equal(t, StatusBlocked, r.Status, name)
		assert.Contains(t, r.Reason, "no Edit follows", name)
	}
	assert.Nil(t, b.Result)
}

func TestSyncLimitedToAPinningHoldingKeepsItsCompanion(t *testing.T) {
	ws, _ := newPinWS(t, consGoMod, true)

	plan, err := Run(bg, ws, Input{NoPush: true, Holdings: []string{wrHolding}}, nil)

	require.NoError(t, err)
	b := branchOf(t, plan, "main")
	assert.Len(t, b.Rows, 2)
	assert.Equal(t, "v0.2.6", rowNamed(t, b, laHolding).Want)
}
