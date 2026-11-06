package ship

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	err      error
	files    map[string]string
	cloneErr error // for CloneDir
}

func sccProducer() config.Project {
	return config.Project{
		Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}, Release: config.Release{TagPattern: "v*"},
		Downstream: []config.Edge{{
			To: "rancher", Method: "build-yaml-bump",
			Params: map[string]string{"updateKey": "image", "value": "{image}:{tag}"},
		}},
	}
}

func (f *fakeWorkspace) Project(string) (config.Project, error) {
	if f.err != nil {
		return config.Project{}, f.err
	}
	return sccProducer(), nil
}

func (f *fakeWorkspace) Updates() (*updatedispatcher.Engine, error) {
	rancher := config.Project{
		Name: "rancher", Repo: config.Repo{Name: "rancher/rancher", DefaultBranch: "main"},
		UpdateMethods: []config.UpdateMethod{{
			Name: "build-yaml-bump", Strategy: "yamlBump", With: map[string]string{"file": "build.yaml"},
			AllowedParams: []config.AllowedParam{{Key: "updateKey"}, {Key: "value", Default: "{tag}"}},
			Accept:        config.Accept{When: config.AcceptStable},
		}},
	}
	return updatedispatcher.NewEngine(updatedispatcher.Builtin(), config.Config{Projects: []config.Project{sccProducer(), rancher}})
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

func (f *fakeWorkspace) PushSettings(string) domain.PushSettings { return domain.PushSettings{} }

type fakeProber struct{ result registry.Result }

func (f fakeProber) Exists(context.Context, string, string, string) registry.Result { return f.result }

var in = domain.Input{Project: "scc-operator", Branch: "main"}

func readyWorkspace() *fakeWorkspace {
	return &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.5.2\n"}}
}

func TestRunReady(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, nil, clicfg.OutputText, in)

	require.NoError(t, err)
	for _, want := range []string{
		"rancher/scc-operator", "v0.5.3", "stable", "rancher/rancher", "main", "2222222",
		"build-yaml-bump", "yamlBump", "build.yaml", "rancher/scc-operator:v0.5.2", "rancher/scc-operator:v0.5.3",
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
	assert.NotContains(t, out, "Images", "the images were not checked")
}

func TestRunBlockedKeepsTheReport(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.NotFound}}, nil, clicfg.OutputText, in)
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
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, nil, clicfg.OutputJSON, in)
	require.NoError(t, err)
	var got domain.Plan
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, domain.StatusReady, got.Status)
	require.Len(t, got.Targets, 1)
	assert.Equal(t, "2222222bbbbbbbb", got.Targets[0].Commit, "JSON carries the full commit")
}

func TestRunJSONWhenBlocked(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.NotFound}}, nil, clicfg.OutputJSON, in)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.True(t, json.Valid([]byte(out)))
}

func TestRunRejectsOtherOutputFormats(t *testing.T) {
	_, err := runWith(context.Background(), readyWorkspace(), fakeProber{}, nil, clicfg.OutputEnv, in)
	assert.ErrorContains(t, err, "not supported by ship")
	err = Run(context.Background(), io.Discard, nil, clicfg.Settings{Output: clicfg.OutputEnv}, t.TempDir(), Options{Project: "scc-operator"})
	assert.ErrorContains(t, err, "not supported by ship")
}

func previewPlan() domain.Plan {
	return domain.Plan{
		Producer: "rancher/scc-operator", Consumer: "rancher/rancher", Method: "build-yaml-bump", Strategy: "yamlBump", File: "build.yaml",
		Release: domain.Release{Tag: "v0.5.3", Stable: true}, Want: "b", Status: domain.StatusReady,
		Targets: []domain.TargetPlan{{
			Target:  domain.Target{Branch: "release/v2.15", Commit: "2222222bbbbbbbb"},
			Current: "a", Status: domain.StatusReady,
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
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
		nil, clicfg.OutputText, in)
	require.NoError(t, err)
	assert.Contains(t, out, "bot/scc-operator-v0.5.3-main")
}

func TestRunJSONHasTheFullPreview(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
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
	_, err := runWith(context.Background(), &fakeWorkspace{err: workspace.ErrNotProject}, fakeProber{}, nil, clicfg.OutputText, domain.Input{Branch: "main"})
	assert.ErrorIs(t, err, workspace.ErrNotProject)
	assert.ErrorContains(t, err, "name the project")
}

func TestRunBlocksADowngradeAndAllowsItOnRequest(t *testing.T) {
	ws := &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.9.0\n"}}
	prober := fakeProber{result: registry.Result{Status: registry.Found}}

	out, err := runWith(context.Background(), ws, prober, nil, clicfg.OutputText, in)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.Contains(t, out, "downgrade: the release is older")
	assert.Contains(t, out, "--allow-downgrade")
	assert.NotContains(t, out, "Images", "a blocked downgrade shows no preview or image check")
	assert.NotContains(t, out, "\nPR\n")

	allowed := in
	allowed.AllowDowngrade = true
	out, err = runWith(context.Background(), ws, prober, nil, clicfg.OutputText, allowed)
	require.NoError(t, err)
	assert.Contains(t, out, "this is a downgrade")
	assert.Contains(t, out, "Images", "allowed, the check goes on")
}

func TestRunWithNoBranchChecksTheBranchesTheReleaseGoesTo(t *testing.T) {
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
		nil, clicfg.OutputText, domain.Input{Project: "scc-operator"})
	require.NoError(t, err)
	assert.Contains(t, out, "Targets")
	assert.Contains(t, out, "main", "the consumer's one branch, since it is not tied to release lines")
}

func previewPlanWithTargets() domain.Plan {
	p := previewPlan()
	ready := p.Targets[0]
	upToDate := domain.TargetPlan{Target: domain.Target{Branch: "main", Commit: "3333333cccccc"}, Current: "b", Status: domain.StatusUpToDate}
	failed := domain.TargetPlan{Target: domain.Target{Branch: "release/v2.12"}, Status: domain.StatusFailed, Error: "read build.yaml: gone"}
	blocked := domain.TargetPlan{
		Target: domain.Target{Branch: "release/v2.14", Commit: "4444444dddddd"}, Current: "c", Status: domain.StatusBlocked,
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
			Target: domain.Target{Branch: branch + "-2", Commit: "5555555eeeeeee"}, Current: "c", Status: domain.StatusBlocked,
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
	out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
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
		"Commits", "committed", "in place", "abcdef0", "bot/scc-operator-v0.5.3-v2.15",
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
		out, err := runWith(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
			nil, clicfg.OutputJSON, opts)
		require.NoError(t, err)
		var got domain.Plan
		require.NoError(t, json.Unmarshal([]byte(out), &got))
		assert.Equal(t, tc.want, got.NoPush, "dry run %v: with both, it is a dry run", tc.dry)
		assert.Equal(t, tc.dry, got.DryRun)
	}
}

func TestWantsCommitsOnlyForNoPushAndDryRun(t *testing.T) {
	assert.False(t, wantsCommits(false, Options{}), "a plain run only checks until the push is built")
	assert.True(t, wantsCommits(false, Options{NoPush: true}))
	assert.True(t, wantsCommits(true, Options{}))
	assert.True(t, wantsCommits(true, Options{NoPush: true}))
}

func TestRunWithAnExecutorAndNoCloneReportsThePlanAndTheError(t *testing.T) {
	ws := readyWorkspace()
	ws.cloneErr = errors.New("rancher/rancher: clone it under a repos.search directory")
	opts := in
	opts.NoPush = true

	out, err := runWith(context.Background(), ws, fakeProber{result: registry.Result{Status: registry.Found}},
		&domain.Executor{}, clicfg.OutputText, opts)

	require.ErrorIs(t, err, domain.ErrNoClone)
	assert.Contains(t, err.Error(), "repos.search")
	assert.Contains(t, out, "ready", "the report is still written")
	assert.Contains(t, out, "bot/scc-operator-v0.5.3-main")
}

func TestRunWithoutAnExecutorOnlyChecks(t *testing.T) {
	ws := readyWorkspace()
	ws.cloneErr = errors.New("never asked")

	out, err := runWith(context.Background(), ws, fakeProber{result: registry.Result{Status: registry.Found}},
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
	p.Images = &registry.Report{
		Rows:     []registry.Row{{Image: "rancher/scc-operator", Label: "Docker Hub", Status: "found", Reference: "docker.io/rancher/scc-operator:v0.5.3"}},
		Warnings: p.Warnings,
	}
	p = withResults(p, committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, ""))

	out := renderText(p, false)

	// A warning is shown where it is found: the branch's with its branch, and the
	// images' with the images.
	order := []string{
		"Ship rancher/scc-operator", "Targets", "Warning (release/v2.15): this is a downgrade",
		"Images", "Warning: image rancher/x", "\nPR\n", "Commits", "Review:", "No push: the commits are on branches",
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

func (p observingProber) Exists(context.Context, string, string, string) registry.Result {
	*p.seen = p.out.String()
	return registry.Result{Status: registry.Found}
}

func TestRunToShowsTheBranchesBeforeTheSlowImageCheck(t *testing.T) {
	var out bytes.Buffer
	var seen string

	err := runTo(context.Background(), &out, nil, readyWorkspace(), observingProber{out: &out, seen: &seen}, nil, clicfg.OutputText, in, false)

	require.NoError(t, err)
	assert.Contains(t, seen, "Ship rancher/scc-operator", "the title was already written")
	assert.Contains(t, seen, "Targets", "and the branches, while the images were being looked for")
	assert.Contains(t, seen, "main", "and what each holds")
	assert.NotContains(t, seen, "Images", "but not the images, which were not known yet")
	assert.Contains(t, out.String(), "Images", "they come after")
}

func TestRunToTextIsTheReportRunWithReturns(t *testing.T) {
	ws, prober := readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}
	var out bytes.Buffer

	err := runTo(context.Background(), &out, nil, ws, prober, nil, clicfg.OutputText, in, false)
	want, wantErr := runWith(context.Background(), ws, prober, nil, clicfg.OutputText, in)

	require.NoError(t, err)
	require.NoError(t, wantErr)
	assert.Equal(t, want, out.String())
}

func TestRunToJSONIsOneDocumentAtTheEnd(t *testing.T) {
	var out bytes.Buffer
	var seen string

	err := runTo(context.Background(), &out, nil, readyWorkspace(), observingProber{out: &out, seen: &seen}, nil, clicfg.OutputJSON, in, false)

	require.NoError(t, err)
	assert.Empty(t, seen, "nothing was written while the run was going")
	var got domain.Plan
	require.NoError(t, json.Unmarshal(out.Bytes(), &got), out.String())
	assert.Equal(t, "rancher/scc-operator", got.Producer)
}

func TestRunToReturnsTheErrorAfterTheReport(t *testing.T) {
	var out bytes.Buffer

	err := runTo(context.Background(), &out, nil, readyWorkspace(), fakeProber{result: registry.Result{Status: registry.NotFound}}, nil, clicfg.OutputText, in, false)

	require.ErrorIs(t, err, ErrBlocked)
	assert.Contains(t, out.String(), "status  blocked", "the report was written first")
}

func TestRunToLogsTheEventsBesideTheTextReport(t *testing.T) {
	var out, logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))

	err := runTo(context.Background(), &out, log, readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, nil, clicfg.OutputText, in, false)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Targets", "the report is still written")
	assert.Contains(t, logs.String(), `"msg":"ship resolved"`)
	assert.Contains(t, logs.String(), `"msg":"target checked"`)
	assert.Contains(t, logs.String(), `"msg":"ship done"`)
}

func TestRunToLogsTheEventsAndKeepsJSONOnStdoutAsOneDocument(t *testing.T) {
	var out, logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))

	err := runTo(context.Background(), &out, log, readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, nil, clicfg.OutputJSON, in, false)

	require.NoError(t, err)
	var plan domain.Plan
	require.NoError(t, json.Unmarshal(out.Bytes(), &plan), "stdout is the document and nothing else: %s", out.String())
	assert.Contains(t, logs.String(), `"msg":"target checked"`, "the progress is in the log, which is stderr")
	assert.NotContains(t, out.String(), "target checked")
}

func TestRunToWithNoLoggerWritesNoLogs(t *testing.T) {
	var out bytes.Buffer

	err := runTo(context.Background(), &out, nil, readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, nil, clicfg.OutputText, in, false)

	require.NoError(t, err)
}
