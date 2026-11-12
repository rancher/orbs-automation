package ship

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/rancher/orbs-automation/internal/platform/ociregistry"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	err      error
	files    map[string]string
	cloneErr error // for CloneDir
}

var (
	graphOnce sync.Once
	theGraph  *chain.Graph
)

// graph is the config of the tests: scc-operator, which is universal, ships its
// image to rancher, whose one line is on its default branch.
func graph() *chain.Graph {
	graphOnce.Do(func() {
		fsys := fstest.MapFS{}
		for i, d := range []string{
			chaintest.Lines("v2.16"),
			chaintest.Doc("Project", "scc-operator", "", "  repo: {name: rancher/scc-operator, managedBy: orbs}\n"),
			chaintest.Doc("Stream", "scc-operator", "{project: scc-operator}", "  universal: true\n"),
			chaintest.Doc("Artifact", "scc-image", "{stream: scc-operator}", "  kind: image\n  name: rancher/scc-operator\n"),
			chaintest.Doc("Project", "rancher", "", "  repo: {name: rancher/rancher}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}]\n"),
			chaintest.Doc("Holding", "rancher-holds-scc", "{project: rancher, artifact: scc-image}",
				"  at: {file: build.yaml, key: image}\n  holds: {kind: release, form: \"{image}:{tag}\"}\n"),
		} {
			fsys[fmt.Sprintf("doc%d.yaml", i)] = &fstest.MapFile{Data: []byte(d)}
		}
		g, rep := chaintest.LoadFS(fsys)
		if err := rep.Err(); err != nil {
			panic(err)
		}
		theGraph = g
	})
	return theGraph
}

func (f *fakeWorkspace) Project(string) (*chain.Project, error) {
	if f.err != nil {
		return nil, f.err
	}
	return graph().Project("scc-operator"), nil
}

func (f *fakeWorkspace) Bump() (*bump.Engine, error) {
	return bump.NewEngine(bump.Builtin(), graph())
}

func (f *fakeWorkspace) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	if slug == "rancher/scc-operator" {
		return git.Refs{Tags: []git.Tag{{Name: "v0.5.3", Commit: "1111111aaaaaaaa"}}}, nil
	}
	return git.Refs{Branches: []git.Branch{{Name: "main", Commit: "2222222bbbbbbbb"}}}, nil
}

func (f *fakeWorkspace) UpstreamFile(context.Context, string, string, string) ([]byte, error) {
	return []byte(f.files["build.yaml"]), nil
}

func (f *fakeWorkspace) CloneDir(string) (string, error) { return "/clone", f.cloneErr }

func (f *fakeWorkspace) Remote(string) (string, error) { return "upstream", nil }

func (f *fakeWorkspace) PushSettings(string) domain.PushSettings {
	return domain.PushSettings{Upstream: "upstream"}
}

type fakeProber struct{ result ociregistry.Result }

func (f fakeProber) Exists(context.Context, string, string, string) ociregistry.Result {
	return f.result
}

var in = domain.Input{Team: "orbs", Project: "scc-operator", Lines: []string{"v2.16"}}

func readyWorkspace() *fakeWorkspace {
	return &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.5.2\n"}}
}

func TestRunReady(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}}, nil, clicfg.OutputText, in)

	require.NoError(t, err)
	for _, want := range []string{
		"rancher/scc-operator", "v0.5.3", "stable", "rancher/rancher", "main", "2222222",
		"rancher-holds-scc", "build.yaml", "rancher/scc-operator:v0.5.2", "rancher/scc-operator:v0.5.3",
		"ready", "Nothing was written",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "2222222bbbbbbbb", "commits are shortened")
}

func TestRunAlreadyUpToDate(t *testing.T) {
	ws := &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.5.3\n"}}
	out, err := runWith(context.Background(), ws, fakeProber{}, nil, clicfg.OutputText, in)
	require.NoError(t, err)
	assert.Contains(t, out, "already up to date")
	assert.NotContains(t, out, "Artifacts", "the artifacts were not checked")
}

func TestRunBlockedKeepsTheReport(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.NotFound}}, nil, clicfg.OutputText, in)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.Contains(t, out, "blocked")
	assert.Contains(t, out, "not found", "the missing images are listed")
}

func TestRunErrorsPrintNothing(t *testing.T) {
	boom := errors.New("boom")
	out, err := runWith(context.Background(), &fakeWorkspace{err: boom}, fakeProber{}, nil, clicfg.OutputText, in)
	assert.ErrorIs(t, err, boom)
	assert.Empty(t, out)
}

func TestRunJSON(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}}, nil, clicfg.OutputJSON, in)
	require.NoError(t, err)
	var got domain.Plan
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, domain.StatusReady, got.Status)
	require.Len(t, got.Targets, 1)
	assert.Equal(t, "2222222bbbbbbbb", got.Targets[0].Commit, "JSON carries the full commit")
}

func TestRunJSONWhenBlocked(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.NotFound}}, nil, clicfg.OutputJSON, in)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.True(t, json.Valid([]byte(out)))
}

func TestRunRejectsOtherOutputFormats(t *testing.T) {
	_, err := runWith(context.Background(), readyWorkspace(), fakeProber{}, nil, clicfg.OutputEnv, in)
	assert.ErrorContains(t, err, "not supported by ship")
	err = Run(context.Background(), io.Discard, nil, clicfg.Settings{Output: clicfg.OutputEnv}, t.TempDir(), Options{Project: "scc-operator"})
	assert.ErrorContains(t, err, "not supported by ship")
}

// hp is the one holding of a target: its file, what it holds and what it should.
func hp(current, want string) []domain.HoldingPlan {
	return []domain.HoldingPlan{{Name: "rancher-holds-scc", File: "build.yaml", Current: current, Want: want}}
}

func previewPlan() domain.Plan {
	return domain.Plan{
		Producer: "rancher/scc-operator", Consumer: "rancher/rancher", Holdings: []string{"rancher-holds-scc"},
		Releases: []domain.Release{{Tag: "v0.5.3", Stable: true}}, Status: domain.StatusReady,
		Targets: []domain.TargetPlan{{
			Target:  domain.Target{Branch: "release/v2.15", Commit: "2222222bbbbbbbb"},
			Release: "v0.5.3", Holdings: hp("a", "b"), Status: domain.StatusReady,
			Preview: &domain.Preview{
				Branch: "bot/scc-operator-v0.5.3-v2.15", Title: "[v2.15] Update scc-operator to v0.5.3",
				Labels:        []string{"status/auto-created", "team/orbs"},
				CommitMessage: "Update scc-operator to v0.5.3\n\nAutomation: gero ship\n",
				Body:          "## Summary\nbody text\n",
			},
		}},
	}
}

func TestRenderTextShowsTheBranchTitleAndLabels(t *testing.T) {
	out := renderText(previewPlan(), false)
	for _, want := range []string{
		"bot/scc-operator-v0.5.3-v2.15", "[v2.15] Update scc-operator to v0.5.3", "status/auto-created, team/orbs",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "body text", "the body only with the full preview")
	assert.NotContains(t, out, "Automation: gero ship")
}

func TestRenderTextFullPreviewShowsTheCommitMessageAndBody(t *testing.T) {
	out := renderText(previewPlan(), true)
	assert.Contains(t, out, "Commit message")
	assert.Contains(t, out, "Automation: gero ship")
	assert.Contains(t, out, "Body")
	assert.Contains(t, out, "body text")
}

func TestRenderTextHasNoPreviewWhenThereIsNone(t *testing.T) {
	p := previewPlan()
	p.Targets[0].Preview, p.Targets[0].Status, p.Status = nil, domain.StatusUpToDate, domain.StatusUpToDate
	out := renderText(p, true)
	assert.NotContains(t, out, "\nPR\n")
	assert.NotContains(t, out, "Commit message")
}

func TestRunShowsThePreviewFromTheDomain(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
		nil, clicfg.OutputText, in)
	require.NoError(t, err)
	assert.Contains(t, out, "bot/scc-operator-v0.5.3-main")
}

func TestRunJSONHasTheFullPreview(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
		nil, clicfg.OutputJSON, in)
	require.NoError(t, err)
	var got domain.Plan
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Len(t, got.Targets, 1)
	require.NotNil(t, got.Targets[0].Preview)
	assert.Contains(t, got.Targets[0].Preview.Body, "gero:ship v1", "JSON always carries the body and message")
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name, project, arg string
		args               []string
	}{
		{"nothing is the repository you are in, latest", "", "", nil},
		{"a project name", "scc-operator", "", []string{"scc-operator"}},
		{"a project slug", "rancher/scc-operator", "", []string{"rancher/scc-operator"}},
		{"a tag alone is for the repository you are in", "", "v0.5.3", []string{"v0.5.3"}},
		{"a tag without its v", "", "0.5.3", []string{"0.5.3"}},
		{"a prerelease tag", "", "v0.6.0-rc.1", []string{"v0.6.0-rc.1"}},
		{"a line alone", "", "v2.15", []string{"v2.15"}},
		{"a project and a tag", "scc-operator", "v0.5.3", []string{"scc-operator", "v0.5.3"}},
		{"a project and a line", "scc-operator", "v2.15", []string{"scc-operator", "v2.15"}},
		{"a name with digits is still a project", "scc-operator2", "", []string{"scc-operator2"}},
		{"a name that starts with v is a project", "vault", "", []string{"vault"}},
		{"a slug with a version-like name is a project", "rancher/v2.15", "", []string{"rancher/v2.15"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, arg := ParseArgs(tt.args)
			assert.Equal(t, tt.project, project)
			assert.Equal(t, tt.arg, arg)
		})
	}
}

func TestRunNotInAProjectSaysToNameOne(t *testing.T) {
	_, err := runWith(context.Background(), &fakeWorkspace{err: workspace.ErrNotProject}, fakeProber{}, nil, clicfg.OutputText, domain.Input{Lines: []string{"v2.16"}})
	assert.ErrorIs(t, err, workspace.ErrNotProject)
	assert.ErrorContains(t, err, "name the project")
}

func TestRunBlocksADowngradeAndAllowsItOnRequest(t *testing.T) {
	ws := &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.9.0\n"}}
	prober := fakeProber{result: ociregistry.Result{Status: ociregistry.Found}}

	out, err := runWith(context.Background(), ws, prober, nil, clicfg.OutputText, in)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.Contains(t, out, "downgrade: the release is older")
	assert.Contains(t, out, "--allow-downgrade")
	assert.NotContains(t, out, "Artifacts", "a blocked downgrade shows no preview or artifact check")
	assert.NotContains(t, out, "\nPR\n")

	allowed := in
	allowed.AllowDowngrade = true
	out, err = runWith(context.Background(), ws, prober, nil, clicfg.OutputText, allowed)
	require.NoError(t, err)
	assert.Contains(t, out, "this is a downgrade")
	assert.Contains(t, out, "Artifacts", "allowed, the check goes on")
}

func TestRunWithNoBranchChecksTheBranchesTheReleaseGoesTo(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
		nil, clicfg.OutputText, domain.Input{Team: "orbs", Project: "scc-operator"})
	require.NoError(t, err)
	assert.Contains(t, out, "Targets")
	assert.Contains(t, out, "main", "the consumer's one branch, since it is not tied to release lines")
}

func previewPlanWithTargets() domain.Plan {
	p := previewPlan()
	ready := p.Targets[0]
	upToDate := domain.TargetPlan{Target: domain.Target{Branch: "main", Commit: "3333333cccccc"}, Holdings: hp("b", "b"), Status: domain.StatusUpToDate}
	failed := domain.TargetPlan{Target: domain.Target{Branch: "release/v2.12"}, Status: domain.StatusFailed, Error: "read build.yaml: gone"}
	blocked := domain.TargetPlan{
		Target: domain.Target{Branch: "release/v2.14", Commit: "4444444dddddd"}, Holdings: hp("c", "b"), Status: domain.StatusBlocked,
		Blockers: []string{"downgrade: older"},
	}
	p.Targets = []domain.TargetPlan{upToDate, ready, blocked, failed}
	p.Status = domain.StatusBlocked
	return p
}

func TestRenderTextListsEveryTarget(t *testing.T) {
	out := renderText(previewPlanWithTargets(), false)
	for _, want := range []string{
		"BRANCH", "main", "already up to date", "release/v2.15", "ready", "release/v2.14", "blocked", "release/v2.12", "failed",
		"blocked (release/v2.14): downgrade: older", "release/v2.12 failed: read build.yaml: gone",
	} {
		assert.Contains(t, out, want)
	}
	assert.Contains(t, out, "bot/scc-operator-v0.5.3-v2.15", "only the target that needs a PR has a preview")
	assert.Equal(t, 1, strings.Count(out, "status/auto-created"), "the labels once")
}

func TestRenderTextGroupsTheSameReasonAcrossBranches(t *testing.T) {
	p := previewPlanWithTargets()
	for _, branch := range []string{"main", "release/v2.15"} {
		p.Targets = append(p.Targets, domain.TargetPlan{
			Target: domain.Target{Branch: branch + "-2", Commit: "5555555eeeeeee"}, Holdings: hp("c", "b"), Status: domain.StatusBlocked,
			Blockers: []string{"downgrade: older"}, Warnings: []string{"this is a downgrade"},
		})
	}
	p.Targets[2].Warnings = []string{"this is a downgrade"}
	out := renderText(p, false)

	assert.Equal(t, 1, strings.Count(out, "downgrade: older"), "one line for the same reason")
	assert.Contains(t, out, "release/v2.14, main-2, release/v2.15-2", "naming the branches it applies to")
	assert.Equal(t, 1, strings.Count(out, "this is a downgrade"))
}

func TestRenderTextSaysWhetherItWasADryRun(t *testing.T) {
	p := previewPlan()

	p.DryRun = true
	out := renderText(p, false)
	assert.Contains(t, out, "Dry run: nothing was written, and no branch or PR was created.")

	p.DryRun = false
	out = renderText(p, false)
	assert.NotContains(t, out, "Dry run")
	assert.Contains(t, out, "Nothing was written", "creating them is not built yet")
}

func TestRunPassesADryRunToThePlan(t *testing.T) {
	dry := in
	dry.DryRun = true
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
		nil, clicfg.OutputJSON, dry)
	require.NoError(t, err)
	var got domain.Plan
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.True(t, got.DryRun)
}

func TestRenderTextNoPushWithNoCommitsSaysSo(t *testing.T) {
	p := previewPlan()
	p.NoPush = true

	out := renderText(p, false)

	assert.Contains(t, out, "No push: no commit was made.")
	assert.NotContains(t, out, "Dry run")
}

// withResults is the plan with one result per target that has a PR.
func withResults(p domain.Plan, results ...*domain.TargetResult) domain.Plan {
	for i, r := range results {
		p.Targets[i].Result = r
	}
	return p
}

func committed(branch, commit string, place domain.Place, reason string) *domain.TargetResult {
	return &domain.TargetResult{Status: domain.ResultCommitted, Branch: branch, Commits: []domain.CommitInfo{{Hash: commit, Subject: "s"}}, Place: place, Reason: reason}
}

func TestRenderTextShowsTheCommitsAndHowToReviewThem(t *testing.T) {
	p := previewPlan()
	p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	p = withResults(p, committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, ""))

	out := renderText(p, false)

	for _, want := range []string{
		"Results", "committed", "in place", "abcdef0", "bot/scc-operator-v0.5.3-v2.15",
		"git -C /src/rancher diff upstream/release/v2.15...bot/scc-operator-v0.5.3-v2.15",
		"No push: the commits are on branches in your clone, with nothing pushed and no PR created.",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "abcdef0123456789", "commits are shortened")
	assert.NotContains(t, out, "Nothing was written")
}

func TestRenderTextADryRunShowsWouldCommitAndNoReview(t *testing.T) {
	p := previewPlan()
	p.DryRun, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	r := committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.Worktree, "a dry run never moves your checkout")
	r.Status = domain.ResultWouldCommit
	p = withResults(p, r)

	out := renderText(p, false)

	assert.Contains(t, out, "would commit")
	assert.Contains(t, out, "worktree")
	assert.NotContains(t, out, "git -C", "the branch was removed, so there is nothing to review")
	assert.Contains(t, out, "Dry run: the commits were made in temporary worktrees and removed. Nothing was pushed, and no branch or PR was created.")
	assert.NotContains(t, out, "Dry run: nothing was written")
}

func TestRenderTextGroupsTheReasonForAWorktreeAcrossBranches(t *testing.T) {
	p := previewPlanWithTargets()
	ready := p.Targets[1]
	second := ready
	second.Branch = "release/v2.13"
	second.Preview = &domain.Preview{Branch: "bot/scc-operator-v0.5.3-v2.13", Title: "t"}
	p.Targets = []domain.TargetPlan{ready, second}
	p.Status, p.Clone, p.Remote, p.NoPush = domain.StatusReady, "/src/rancher", "upstream", true
	reason := "the clone has uncommitted changes"
	p = withResults(p,
		committed("bot/scc-operator-v0.5.3-v2.15", "1111111aaaa", domain.Worktree, reason),
		committed("bot/scc-operator-v0.5.3-v2.13", "2222222bbbb", domain.Worktree, reason))

	out := renderText(p, false)

	assert.Equal(t, 1, strings.Count(out, reason), "once, naming the branches")
	assert.Contains(t, out, "worktree (release/v2.15, release/v2.13): "+reason)
}

func TestRenderTextShowsAFailedCommitAndWhy(t *testing.T) {
	p := previewPlan()
	p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	p = withResults(p, &domain.TargetResult{
		Status: domain.ResultFailed, Branch: "bot/scc-operator-v0.5.3-v2.15",
		Error: "method postEdit[0] ./scripts/gen.sh: exit status 1: no go toolchain",
	})

	out := renderText(p, false)

	assert.Contains(t, out, "failed (release/v2.15): method postEdit[0] ./scripts/gen.sh: exit status 1: no go toolchain")
	assert.NotContains(t, out, "git -C", "nothing to review")
}

func TestRenderTextAnUpToDateResultHasNoBranch(t *testing.T) {
	p := previewPlan()
	p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	p = withResults(p, &domain.TargetResult{Status: domain.ResultUpToDate, Branch: "bot/scc-operator-v0.5.3-v2.15"})

	out := renderText(p, false)

	assert.Contains(t, out, "already up to date")
	assert.NotContains(t, out, "git -C")
}

func TestRunReportsFailedTargetsAndReturnsErrFailed(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{Status: domain.ResultFailed, Branch: "b", Error: "boom"})

	assert.ErrorIs(t, failedErr(p), ErrFailed)
	p.Targets[0].Result.Status = domain.ResultCommitted
	assert.NoError(t, failedErr(p))
	p.Targets[0].Result = nil
	assert.NoError(t, failedErr(p))
}

func TestRunPassesNoPushToThePlanExceptInADryRun(t *testing.T) {
	for _, tc := range []struct{ dry, want bool }{{false, true}, {true, false}} {
		opts := in
		opts.NoPush, opts.DryRun = true, tc.dry
		out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
			nil, clicfg.OutputJSON, opts)
		require.NoError(t, err)
		var got domain.Plan
		require.NoError(t, json.Unmarshal([]byte(out), &got))
		assert.Equal(t, tc.want, got.NoPush, "dry run %v: with both, it is a dry run", tc.dry)
		assert.Equal(t, tc.dry, got.DryRun)
	}
}

func TestRunWithAnExecutorAndNoCloneReportsThePlanAndTheError(t *testing.T) {
	ws := readyWorkspace()
	ws.cloneErr = errors.New("rancher/rancher: clone it under a repos.search directory")
	opts := in
	opts.NoPush = true

	out, err := runWith(context.Background(), ws, fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
		&domain.Executor{}, clicfg.OutputText, opts)

	require.ErrorIs(t, err, domain.ErrNoClone)
	assert.Contains(t, err.Error(), "repos.search")
	assert.Contains(t, out, "ready", "the report is still written")
	assert.Contains(t, out, "bot/scc-operator-v0.5.3-main")
}

func TestRunWithoutAnExecutorOnlyChecks(t *testing.T) {
	ws := readyWorkspace()
	ws.cloneErr = errors.New("never asked")

	out, err := runWith(context.Background(), ws, fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
		nil, clicfg.OutputText, in)

	require.NoError(t, err)
	assert.Contains(t, out, "Nothing was written")
}

// TestRenderTextSectionsFollowTheOrderOfTheProcessing pins the order a reader
// follows: what each branch holds, then what stands in the way, the images, what
// the PR would be, and last what was done and where to see it.
func TestRenderTextSectionsFollowTheOrderOfTheProcessing(t *testing.T) {
	p := previewPlan()
	p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	p.Warnings = []string{"image rancher/x is not on Prime production"}
	p.Targets[0].Warnings = []string{"this is a downgrade"}
	p.Images = []domain.ReleaseImages{{Release: "v0.5.3", Report: registry.Report{
		Rows:     []registry.Row{{Image: "rancher/scc-operator", Label: "Docker Hub", Status: "found", Reference: "docker.io/rancher/scc-operator:v0.5.3"}},
		Warnings: p.Warnings,
	}}}
	p = withResults(p, committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, ""))

	out := renderText(p, false)

	// A warning is shown where it is found: the branch's with its branch, and the
	// images' with the images.
	order := []string{
		"Ship rancher/scc-operator", "Targets", "Warning (release/v2.15): this is a downgrade",
		"Artifacts", "Warning: image rancher/x", "\nPR\n", "Results", "Review:", "No push: the commits are on branches",
	}
	last := -1
	for _, section := range order {
		at := strings.Index(out, section)
		require.GreaterOrEqual(t, at, 0, "%q is in the output:\n%s", section, out)
		assert.Greater(t, at, last, "%q comes after the one before it", section)
		last = at
	}
}

func TestRenderTextListsEachCommitUnderItsBranchWithItsSubject(t *testing.T) {
	p := previewPlan()
	p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	r := committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, "")
	r.Commits = []domain.CommitInfo{
		{Hash: "abcdef0123456789", Subject: "Update scc-operator to v0.5.3"},
		{Hash: "1234567890abcdef", Subject: "Run `./scripts/generate-buildconfig`"},
	}
	p = withResults(p, r)

	out := renderText(p, false)

	assert.Contains(t, out, "abcdef0  Update scc-operator to v0.5.3")
	assert.Contains(t, out, "1234567  Run `./scripts/generate-buildconfig`")
	assert.Less(t, strings.Index(out, "abcdef0  Update"), strings.Index(out, "1234567  Run"), "oldest first")
	assert.Contains(t, out, "git -C /src/rancher log --oneline upstream/release/v2.15..bot/scc-operator-v0.5.3-v2.15",
		"the series, as well as the combined diff")
	assert.Contains(t, out, "git -C /src/rancher diff upstream/release/v2.15...bot/scc-operator-v0.5.3-v2.15")
}

func TestRenderTextACommitlessResultListsNoCommits(t *testing.T) {
	p := previewPlan()
	p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	p = withResults(p, &domain.TargetResult{Status: domain.ResultUpToDate, Branch: "bot/x"})

	out := renderText(p, false)

	assert.NotContains(t, out, "log --oneline")
}

// observingProber looks at what has been written when an image is looked for, to
// see what the run showed before its slowest check.
type observingProber struct {
	out  *bytes.Buffer
	seen *string
}

func (p observingProber) Exists(context.Context, string, string, string) ociregistry.Result {
	*p.seen = p.out.String()
	return ociregistry.Result{Status: ociregistry.Found}
}

func TestRunToShowsTheBranchesBeforeTheSlowImageCheck(t *testing.T) {
	var out bytes.Buffer
	var seen string

	err := runPlain(context.Background(), &out, nil, readyWorkspace(), observingProber{out: &out, seen: &seen}, nil, clicfg.OutputText, in)

	require.NoError(t, err)
	assert.Contains(t, seen, "Ship rancher/scc-operator", "the title was already written")
	assert.Contains(t, seen, "Targets", "and the branches, while the images were being looked for")
	assert.Contains(t, seen, "main", "and what each holds")
	assert.NotContains(t, seen, "Artifacts", "but not the artifacts, which were not known yet")
	assert.Contains(t, out.String(), "Artifacts", "they come after")
}

func TestRunToTextIsTheReportRunWithReturns(t *testing.T) {
	ws, prober := readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}}
	var out bytes.Buffer

	err := runPlain(context.Background(), &out, nil, ws, prober, nil, clicfg.OutputText, in)
	want, wantErr := runWith(context.Background(), ws, prober, nil, clicfg.OutputText, in)

	require.NoError(t, err)
	require.NoError(t, wantErr)
	assert.Equal(t, want, out.String())
}

func TestRunToJSONIsOneDocumentAtTheEnd(t *testing.T) {
	var out bytes.Buffer
	var seen string

	err := runPlain(context.Background(), &out, nil, readyWorkspace(), observingProber{out: &out, seen: &seen}, nil, clicfg.OutputJSON, in)

	require.NoError(t, err)
	assert.Empty(t, seen, "nothing was written while the run was going")
	var got domain.Plan
	require.NoError(t, json.Unmarshal(out.Bytes(), &got), out.String())
	assert.Equal(t, "rancher/scc-operator", got.Producer)
}

func TestRunToReturnsTheErrorAfterTheReport(t *testing.T) {
	var out bytes.Buffer

	err := runPlain(context.Background(), &out, nil, readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.NotFound}}, nil, clicfg.OutputText, in)

	require.ErrorIs(t, err, ErrBlocked)
	assert.Contains(t, out.String(), "status  blocked", "the report was written first")
}

func TestRunToLogsTheEventsBesideTheTextReport(t *testing.T) {
	var out, logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))

	err := runPlain(context.Background(), &out, log, readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}}, nil, clicfg.OutputText, in)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Targets", "the report is still written")
	assert.Contains(t, logs.String(), `"msg":"ship resolved"`)
	assert.Contains(t, logs.String(), `"msg":"target checked"`)
	assert.Contains(t, logs.String(), `"msg":"ship done"`)
}

func TestRunToLogsTheEventsAndKeepsJSONOnStdoutAsOneDocument(t *testing.T) {
	var out, logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))

	err := runPlain(context.Background(), &out, log, readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}}, nil, clicfg.OutputJSON, in)

	require.NoError(t, err)
	var plan domain.Plan
	require.NoError(t, json.Unmarshal(out.Bytes(), &plan), "stdout is the document and nothing else: %s", out.String())
	assert.Contains(t, logs.String(), `"msg":"target checked"`, "the progress is in the log, which is stderr")
	assert.NotContains(t, out.String(), "target checked")
}

func TestRunToWithNoLoggerWritesNoLogs(t *testing.T) {
	var out bytes.Buffer

	err := runPlain(context.Background(), &out, nil, readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}}, nil, clicfg.OutputText, in)

	require.NoError(t, err)
}

func TestInputFromCarriesTheFlagsAndTheSettings(t *testing.T) {
	s := clicfg.Settings{BranchPrefix: "dan/", DryRun: true, Team: "orbs"}
	opts := Options{Project: "scc-operator", Arg: "v0.5.3", To: "rancher", Lines: []string{"v2.16"}, Prerelease: true, AllowDowngrade: true, NoPush: true, PushTo: "upstream"}

	got := inputFrom(s, opts)

	assert.Equal(t, domain.Input{
		Project: "scc-operator", Arg: "v0.5.3", To: "rancher", Lines: []string{"v2.16"}, Prerelease: true, Team: "orbs",
		BranchPrefix: "dan/", AllowDowngrade: true, DryRun: true, NoPush: true, PushTo: "upstream",
	}, got)
}

func TestInputFromTheActorIsTheBotInCIAndTheEngineerOtherwise(t *testing.T) {
	assert.True(t, inputFrom(clicfg.Settings{CI: true}, Options{}).CI)
	assert.False(t, inputFrom(clicfg.Settings{}, Options{}).CI)
}

// published is a target whose branch was pushed and PR opened.
func published() *domain.TargetResult {
	r := committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, "")
	r.Status = domain.ResultOpened
	r.Pushed = "origin"
	r.PR = &domain.PRRef{Number: 104, URL: "https://github.com/rancher/rancher/pull/104"}
	r.Superseded = []domain.PRRef{{Number: 101, URL: "https://github.com/rancher/rancher/pull/101"}}
	return r
}

func TestRenderTextShowsEachStepOfPublishingAndTheLinkToThePR(t *testing.T) {
	p := previewPlan()
	p.Clone, p.Remote = "/src/rancher", "upstream"
	p = withResults(p, published())

	out := renderText(p, false)

	for _, want := range []string{
		"release/v2.15: pushed bot/scc-operator-v0.5.3-v2.15 to origin",
		"release/v2.15: opened #104 https://github.com/rancher/rancher/pull/104",
		"release/v2.15: closed #101",
		"opened",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "git -C", "an opened PR has nothing to review locally")
	assert.Contains(t, out, "Opened 1 PR.")
	assert.NotContains(t, out, "not built yet")
}

func TestRenderTextAnAlreadyOpenTargetNamesThePR(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{Status: domain.ResultAlreadyOpen, Branch: "bot/x", PR: &domain.PRRef{Number: 104, URL: "https://github.com/rancher/rancher/pull/104"}})

	out := renderText(p, false)

	assert.Contains(t, out, "release/v2.15: already open #104 https://github.com/rancher/rancher/pull/104")
	assert.Contains(t, out, "No PR was opened.")
}

func TestRenderTextABlockedTargetSaysWhy(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{Status: domain.ResultBlocked, Branch: "bot/x", Error: "#110 already ships v0.5.4 to release/v2.15, which is newer than v0.5.3"})

	out := renderText(p, false)

	assert.Contains(t, out, "release/v2.15: blocked: #110 already ships v0.5.4")
	assert.Contains(t, out, "blocked (release/v2.15): #110 already ships v0.5.4", "and in the summary, as a failure is")
	assert.NotContains(t, out, "failed (", "a blocked target did not fail")
}

func TestRenderTextWarningsOfAResultAreGroupedAcrossBranches(t *testing.T) {
	p := previewPlanWithTargets()
	ready := p.Targets[1]
	second := ready
	second.Branch = "release/v2.13"
	second.Preview = &domain.Preview{Branch: "bot/scc-operator-v0.5.3-v2.13", Title: "t"}
	p.Targets = []domain.TargetPlan{ready, second}
	p.Clone, p.Remote = "/src/rancher", "upstream"
	a, b := published(), published()
	b.Branch = "bot/scc-operator-v0.5.3-v2.13"
	a.Warnings = []string{"label team/orbs is not in rancher/rancher, so it was not applied"}
	b.Warnings = a.Warnings
	p = withResults(p, a, b)

	out := renderText(p, false)

	assert.Equal(t, 1, strings.Count(out, "label team/orbs is not in rancher/rancher"), "once, naming the branches")
	assert.Contains(t, out, "warning (release/v2.15, release/v2.13): label team/orbs is not in rancher/rancher")
}

func TestRenderTextADryRunSaysWhatItWouldClose(t *testing.T) {
	p := previewPlan()
	p.DryRun, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	r := committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.Worktree, "a dry run never moves your checkout")
	r.Status = domain.ResultWouldCommit
	r.WouldSupersede = []domain.PRRef{{Number: 101}, {Number: 90}}
	p = withResults(p, r)

	out := renderText(p, false)

	assert.Contains(t, out, "release/v2.15: would close #101, #90")
}

func TestRenderTextWhereItPushesIsShown(t *testing.T) {
	p := previewPlan()
	p.Clone, p.Remote = "/src/rancher", "upstream"
	p.Push = &domain.PushTarget{Role: domain.RoleFork, Remote: "origin", Source: "a fork is set"}
	p = withResults(p, published())

	out := renderText(p, false)

	assert.Contains(t, out, "push to  fork (origin), as a fork is set")
}

func TestRenderTextTheDecisionAboutEachTargetsExistingPRsIsShown(t *testing.T) {
	p := previewPlan()
	p = withResults(p, published())
	events := replay(p)
	events = append([]domain.Event{events[0]}, append([]domain.Event{{
		Kind: domain.EventDecided, Branch: "release/v2.15",
		Decision: &domain.Publish{Action: domain.PublishCreate, Supersede: []pulls.PR{{Number: 101}, {Number: 90}}},
	}}, events[1:]...)...)
	var buf bytes.Buffer
	stream := ui.NewStream(&buf)
	sink := newHumanSink(stream, false)

	for _, e := range events {
		sink.Event(e)
	}
	stream.Flush()

	assert.Contains(t, buf.String(), "release/v2.15: a new PR, and #101, #90 will be closed")
}

// quietRemote is a GitHub with no open PRs, which is asked nothing it cannot answer.
type quietRemote struct{ domain.Remote }

func (quietRemote) Me(context.Context) (string, error) { return "danpock", nil }

func (quietRemote) OpenForBase(context.Context, string, string, int) (pulls.Open, error) {
	return pulls.Open{}, nil
}

func (quietRemote) OpenForHead(context.Context, string, string) ([]pulls.PR, error) { return nil, nil }

func declining() *domain.Executor {
	return &domain.Executor{
		Publish: &domain.Publisher{Remote: quietRemote{}},
		Confirm: func(context.Context, domain.Confirmation) (bool, error) { return false, nil },
	}
}

func TestRunToADeclinedRunShowsWhatWasDecidedAndReturnsTheError(t *testing.T) {
	var out bytes.Buffer

	err := runPlain(context.Background(), &out, nil, readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
		declining(), clicfg.OutputText, in)

	require.ErrorIs(t, err, domain.ErrDeclined)
	assert.Contains(t, out.String(), "v2.16: a new PR", "what it was going to do was shown before it asked")
	assert.Contains(t, out.String(), "Nothing was written.")
}

func TestRunToADeclinedRunInJSONStillWritesThePlan(t *testing.T) {
	var out bytes.Buffer

	err := runPlain(context.Background(), &out, nil, readyWorkspace(), fakeProber{result: ociregistry.Result{Status: ociregistry.Found}},
		declining(), clicfg.OutputJSON, in)

	require.ErrorIs(t, err, domain.ErrDeclined)
	var plan domain.Plan
	require.NoError(t, json.Unmarshal(out.Bytes(), &plan), out.String())
	assert.Equal(t, "rancher/scc-operator", plan.Producer)
}

func TestRunToAnErrorBeforeAnyPlanIsJustTheError(t *testing.T) {
	var out bytes.Buffer

	err := runPlain(context.Background(), &out, nil, &fakeWorkspace{err: errors.New("no such project")}, fakeProber{},
		nil, clicfg.OutputJSON, in)

	require.Error(t, err)
	assert.Empty(t, out.String(), "no document for a run that never made a plan")
}

func TestAResultThatIsBlockedMakesTheRunBlocked(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{Status: domain.ResultBlocked, Branch: "b", Error: "#110 is newer"})

	assert.ErrorIs(t, failedErr(p), ErrBlocked)

	p.Targets[0].Result.Status = domain.ResultFailed
	assert.ErrorIs(t, failedErr(p), ErrFailed, "a failure outranks")
	p.Targets[0].Result.Status = domain.ResultAlreadyOpen
	assert.NoError(t, failedErr(p), "already open is fine")
	p.Targets[0].Result.Status = domain.ResultOpened
	assert.NoError(t, failedErr(p))
}

func TestRenderTextAnAlreadyOpenPRThatIsBehindSaysSoAndHowToRebuildIt(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{
		Status: domain.ResultAlreadyOpen, Branch: "bot/x", BehindBy: 7,
		PR: &domain.PRRef{Number: 104, URL: "https://github.com/rancher/rancher/pull/104"},
	})

	out := renderText(p, false)

	assert.Contains(t, out, "release/v2.15: already open #104 https://github.com/rancher/rancher/pull/104 (behind release/v2.15 by 7; --refresh to rebuild)")
}

func TestRenderTextAnAlreadyOpenPRThatIsCurrentHasNoBehindNote(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{Status: domain.ResultAlreadyOpen, Branch: "bot/x", PR: &domain.PRRef{Number: 104}})

	out := renderText(p, false)

	assert.NotContains(t, out, "behind")
}

func TestInputFromCarriesRefresh(t *testing.T) {
	assert.True(t, inputFrom(clicfg.Settings{}, Options{Refresh: true}).Refresh)
	assert.False(t, inputFrom(clicfg.Settings{}, Options{}).Refresh)
}

func refreshed() *domain.TargetResult {
	return &domain.TargetResult{
		Status: domain.ResultRefreshed, Branch: "bot/scc-operator-v0.5.3-v2.15", Pushed: "origin", Place: domain.InPlace,
		PR:      &domain.PRRef{Number: 104, URL: "https://github.com/rancher/rancher/pull/104"},
		Commits: []domain.CommitInfo{{Hash: "abcdef0123456789", Subject: "Update"}, {Hash: "1234567890abcdef", Subject: "Run gen"}},
	}
}

func TestRenderTextARefreshedPRIsShownWithItsNewCommits(t *testing.T) {
	p := previewPlan()
	p.Clone, p.Remote = "/src/rancher", "upstream"
	p = withResults(p, refreshed())

	out := renderText(p, false)

	assert.Contains(t, out, "release/v2.15: refreshed #104 https://github.com/rancher/rancher/pull/104")
	assert.Contains(t, out, "release/v2.15: pushed bot/scc-operator-v0.5.3-v2.15 to origin")
	assert.Contains(t, out, "refreshed")
	assert.Contains(t, out, "Refreshed 1 PR.")
	assert.NotContains(t, out, "git -C", "nothing is waiting in the clone to be reviewed")
}

func TestRenderTextADryRunThatWouldRefreshSaysSo(t *testing.T) {
	p := previewPlan()
	p.DryRun, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	r := refreshed()
	r.Status, r.Pushed = domain.ResultWouldRefresh, ""
	p = withResults(p, r)

	out := renderText(p, false)

	assert.Contains(t, out, "release/v2.15: would refresh #104")
}

func TestRenderTextTheNoteOnAnAlreadyOpenPRIsShown(t *testing.T) {
	for _, note := range []string{"identical to a fresh build", "not refreshed: commit 1187df75 is by t@example.com, not dev@example.com"} {
		p := previewPlan()
		p = withResults(p, &domain.TargetResult{Status: domain.ResultAlreadyOpen, Branch: "bot/x", PR: &domain.PRRef{Number: 104}, Note: note})

		out := renderText(p, false)

		assert.Contains(t, out, "release/v2.15: already open #104")
		assert.Contains(t, out, "("+note+")", note)
	}
}

func TestReplayOfARefreshedTargetHasTheRefreshedEventAfterThePush(t *testing.T) {
	p := previewPlan()
	p = withResults(p, refreshed())

	got := kinds(replay(p))

	assert.Equal(t, []domain.EventKind{
		domain.EventResolved, domain.EventTargetChecked, domain.EventTargetsChecked,
		domain.EventTargetStarted, domain.EventCommit, domain.EventCommit, domain.EventPushed, domain.EventRefreshed,
		domain.EventTargetDone, domain.EventDone,
	}, got)
}

func TestRenderTextEndsWithTheLinksToThePRsThatWereOpenedRefreshedOrAlreadyOpen(t *testing.T) {
	p := previewPlanWithTargets()
	ready := p.Targets[1]
	mk := func(branch string, r *domain.TargetResult) domain.TargetPlan {
		t := ready
		t.Branch = branch
		t.Preview = &domain.Preview{Branch: "bot/scc-operator-v0.5.3-" + branch, Title: "t"}
		t.Result = r
		return t
	}
	opened := published()
	again := refreshed()
	open := &domain.TargetResult{Status: domain.ResultAlreadyOpen, Branch: "bot/x", PR: &domain.PRRef{Number: 90, URL: "https://github.com/rancher/rancher/pull/90"}}
	failed := &domain.TargetResult{Status: domain.ResultFailed, Branch: "bot/y", Error: "push rejected"}
	p.Targets = []domain.TargetPlan{mk("release/v2.15", opened), mk("main", again), mk("release/v2.14", open), mk("release/v2.13", failed)}
	p.Clone, p.Remote = "/src/rancher", "upstream"

	out := renderText(p, false)

	i := strings.LastIndex(out, "Pull requests")
	require.GreaterOrEqual(t, i, 0, "a list of them: %s", out)
	list := out[i:]
	assert.Greater(t, i, strings.Index(out, "Opened"), "after the line that says what the run did")
	for _, line := range strings.Split(strings.TrimRight(list, "\n"), "\n")[1:] {
		assert.True(t, strings.HasPrefix(line, "  "), "only rows of the list follow its heading, as it is the last thing printed: %q", line)
	}
	for _, want := range []string{
		"opened", "release/v2.15", "https://github.com/rancher/rancher/pull/104",
		"refreshed", "main",
		"already open", "release/v2.14", "https://github.com/rancher/rancher/pull/90",
	} {
		assert.Contains(t, list, want)
	}
	assert.NotContains(t, list, "release/v2.13", "a target that failed has no PR to link")
	assert.NotContains(t, list, "push rejected")
}

func TestRenderTextHasNoListOfPRsWhenThereAreNone(t *testing.T) {
	p := previewPlan()
	p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	p = withResults(p, committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, ""))

	assert.NotContains(t, renderText(p, false), "Pull requests")
	assert.NotContains(t, renderText(previewPlan(), false), "Pull requests")
}

func TestRenderTextADryRunListsNoPRs(t *testing.T) {
	p := previewPlan()
	p.DryRun, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	r := refreshed()
	r.Status = domain.ResultWouldRefresh
	p = withResults(p, r)

	assert.NotContains(t, renderText(p, false), "Pull requests", "nothing was opened, so there is nothing to click")
}

// onLines is the plan of previewPlan with its target on a Rancher line, as a run
// against rancher gives: the line is v2.15 and the consumer's branch is
// release/v2.15.
func onLines() domain.Plan {
	p := previewPlan()
	p.Targets[0].Line = "v2.15"
	p.Clone, p.Remote = "/src/rancher", "upstream"
	return p
}

func TestRenderTextTargetsAreKeyedByLineWithTheBranchBesideIt(t *testing.T) {
	out := renderText(onLines(), false)

	assert.Regexp(t, `(?m)^  LINE\s+BRANCH\s+HEAD\s+ARTIFACT\s+NOW\s+STATUS$`, out)
	assert.Regexp(t, `(?m)^  v2\.15\s+release/v2\.15\s+2222222`, out)
}

func TestRenderTextStreamedLinesAreKeyedByLineAndAPushNamesTheBranch(t *testing.T) {
	p := onLines()
	r := published()
	p = withResults(p, r)

	out := renderText(p, false)

	for _, want := range []string{
		"v2.15: bot/scc-operator-v0.5.3-v2.15, in place",
		"v2.15: pushed bot/scc-operator-v0.5.3-v2.15 to origin",
		"v2.15: opened #104 https://github.com/rancher/rancher/pull/104",
		"v2.15: closed #101",
		"v2.15: opened",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "release/v2.15: ", "the line is the key, not the base branch")
}

func TestRenderTextABranchInNoLineIsKeyedByItsBranch(t *testing.T) {
	p := previewPlan()
	p.Clone, p.Remote = "/src/rancher", "upstream"
	p = withResults(p, published())

	out := renderText(p, false)

	assert.Contains(t, out, "release/v2.15: pushed bot/scc-operator-v0.5.3-v2.15 to origin")
	assert.Regexp(t, `(?m)^  release/v2\.15\s+release/v2\.15\s+2222222`, out, "the branch twice, as there is no line to name it")
}

func TestRenderTextTheSummaryTablesAndGroupsAreKeyedByLine(t *testing.T) {
	p := onLines()
	r := committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.Worktree, "the clone has uncommitted changes")
	p.NoPush = true
	p = withResults(p, r)

	out := renderText(p, false)

	assert.Regexp(t, `(?m)^  LINE\s+RESULT\s+WHERE\s+BRANCH$`, out)
	assert.Regexp(t, `(?m)^  v2\.15\s+committed\s+worktree\s+bot/scc-operator-v0\.5\.3-v2\.15$`, out)
	assert.Contains(t, out, "worktree (v2.15): the clone has uncommitted changes")
	assert.Contains(t, out, "git -C /src/rancher log --oneline upstream/release/v2.15..bot/scc-operator-v0.5.3-v2.15", "a command to copy keeps the branch")
}

func TestRenderTextThePRPreviewAndTheClosingListAreKeyedByLine(t *testing.T) {
	p := onLines()
	p = withResults(p, published())

	out := renderText(p, false)

	assert.Regexp(t, `(?m)^  LINE\s+NEW BRANCH\s+TITLE$`, out)
	assert.Regexp(t, `(?m)^  v2\.15\s+bot/scc-operator-v0\.5\.3-v2\.15\s+\[v2\.15\] Update`, out)
	assert.Regexp(t, `(?m)^  RESULT\s+LINE\s+BASE\s+LINK$`, out)
	assert.Regexp(t, `(?m)^  opened\s+v2\.15\s+release/v2\.15\s+https://github.com/rancher/rancher/pull/104$`, out)
}

func TestRenderTextWarningsAndBlockedAreGroupedByLine(t *testing.T) {
	p := previewPlanWithTargets()
	p.Targets[2].Line = "v2.14"
	p.Targets[2].Warnings = []string{"this is a downgrade"}

	out := renderText(p, false)

	assert.Contains(t, out, "blocked (v2.14): downgrade: older")
	assert.Contains(t, out, "Warning (v2.14): this is a downgrade")
}

func TestRenderTextADryRunsWouldCloseIsKeyedByLine(t *testing.T) {
	p := onLines()
	p.DryRun = true
	r := committed("bot/x", "abcdef0123456789", domain.Worktree, "a dry run never moves your checkout")
	r.Status = domain.ResultWouldCommit
	r.WouldSupersede = []domain.PRRef{{Number: 101}}
	p = withResults(p, r)

	out := renderText(p, false)

	assert.Regexp(t, `(?m)^  v2\.15: would close #101$`, out)
}

func TestReplayCarriesTheLineAndWhatWasPushed(t *testing.T) {
	p := onLines()
	p = withResults(p, published())

	events := replay(p)

	for _, e := range events {
		switch e.Kind {
		case domain.EventTargetChecked, domain.EventTargetStarted, domain.EventCommit, domain.EventPushed, domain.EventPROpened,
			domain.EventSuperseded, domain.EventTargetDone:
			assert.Equal(t, "v2.15", e.Line, string(e.Kind))
		}
		if e.Kind == domain.EventPushed {
			assert.Equal(t, "bot/scc-operator-v0.5.3-v2.15", e.NewBranch)
		}
	}
}

func TestLogSinkRecordsTheLineAndWhatWasPushed(t *testing.T) {
	recs := logged(t, slog.LevelInfo,
		domain.Event{Kind: domain.EventPushed, Branch: "release/v2.15", Line: "v2.15", Remote: "origin", NewBranch: "bot/x"},
		domain.Event{Kind: domain.EventTargetChecked, Branch: "main", Target: &domain.TargetPlan{Target: domain.Target{Branch: "main"}}},
	)

	require.Len(t, recs, 2)
	assert.Equal(t, "v2.15", recs[0]["line"])
	assert.Equal(t, "release/v2.15", recs[0]["branch"])
	assert.Equal(t, "bot/x", recs[0]["new_branch"])
	_, hasLine := recs[1]["line"]
	assert.False(t, hasLine, "no line attribute for a branch in no line")
}

func behind() *domain.TargetResult {
	return &domain.TargetResult{
		Status: domain.ResultAlreadyOpen, Branch: "bot/x", BehindBy: 7,
		PR: &domain.PRRef{Number: 104, URL: "https://github.com/rancher/rancher/pull/104"},
	}
}

func TestRenderTextDoesNotSuggestRefreshWhenItWasGiven(t *testing.T) {
	p := onLines()
	p.Refresh = true
	r := behind()
	r.Note = "identical to a fresh build"
	p = withResults(p, r)

	out := renderText(p, false)

	assert.Contains(t, out, "v2.15: already open #104 https://github.com/rancher/rancher/pull/104 (behind release/v2.15 by 7) (identical to a fresh build)",
		"how far behind is still a fact worth showing")
	assert.NotContains(t, out, "--refresh")
}

func TestRenderTextStillSuggestsRefreshWhenItWasNotGiven(t *testing.T) {
	p := onLines()
	p = withResults(p, behind())

	assert.Contains(t, renderText(p, false), "(behind release/v2.15 by 7; --refresh to rebuild)")
}

func TestRenderTextResultsOfTargetsThatNeededNoWorkHaveNoWhereColumn(t *testing.T) {
	p := onLines()
	p = withResults(p, behind()) // already open: nothing was built, so nothing was built anywhere

	out := renderText(p, false)

	assert.Contains(t, out, "\nResults\n")
	assert.NotContains(t, out, "\nCommits\n", "there are no commits to list")
	assert.Regexp(t, `(?m)^  LINE\s+RESULT\s+BRANCH$`, out)
	assert.NotContains(t, out, "WHERE")
	assert.Regexp(t, `(?m)^  v2\.15\s+already open\s+bot/x$`, out)
}

func TestRenderTextAMixOfWorkAndNoWorkShowsWhereWithADashForNoWork(t *testing.T) {
	p := previewPlanWithTargets()
	ready := p.Targets[1]
	second := ready
	second.Branch, second.Line = "release/v2.14", "v2.14"
	second.Preview = &domain.Preview{Branch: "bot/scc-operator-v0.5.3-v2.14", Title: "t"}
	ready.Line = "v2.15"
	p.Targets = []domain.TargetPlan{ready, second}
	p.Clone, p.Remote, p.NoPush = "/src/rancher", "upstream", true
	p = withResults(p, committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, ""),
		&domain.TargetResult{Status: domain.ResultAlreadyOpen, Branch: "bot/scc-operator-v0.5.3-v2.14", PR: &domain.PRRef{Number: 90}})

	out := renderText(p, false)

	assert.Regexp(t, `(?m)^  LINE\s+RESULT\s+WHERE\s+BRANCH$`, out)
	assert.Regexp(t, `(?m)^  v2\.15\s+committed\s+in place\s+bot/scc-operator-v0\.5\.3-v2\.15$`, out)
	assert.Regexp(t, `(?m)^  v2\.14\s+already open\s+-\s+bot/scc-operator-v0\.5\.3-v2\.14$`, out)
}

// eachLine is the plan of a producer whose lines each have a release: two targets,
// two releases, two values.
func eachLine() domain.Plan {
	p := previewPlan()
	p.Producer = "rancher/shell"
	p.Releases = []domain.Release{{Tag: "v0.9.1-rc.2"}, {Tag: "v0.8.3", Stable: true}}
	p.Notes = []string{"v2.14: no such release: rancher/shell has no tags in line v2.14"}
	second := p.Targets[0]
	p.Targets[0].Line, p.Targets[0].Release, p.Targets[0].Holdings = "v2.16", "v0.9.1-rc.2", hp("a", "rancher/shell:v0.9.1-rc.2")
	second.Line, second.Release, second.Holdings = "v2.15", "v0.8.3", hp("a", "rancher/shell:v0.8.3")
	second.Preview = nil
	p.Targets = append(p.Targets, second)
	p.Images = []domain.ReleaseImages{
		{Release: "v0.9.1-rc.2", Report: registry.Report{Rows: []registry.Row{{Image: "rancher/shell", Label: "Docker Hub", Status: "found", Reference: "docker.io/rancher/shell:v0.9.1-rc.2"}}}},
		{Release: "v0.8.3", Report: registry.Report{Rows: []registry.Row{{Image: "rancher/shell", Label: "Docker Hub", Status: "found", Reference: "docker.io/rancher/shell:v0.8.3"}}}},
	}
	return p
}

func TestRenderTextOfALinePerReleasePlanNamesTheReleaseAndValueOfEachTarget(t *testing.T) {
	out := renderText(eachLine(), false)

	assert.Contains(t, out, "Ship rancher/shell to rancher/rancher\n", "no one release in the title")
	assert.NotContains(t, out, "want    ", "no one value either")
	for _, want := range []string{"RELEASE", "WANT", "v2.16", "v0.9.1-rc.2", "rancher/shell:v0.9.1-rc.2", "v2.15", "v0.8.3", "rancher/shell:v0.8.3"} {
		assert.Contains(t, out, want)
	}
	assert.Contains(t, out, "Artifacts (v0.9.1-rc.2)")
	assert.Contains(t, out, "Artifacts (v0.8.3)")
	assert.Contains(t, out, "left out  v2.14: no such release")
}

func TestRenderTextOfASingleReleasePlanIsAsItWas(t *testing.T) {
	out := renderText(previewPlan(), false)

	assert.Contains(t, out, "Ship rancher/scc-operator v0.5.3 (stable) to rancher/rancher")
	assert.Contains(t, out, "  want    b")
	assert.NotContains(t, out, "RELEASE")
	assert.NotContains(t, out, "WANT")
	assert.NotContains(t, out, "Artifacts (")
}

func actingPlan(signing bool, login string) domain.Plan {
	p := previewPlan()
	p.Actor = &domain.Actor{Identity: domain.Identity{Name: "Dan Pock", Email: "dan@example.com"}, Login: login, Signing: signing}
	return p
}

func TestRenderTextSaysWhoTheRunActsAsAfterWhatItWillOpenAndBeforeTheWork(t *testing.T) {
	p := actingPlan(false, "danpock")
	p = withResults(p, committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, ""))

	out := renderText(p, false)

	assert.Contains(t, out, "  acting as  Dan Pock <dan@example.com>, commits not signed, PRs by danpock\n")
	assert.Less(t, strings.Index(out, "\nPR\n"), strings.Index(out, "acting as"))
	assert.Less(t, strings.Index(out, "acting as"), strings.Index(out, "Working"))
}

func TestRenderTextOfASignedRunWithNoLoginSaysSoAndLeavesOutThePRAuthor(t *testing.T) {
	out := renderText(actingPlan(true, ""), false)

	assert.Contains(t, out, "acting as  Dan Pock <dan@example.com>, commits signed\n")
	assert.NotContains(t, out, "PRs by")
}

func TestRenderTextOfAPlanWithNoActorHasNoSuchLine(t *testing.T) {
	assert.NotContains(t, renderText(previewPlan(), false), "acting as")
}

func TestReplayOfAPlanWithAnActorHasTheEvent(t *testing.T) {
	events := replay(actingPlan(false, "danpock"))

	assert.Contains(t, kinds(events), domain.EventActor)
}

func TestRenderTextShowsTheFilesEachCommitChangedUnderIt(t *testing.T) {
	p := previewPlan()
	p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
	r := committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, "")
	var many []string
	for i := 1; i <= 12; i++ {
		many = append(many, fmt.Sprintf("generated/f%d.go", i))
	}
	r.Commits = []domain.CommitInfo{
		{Hash: "abcdef0123456789", Subject: "Update scc-operator to v0.5.3", Files: []string{"build.yaml"}},
		{Hash: "1234567890abcdef", Subject: "Run `./scripts/generate-buildconfig`", Files: many},
		{Hash: "fedcba9876543210", Subject: "Run `go mod tidy`", Files: []string{"go.mod", "go.sum"}},
		{Hash: "0f0f0f0f0f0f0f0f", Subject: "Run `nothing`"},
	}
	p = withResults(p, r)

	out := renderText(p, false)

	for _, want := range []string{
		"    abcdef0  Update scc-operator to v0.5.3\n             build.yaml\n",
		"    1234567  Run `./scripts/generate-buildconfig`\n             12 files: generated/f1.go, generated/f2.go, generated/f3.go and 9 more\n",
		"    fedcba9  Run `go mod tidy`\n             2 files: go.mod, go.sum\n",
		"    0f0f0f0  Run `nothing`\n  release/v2.15: committed",
	} {
		assert.Contains(t, out, want)
	}
}

func TestFilesText(t *testing.T) {
	assert.Equal(t, "", filesText(nil))
	assert.Equal(t, "a", filesText([]string{"a"}))
	assert.Equal(t, "2 files: a, b", filesText([]string{"a", "b"}))
	assert.Equal(t, "4 files: a, b, c, d", filesText([]string{"a", "b", "c", "d"}))
	assert.Equal(t, "5 files: a, b, c and 2 more", filesText([]string{"a", "b", "c", "d", "e"}))
}

func TestRenderTextWithThePreviewShowsTheCommitEachStepWouldMake(t *testing.T) {
	p := previewPlan()
	p.Targets[0].Line = "v2.15"
	p.Targets[0].Preview.Steps = []domain.StepPreview{
		{Phase: "method preEdit", Run: "scripts/prepare", BeforeEdit: true, CommitMessage: "Run `scripts/prepare`\n\nPart of updating scc-operator to v0.5.3: the method preEdit step.\n\nAutomation: gero ship\n"},
		{Phase: "edge postEdit", Run: "scripts/generate-buildconfig", CommitMessage: "Run `scripts/generate-buildconfig`\n\nAutomation: gero ship\n"},
	}

	out := renderText(p, true)

	for _, want := range []string{
		"Step commits (v2.15)",
		"a step's commit is made only if it changes files",
		"before the edit  Run `scripts/prepare`  (method preEdit)",
		"after the edit   Run `scripts/generate-buildconfig`  (edge postEdit)",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRenderTextWithoutThePreviewFlagLeavesTheStepCommitsOut(t *testing.T) {
	p := previewPlan()
	p.Targets[0].Preview.Steps = []domain.StepPreview{{Phase: "edge postEdit", Run: "gen", CommitMessage: "Run `gen`"}}

	assert.NotContains(t, renderText(p, false), "Step commits")
}

func TestRenderTextWithNoStepsHasNoStepCommitsBlock(t *testing.T) {
	assert.NotContains(t, renderText(previewPlan(), true), "Step commits")
}
