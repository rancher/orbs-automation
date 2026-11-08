package ship

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withFork gives the clone a second remote, "fork", that is a bare repository,
// and the settings of an engineer who has a fork.
func withFork(t *testing.T, ws *realWorkspace, clone string) (bare string) {
	t.Helper()
	bare = filepath.Join(t.TempDir(), "fork.git")
	runGit(t, t.TempDir(), "init", "--quiet", "--bare", bare)
	runGit(t, clone, "remote", "add", "fork", bare)
	ws.push = PushSettings{Fork: "fork", Upstream: "upstream", GitHubUser: "danpock"}
	return bare
}

func TestShipPushesToTheForkWhenOneIsSetAndTheHeadNamesItsOwner(t *testing.T) {
	ws, clone, ex, _, lg := publishing(t)
	bare := withFork(t, ws, clone)

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultOpened, res.Status)
	assert.Equal(t, "fork", res.Pushed)
	assert.Equal(t, res.Tip(), runGit(t, bare, "rev-parse", botV215), "the branch is in the fork")
	assert.NotContains(t, runGit(t, ws.up, "branch", "--list", botV215), "bot/", "and not in the upstream")
	assert.True(t, lg.has("create rancher/rancher head=danpock:"+botV215), "the head is the fork's owner and the branch")
	require.NotNil(t, plan.Push)
	assert.Equal(t, PushTarget{Role: RoleFork, Remote: "fork", Source: "a fork is set"}, *plan.Push)
}

func TestShipThePushToFlagBeatsTheForkAndPushesToTheUpstream(t *testing.T) {
	ws, clone, ex, _, lg := publishing(t)
	bare := withFork(t, ws, clone)
	in := realRun()
	in.PushTo = "upstream"

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.Equal(t, "upstream", onlyResult(t, plan).Pushed)
	assert.Equal(t, onlyResult(t, plan).Tip(), runGit(t, ws.up, "rev-parse", botV215))
	assert.Empty(t, runGit(t, bare, "branch", "--list", botV215))
	assert.True(t, lg.has("create rancher/rancher head="+botV215+" "), "the bare branch for the upstream")
	require.NotNil(t, plan.Push)
	assert.Equal(t, "--push-to", plan.Push.Source)
}

func TestShipTheProjectsPushToIsUsedWhenTheUserSaysNothing(t *testing.T) {
	ws, clone, ex, _, _ := publishing(t)
	withFork(t, ws, clone)
	ws.links[0].Consumer.PushTo = "upstream"

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, "upstream", onlyResult(t, plan).Pushed)
	require.NotNil(t, plan.Push)
	assert.Equal(t, "the project's pullRequests.pushTo", plan.Push.Source)
}

func TestShipTheUsersPerRepoSettingBeatsTheProjects(t *testing.T) {
	ws, clone, ex, _, _ := publishing(t)
	withFork(t, ws, clone)
	ws.push.UserRepo = "fork"
	ws.links[0].Consumer.PushTo = "upstream"

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, "fork", onlyResult(t, plan).Pushed)
}

func TestShipInCIPushesToTheUpstreamWhateverTheSettingsSayAndLabelsAsTheBot(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	withFork(t, ws, clone)
	ws.push.UserDefault = "fork"
	ws.links[0].Consumer.PushTo = "fork"
	ws.links[0].Consumer.Labels = []string{"status/auto-created"}
	ws.links[0].Consumer.BotLabels = []string{"origin/bot"}
	remote.labels = []string{"status/auto-created", "origin/bot"}
	in := realRun()
	in.CI = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.Equal(t, "upstream", onlyResult(t, plan).Pushed)
	require.NotNil(t, plan.Push)
	assert.Equal(t, "CI always pushes to the upstream", plan.Push.Source)
	assert.True(t, lg.has("add labels #104 status/auto-created,origin/bot"), "the bot's labels are added in CI")
}

func TestShipAnEngineersPRDoesNotGetTheBotLabels(t *testing.T) {
	ws, _, ex, remote, lg := publishing(t)
	ws.links[0].Consumer.Labels = []string{"status/auto-created"}
	ws.links[0].Consumer.BotLabels = []string{"origin/bot"}
	remote.labels = []string{"status/auto-created", "origin/bot"}

	_, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.True(t, lg.has("add labels #104 status/auto-created"))
	assert.False(t, lg.has("add labels #104 status/auto-created,origin/bot"))
}

func TestShipAForkPushWithNoGitHubUserStopsBeforeAnyWork(t *testing.T) {
	ws, clone, ex, _, lg := publishing(t)
	withFork(t, ws, clone)
	ws.push.GitHubUser = ""

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "githubUser")
	assert.Equal(t, StatusReady, plan.Status, "the plan is still returned")
	assert.False(t, branchExistsIn(t, clone, botV215))
	assert.False(t, lg.has("push"))
}

func TestShipARoleWithNoRemoteStopsBeforeAnyWorkAndSaysWhatToSet(t *testing.T) {
	ws, clone, ex, _, _ := publishing(t)
	in := realRun()
	in.PushTo = "fork" // and no fork is set

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fork")
	assert.Contains(t, err.Error(), "--push-to upstream")
	assert.False(t, branchExistsIn(t, clone, botV215))
}

func TestShipAPushToThatIsNeitherRoleIsAnError(t *testing.T) {
	ws, _, ex, _, _ := publishing(t)
	in := realRun()
	in.PushTo = "origin"

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--push-to")
}

func TestShipADryRunSaysWhereItWouldPushAndANoPushRunDoesNot(t *testing.T) {
	ws, clone, ex, _, _ := publishing(t)
	withFork(t, ws, clone)

	dry := realRun()
	dry.DryRun = true
	plan, err := Ship(bg, ws, &fakeProber{}, dry, ex, nil)
	require.NoError(t, err)
	require.NotNil(t, plan.Push)
	assert.Equal(t, "fork", plan.Push.Remote)

	noPush := realRun()
	noPush.NoPush = true
	plan, err = Ship(bg, ws, &fakeProber{}, noPush, ex, nil)
	require.NoError(t, err)
	assert.Nil(t, plan.Push, "nothing is pushed, so there is no target to report")
}
