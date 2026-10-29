package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/actor"
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuth struct{ token string }

func (a fakeAuth) TokenForHost(string) (string, string) { return a.token, "GH_TOKEN" }
func (fakeAuth) DefaultHost() (string, string)          { return "github.com", "default" }
func (fakeAuth) IsEnterprise(string) bool               { return false }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// userAPI answers GET /users/<login> with id, and counts the requests.
func userAPI(id int64, calls *int) gogithub.ClientOptionsFunc {
	return gogithub.WithTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		*calls++
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		_, _ = rec.WriteString(`{"id":` + strconv.FormatInt(id, 10) + `}`)
		return rec.Result(), nil
	}))
}

func itoa(n int64) string {
	return string(rune('0'+n/100%10)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}

func gitIdentity(context.Context, string) (string, error) { return "Dan Pock", nil }

func gitConfig(m map[string]string) actor.ConfigFunc {
	return func(_ context.Context, key string) (string, error) { return m[key], nil }
}

func TestCIIsTheBot(t *testing.T) {
	calls := 0

	got, err := resolve(t.Context(), clicfg.Settings{CI: true, BotUser: "gero-bot[bot]"},
		fakeAuth{token: "tok"}, gitIdentity, userAPI(123, &calls))

	require.NoError(t, err)
	assert.Equal(t, actor.Identity{Name: "gero-bot[bot]", Email: "123+gero-bot[bot]@users.noreply.github.com"}, got)
	assert.Equal(t, 1, calls)
}

func TestWorkstationIsTheEngineerAndMakesNoAPICall(t *testing.T) {
	calls := 0
	git := gitConfig(map[string]string{"user.name": "Dan Pock", "user.email": "dan@example.com"})

	got, err := resolve(t.Context(), clicfg.Settings{BotUser: "gero-bot[bot]"},
		fakeAuth{}, git, userAPI(123, &calls))

	require.NoError(t, err)
	assert.Equal(t, actor.Identity{Name: "Dan Pock", Email: "dan@example.com"}, got)
	assert.Zero(t, calls)
}

func TestCIWithoutABotLoginFailsBeforeAnyCall(t *testing.T) {
	calls := 0

	_, err := resolve(t.Context(), clicfg.Settings{CI: true},
		fakeAuth{token: "tok"}, gitIdentity, userAPI(123, &calls))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GERO_BOT_USER")
	assert.Zero(t, calls)
}

func TestCIWithoutATokenSaysSo(t *testing.T) {
	calls := 0

	_, err := resolve(t.Context(), clicfg.Settings{CI: true, BotUser: "gero-bot[bot]"},
		fakeAuth{}, gitIdentity, userAPI(123, &calls))

	require.ErrorIs(t, err, github.ErrNoToken)
	assert.Zero(t, calls)
}
