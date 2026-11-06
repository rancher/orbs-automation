package ship

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var both = PushRemotes{Fork: "origin", Upstream: "upstream"}

func TestPushTargetWithNoChoiceIsTheForkWhenOneIsSet(t *testing.T) {
	got, err := resolvePushTarget(PushChoice{}, both, false)

	require.NoError(t, err)
	assert.Equal(t, PushTarget{Role: RoleFork, Remote: "origin", Source: "a fork is set"}, got)
}

func TestPushTargetWithNoChoiceAndNoForkIsTheUpstream(t *testing.T) {
	got, err := resolvePushTarget(PushChoice{}, PushRemotes{Upstream: "upstream"}, false)

	require.NoError(t, err)
	assert.Equal(t, PushTarget{Role: RoleUpstream, Remote: "upstream", Source: "no fork is set"}, got)
}

func TestPushTargetTheNearestLayerWins(t *testing.T) {
	cases := []struct {
		name   string
		choice PushChoice
		want   PushRole
		source string
	}{
		{"the flag beats everything", PushChoice{Flag: "upstream", UserRepo: "fork", Project: "fork", UserDefault: "fork"}, RoleUpstream, "--push-to"},
		{"the user's per-repo setting beats the project and the default", PushChoice{UserRepo: "upstream", Project: "fork", UserDefault: "fork"}, RoleUpstream, "repos.overrides.<repo>.pushTo"},
		{"the project beats the user's default", PushChoice{Project: "upstream", UserDefault: "fork"}, RoleUpstream, "the project's pullRequests.pushTo"},
		{"the user's default beats the fallback", PushChoice{UserDefault: "upstream"}, RoleUpstream, "pushTo in the user file"},
		{"a layer that says fork", PushChoice{UserDefault: "fork"}, RoleFork, "pushTo in the user file"},
	}
	for _, c := range cases {
		got, err := resolvePushTarget(c.choice, both, false)

		require.NoError(t, err, c.name)
		assert.Equal(t, c.want, got.Role, c.name)
		assert.Equal(t, c.source, got.Source, c.name)
		want := map[PushRole]string{RoleFork: "origin", RoleUpstream: "upstream"}[c.want]
		assert.Equal(t, want, got.Remote, c.name)
	}
}

func TestPushTargetInCIIsAlwaysTheUpstream(t *testing.T) {
	got, err := resolvePushTarget(PushChoice{Flag: "fork", UserRepo: "fork", Project: "fork", UserDefault: "fork"}, both, true)

	require.NoError(t, err)
	assert.Equal(t, PushTarget{Role: RoleUpstream, Remote: "upstream", Source: "CI always pushes to the upstream"}, got)
}

func TestPushTargetARoleWithNoRemoteSaysWhatToSet(t *testing.T) {
	_, err := resolvePushTarget(PushChoice{Flag: "fork"}, PushRemotes{Upstream: "upstream"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fork")
	assert.Contains(t, err.Error(), "--push-to upstream", "and how to get past it")

	_, err = resolvePushTarget(PushChoice{UserDefault: "upstream"}, PushRemotes{Fork: "origin"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream")

	_, err = resolvePushTarget(PushChoice{}, PushRemotes{}, false)
	require.Error(t, err, "no remote at all")

	_, err = resolvePushTarget(PushChoice{}, PushRemotes{Fork: "origin"}, true)
	require.Error(t, err, "CI with no upstream remote")
}

func TestPushTargetAValueThatIsNeitherRoleIsAnErrorNamingTheLayer(t *testing.T) {
	_, err := resolvePushTarget(PushChoice{Project: "forks"}, both, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "forks")
	assert.Contains(t, err.Error(), "the project's pullRequests.pushTo")
	assert.Contains(t, err.Error(), "fork or upstream")
}

func TestPushTargetAnInvalidLayerThatIsOverriddenIsNotChecked(t *testing.T) {
	got, err := resolvePushTarget(PushChoice{Flag: "upstream", Project: "forks"}, both, false)

	require.NoError(t, err, "only the layer that decides is read")
	assert.Equal(t, RoleUpstream, got.Role)
}
