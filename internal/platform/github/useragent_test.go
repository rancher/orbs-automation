package github

import (
	"net/http"
	"testing"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserAgentIsProductSlashVersion(t *testing.T) {
	t.Cleanup(func() { SetUserAgentVersion("dev") })

	SetUserAgentVersion("1.2.3")

	assert.Equal(t, "gero/1.2.3 (+https://github.com/rancher/orbs-automation; ORBS Team Automation)", UserAgent())
}

func TestUserAgentDefaultsToDev(t *testing.T) {
	assert.Equal(t, "gero/dev (+https://github.com/rancher/orbs-automation; ORBS Team Automation)", UserAgent())
}

func TestClientSendsTheUserAgent(t *testing.T) {
	t.Cleanup(func() { SetUserAgentVersion("dev") })
	SetUserAgentVersion("1.2.3")
	var last *http.Request

	_, err := Login(t.Context(), Credentials{Host: "github.com", Token: "tok"},
		respondWith(&last, http.StatusOK, `{"login":"octocat"}`))

	require.NoError(t, err)
	require.NotNil(t, last)
	assert.Equal(t, UserAgent(), last.Header.Get("User-Agent"))
}

func TestCallerOptionsOverrideTheUserAgent(t *testing.T) {
	var last *http.Request

	_, err := Login(t.Context(), Credentials{Host: "github.com", Token: "tok"},
		respondWith(&last, http.StatusOK, `{"login":"octocat"}`), gogithub.WithUserAgent("custom/1"))

	require.NoError(t, err)
	assert.Equal(t, "custom/1", last.Header.Get("User-Agent"))
}
