package repoclone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

var bg = context.Background()

type fakeWorkspace struct {
	upstream string
	paths    map[string]workspace.RepoPath // by slug; a slug not here is not found
}

func (f *fakeWorkspace) UpstreamFor(string) string { return f.upstream }

func (f *fakeWorkspace) ConfiguredRepoPath(slug string) (workspace.RepoPath, error) {
	if rp, ok := f.paths[slug]; ok {
		return rp, rp.Err
	}
	return workspace.RepoPath{Slug: slug, Err: workspace.ErrRepoNotFound}, workspace.ErrRepoNotFound
}

// fakeSource serves a graph of Projects.
type fakeSource struct{ g *chain.Graph }

func (f fakeSource) Project(key string) (*chain.Project, error) {
	if p := f.g.Project(key); p != nil {
		return p, nil
	}
	return nil, fmt.Errorf("%q is %w", key, workspace.ErrUnknownProject)
}

func (f fakeSource) Projects() ([]*chain.Project, error) { return f.g.Projects(), nil }

// fakeGit records what it was asked to do, in order.
type fakeGit struct {
	calls    []string
	remotes  map[string]string // "dir|remote" to the URL it has
	cloneErr map[string]error  // by URL
}

// first is the first call, or a marker when there was none, so a test that
// expects one fails on its assertion and does not panic.
func (f *fakeGit) first() string {
	if len(f.calls) == 0 {
		return "(no git call)"
	}
	return f.calls[0]
}

func (f *fakeGit) Clone(_ context.Context, url, dir, remote string) error {
	f.calls = append(f.calls, fmt.Sprintf("clone %s %s %s", url, dir, remote))
	return f.cloneErr[url]
}

func (f *fakeGit) AddRemote(_ context.Context, dir, name, url string) error {
	f.calls = append(f.calls, fmt.Sprintf("add %s %s %s", dir, name, url))
	return nil
}

func (f *fakeGit) RemoteURL(_ context.Context, dir, name string) (string, error) {
	return f.remotes[dir+"|"+name], nil
}

// fakeForks says which users have a fork of which repository.
type fakeForks struct {
	have map[string]bool // "user slug"
	err  error
}

func (f fakeForks) ForkExists(_ context.Context, user, slug string) (bool, error) {
	return f.have[user+" "+slug], f.err
}

const (
	rancher = "rancher/rancher"
	scc     = "rancher/scc-operator"
	ext     = "third-party/sdk"
)

func projects(t *testing.T) fakeSource {
	t.Helper()
	return fakeSource{chaintest.Load(t,
		chaintest.Doc("Project", "rancher", "", "  repo: {name: "+rancher+", owned: true}\n"),
		chaintest.Doc("Project", "scc-operator", "", "  repo: {name: "+scc+", owned: true}\n"),
		chaintest.Doc("Project", "sdk", "", "  repo: {name: "+ext+", external: true}\n"),
		chaintest.Doc("Project", "other", "", "  repo: {name: rancher/other}\n"),
	)}
}

type rig struct {
	t     *testing.T
	root  string
	ws    *fakeWorkspace
	git   *fakeGit
	forks fakeForks
	s     clicfg.Settings
}

// newRig is a workstation with a clone root, an upstream remote named upstream and
// no fork configured.
func newRig(t *testing.T) *rig {
	t.Helper()
	root := t.TempDir()
	return &rig{
		t: t, root: root,
		ws:    &fakeWorkspace{upstream: "upstream", paths: map[string]workspace.RepoPath{}},
		git:   &fakeGit{remotes: map[string]string{}, cloneErr: map[string]error{}},
		forks: fakeForks{have: map[string]bool{}},
		s:     clicfg.Settings{Output: clicfg.OutputText, Upstream: "upstream", Repos: clicfg.Repos{CloneRoot: root}},
	}
}

func (r *rig) withFork() *rig {
	r.s.Fork, r.s.GitHubUser = "origin", "danpock"
	return r
}

func (r *rig) do(opts Options) (string, error) {
	r.t.Helper()
	var buf bytes.Buffer
	d := Deps{Workspace: r.ws, Source: projects(r.t), Git: r.git, Forks: r.forks}
	err := run(bg, &buf, d, r.s, r.t.TempDir(), opts)
	return buf.String(), err
}

func named(keys ...string) Options { return Options{Projects: keys} }

func TestRunClonesTheUpstreamOverSSHUnderTheRootWhenNoForkIsConfigured(t *testing.T) {
	r := newRig(t)

	out, err := r.do(named("rancher"))

	require.NoError(t, err)
	assert.Equal(t, []string{
		"clone git@github.com:rancher/rancher.git " + filepath.Join(r.root, "rancher") + " upstream",
	}, r.git.calls)
	assert.Contains(t, out, "Cloned rancher/rancher into "+filepath.Join(r.root, "rancher"))
}

func TestRunTheDirectoryIsNamedAfterTheRepositoryNotTheProject(t *testing.T) {
	r := newRig(t)

	_, err := r.do(named("scc-operator"))

	require.NoError(t, err)
	assert.Contains(t, r.git.first(), " "+filepath.Join(r.root, "scc-operator")+" ")
}

func TestRunClonesTheForkAndAddsTheUpstreamWhenTheForkExists(t *testing.T) {
	r := newRig(t).withFork()
	r.forks.have["danpock "+rancher] = true
	dir := filepath.Join(r.root, "rancher")

	out, err := r.do(named("rancher"))

	require.NoError(t, err)
	assert.Equal(t, []string{
		"clone git@github.com:danpock/rancher.git " + dir + " origin",
		"add " + dir + " upstream git@github.com:rancher/rancher.git",
	}, r.git.calls)
	assert.Contains(t, out, "(origin, upstream)")
}

func TestRunClonesTheUpstreamAloneWhenTheConfiguredForkDoesNotExistAndSaysSo(t *testing.T) {
	r := newRig(t).withFork()

	out, err := r.do(named("rancher"))

	require.NoError(t, err)
	assert.Equal(t, []string{
		"clone git@github.com:rancher/rancher.git " + filepath.Join(r.root, "rancher") + " upstream",
	}, r.git.calls)
	assert.Contains(t, out, "danpock has no fork of rancher/rancher")
	assert.Contains(t, out, "cloned the upstream alone")
}

func TestRunAFailedForkLookupFailsThatProjectAndClonesNothing(t *testing.T) {
	r := newRig(t).withFork()
	r.forks.err = errors.New("rate limited")

	out, err := r.do(named("rancher"))

	require.Error(t, err)
	assert.Empty(t, r.git.calls)
	assert.Contains(t, out, "Failed rancher/rancher")
	assert.Contains(t, out, "rate limited")
}

func TestRunAllIsTheOwnedProjectsThatAreNotExternal(t *testing.T) {
	r := newRig(t)

	out, err := r.do(Options{All: true})

	require.NoError(t, err)
	assert.Len(t, r.git.calls, 2)
	assert.Contains(t, out, "Cloned rancher/rancher")
	assert.Contains(t, out, "Cloned rancher/scc-operator")
	assert.NotContains(t, out, ext, "an external project needs its name")
	assert.NotContains(t, out, "rancher/other", "a project that is not owned is not cloned by --all")
}

func TestRunAnExternalProjectIsClonedWhenNamed(t *testing.T) {
	r := newRig(t)

	_, err := r.do(named("sdk"))

	require.NoError(t, err)
	assert.Contains(t, r.git.first(), "git@github.com:third-party/sdk.git")
}

func TestRunNeedsProjectsOrAllAndNotBoth(t *testing.T) {
	r := newRig(t)

	_, err := r.do(Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all")

	_, err = r.do(Options{All: true, Projects: []string{"rancher"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all")
	assert.Empty(t, r.git.calls)
}

func TestRunAnUnknownProjectIsAnErrorBeforeAnythingIsCloned(t *testing.T) {
	r := newRig(t)

	_, err := r.do(named("rancher", "nope"))

	require.ErrorIs(t, err, workspace.ErrUnknownProject)
	assert.Empty(t, r.git.calls)
}

func TestRunIntoBeatsTheConfiguredRoot(t *testing.T) {
	r := newRig(t)
	into := t.TempDir()

	_, err := r.do(Options{Projects: []string{"rancher"}, Into: into})

	require.NoError(t, err)
	assert.Contains(t, r.git.first(), " "+filepath.Join(into, "rancher")+" ")
}

func TestRunWithNoRootNamesTheThreeWaysToSetOneAndClonesNothing(t *testing.T) {
	r := newRig(t)
	r.s.Repos.CloneRoot = ""

	_, err := r.do(named("rancher"))

	require.Error(t, err)
	for _, way := range []string{"--into", "GERO_REPOS_CLONE_ROOT", "repos.cloneRoot"} {
		assert.Contains(t, err.Error(), way)
	}
	assert.Empty(t, r.git.calls)
}

func TestRunAReposPathsEntryBeatsTheRootAndNeedsNoRoot(t *testing.T) {
	r := newRig(t)
	r.s.Repos.CloneRoot = ""
	where := filepath.Join(t.TempDir(), "rancher-src")
	r.ws.paths[rancher] = workspace.RepoPath{Slug: rancher, Path: where, Source: workspace.PathSettings}

	_, err := r.do(named("rancher"))

	require.NoError(t, err)
	assert.Contains(t, r.git.first(), " "+where+" ")
}

func TestRunAClonePlacedByTheScanIsSkipped(t *testing.T) {
	r := newRig(t)
	r.ws.paths[rancher] = workspace.RepoPath{Slug: rancher, Path: "/src/rancher", Source: workspace.PathScan}

	out, err := r.do(named("rancher"))

	require.NoError(t, err)
	assert.Empty(t, r.git.calls)
	assert.Contains(t, out, "Skipped rancher/rancher: already cloned at /src/rancher")
}

func TestRunAnAmbiguousCloneFailsThatProject(t *testing.T) {
	r := newRig(t)
	r.ws.paths[rancher] = workspace.RepoPath{Slug: rancher, Candidates: []string{"/a", "/b"}, Err: workspace.ErrRepoAmbiguous}

	out, err := r.do(named("rancher", "scc-operator"))

	require.Error(t, err)
	assert.Contains(t, out, "Failed rancher/rancher")
	assert.Contains(t, out, "Cloned rancher/scc-operator", "the rest go on")
}

func TestRunADirectoryHoldingTheRightRepositoryIsSkipped(t *testing.T) {
	r := newRig(t)
	dir := filepath.Join(r.root, "rancher")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x"), 0o644))
	r.git.remotes[dir+"|upstream"] = "https://github.com/rancher/rancher.git"

	out, err := r.do(named("rancher"))

	require.NoError(t, err)
	assert.Empty(t, r.git.calls)
	assert.Contains(t, out, "Skipped rancher/rancher: already cloned at "+dir)
}

func TestRunANonEmptyDirectoryThatIsSomethingElseFailsThatProjectAndTheRestGoOn(t *testing.T) {
	r := newRig(t)
	dir := filepath.Join(r.root, "rancher")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o644))

	out, err := r.do(named("rancher", "scc-operator"))

	require.Error(t, err)
	assert.Contains(t, out, "Failed rancher/rancher")
	assert.Contains(t, out, dir)
	assert.Contains(t, out, "Cloned rancher/scc-operator")
	assert.Len(t, r.git.calls, 1)
	_, statErr := os.Stat(filepath.Join(dir, "notes.txt"))
	assert.NoError(t, statErr, "what was there is left alone")
}

func TestRunADirectoryHoldingAnotherRepositoryFails(t *testing.T) {
	r := newRig(t)
	dir := filepath.Join(r.root, "rancher")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x"), 0o644))
	r.git.remotes[dir+"|upstream"] = "git@github.com:someone/else.git"

	out, err := r.do(named("rancher"))

	require.Error(t, err)
	assert.Contains(t, out, "Failed rancher/rancher")
	assert.Contains(t, out, "someone/else")
}

func TestRunAnEmptyDirectoryIsClonedInto(t *testing.T) {
	r := newRig(t)
	require.NoError(t, os.MkdirAll(filepath.Join(r.root, "rancher"), 0o755))

	_, err := r.do(named("rancher"))

	require.NoError(t, err)
	assert.Len(t, r.git.calls, 1)
}

func TestRunAFailedCloneIsReportedAndTheRestGoOn(t *testing.T) {
	r := newRig(t)
	r.git.cloneErr["git@github.com:rancher/rancher.git"] = errors.New("permission denied (publickey)")

	out, err := r.do(named("rancher", "scc-operator"))

	require.Error(t, err)
	assert.Contains(t, out, "Failed rancher/rancher")
	assert.Contains(t, out, "permission denied (publickey)")
	assert.Contains(t, out, "Cloned rancher/scc-operator")
}

func TestRunNoUpstreamRemoteNameIsAnError(t *testing.T) {
	r := newRig(t)
	r.ws.upstream = ""

	_, err := r.do(named("rancher"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream")
	assert.Empty(t, r.git.calls)
}

func TestRunDryRunClonesNothingAndSaysWhatItWould(t *testing.T) {
	r := newRig(t).withFork()
	r.forks.have["danpock "+rancher] = true
	r.s.DryRun = true

	out, err := r.do(named("rancher"))

	require.NoError(t, err)
	assert.Empty(t, r.git.calls)
	assert.Contains(t, out, "Would clone rancher/rancher into "+filepath.Join(r.root, "rancher"))
	assert.Contains(t, out, "(origin, upstream)")
}

func TestRunJSONReportsEveryProject(t *testing.T) {
	r := newRig(t)
	r.s.Output = clicfg.OutputJSON
	r.ws.paths[scc] = workspace.RepoPath{Slug: scc, Path: "/src/scc", Source: workspace.PathScan}

	out, err := r.do(named("rancher", "scc-operator"))

	require.NoError(t, err)
	var got struct{ Repos []Result }
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	require.Len(t, got.Repos, 2)
	assert.Equal(t, Result{
		Project: rancher, Outcome: Cloned, Path: filepath.Join(r.root, "rancher"), Remotes: []string{"upstream"},
	}, got.Repos[0])
	assert.Equal(t, Skipped, got.Repos[1].Outcome)
	assert.NotContains(t, out, "\x1b")
}

func TestRunAFormatThatIsNotTextOrJSONIsRefused(t *testing.T) {
	r := newRig(t)
	r.s.Output = clicfg.OutputEnv

	_, err := r.do(named("rancher"))

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "text or json"), err.Error())
}
