package ship

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// whoAmI answers GET /user with a login, and counts the requests.
func whoAmI(login string, calls *int) gogithub.ClientOptionsFunc {
	return gogithub.WithTransport(roundTrip(func(*http.Request) (*http.Response, error) {
		*calls++
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		_, _ = rec.WriteString(`{"login":"` + login + `"}`)
		return rec.Result(), nil
	}))
}

var creds = github.Credentials{Host: "github.com", Token: "tok"}

func TestMeOnAWorkstationIsTheLoginTheTokenBelongsTo(t *testing.T) {
	var calls int
	r, err := newRemote(creds, clicfg.Settings{}, whoAmI("danpock", &calls))
	require.NoError(t, err)

	me, err := r.Me(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "danpock", me)
	assert.Equal(t, 1, calls)
}

func TestMeInCIIsTheBotNamedByTheWorkflowWithoutAskingGitHub(t *testing.T) {
	var calls int
	r, err := newRemote(creds, clicfg.Settings{CI: true, BotUser: "rancher-scc-bot[bot]"}, whoAmI("someone", &calls))
	require.NoError(t, err)

	me, err := r.Me(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "rancher-scc-bot[bot]", me, "an installation token has no user to ask about")
	assert.Zero(t, calls)
}

func TestMeInCIWithNoBotUserSaysWhatToSet(t *testing.T) {
	var calls int
	r, err := newRemote(creds, clicfg.Settings{CI: true}, whoAmI("someone", &calls))
	require.NoError(t, err)

	_, err = r.Me(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GERO_BOT_USER")
}

func TestNewRemoteWithNoTokenIsAnError(t *testing.T) {
	_, err := newRemote(github.Credentials{Host: "github.com"}, clicfg.Settings{})

	require.ErrorIs(t, err, github.ErrNoToken)
}
