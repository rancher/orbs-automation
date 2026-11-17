package proposal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal/proposaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// site is the engineer's clone of a holder, served as the workspace would.
type site struct {
	clone    string
	cloneErr error
	push     PushSettings
}

func (s site) CloneDir(string) (string, error)  { return s.clone, s.cloneErr }
func (s site) Remote(string) (string, error)    { return "upstream", nil }
func (s site) PushSettings(string) PushSettings { return s.push }

func holder(t *testing.T) *chain.Project {
	t.Helper()
	return chaintest.Load(t, chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n")).Project("cons")
}

// scene is a holder with its engineer's clone, and the executor that publishes to the fake GitHub.
type scene struct {
	up, clone string
	ex        *Executor
	remote    *proposaltest.FakeRemote
	lg        *proposaltest.Log
	site      site
}

func newScene(t *testing.T) *scene {
	t.Helper()
	up := remote(t)
	clone := engineer(t, up)
	lg := &proposaltest.Log{}
	fr := &proposaltest.FakeRemote{Log: lg, Labels: []string{"auto"}}
	return &scene{
		up: up, clone: clone, remote: fr, lg: lg,
		site: site{clone: clone, push: PushSettings{Upstream: "upstream"}},
		ex:   &Executor{Git: GitCLI{}, TempDir: t.TempDir(), Publish: &Publisher{Git: GitCLI{}, Remote: fr}},
	}
}

// subject is the proposal to write build.yaml on main, from the commit the host has for it.
func (s *scene) subject(t *testing.T) Subject {
	t.Helper()
	return Subject{
		Branch: "main", Commit: runGit(t, s.up, "rev-parse", "main"), Line: "v2.16",
		Preview: &Preview{Branch: "bot/sync-1-main", Key: "sync-1-main", Title: "[main] Sync", Body: "body"},
	}
}

// writeImage is an Apply that writes build.yaml and commits it.
func writeImage(value string) func(*Work, func(CommitInfo)) (ApplyResult, error) {
	return func(w *Work, onCommit func(CommitInfo)) (ApplyResult, error) {
		if err := os.WriteFile(filepath.Join(w.Dir, "build.yaml"), []byte("image: "+value+"\n"), 0o644); err != nil {
			return ApplyResult{}, err
		}
		hash, err := w.Commit(context.Background(), GitCommit{Message: "Set image\n\nAutomation: gero ship"})
		if err != nil {
			return ApplyResult{}, ErrNothingChanged
		}
		c := CommitInfo{Hash: hash, Subject: "Set image"}
		onCommit(c)
		return ApplyResult{Commits: []CommitInfo{c}, Changes: []bump.Change{{Description: "Set image to " + value}}}, nil
	}
}

func TestBeginWithoutAClonePointsAtIt(t *testing.T) {
	s := newScene(t)
	s.site.cloneErr = errors.New("clone it first")

	_, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})

	require.ErrorIs(t, err, ErrNoClone)
	assert.ErrorContains(t, err, "clone it first")
}

func TestBeginGivesTheCloneAndOnlyAPublisherWhenItPushes(t *testing.T) {
	s := newScene(t)

	plain, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t), NoPush: true})
	require.NoError(t, err)
	assert.Equal(t, s.clone, plain.Clone)
	assert.Equal(t, "upstream", plain.Remote)
	assert.Nil(t, plain.Pub, "--no-push has no publisher")

	pushing, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	require.NotNil(t, pushing.Pub)
	assert.Equal(t, "danpock", pushing.Me)
	require.NotNil(t, pushing.Push)
	assert.Equal(t, RoleUpstream, pushing.Push.Role)
	assert.Equal(t, s.clone, pushing.Pub.Clone)
}

func TestBeginInCIActsAsTheBotsIdentity(t *testing.T) {
	s := newScene(t)
	s.ex.Identify = func(context.Context, string) (Identity, error) {
		return Identity{Name: "bot", Email: "bot@example.com"}, nil
	}

	ci, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t), CI: true})
	require.NoError(t, err)
	assert.Equal(t, Identity{Name: "bot", Email: "bot@example.com"}, ci.Committer, "the commits are made as the bot")
	require.NotNil(t, ci.Actor)

	me, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	assert.Empty(t, me.Committer, "on a workstation git's own configuration decides")
	require.NotNil(t, me.Actor)
}

func TestSessionConfirmAsksOnlyForARunThatPublishesAndWrites(t *testing.T) {
	s := newScene(t)
	asked := 0
	answer := true
	s.ex.Confirm = func(context.Context, Confirmation) (bool, error) { asked++; return answer, nil }
	pushing, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	plain, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t), NoPush: true})
	require.NoError(t, err)
	c := Confirmation{Consumer: "acme/cons"}

	require.NoError(t, plain.Confirm(bg, s.ex, false, c, true), "no publisher")
	require.NoError(t, pushing.Confirm(bg, s.ex, true, c, true), "a dry run")
	require.NoError(t, pushing.Confirm(bg, s.ex, false, c, false), "nothing to write")
	assert.Zero(t, asked)

	require.NoError(t, pushing.Confirm(bg, s.ex, false, c, true))
	assert.Equal(t, 1, asked)

	answer = false
	require.ErrorIs(t, pushing.Confirm(bg, s.ex, false, c, true), ErrDeclined)
}

func ownPR(number int, m Marker) pulls.PR {
	return pr(number, "danpock", m)
}

func TestDecideWithoutAPublisherDecidesNothing(t *testing.T) {
	got := Decide(bg, nil, "danpock", DecideInput{}, nil)

	assert.Equal(t, Decided{}, got)
}

func TestDecideFindsMyOpenPRAndOneThatBlocksIt(t *testing.T) {
	s := newScene(t)
	sess, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	in := DecideInput{
		Repo: "acme/cons", Subject: s.subject(t), Branches: nil,
		Marker: Marker{Source: "x", Consumer: "acme/cons", Target: "main", Version: "v1.2.0"},
	}

	s.remote.Open = pulls.Open{PRs: []pulls.PR{ownPR(7, in.Marker)}}
	open := Decide(bg, sess.Pub, sess.Me, in, nil)
	assert.Equal(t, PublishAlreadyOpen, open.Decision.Action)
	assert.Nil(t, open.Done)

	newer := in.Marker
	newer.Version = "v1.3.0"
	s.remote.Open = pulls.Open{PRs: []pulls.PR{ownPR(8, newer)}}
	blocked := Decide(bg, sess.Pub, sess.Me, in, nil)
	assert.Equal(t, PublishBlocked, blocked.Decision.Action)
	require.NotNil(t, blocked.Done)
	assert.Equal(t, ResultBlocked, blocked.Done.Status)
	assert.Contains(t, blocked.Done.Error, "#8 already writes v1.3.0")

	in.Blocked = func(newer Marker, _ pulls.PR) string { return "later: " + newer.Version }
	assert.Equal(t, "later: v1.3.0", Decide(bg, sess.Pub, sess.Me, in, nil).Done.Error)
}

func TestDecideUsesTheOrderItIsGiven(t *testing.T) {
	s := newScene(t)
	sess, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	m := Marker{Source: "sync", Consumer: "acme/cons", Target: "main", Version: "aaaa"}
	stale := m
	stale.Version = "bbbb"
	s.remote.Open = pulls.Open{PRs: []pulls.PR{ownPR(5, stale)}}
	asStale := func(found, mine string) (int, bool) {
		if found == mine {
			return 0, true
		}
		return -1, true
	}

	got := Decide(bg, sess.Pub, sess.Me, DecideInput{Repo: "acme/cons", Subject: s.subject(t), Marker: m, Compare: asStale}, nil)

	assert.Equal(t, PublishCreate, got.Decision.Action)
	require.Len(t, got.Decision.Supersede, 1, "a PR of another fingerprint is stale and is replaced")
	assert.Equal(t, 5, got.Decision.Supersede[0].Number)
}

func TestDecideAFailedLookupFailsTheProposalBeforeAnyWork(t *testing.T) {
	s := newScene(t)
	sess, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	s.remote.OpenErr = errors.New("rate limited")

	got := Decide(bg, sess.Pub, sess.Me, DecideInput{Repo: "acme/cons", Subject: s.subject(t)}, nil)

	require.NotNil(t, got.Done)
	assert.Equal(t, ResultFailed, got.Done.Status)
	assert.Contains(t, got.Done.Error, "rate limited")
}

func TestCommitWritesOnANewBranchAndLeavesTheCloneAsItWas(t *testing.T) {
	s := newScene(t)
	rec := &recording{}

	res := Commit(bg, s.ex, CommitInput{Clone: s.clone, Remote: "upstream", Subject: s.subject(t), Apply: writeImage("new")}, rec)

	assert.Equal(t, ResultCommitted, res.Status)
	assert.Equal(t, "bot/sync-1-main", res.Branch)
	assert.Equal(t, "image: new\n", runGit(t, s.clone, "show", res.Branch+":build.yaml")+"\n")
	assert.Equal(t, "main", current(t, s.clone))
	assert.Equal(t, "image: old\n", slurp(t, s.clone+"/build.yaml"), "the engineer's checkout is untouched")
	assert.Equal(t, []EventKind{EventTargetStarted, EventCommit}, rec.kinds())
	assert.Equal(t, []string{"Set image to new"}, res.Changes)
}

func TestCommitOfAChangeThatChangesNothingIsUpToDate(t *testing.T) {
	s := newScene(t)

	res := Commit(bg, s.ex, CommitInput{Clone: s.clone, Remote: "upstream", Subject: s.subject(t), Apply: writeImage("old")}, nil)

	assert.Equal(t, ResultUpToDate, res.Status)
	assert.Empty(t, runGit(t, s.clone, "branch", "--list", "bot/*"), "no branch is left")
}

func TestCommitAFailureAbortsTheWork(t *testing.T) {
	s := newScene(t)
	failing := func(*Work, func(CommitInfo)) (ApplyResult, error) { return ApplyResult{}, errors.New("a step failed") }

	res := Commit(bg, s.ex, CommitInput{Clone: s.clone, Remote: "upstream", Subject: s.subject(t), Apply: failing}, nil)

	assert.Equal(t, ResultFailed, res.Status)
	assert.Contains(t, res.Error, "a step failed")
	assert.Empty(t, runGit(t, s.clone, "branch", "--list", "bot/*"))
	assert.Equal(t, "main", current(t, s.clone))
}

func TestCommitDryRunWorksInAWorktreeAndRemovesIt(t *testing.T) {
	s := newScene(t)

	res := Commit(bg, s.ex, CommitInput{Clone: s.clone, Remote: "upstream", Subject: s.subject(t), DryRun: true, Apply: writeImage("new")}, nil)

	assert.Equal(t, ResultWouldCommit, res.Status)
	assert.Equal(t, Worktree, res.Place)
	assert.Empty(t, runGit(t, s.clone, "branch", "--list", "bot/*"))
}

func TestEnactPushesAndOpensThePRForADecidedProposal(t *testing.T) {
	s := newScene(t)
	sess, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	sub := s.subject(t)
	m := Marker{Source: "sync", Consumer: "acme/cons", Target: "main", Version: "aaaa"}
	decided := Decide(bg, sess.Pub, sess.Me, DecideInput{Repo: "acme/cons", Subject: sub, Marker: m}, nil)

	res := Enact(bg, s.ex, EnactInput{Repo: "acme/cons", Session: sess, Subject: sub, Marker: m, Decided: decided, Apply: writeImage("new")}, nil)

	assert.Equal(t, ResultOpened, res.Status)
	require.NotNil(t, res.PR)
	assert.Equal(t, 104, res.PR.Number)
	assert.Equal(t, res.Tip(), runGit(t, s.up, "rev-parse", "bot/sync-1-main"), "the branch is on the remote")
	assert.Equal(t, 1, s.lg.Count("create "))
}

func TestEnactAnAlreadyOpenPRMakesNothingAndRunsTheRefreshHook(t *testing.T) {
	s := newScene(t)
	sess, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	sub := s.subject(t)
	m := Marker{Source: "sync", Consumer: "acme/cons", Target: "main", Version: "aaaa"}
	s.remote.Open = pulls.Open{PRs: []pulls.PR{ownPR(77, m)}}
	decided := Decide(bg, sess.Pub, sess.Me, DecideInput{Repo: "acme/cons", Subject: sub, Marker: m}, nil)
	hooked := false

	res := Enact(bg, s.ex, EnactInput{
		Repo: "acme/cons", Session: sess, Subject: sub, Marker: m, Decided: decided, Apply: writeImage("new"),
		Refresh: func(*TargetResult) { hooked = true },
	}, nil)

	assert.Equal(t, ResultAlreadyOpen, res.Status)
	assert.Equal(t, 77, res.PR.Number)
	assert.True(t, hooked)
	assert.False(t, s.lg.Has("create"))
	assert.Empty(t, runGit(t, s.clone, "branch", "--list", "bot/*"), "no branch was made")
}

func TestEnactADecidedBlockIsItsResultWithoutWork(t *testing.T) {
	s := newScene(t)
	sess, err := Begin(bg, s.site, s.ex, SessionInput{Repo: holder(t)})
	require.NoError(t, err)
	blocked := &TargetResult{Status: ResultBlocked, Error: "newer"}

	res := Enact(bg, s.ex, EnactInput{Repo: "acme/cons", Session: sess, Subject: s.subject(t), Decided: Decided{Done: blocked}, Apply: writeImage("new")}, nil)

	assert.Same(t, blocked, res)
	assert.Empty(t, runGit(t, s.clone, "branch", "--list", "bot/*"))
}
