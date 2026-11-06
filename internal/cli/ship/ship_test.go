package ship

import (
	"context"
	"encoding/json"
	"errors"
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
	err   error
	files map[string]string
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

func (f *fakeWorkspace) CloneDir(string) (string, error) { return "/clone", nil }

func (f *fakeWorkspace) Remote(string) (string, error) { return "upstream", nil }

type fakeProber struct{ result registry.Result }

func (f fakeProber) Exists(context.Context, string, string, string) registry.Result { return f.result }

var in = domain.Input{Project: "scc-operator", Branch: "main"}

func readyWorkspace() *fakeWorkspace {
	return &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.5.2\n"}}
}

func TestRunReady(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, clicfg.OutputText, in, false)

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
	out, err := run(context.Background(), ws, fakeProber{}, clicfg.OutputText, in, false)
	require.NoError(t, err)
	assert.Contains(t, out, "already up to date")
	assert.NotContains(t, out, "Images", "the images were not checked")
}

func TestRunBlockedKeepsTheReport(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.NotFound}}, clicfg.OutputText, in, false)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.Contains(t, out, "blocked")
	assert.Contains(t, out, "not found", "the missing images are listed")
}

func TestRunErrorsPrintNothing(t *testing.T) {
	boom := errors.New("boom")
	out, err := run(context.Background(), &fakeWorkspace{err: boom}, fakeProber{}, clicfg.OutputText, in, false)
	assert.ErrorIs(t, err, boom)
	assert.Empty(t, out)
}

func TestRunJSON(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, clicfg.OutputJSON, in, false)
	require.NoError(t, err)
	var got domain.Plan
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, domain.StatusReady, got.Status)
	require.Len(t, got.Targets, 1)
	assert.Equal(t, "2222222bbbbbbbb", got.Targets[0].Commit, "JSON carries the full commit")
}

func TestRunJSONWhenBlocked(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.NotFound}}, clicfg.OutputJSON, in, false)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.True(t, json.Valid([]byte(out)))
}

func TestRunRejectsOtherOutputFormats(t *testing.T) {
	_, err := run(context.Background(), readyWorkspace(), fakeProber{}, clicfg.OutputEnv, in, false)
	assert.ErrorContains(t, err, "not supported by ship")
	_, err = Run(context.Background(), clicfg.Settings{Output: clicfg.OutputEnv}, t.TempDir(), Options{Project: "scc-operator"})
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
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
		clicfg.OutputText, in, false)
	require.NoError(t, err)
	assert.Contains(t, out, "bot/scc-operator-v0.5.3-main")
}

func TestRunJSONHasTheFullPreview(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
		clicfg.OutputJSON, in, false)
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
	_, err := run(context.Background(), &fakeWorkspace{err: workspace.ErrNotProject}, fakeProber{}, clicfg.OutputText, domain.Input{Branch: "main"}, false)
	assert.ErrorIs(t, err, workspace.ErrNotProject)
	assert.ErrorContains(t, err, "name the project")
}

func TestRunBlocksADowngradeAndAllowsItOnRequest(t *testing.T) {
	ws := &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.9.0\n"}}
	prober := fakeProber{result: registry.Result{Status: registry.Found}}

	out, err := run(context.Background(), ws, prober, clicfg.OutputText, in, false)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.Contains(t, out, "downgrade: the release is older")
	assert.Contains(t, out, "--allow-downgrade")
	assert.NotContains(t, out, "Images", "a blocked downgrade shows no preview or image check")
	assert.NotContains(t, out, "\nPR\n")

	allowed := in
	allowed.AllowDowngrade = true
	out, err = run(context.Background(), ws, prober, clicfg.OutputText, allowed, false)
	require.NoError(t, err)
	assert.Contains(t, out, "this is a downgrade")
	assert.Contains(t, out, "Images", "allowed, the check goes on")
}

func TestRunWithNoBranchChecksTheBranchesTheReleaseGoesTo(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
		clicfg.OutputText, domain.Input{Project: "scc-operator"}, false)
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
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
		clicfg.OutputJSON, dry, false)
	require.NoError(t, err)
	var got domain.Plan
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.True(t, got.DryRun)
}

func TestRenderTextSaysWhatNoPushWillDo(t *testing.T) {
	p := previewPlan()
	p.NoPush = true

	out := renderText(p, false)

	assert.Contains(t, out, "No push: the commit will be left on a branch in your clone for review")
	assert.NotContains(t, out, "Dry run")
}

func TestRunPassesNoPushToThePlanExceptInADryRun(t *testing.T) {
	for _, tc := range []struct{ dry, want bool }{{false, true}, {true, false}} {
		opts := in
		opts.NoPush, opts.DryRun = true, tc.dry
		out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}},
			clicfg.OutputJSON, opts, false)
		require.NoError(t, err)
		var got domain.Plan
		require.NoError(t, json.Unmarshal([]byte(out), &got))
		assert.Equal(t, tc.want, got.NoPush, "dry run %v: with both, it is a dry run", tc.dry)
		assert.Equal(t, tc.dry, got.DryRun)
	}
}
