package identity

import (
	"context"
	"os/exec"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/actor"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuth struct{ token string }

func (a fakeAuth) TokenForHost(string) (string, string) { return a.token, "GH_TOKEN" }
func (fakeAuth) DefaultHost() (string, string)          { return "github.com", "default" }
func (fakeAuth) IsEnterprise(string) bool               { return false }

// lookupID answers every user lookup with id 123, and counts the calls.
func lookupID(calls *int) userLookup {
	return func(context.Context, github.Credentials, string) (int64, error) {
		*calls++
		return 123, nil
	}
}

func gitIdentity(context.Context, string) (string, error) { return "Dan Pock", nil }

func gitConfig(m map[string]string) actor.ConfigFunc {
	return func(_ context.Context, key string) (string, error) { return m[key], nil }
}

func TestCIIsTheBot(t *testing.T) {
	calls := 0

	got, err := resolve(t.Context(), clicfg.Settings{CI: true, BotUser: "gero-bot[bot]"},
		fakeAuth{token: "tok"}, gitIdentity, lookupID(&calls))

	require.NoError(t, err)
	assert.Equal(t, actor.Identity{Name: "gero-bot[bot]", Email: "123+gero-bot[bot]@users.noreply.github.com"}, got)
	assert.Equal(t, 1, calls)
}

func TestWorkstationIsTheEngineerAndMakesNoAPICall(t *testing.T) {
	calls := 0
	git := gitConfig(map[string]string{"user.name": "Dan Pock", "user.email": "dan@example.com"})

	got, err := resolve(t.Context(), clicfg.Settings{BotUser: "gero-bot[bot]"},
		fakeAuth{}, git, lookupID(&calls))

	require.NoError(t, err)
	assert.Equal(t, actor.Identity{Name: "Dan Pock", Email: "dan@example.com"}, got)
	assert.Zero(t, calls)
}

func TestCIWithoutABotLoginFailsBeforeAnyCall(t *testing.T) {
	calls := 0

	_, err := resolve(t.Context(), clicfg.Settings{CI: true},
		fakeAuth{token: "tok"}, gitIdentity, lookupID(&calls))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GERO_BOT_USER")
	assert.Zero(t, calls)
}

func TestCIWithoutATokenSaysSo(t *testing.T) {
	calls := 0

	_, err := resolve(t.Context(), clicfg.Settings{CI: true, BotUser: "gero-bot[bot]"},
		fakeAuth{}, gitIdentity, lookupID(&calls))

	require.ErrorIs(t, err, github.ErrNoToken)
	assert.Zero(t, calls)
}

func TestResolveInReadsTheCloneOwnConfigOnAWorkstation(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "--quiet"}, {"config", "user.name", "Clone Only"}, {"config", "user.email", "clone@example.com"},
	} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}

	got, err := ResolveIn(t.Context(), clicfg.Settings{}, dir)

	require.NoError(t, err)
	assert.Equal(t, actor.Identity{Name: "Clone Only", Email: "clone@example.com"}, got)
}
