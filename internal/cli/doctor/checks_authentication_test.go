package doctor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testSecret = "ghp_SECRETSECRETSECRET"

// fakeGitHubAuth answers the github.Auth questions from fixed values.
type fakeGitHubAuth struct {
	token, tokenSource string
	defaultHost        string
	enterprise         bool
}

func (f fakeGitHubAuth) TokenForHost(string) (string, string) { return f.token, f.tokenSource }
func (f fakeGitHubAuth) DefaultHost() (string, string)        { return f.defaultHost, "default" }
func (f fakeGitHubAuth) IsEnterprise(string) bool             { return f.enterprise }

func TestAuthTokenCheckMetadata(t *testing.T) {
	c := NewAuthTokenCheck(fakeGitHubAuth{})

	assert.Equal(t, "auth-token", c.Name())
	assert.Equal(t, "authentication", c.Category())
	assert.False(t, c.Online(), "finding a token needs no network")
	assert.Empty(t, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestAuthTokenCheckFound(t *testing.T) {
	tests := []struct {
		name string
		auth fakeGitHubAuth
		want string
	}{
		{
			name: "environment variable",
			auth: fakeGitHubAuth{token: testSecret, tokenSource: "GH_TOKEN", defaultHost: "github.com"},
			want: "environment variable GH_TOKEN for github.com",
		},
		{
			name: "gh keyring",
			auth: fakeGitHubAuth{token: testSecret, tokenSource: "gh", defaultHost: "github.com"},
			want: "gh auth token (keyring) for github.com",
		},
		{
			name: "enterprise host from the gh config file",
			auth: fakeGitHubAuth{token: testSecret, tokenSource: "oauth_token", defaultHost: "ghe.example.com", enterprise: true},
			want: "gh config file for ghe.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAuthTokenCheck(tt.auth).Run(context.Background())

			assert.Equal(t, Result{Status: StatusOK, Message: tt.want}, got)
			assert.NotContains(t, got.Message, testSecret)
		})
	}
}

func TestAuthTokenCheckMissingFailsWithTheHint(t *testing.T) {
	tests := []struct {
		name            string
		auth            fakeGitHubAuth
		wantMessage     string
		wantRemediation string
	}{
		{
			name:            "github.com",
			auth:            fakeGitHubAuth{defaultHost: "github.com"},
			wantMessage:     "no token found for github.com",
			wantRemediation: `set GH_TOKEN or GITHUB_TOKEN, or run "gh auth login"`,
		},
		{
			name:            "enterprise host",
			auth:            fakeGitHubAuth{defaultHost: "ghe.example.com", enterprise: true},
			wantMessage:     "no token found for ghe.example.com",
			wantRemediation: `set GH_ENTERPRISE_TOKEN or GITHUB_ENTERPRISE_TOKEN, or run "gh auth login --hostname ghe.example.com"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAuthTokenCheck(tt.auth).Run(context.Background())

			assert.Equal(t, Result{Status: StatusFail, Message: tt.wantMessage, Remediation: tt.wantRemediation}, got)
		})
	}
}
