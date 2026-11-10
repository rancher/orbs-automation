package ship

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/ociregistry"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/yamledit"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.Background()

func TestCheckReady(t *testing.T) {
	ws, p := fixture(t), &fakeProber{}

	got, err := Check(ctx, ws, p, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", got.Producer)
	assert.Equal(t, []Release{{Tag: "v0.5.3", Commit: "sha-v0.5.3", Stable: true}}, got.Releases)
	assert.Equal(t, "rancher/rancher", got.Consumer)
	assert.Equal(t, []string{"rancher-h0"}, got.Holdings)
	assert.Equal(t, Target{Branch: "release/v2.15", Commit: "sha-215"}, got.Targets[0].Target)
	require.Len(t, got.Targets[0].Holdings, 1)
	assert.Equal(t, HoldingPlan{Name: "rancher-h0", File: "build.yaml", Current: "rancher/scc-operator:v0.5.2", Want: "rancher/scc-operator:v0.5.3"}, got.Targets[0].Holdings[0])
	assert.Equal(t, StatusReady, got.Status)
	assert.Empty(t, got.Blockers)
	require.Len(t, got.Images, 1)
	assert.Equal(t, "v0.5.3", got.Images[0].Release)
	assert.False(t, got.Images[0].Failed())
	assert.Equal(t, "scc-operator", ws.asked[0][len("project "):])
}

func TestCheckReadsTheFileAtTheCommitItListed(t *testing.T) {
	ws := fixture(t)
	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Contains(t, ws.asked, "file rancher/rancher@sha-215:build.yaml", "the head commit, so the read is exact")
}

func TestCheckAlreadyUpToDate(t *testing.T) {
	p := &fakeProber{}
	got, err := Check(ctx, withDevLine(fixture(t)), p, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})
	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, got.Status)
	assert.Equal(t, got.Targets[0].Holdings[0].Current, got.Targets[0].Holdings[0].Want)
	assert.Empty(t, got.Images, "nothing else runs")
	assert.Zero(t, p.calls)
}

func TestCheckBlockedByAMissingImage(t *testing.T) {
	p := &fakeProber{results: map[string]ociregistry.Result{
		"registry.rancher.com rancher/scc-operator:v0.5.3": {Status: ociregistry.NotFound},
	}}
	got, err := Check(ctx, fixture(t), p, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Equal(t, StatusBlocked, got.Status)
	require.Len(t, got.Blockers, 1)
	assert.Contains(t, got.Blockers[0], "images")
	require.Len(t, got.Images, 1)
	assert.True(t, got.Images[0].Failed())
}

func TestCheckAnExplicitTagAndALine(t *testing.T) {
	got, err := Check(ctx, fixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Arg: "v0.5.2", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	require.Len(t, got.Releases, 1)
	assert.Equal(t, "v0.5.2", got.Releases[0].Tag)
	assert.Equal(t, StatusUpToDate, got.Status, "the consumer already holds v0.5.2")
}

func TestCheckARefusedPrerelease(t *testing.T) {
	_, err := Check(ctx, withDevLine(fixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Arg: "v0.6.0-rc.1", Lines: []string{"v2.16"}})
	assert.ErrorIs(t, err, ErrPrerelease)

	got, err := Check(ctx, withDevLine(fixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Arg: "v0.6.0-rc.1", Lines: []string{"v2.16"}, Prerelease: true})
	require.NoError(t, err)
	require.Len(t, got.Releases, 1)
	assert.Equal(t, "v0.6.0-rc.1", got.Releases[0].Tag)
	assert.False(t, got.Releases[0].Stable)
}

func TestCheckABranchTheConsumerDoesNotHave(t *testing.T) {
	_, err := Check(ctx, fixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.14"}})
	assert.ErrorIs(t, err, ErrBranchNotFound)
	assert.ErrorContains(t, err, "release/v2.14")
}

func TestCheckConsumers(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		ws := withDevLine(fixture(t))
		ws.links = nil
		_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})
		assert.ErrorIs(t, err, ErrNoConsumer)
	})
	t.Run("several and none chosen", func(t *testing.T) {
		ws := withDevLine(fixture(t))
		other := riLink()
		other.Consumer.Name, other.Consumer.Repo = "charts", "rancher/charts"
		ws.links = append(ws.links, other)
		_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})
		assert.ErrorIs(t, err, ErrAmbiguousConsumer)
		assert.ErrorContains(t, err, "charts")
		assert.ErrorContains(t, err, "rancher")
	})
	t.Run("chosen by name or slug", func(t *testing.T) {
		for _, to := range []string{"charts", "rancher/charts", "Rancher/Charts"} {
			ws := withDevLine(fixture(t))
			other := riLink()
			other.Consumer.Name, other.Consumer.Repo = "charts", "rancher/charts"
			other.Consumer.Lines = ws.links[0].Consumer.Lines
			ws.links = append(ws.links, other)
			ws.refs["rancher/charts"] = ws.refs["rancher/rancher"]
			ws.files["rancher/charts@sha-main:build.yaml"] = "defaultSccOperatorImage: x\n"
			got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: to, Lines: []string{"v2.16"}})
			require.NoError(t, err, to)
			assert.Equal(t, "rancher/charts", got.Consumer)
		}
	})
	t.Run("a --to that is not a consumer", func(t *testing.T) {
		_, err := Check(ctx, withDevLine(fixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: "nope", Lines: []string{"v2.16"}})
		assert.ErrorIs(t, err, ErrNoConsumer)
	})
}

func TestCheckAStrategyThatIsNotBuiltStopsBeforeAnythingIsRead(t *testing.T) {
	ws := withDevLine(fixture(t))
	ws.links[0].Strategy = "chartsPackageBump"
	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, bump.ErrNotBuilt)
	assert.Contains(t, err.Error(), "chartsPackageBump", "it says which strategy is not built")
	assert.NotContains(t, ws.asked, "file rancher/rancher@sha-main:build.yaml", "nothing of the consumer is read")
}

func TestCheckReadFailures(t *testing.T) {
	t.Run("the file is missing", func(t *testing.T) {
		ws := fixture(t)
		delete(ws.files, "rancher/rancher@sha-215:build.yaml")
		_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
		require.Error(t, err)
		assert.ErrorContains(t, err, "build.yaml")
		assert.ErrorContains(t, err, "release/v2.15")
	})
	t.Run("the key is missing", func(t *testing.T) {
		ws := fixture(t)
		ws.files["rancher/rancher@sha-215:build.yaml"] = "other: 1\n"
		_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
		assert.ErrorIs(t, err, yamledit.ErrKeyNotFound)
		assert.ErrorContains(t, err, "defaultSccOperatorImage")
	})
}

func TestCheckHostErrors(t *testing.T) {
	boom := errors.New("network down")
	for slug := range map[string]bool{"rancher/scc-operator": true, "rancher/rancher": true} {
		ws := withDevLine(fixture(t))
		ws.refsErr = map[string]error{slug: boom}
		_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})
		assert.ErrorIs(t, err, boom, slug)
	}
}

func TestCheckProducerErrors(t *testing.T) {
	ws := withDevLine(fixture(t))
	ws.projectErr = workspace.ErrUnknownProject
	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "nope", Lines: []string{"v2.16"}})
	assert.ErrorIs(t, err, workspace.ErrUnknownProject)
}

func TestCheckValueVariables(t *testing.T) {
	ws := fixture(t)
	c := riLink()
	c.Form = "{sourceRepo}@{version}"
	ws.links = []link{c}
	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator@0.5.3", got.Targets[0].Holdings[0].Want)
}

func TestCheckPreviewsWhatARunWouldWrite(t *testing.T) {
	got, err := Check(ctx, fixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	require.NotNil(t, got.Targets[0].Preview)
	assert.Equal(t, "bot/scc-operator-v0.5.3-v2.15", got.Targets[0].Preview.Branch)
	assert.Equal(t, "[v2.15] Update scc-operator to v0.5.3", got.Targets[0].Preview.Title)
	assert.Contains(t, got.Targets[0].Preview.Body, "rancher/scc-operator:v0.5.2")
	assert.Contains(t, got.Targets[0].Preview.CommitMessage, "Automation: gero ship")
}

func TestCheckPreviewUsesTheUsersBranchPrefix(t *testing.T) {
	got, err := Check(ctx, fixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}, BranchPrefix: "dan/"})
	require.NoError(t, err)
	assert.Equal(t, "dan/v0.5.3-v2.15", got.Targets[0].Preview.Branch)
}

func TestCheckHasNoPreviewWhenAlreadyUpToDate(t *testing.T) {
	got, err := Check(ctx, withDevLine(fixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})
	require.NoError(t, err)
	assert.Nil(t, got.Targets[0].Preview)
}

func TestCheckPreviewIsThereWhenBlocked(t *testing.T) {
	p := &fakeProber{results: map[string]ociregistry.Result{"registry.rancher.com rancher/scc-operator:v0.5.3": {Status: ociregistry.NotFound}}}
	got, err := Check(ctx, fixture(t), p, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Equal(t, StatusBlocked, got.Status)
	assert.NotNil(t, got.Targets[0].Preview, "so the reader sees what would have been written")
}

func TestCheckABlockedDowngrade(t *testing.T) {
	// main holds v0.5.3; shipping the older v0.5.2 is a downgrade.
	got, err := Check(ctx, withDevLine(fixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Arg: "v0.5.2", Lines: []string{"v2.16"}})

	require.NoError(t, err)
	assert.True(t, got.Targets[0].Downgrade)
	assert.Equal(t, StatusBlocked, got.Status)
	require.NotEmpty(t, got.Targets[0].Blockers)
	assert.Contains(t, got.Targets[0].Blockers[0], "downgrade")
	assert.NotContains(t, got.Targets[0].Blockers[0], "0.5.3", "the values are in the table, so the text is the same for every branch and can be grouped")
	assert.Contains(t, got.Targets[0].Blockers[0], "--allow-downgrade")
	assert.Empty(t, got.Images, "a blocked downgrade stops here, as an up to date consumer does")
	assert.Nil(t, got.Targets[0].Preview)
}

func TestCheckAllowedDowngradeIsAWarning(t *testing.T) {
	got, err := Check(ctx, withDevLine(fixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Arg: "v0.5.2", Lines: []string{"v2.16"}, AllowDowngrade: true})
	require.NoError(t, err)
	assert.True(t, got.Targets[0].Downgrade, "it is still a downgrade")
	assert.Equal(t, StatusReady, got.Status)
	assert.Empty(t, got.Targets[0].Blockers)
	require.NotEmpty(t, got.Targets[0].Warnings)
	assert.Contains(t, got.Targets[0].Warnings[0], "downgrade")
	assert.NotEmpty(t, got.Images, "once it is allowed the rest of the check runs")
	assert.NotNil(t, got.Targets[0].Preview)
}

func TestCheckNewerIsNotADowngrade(t *testing.T) {
	got, err := Check(ctx, fixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.False(t, got.Targets[0].Downgrade, "v0.5.2 to v0.5.3")
}

func TestCheckAlreadyUpToDateIsNotADowngrade(t *testing.T) {
	got, err := Check(ctx, withDevLine(fixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})
	require.NoError(t, err)
	assert.False(t, got.Targets[0].Downgrade)
	assert.Equal(t, StatusUpToDate, got.Status)
}

func TestCheckAPrereleaseOfTheSameVersionIsADowngrade(t *testing.T) {
	// main holds v0.5.3; v0.5.3-rc.1 is lower, though it has the same numbers.
	ws := withDevLine(fixture(t))
	ws.refs["rancher/scc-operator"] = git.Refs{Tags: tags("v0.5.3", "v0.5.3-rc.1")}
	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Arg: "v0.5.3-rc.1", Lines: []string{"v2.16"}, Prerelease: true})
	require.NoError(t, err)
	assert.True(t, got.Targets[0].Downgrade)
}

func TestCheckFromAPrereleaseToTheStableReleaseIsNotADowngrade(t *testing.T) {
	ws := fixture(t)
	ws.files["rancher/rancher@sha-215:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3-rc.1\n"
	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.False(t, got.Targets[0].Downgrade)
	assert.Equal(t, StatusReady, got.Status)
}

func TestCheckAValueWithNoVersionIsNotJudged(t *testing.T) {
	ws := fixture(t)
	ws.files["rancher/rancher@sha-215:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:latest\n"
	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.False(t, got.Targets[0].Downgrade, "there is nothing to compare")
	assert.Equal(t, StatusReady, got.Status)
}

// loopFixture is rancher with two release lines, each with a branch and a file,
// taking updates from scc-operator, which is universal.
func loopFixture(t *testing.T) *fakeWorkspace {
	t.Helper()
	ws := fixture(t)
	ws.refs["rancher/rancher"] = git.Refs{Branches: []git.Branch{
		{Name: "main", Commit: "sha-main"}, {Name: "release/v2.15", Commit: "sha-215"}, {Name: "release/v2.14", Commit: "sha-214"},
	}}
	ws.files["rancher/rancher@sha-214:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.1\n"
	return ws
}

func branchesOf(p Plan) []string {
	out := make([]string, len(p.Targets))
	for i, t := range p.Targets {
		out[i] = t.Branch
	}
	return out
}

func TestCheckWithNoBranchChecksEveryBranchAUniversalReleaseGoesTo(t *testing.T) {
	got, err := Check(ctx, loopFixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})

	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.15", "release/v2.14"}, branchesOf(got), "the consumer's release branches, in config order")
	assert.Equal(t, "rancher/scc-operator:v0.5.2", got.Targets[0].Holdings[0].Current)
	assert.Equal(t, "rancher/scc-operator:v0.5.1", got.Targets[1].Holdings[0].Current)
	assert.Equal(t, StatusReady, got.Status)
}

func TestCheckChecksTheImagesOnceForAllTheTargets(t *testing.T) {
	p := &fakeProber{}
	_, err := Check(ctx, loopFixture(t), p, Input{Team: testTeam, Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, 3, p.calls, "one check per registry for the release, not one per branch")
}

func TestCheckNoImagesWhenEveryTargetIsUpToDate(t *testing.T) {
	ws := loopFixture(t)
	ws.files["rancher/rancher@sha-215:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n"
	ws.files["rancher/rancher@sha-214:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n"
	p := &fakeProber{}
	got, err := Check(ctx, ws, p, Input{Team: testTeam, Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, got.Status)
	assert.Empty(t, got.Images)
	assert.Zero(t, p.calls)
}

func TestCheckAMixOfUpToDateAndReady(t *testing.T) {
	ws := loopFixture(t)
	ws.files["rancher/rancher@sha-215:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n"
	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, got.Targets[0].Status)
	assert.Equal(t, StatusReady, got.Targets[1].Status)
	assert.Equal(t, StatusReady, got.Status)
	assert.NotEmpty(t, got.Images, "one of them needs a PR")
	assert.Nil(t, got.Targets[0].Preview)
	assert.NotNil(t, got.Targets[1].Preview)
}

func TestCheckABranchThatCannotBeReadFailsOnlyItself(t *testing.T) {
	ws := loopFixture(t)
	delete(ws.files, "rancher/rancher@sha-214:build.yaml")
	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})

	require.NoError(t, err, "one branch does not stop the others")
	assert.Equal(t, StatusReady, got.Targets[0].Status)
	assert.Equal(t, StatusFailed, got.Targets[1].Status)
	assert.Contains(t, got.Targets[1].Error, "build.yaml")
	assert.Equal(t, "release/v2.14", got.Targets[1].Branch)
	assert.Equal(t, StatusBlocked, got.Status, "a failed branch blocks the whole")
}

func TestCheckABranchTheHostLacksFailsOnlyItself(t *testing.T) {
	ws := loopFixture(t)
	ws.refs["rancher/rancher"] = git.Refs{Branches: []git.Branch{{Name: "release/v2.15", Commit: "sha-215"}}}
	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, StatusReady, got.Targets[0].Status)
	assert.Equal(t, StatusFailed, got.Targets[1].Status)
	assert.Contains(t, got.Targets[1].Error, "release/v2.14")
}

func TestCheckABranchFlagChecksOnlyThatBranch(t *testing.T) {
	got, err := Check(ctx, loopFixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.14"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.14"}, branchesOf(got))
}

func TestCheckABranchFlagOutsideTheReleaseBranchesIsStillAllowed(t *testing.T) {
	got, err := Check(ctx, withDevLine(loopFixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, branchesOf(got))
}

func TestCheckANamedBranchThatCannotBeReadIsAnError(t *testing.T) {
	ws := loopFixture(t)
	delete(ws.files, "rancher/rancher@sha-214:build.yaml")
	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.14"}})
	require.Error(t, err, "asked for that one branch, so failing to read it is the answer")
	assert.ErrorContains(t, err, "build.yaml")
}

// A producer with lines of its own feeds the consumer's branch for the line of
// the release.
func linedLoop(t *testing.T) *fakeWorkspace {
	t.Helper()
	ws := loopFixture(t)
	producer := projectSpec{
		Name: "kuberlr", Repo: "rancher/kuberlr", ManagedBy: testTeam, Images: []string{"rancher/kuberlr"},
		Stream: "  lines:\n    - {line: v2.16, series: \"9.x\"}\n    - {line: v2.15, series: \"8.x\"}\n",
	}
	c := riLink()
	c.Consumer.Lines = []lineSpec{
		{Line: "v2.16"}, // development: the default branch
		{Line: "v2.15", Branch: "release/v2.15"},
		{Line: "v2.14", Branch: "release/v2.14"},
	}
	ws.producer, ws.links = producer, []link{c}
	ws.refs["rancher/kuberlr"] = git.Refs{Tags: tags("v8.1.0", "v9.0.0", "v7.0.0")}
	ws.files["rancher/rancher@sha-main:build.yaml"] = "defaultSccOperatorImage: rancher/kuberlr:v9.0.0\n"
	return ws
}

func TestCheckALinedProducerFeedsTheConsumersBranchForTheLine(t *testing.T) {
	for tag, want := range map[string]string{"v8.1.0": "release/v2.15", "v9.0.0": "main"} {
		ws := linedLoop(t)
		got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Arg: tag})
		require.NoError(t, err, tag)
		assert.Equal(t, []string{want}, branchesOf(got), tag)
	}
}

func TestCheckATagInNoLineOfTheProducerHasNoTarget(t *testing.T) {
	ws := linedLoop(t)
	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Arg: "v7.0.0"})
	assert.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "--line")
}

func TestCheckAConsumerWithoutTheLineHasNoTarget(t *testing.T) {
	ws := linedLoop(t)
	c := ws.links[0]
	c.Consumer.Lines = []lineSpec{{Line: "v2.14", Branch: "release/v2.14"}}
	ws.links[0] = c
	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Arg: "v8.1.0"})
	assert.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "v2.15")
}

func TestCheckASingleBranchConsumerTakesEveryReleaseOnItsDefaultBranch(t *testing.T) {
	ws := loopFixture(t)
	c := ws.links[0]
	c.Consumer.Lines = nil
	ws.links[0] = c
	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, branchesOf(got))
}

func TestCheckCarriesADryRunIntoThePlan(t *testing.T) {
	got, err := Check(ctx, loopFixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", DryRun: true})
	require.NoError(t, err)
	assert.True(t, got.DryRun)

	got, err = Check(ctx, loopFixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})
	require.NoError(t, err)
	assert.False(t, got.DryRun)
}

func TestCheckADryRunChecksTheSameThings(t *testing.T) {
	// A dry run is the same check: it previews and checks the images, so it shows
	// what a real run would do.
	got, err := Check(ctx, loopFixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, StatusReady, got.Status)
	assert.NotEmpty(t, got.Images)
	assert.NotNil(t, got.Targets[0].Preview)
}

func TestCheckNamesEachTargetByItsRancherLine(t *testing.T) {
	ws := fixture(t)

	onALine, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	single := fixture(t)
	single.links[0].Consumer.Lines = nil
	inNoLine, err := Check(ctx, single, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})
	require.NoError(t, err)
	require.Len(t, inNoLine.Targets, 1)

	assert.Equal(t, "v2.15", onALine.Targets[0].Line, "a line means the same in every project, which a branch does not")
	assert.Empty(t, inNoLine.Targets[0].Line, "a branch that is in no line has none, and is named by its branch")
}

func TestCheckALineIsTheBranchTheConsumerKeepsItOn(t *testing.T) {
	for _, line := range []string{"v2.15", "2.15"} {
		got, err := Check(ctx, fixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{line}})
		require.NoError(t, err, line)
		assert.Equal(t, []string{"release/v2.15"}, branchesOf(got), line)
	}
}

func TestCheckALineWithNoBranchOfItsOwnIsTheDefaultBranch(t *testing.T) {
	ws := fixture(t)
	ws.links[0].Consumer.Lines = append(ws.links[0].Consumer.Lines, lineSpec{Line: "v2.16"})

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.16"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, branchesOf(got))
	assert.Equal(t, "v2.16", got.Targets[0].Line)
}

func TestCheckALineTheConsumerDoesNotHaveSaysWhichItDoes(t *testing.T) {
	_, err := Check(ctx, fixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.99"}})

	require.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "v2.99")
	assert.ErrorContains(t, err, "v2.15, v2.14")
}

func TestCheckALineOfAConsumerWithoutLinesIsAnError(t *testing.T) {
	ws := fixture(t)
	ws.links[0].Consumer.Lines = nil

	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}})

	require.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "no release lines")
}

func TestCheckSeveralLinesAreEachATargetOnceInTheOrderGiven(t *testing.T) {
	got, err := Check(ctx, loopFixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.14", "2.15", "v2.14"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.14", "release/v2.15"}, branchesOf(got))
}

// laned is a producer with a series for each of two lines, taking its releases to
// the consumer's branch for each: kuberlr, 9.x for v2.16 and 8.x for v2.15.
func laned(t *testing.T, tagNames ...string) *fakeWorkspace {
	t.Helper()
	ws := linedLoop(t)
	ws.refs["rancher/kuberlr"] = git.Refs{Tags: tags(tagNames...)}
	ws.files["rancher/rancher@sha-main:build.yaml"] = "defaultSccOperatorImage: rancher/kuberlr:v8.9.0\n"
	return ws
}

func releasesOf(p Plan) []string {
	out := make([]string, len(p.Targets))
	for i, t := range p.Targets {
		out[i] = t.Release
	}
	return out
}

func TestCheckALinedProducerWithNoReleaseGivesEachLineTheLatestOfItsOwnSeries(t *testing.T) {
	ws := laned(t, "v8.0.9", "v8.1.0", "v9.0.0", "v9.0.1-rc.1", "v7.0.0")

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr"})

	require.NoError(t, err)
	assert.Equal(t, []string{"main", "release/v2.15"}, branchesOf(got), "the lines both have, in the producer's order")
	assert.Equal(t, []string{"v9.0.0", "v8.1.0"}, releasesOf(got), "the latest stable of each series")
	require.Len(t, got.Targets, 2)
	assert.Equal(t, "rancher/kuberlr:v9.0.0", got.Targets[0].Holdings[0].Want)
	assert.Equal(t, "rancher/kuberlr:v8.1.0", got.Targets[1].Holdings[0].Want)
	assert.False(t, got.Uniform())
	assert.Len(t, got.Images, 2, "each release's images are checked")
}

func TestCheckALinedProducerWithPrereleaseTakesEachSeriesLatestOfAnyKind(t *testing.T) {
	ws := laned(t, "v8.1.0", "v9.0.0", "v9.0.1-rc.1")

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Prerelease: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"v9.0.1-rc.1", "v8.1.0"}, releasesOf(got), "a series with no prerelease gets its stable one")
}

func TestCheckALinedProducerWithNamedLinesGivesEachItsOwnRelease(t *testing.T) {
	ws := laned(t, "v8.0.9", "v8.1.0", "v9.0.0")

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Lines: []string{"v2.15"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.15"}, branchesOf(got))
	assert.Equal(t, []string{"v8.1.0"}, releasesOf(got))
	assert.True(t, got.Uniform())
}

func TestCheckALinedProducerSkipsALineWithNoReleaseButNamingItIsAnError(t *testing.T) {
	ws := laned(t, "v8.1.0")

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr"})
	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.15"}, branchesOf(got), "nothing to ship to v2.16")
	require.Len(t, got.Notes, 1)
	assert.Contains(t, got.Notes[0], "v2.16", "and it says so")

	_, err = Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Lines: []string{"v2.16"}})
	require.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "v2.16")
}

func TestCheckATagIsOnlyShippedToTheLineItIsIn(t *testing.T) {
	ws := laned(t, "v8.1.0", "v9.0.0")

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Arg: "v8.1.0", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.15"}, branchesOf(got))

	_, err = Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Arg: "v8.1.0", Lines: []string{"v2.16"}})
	require.ErrorIs(t, err, ErrLineMismatch)
	assert.ErrorContains(t, err, "v8.1.0 is in v2.15, not v2.16")
}

func TestCheckALineOfTheProducerIsOnlyShippedToTheSameLine(t *testing.T) {
	ws := laned(t, "v8.1.0", "v9.0.0")

	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr", Arg: "v2.15", Lines: []string{"v2.16"}})

	require.ErrorIs(t, err, ErrLineMismatch)
}

func TestCheckAUniversalProducerShipsOneReleaseToWhateverLinesAreNamed(t *testing.T) {
	got, err := Check(ctx, loopFixture(t), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", Arg: "v0.5.3", Lines: []string{"v2.14", "v2.15"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"v0.5.3", "v0.5.3"}, releasesOf(got))
	assert.True(t, got.Uniform())
}

func TestCheckEachTargetOfALinedProducerPreviewsItsOwnRelease(t *testing.T) {
	ws := laned(t, "v8.1.0", "v9.0.0")

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "kuberlr"})

	require.NoError(t, err)
	require.Len(t, got.Targets, 2)
	for i, want := range []string{"v9.0.0", "v8.1.0"} {
		pv := got.Targets[i].Preview
		require.NotNil(t, pv, want)
		assert.Contains(t, pv.Title, strings.TrimPrefix(want, "v"), "the title is for its own release")
		assert.Contains(t, pv.Body, "version: "+want, "and so is the marker")
	}
}
