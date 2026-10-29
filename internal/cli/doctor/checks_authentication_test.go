package doctor

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rancher/orbs-automation/internal/cli/authstatus"
	"github.com/rancher/orbs-automation/internal/github"
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

// fakeVerifier answers Verify from fixed values.
type fakeVerifier struct {
	status authstatus.Status
	err    error
	calls  int
}

func (f *fakeVerifier) Verify(context.Context) (authstatus.Status, error) {
	f.calls++
	return f.status, f.err
}

func TestAuthAPICheckMetadata(t *testing.T) {
	c := NewAuthAPICheck(&fakeVerifier{})

	assert.Equal(t, "auth-api", c.Name())
	assert.Equal(t, "authentication", c.Category())
	assert.True(t, c.Online(), "it calls GitHub")
	assert.Equal(t, []string{"auth-token"}, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestAuthAPICheckAcceptedToken(t *testing.T) {
	v := &fakeVerifier{status: authstatus.Status{Host: "github.com", TokenSource: "GH_TOKEN", Valid: true, Login: "octocat"}}

	got := NewAuthAPICheck(v).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "signed in as octocat on github.com"}, got)
	assert.Equal(t, 1, v.calls)
}

func TestAuthAPICheckRejectedTokenFailsAndSaysWhereItCameFrom(t *testing.T) {
	tests := []struct {
		name            string
		status          authstatus.Status
		wantMessage     string
		wantRemediation string
	}{
		{
			name:        "environment variable",
			status:      authstatus.Status{Host: "github.com", TokenSource: "GH_TOKEN"},
			wantMessage: "GitHub rejected the token from environment variable GH_TOKEN on github.com",
			wantRemediation: "the token in GH_TOKEN is invalid or expired: replace it, " +
				"or unset it to fall back to your gh login",
		},
		{
			name:            "enterprise environment variable",
			status:          authstatus.Status{Host: "ghe.example.com", TokenSource: "GH_ENTERPRISE_TOKEN"},
			wantMessage:     "GitHub rejected the token from environment variable GH_ENTERPRISE_TOKEN on ghe.example.com",
			wantRemediation: "the token in GH_ENTERPRISE_TOKEN is invalid or expired: replace it, or unset it to fall back to your gh login",
		},
		{
			name:            "gh keyring",
			status:          authstatus.Status{Host: "github.com", TokenSource: "gh"},
			wantMessage:     "GitHub rejected the token from gh auth token (keyring) on github.com",
			wantRemediation: `the gh login is invalid or expired: sign in again with "gh auth login"`,
		},
		{
			name:            "gh config file on an enterprise host",
			status:          authstatus.Status{Host: "ghe.example.com", TokenSource: "oauth_token"},
			wantMessage:     "GitHub rejected the token from gh config file on ghe.example.com",
			wantRemediation: `the gh login is invalid or expired: sign in again with "gh auth login --hostname ghe.example.com"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fmt.Errorf("%w on %s", github.ErrBadCredentials, tt.status.Host)

			got := NewAuthAPICheck(&fakeVerifier{status: tt.status, err: err}).Run(context.Background())

			assert.Equal(t, Result{Status: StatusFail, Message: tt.wantMessage, Remediation: tt.wantRemediation}, got)
		})
	}
}

func TestAuthAPICheckCannotReachGitHubIsAWarning(t *testing.T) {
	v := &fakeVerifier{err: errors.New("dial tcp: connection refused")}

	got := NewAuthAPICheck(v).Run(context.Background())

	assert.Equal(t, Result{
		Status:      StatusWarn,
		Message:     "could not ask GitHub: dial tcp: connection refused",
		Remediation: "check your network connection, proxy settings or VPN, then run again",
	}, got, "a network problem says nothing about the token")
}
