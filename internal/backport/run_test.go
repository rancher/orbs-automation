package backport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/proposal/proposaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	engineer = proposaltest.Engineer
	current  = proposaltest.Current
)

type (
	fakeRemote = proposaltest.FakeRemote
	log        = proposaltest.Log
)

// fakeHost is the PRs on the host, by number.
type fakeHost struct {
	details map[int]pulls.Detail
	commits map[int][]pulls.Commit
}

func (h *fakeHost) Get(_ context.Context, slug string, n int) (pulls.Detail, error) {
	d, ok := h.details[n]
	if !ok {
		return pulls.Detail{}, fmt.Errorf("read %s#%d: not found", slug, n)
	}
	return d, nil
}

func (h *fakeHost) Commits(_ context.Context, _ string, n int) ([]pulls.Commit, error) {
	return h.commits[n], nil
}

// scripted is a Resolver that answers each conflict from a function and keeps them.
type scripted struct {
	seen []Conflict
	do   func(c Conflict) (Choice, error)
}

func (s *scripted) Resolve(_ context.Context, c Conflict) (Choice, error) {
	s.seen = append(s.seen, c)
	return s.do(c)
}

// backWS is the project served from a real repository, with the engineer's clone. main is
// v2.16, release/v1 is v2.15 and release/v0 is v2.14. Two commits by Author are on main:
// addB adds b.txt, which every branch takes, and newImage changes build.yaml, which
// release/v1 changed too, so picking it there conflicts.
type backWS struct {
	t        *testing.T
	graph    *chain.Graph
	up       string
	clone    string
	push     proposal.PushSettings
	host     *fakeHost
	addB     string
	newImage string
	addC     string
}

func newBackWS(t *testing.T) (*backWS, *proposal.Executor) {
	t.Helper()
	up := proposaltest.Remote(t)
	runGit(t, up, "config", "user.name", "Dev")
	runGit(t, up, "config", "user.email", "dev@example.com")
	runGit(t, up, "branch", "release/v0", "main")
	runGit(t, up, "checkout", "--quiet", "release/v1")
	put(t, up, "build.yaml", "image: legacy\n")
	runGit(t, up, "commit", "--quiet", "-a", "-m", "Legacy image")
	runGit(t, up, "checkout", "--quiet", "main")
	w := &backWS{t: t, up: up, host: &fakeHost{details: map[int]pulls.Detail{}, commits: map[int][]pulls.Commit{}}}
	w.addB = commit(t, up, "b.txt", "Add b")
	put(t, up, "build.yaml", "image: new\n")
	runGit(t, up, "commit", "--quiet", "-a", "--author=Author <author@example.com>", "-m", "Update image")
	w.newImage = runGit(t, up, "rev-parse", "HEAD")
	w.addC = commit(t, up, "c.txt", "Add c")
	w.graph = chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: release/v1}, {line: v2.14, branch: release/v0}]\n"))
	w.clone = engineer(t, up)
	w.mergedPR(12, w.addB, "Add b")
	return w, &proposal.Executor{Git: proposal.GitCLI{}, TempDir: t.TempDir()}
}

// merged says PR 12 on the host landed as the one commit, with its title.
func (w *backWS) merged(sha, title string) { w.mergedPR(12, sha, title) }

// mergedPR says a PR on the host landed as the one commit, with its title, to main.
func (w *backWS) mergedPR(n int, sha, title string) {
	w.host.details[n] = pulls.Detail{
		PR:     pulls.PR{Number: n, URL: fmt.Sprintf("https://github.com/acme/cons/pull/%d", n), Title: title, Base: "main"},
		Merged: true, MergeCommit: sha,
	}
	w.host.commits[n] = []pulls.Commit{{SHA: fmt.Sprintf("orig%d", n), Message: title, Author: "Author"}}
}

func (w *backWS) Project(string) (*chain.Project, error)    { return w.graph.Project("cons"), nil }
func (w *backWS) PullRequests() Host                        { return w.host }
func (w *backWS) CloneDir(string) (string, error)           { return w.clone, nil }
func (w *backWS) Remote(string) (string, error)             { return "upstream", nil }
func (w *backWS) PushSettings(string) proposal.PushSettings { return w.push }

func (w *backWS) UpstreamRefs(context.Context, string) (git.Refs, error) {
	var refs git.Refs
	for _, line := range strings.Split(runGit(w.t, w.up, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads"), "\n") {
		name, commit, _ := strings.Cut(line, " ")
		refs.Branches = append(refs.Branches, git.Branch{Name: name, Commit: commit})
	}
	return refs, nil
}

func (w *backWS) branchExists(name string) bool {
	return runGit(w.t, w.clone, "branch", "--list", name) != ""
}

func target(t *testing.T, p Plan, branch string) Target {
	t.Helper()
	for _, tg := range p.Targets {
		if tg.Branch == branch {
			return tg
		}
	}
	require.Failf(t, "no target", "%s in %+v", branch, p.Targets)
	return Target{}
}

// onBranch is a file as the branch of the clone has it.
func (w *backWS) onBranch(branch, file string) string {
	return runGit(w.t, w.clone, "show", branch+":"+file)
}

func TestBackportMakesABranchWithThePickedCommitAndItsOrigin(t *testing.T) {
	ws, ex := newBackWS(t)

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true}, ex)

	require.NoError(t, err)
	assert.Equal(t, "acme/cons", plan.Repo)
	require.Len(t, plan.PRs, 1)
	assert.Equal(t, Source{Number: 12, Title: "Add b", URL: "https://github.com/acme/cons/pull/12", Shape: ShapeSingle, Picks: []string{ws.addB}}, plan.PRs[0])
	require.Len(t, plan.Targets, 1)
	tg := target(t, plan, "release/v1")
	assert.Equal(t, "v2.15", tg.Line)
	assert.Equal(t, StatusReady, tg.Status)
	require.NotNil(t, tg.Preview)
	assert.Equal(t, "[Backport v2.15] Add b", tg.Preview.Title)
	assert.True(t, strings.HasPrefix(tg.Preview.Branch, "bot/backport-12-"), tg.Preview.Branch)
	assert.True(t, strings.HasSuffix(tg.Preview.Branch, "-v1"), tg.Preview.Branch)
	assert.Contains(t, tg.Preview.Body, "https://github.com/acme/cons/pull/12")
	assert.Contains(t, tg.Preview.Body, "gero:ship")
	require.NotNil(t, tg.Result)
	assert.Equal(t, proposal.ResultCommitted, tg.Result.Status)
	assert.Equal(t, "Add b", strings.TrimSpace(ws.onBranch(tg.Preview.Branch, "b.txt")))
	message := runGit(t, ws.clone, "log", "-1", "--format=%B", tg.Preview.Branch)
	assert.Contains(t, message, "(cherry picked from commit "+ws.addB+")")
	assert.Equal(t, "Author", runGit(t, ws.clone, "log", "-1", "--format=%an", tg.Preview.Branch))
	assert.Equal(t, "main", current(t, ws.clone), "the clone is as it was found")
}

func TestBackportWorksTheNewestLineFirstAndEachTargetOnce(t *testing.T) {
	ws, ex := newBackWS(t)

	plan, err := Run(bg, ws, Input{Lines: []string{"2.14", "release/v1", "v2.15"}, PRs: []int{12}, NoPush: true}, ex)

	require.NoError(t, err)
	require.Len(t, plan.Targets, 2, "v2.15 and release/v1 are one target")
	assert.Equal(t, "release/v1", plan.Targets[0].Branch)
	assert.Equal(t, "release/v0", plan.Targets[1].Branch)
	assert.Equal(t, "v2.14", plan.Targets[1].Line)
}

func TestBackportRefusesAPRThatIsNotMerged(t *testing.T) {
	ws, ex := newBackWS(t)
	d := ws.host.details[12]
	d.Merged = false
	ws.host.details[12] = d

	_, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true}, ex)

	require.ErrorContains(t, err, "not merged")
	assert.False(t, ws.branchExists("bot/*"))
}

func TestBackportRefusesATargetThatIsTheBranchThePRWasMergedTo(t *testing.T) {
	ws, ex := newBackWS(t)

	_, err := Run(bg, ws, Input{Lines: []string{"v2.15", "main"}, PRs: []int{12}, NoPush: true}, ex)

	require.ErrorContains(t, err, "main")
	require.ErrorContains(t, err, "merged to")
	assert.Empty(t, runGit(t, ws.clone, "branch", "--list", "bot/*"), "nothing was made, since the targets are checked first")
}

func TestBackportRefusesATargetTheProjectDoesNotHave(t *testing.T) {
	ws, ex := newBackWS(t)

	_, err := Run(bg, ws, Input{Lines: []string{"v9.9"}, PRs: []int{12}, NoPush: true}, ex)

	require.ErrorContains(t, err, "v9.9")
}

func TestBackportOfWhatATargetAlreadyHoldsMakesNoBranch(t *testing.T) {
	ws, ex := newBackWS(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v1")
	runGit(t, ws.up, "cherry-pick", "-x", ws.addB)
	runGit(t, ws.up, "checkout", "--quiet", "main")

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15", "v2.14"}, PRs: []int{12}, NoPush: true}, ex)

	require.NoError(t, err)
	done := target(t, plan, "release/v1")
	assert.Equal(t, StatusAlreadyBackported, done.Status)
	assert.Nil(t, done.Result)
	other := target(t, plan, "release/v0")
	assert.Equal(t, StatusReady, other.Status)
	require.NotNil(t, other.Result)
	assert.Equal(t, proposal.ResultCommitted, other.Result.Status)
}

func TestBackportDryRunMakesNothingThatStays(t *testing.T) {
	ws, ex := newBackWS(t)

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true, DryRun: true}, ex)

	require.NoError(t, err)
	tg := target(t, plan, "release/v1")
	require.NotNil(t, tg.Result)
	assert.Equal(t, proposal.ResultWouldCommit, tg.Result.Status)
	assert.Empty(t, runGit(t, ws.clone, "branch", "--list", "bot/*"))
	assert.Equal(t, "main", current(t, ws.clone))
}

// backPublishing is a run that pushes to the clone's upstream and opens the PRs, with the
// fake GitHub.
func backPublishing(t *testing.T) (*backWS, *proposal.Executor, *fakeRemote, *log) {
	t.Helper()
	ws, ex := newBackWS(t)
	lg := &log{}
	remote := &fakeRemote{Log: lg, Labels: []string{"status/auto-created"}}
	ws.push = proposal.PushSettings{Upstream: "upstream"}
	ex.Publish = &proposal.Publisher{Git: proposal.GitCLI{}, Remote: remote}
	return ws, ex, remote, lg
}

func TestBackportPushesAndOpensOnePRForEachTarget(t *testing.T) {
	ws, ex, _, lg := backPublishing(t)

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15", "v2.14"}, PRs: []int{12}}, ex)

	require.NoError(t, err)
	assert.Equal(t, 2, lg.Count("create "))
	first := target(t, plan, "release/v1")
	require.NotNil(t, first.Result)
	assert.Equal(t, proposal.ResultOpened, first.Result.Status)
	assert.Equal(t, first.Result.Tip(), runGit(t, ws.up, "rev-parse", first.Result.Branch), "the branch is on the remote")
	assert.True(t, lg.Has(`create acme/cons head=`+first.Result.Branch+` base=release/v1 title="[Backport v2.15] Add b"`), lg.Calls)
	assert.Less(t, lg.Index("open acme/cons release/v1"), lg.Index("create "), "the open PRs are looked at first")
}

func TestBackportAnAlreadyOpenPRMakesNothing(t *testing.T) {
	ws, ex, remote, lg := backPublishing(t)
	checked, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true}, ex)
	require.NoError(t, err)
	m, ok := proposal.ParseMarker(target(t, checked, "release/v1").Preview.Body)
	require.True(t, ok, "the body carries a marker")
	remote.Open = pulls.Open{PRs: []pulls.PR{proposaltest.PR(77, "danpock", m.String())}}
	runGit(t, ws.clone, "branch", "-D", target(t, checked, "release/v1").Preview.Branch)

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}}, ex)

	require.NoError(t, err)
	res := target(t, plan, "release/v1").Result
	require.NotNil(t, res)
	assert.Equal(t, proposal.ResultAlreadyOpen, res.Status)
	assert.Equal(t, 77, res.PR.Number)
	assert.False(t, lg.Has("create"))
}

func TestBackportOfAnotherPRToTheSameTargetIsNotTheSameBackport(t *testing.T) {
	ws, ex, remote, lg := backPublishing(t)
	checked, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true}, ex)
	require.NoError(t, err)
	m, _ := proposal.ParseMarker(target(t, checked, "release/v1").Preview.Body)
	m.Source = "backport-9" // another PR's backport, open to the same branch
	remote.Open = pulls.Open{PRs: []pulls.PR{proposaltest.PR(70, "danpock", m.String())}}
	runGit(t, ws.clone, "branch", "-D", target(t, checked, "release/v1").Preview.Branch)

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}}, ex)

	require.NoError(t, err)
	assert.Equal(t, proposal.ResultOpened, target(t, plan, "release/v1").Result.Status)
	assert.False(t, lg.Has("close #70"), "the other PR's backport is left alone")
}

// resolving is a resolver that writes the resolution of build.yaml and says continue.
func resolving(content string) *scripted {
	return &scripted{do: func(c Conflict) (Choice, error) {
		putFile(c.Dir, "build.yaml", content)
		return Continue, nil
	}}
}

func TestBackportAConflictWaitsForThePersonAndGoesOnWhenItIsResolved(t *testing.T) {
	ws, ex := newBackWS(t)
	ws.merged(ws.newImage, "Update image")
	resolver := resolving("image: merged\n")

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true, Resolver: resolver}, ex)

	require.NoError(t, err)
	require.Len(t, resolver.seen, 1)
	c := resolver.seen[0]
	assert.Equal(t, "release/v1", c.Branch)
	assert.Equal(t, "v2.15", c.Line)
	assert.Equal(t, 12, c.PR)
	assert.Equal(t, ws.newImage, c.Commit)
	assert.Equal(t, "Update image", c.Subject)
	assert.Equal(t, []string{"build.yaml"}, c.Files)
	tg := target(t, plan, "release/v1")
	require.NotNil(t, tg.Result)
	assert.Equal(t, proposal.ResultCommitted, tg.Result.Status)
	assert.Equal(t, []string{"build.yaml"}, tg.Files)
	assert.Equal(t, "image: merged", strings.TrimSpace(ws.onBranch(tg.Preview.Branch, "build.yaml")))
	assert.Contains(t, runGit(t, ws.clone, "log", "-1", "--format=%B", tg.Preview.Branch), "(cherry picked from commit "+ws.newImage+")")
	assert.Contains(t, tg.Preview.Body, "resolved by hand")
	assert.Contains(t, tg.Preview.Body, "build.yaml")
	assert.Equal(t, "main", current(t, ws.clone))
}

func TestBackportAskedToContinueWithMarkersLeftAsksAgainNamingThem(t *testing.T) {
	ws, ex := newBackWS(t)
	ws.merged(ws.newImage, "Update image")
	answers := 0
	resolver := &scripted{do: func(c Conflict) (Choice, error) {
		answers++
		if answers == 2 {
			putFile(c.Dir, "build.yaml", "image: merged\n")
		}
		return Continue, nil
	}}

	_, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true, Resolver: resolver}, ex)

	require.NoError(t, err)
	require.Len(t, resolver.seen, 2)
	assert.Empty(t, resolver.seen[0].Left)
	assert.Equal(t, []string{"build.yaml"}, resolver.seen[1].Left, "the file still has its markers")
}

func TestBackportAbandoningAConflictEndsTheRunAndLeavesTheCloneAsFound(t *testing.T) {
	ws, ex := newBackWS(t)
	ws.merged(ws.newImage, "Update image")
	resolver := &scripted{do: func(Conflict) (Choice, error) { return Abort, nil }}

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15", "v2.14"}, PRs: []int{12}, NoPush: true, Resolver: resolver}, ex)

	require.ErrorIs(t, err, ErrAborted)
	assert.Len(t, resolver.seen, 1)
	assert.Equal(t, StatusAborted, target(t, plan, "release/v1").Status)
	assert.Equal(t, StatusNotAttempted, target(t, plan, "release/v0").Status, "nothing after a conflict is tried")
	assert.Nil(t, target(t, plan, "release/v0").Result)
	assert.Empty(t, runGit(t, ws.clone, "branch", "--list", "bot/*"))
	assert.Empty(t, runGit(t, ws.clone, "status", "--porcelain"))
	assert.Equal(t, "main", current(t, ws.clone))
	assert.NoFileExists(t, ws.clone+"/.git/CHERRY_PICK_HEAD")
}

func TestBackportWithNobodyToAskAbandonsAConflictAndNamesTheFiles(t *testing.T) {
	ws, ex := newBackWS(t)
	ws.merged(ws.newImage, "Update image")

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true}, ex)

	require.ErrorIs(t, err, ErrAborted)
	require.ErrorContains(t, err, "build.yaml")
	require.NotNil(t, target(t, plan, "release/v1").Result)
	assert.Contains(t, target(t, plan, "release/v1").Result.Error, "build.yaml")
	assert.Empty(t, runGit(t, ws.clone, "status", "--porcelain"))
}

func TestBackportAResolverThatFailsEndsTheRunWithItsError(t *testing.T) {
	ws, ex := newBackWS(t)
	ws.merged(ws.newImage, "Update image")
	boom := errors.New("the editor would not open")
	resolver := &scripted{do: func(Conflict) (Choice, error) { return 0, boom }}

	_, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, NoPush: true, Resolver: resolver}, ex)

	require.ErrorIs(t, err, boom)
	assert.Empty(t, runGit(t, ws.clone, "status", "--porcelain"), "the pick is abandoned, not left half done")
}

// putFile writes a file in a work directory the way a person's editor would.
func putFile(dir, rel, content string) {
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644); err != nil {
		panic(err)
	}
}

func TestBackportDryRunReportsAConflictWithoutAskingAndGoesOn(t *testing.T) {
	ws, ex := newBackWS(t)
	ws.merged(ws.newImage, "Update image")
	resolver := &scripted{do: func(Conflict) (Choice, error) { return Continue, nil }}

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15", "v2.14"}, PRs: []int{12}, NoPush: true, DryRun: true, Resolver: resolver}, ex)

	require.NoError(t, err, "a dry run waits for nobody, so a conflict is a finding and not a stop")
	assert.Empty(t, resolver.seen)
	conflicted := target(t, plan, "release/v1")
	require.NotNil(t, conflicted.Result)
	assert.Equal(t, proposal.ResultFailed, conflicted.Result.Status)
	assert.Contains(t, conflicted.Result.Error, "would conflict")
	assert.Contains(t, conflicted.Result.Error, "build.yaml")
	clean := target(t, plan, "release/v0")
	require.NotNil(t, clean.Result)
	assert.Equal(t, proposal.ResultWouldCommit, clean.Result.Status, "the next target was tried")
	assert.Empty(t, runGit(t, ws.clone, "status", "--porcelain"))
}

func TestBackportRefusesAPRURLOfAnotherRepoThanTheProject(t *testing.T) {
	ws, ex := newBackWS(t)

	_, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, Repo: "acme/other", NoPush: true}, ex)

	require.ErrorContains(t, err, "acme/other")
	require.ErrorContains(t, err, "acme/cons")
	assert.Empty(t, runGit(t, ws.clone, "branch", "--list", "bot/*"))
}

func TestBackportAcceptsAPRURLOfTheProjectsRepo(t *testing.T) {
	ws, ex := newBackWS(t)

	_, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12}, Repo: "acme/cons", NoPush: true}, ex)

	require.NoError(t, err)
}
