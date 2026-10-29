package authstatus

import (
	"encoding/json"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderText(t *testing.T) {
	tests := []struct {
		name string
		st   Status
		want string
	}{
		{
			name: "valid token from an environment variable, default host",
			st:   Status{Host: "github.com", HostSource: "default", TokenSource: "GH_TOKEN", Valid: true, Login: "octocat"},
			want: "Host:   github.com (default)\n" +
				"Token:  environment variable GH_TOKEN\n" +
				"Login:  octocat\n",
		},
		{
			name: "token from gh's config file, host named on the command line",
			st:   Status{Host: "ghe.example.com", HostSource: "explicit", TokenSource: "oauth_token", Valid: true, Login: "octocat"},
			want: "Host:   ghe.example.com (--hostname)\n" +
				"Token:  gh config file\n" +
				"Login:  octocat\n",
		},
		{
			name: "token from the gh keyring",
			st:   Status{Host: "github.com", HostSource: "GH_HOST", TokenSource: "gh", Valid: true, Login: "octocat"},
			want: "Host:   github.com (GH_HOST)\n" +
				"Token:  gh auth token (keyring)\n" +
				"Login:  octocat\n",
		},
		{
			name: "rejected token",
			st:   Status{Host: "github.com", HostSource: "default", TokenSource: "GITHUB_TOKEN"},
			want: "Host:   github.com (default)\n" +
				"Token:  environment variable GITHUB_TOKEN\n" +
				"Login:  token rejected by GitHub\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.st, clicfg.OutputText)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderJSON(t *testing.T) {
	got, err := Render(Status{Host: "github.com", HostSource: "default", TokenSource: "GH_TOKEN", Valid: true, Login: "octocat"},
		clicfg.OutputJSON)

	require.NoError(t, err)
	assert.JSONEq(t, `{"host":"github.com","hostSource":"default","tokenSource":"GH_TOKEN","valid":true,"login":"octocat"}`, got)
	assert.True(t, json.Valid([]byte(got)))
	require.NotEmpty(t, got)
	assert.Equal(t, byte('\n'), got[len(got)-1], "ends with a newline")
}

func TestRenderJSONRejectedTokenHasNoLogin(t *testing.T) {
	got, err := Render(Status{Host: "github.com", HostSource: "default", TokenSource: "GH_TOKEN"}, clicfg.OutputJSON)

	require.NoError(t, err)
	assert.JSONEq(t, `{"host":"github.com","hostSource":"default","tokenSource":"GH_TOKEN","valid":false}`, got)
}

func TestRenderUnsupportedFormats(t *testing.T) {
	for _, format := range []string{clicfg.OutputEnv, "yaml"} {
		t.Run(format, func(t *testing.T) {
			got, err := Render(Status{}, format)

			require.Error(t, err)
			assert.ErrorContains(t, err, format)
			assert.Empty(t, got)
		})
	}
}
