package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// respondWith answers every request with status and body, and records the last
// request.
func respondWith(last **http.Request, status int, body string) gogithub.ClientOptionsFunc {
	return gogithub.WithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*last = r
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		rec.WriteHeader(status)
		_, _ = rec.WriteString(body)
		return rec.Result(), nil
	}))
}

func TestLogin(t *testing.T) {
	var last *http.Request

	login, err := Login(context.Background(), Credentials{Host: "github.com", Token: "tok"},
		respondWith(&last, http.StatusOK, `{"login":"octocat"}`))

	require.NoError(t, err)
	assert.Equal(t, "octocat", login)
	require.NotNil(t, last, "the stub must have made a request")
	assert.Equal(t, "https://api.github.com/user", last.URL.String())
	assert.Equal(t, "Bearer tok", last.Header.Get("Authorization"))
}

func TestLoginRejectedToken(t *testing.T) {
	var last *http.Request

	login, err := Login(context.Background(), Credentials{Host: "github.com", Token: "stale"},
		respondWith(&last, http.StatusUnauthorized, `{"message":"Bad credentials"}`))

	require.ErrorIs(t, err, ErrBadCredentials)
	assert.Empty(t, login)
}

func TestLoginOtherFailureIsNotBadCredentials(t *testing.T) {
	var last *http.Request

	_, err := Login(context.Background(), Credentials{Host: "github.com", Token: "tok"},
		respondWith(&last, http.StatusInternalServerError, `{"message":"boom"}`))

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrBadCredentials)
	assert.ErrorContains(t, err, "boom")
}

func TestLoginWithoutTokenOrWithUnsupportedHost(t *testing.T) {
	_, err := Login(context.Background(), Credentials{Host: "github.com"})
	assert.ErrorIs(t, err, ErrNoToken)

	_, err = Login(context.Background(), Credentials{Host: "acme.ghe.com", Token: "tok"})
	assert.ErrorIs(t, err, ErrUnsupportedHost)
}
