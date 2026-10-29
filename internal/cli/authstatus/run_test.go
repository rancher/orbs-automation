package authstatus

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "ghp_SECRETSECRETSECRET"

type fakeAuth struct {
	token       string
	tokenSource string
	defaultHost string
	hostSource  string
}

func (f fakeAuth) TokenForHost(string) (string, string) { return f.token, f.tokenSource }
func (f fakeAuth) DefaultHost() (string, string)        { return f.defaultHost, f.hostSource }
func (f fakeAuth) IsEnterprise(host string) bool        { return host != "github.com" }

type fakeVerifier struct {
	login string
	err   error
	calls []github.Credentials
}

func (f *fakeVerifier) Login(_ context.Context, c github.Credentials) (string, error) {
	f.calls = append(f.calls, c)
	return f.login, f.err
}

func settings(format string) clicfg.Settings { return clicfg.Settings{Output: format} }

func goodAuth() fakeAuth {
	return fakeAuth{token: secret, tokenSource: "GH_TOKEN", defaultHost: "github.com", hostSource: "default"}
}

func TestRunReportsAWorkingToken(t *testing.T) {
	v := &fakeVerifier{login: "octocat"}

	out, err := run(context.Background(), settings(clicfg.OutputText), "", goodAuth(), v)

	require.NoError(t, err)
	assert.Equal(t, "Host:   github.com (default)\nToken:  environment variable GH_TOKEN\nLogin:  octocat\n", out)
	require.Len(t, v.calls, 1)
	assert.Equal(t, github.Credentials{Host: "github.com", HostSource: "default", Token: secret, TokenSource: "GH_TOKEN"}, v.calls[0])
	assert.NotContains(t, out, secret)
}

func TestRunNamedHostSkipsTheDefault(t *testing.T) {
	v := &fakeVerifier{login: "octocat"}

	out, err := run(context.Background(), settings(clicfg.OutputText), "ghe.example.com", goodAuth(), v)

	require.NoError(t, err)
	assert.Contains(t, out, "Host:   ghe.example.com (--hostname)\n")
	require.Len(t, v.calls, 1)
	assert.Equal(t, "ghe.example.com", v.calls[0].Host)
}

func TestRunJSON(t *testing.T) {
	v := &fakeVerifier{login: "octocat"}

	out, err := run(context.Background(), settings(clicfg.OutputJSON), "", goodAuth(), v)

	require.NoError(t, err)
	assert.JSONEq(t, `{"host":"github.com","hostSource":"default","tokenSource":"GH_TOKEN","valid":true,"login":"octocat"}`, out)
	assert.NotContains(t, out, secret)
}

func TestRunNoTokenFailsWithTheHint(t *testing.T) {
	v := &fakeVerifier{}
	a := fakeAuth{defaultHost: "github.com", hostSource: "default"}

	out, err := run(context.Background(), settings(clicfg.OutputText), "", a, v)

	require.ErrorIs(t, err, github.ErrNoToken)
	assert.ErrorContains(t, err, "GH_TOKEN")
	assert.Empty(t, out)
	assert.Empty(t, v.calls)
}

func TestRunRejectedTokenPrintsTheResultAndFails(t *testing.T) {
	v := &fakeVerifier{err: github.ErrBadCredentials}

	out, err := run(context.Background(), settings(clicfg.OutputText), "", goodAuth(), v)

	require.ErrorIs(t, err, github.ErrBadCredentials)
	assert.Equal(t, "Host:   github.com (default)\nToken:  environment variable GH_TOKEN\nLogin:  token rejected by GitHub\n", out)
	assert.NotContains(t, out, secret)
}

func TestRunOtherFailureHasNoOutput(t *testing.T) {
	boom := errors.New("connection refused")
	v := &fakeVerifier{err: boom}

	out, err := run(context.Background(), settings(clicfg.OutputText), "", goodAuth(), v)

	require.ErrorIs(t, err, boom)
	assert.Empty(t, out)
}

func TestRunUnsupportedFormatFailsBeforeAskingGitHub(t *testing.T) {
	v := &fakeVerifier{login: "octocat"}

	out, err := run(context.Background(), settings(clicfg.OutputEnv), "", goodAuth(), v)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Empty(t, v.calls, "no API call for a format that cannot be rendered")
}
