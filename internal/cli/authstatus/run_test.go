package authstatus

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
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

func TestCheckReturnsTheStatusOfAWorkingToken(t *testing.T) {
	v := &fakeVerifier{login: "octocat"}

	st, err := check(context.Background(), "", goodAuth(), v)

	require.NoError(t, err)
	assert.Equal(t, Status{Host: "github.com", HostSource: "default", TokenSource: "GH_TOKEN", Valid: true, Login: "octocat"}, st)
	require.Len(t, v.calls, 1)
	assert.Equal(t, secret, v.calls[0].Token, "the token goes to the verifier, and only there")
}

func TestCheckNamedHost(t *testing.T) {
	v := &fakeVerifier{login: "octocat"}

	st, err := check(context.Background(), "ghe.example.com", goodAuth(), v)

	require.NoError(t, err)
	assert.Equal(t, "ghe.example.com", st.Host)
	assert.Equal(t, github.SourceExplicit, st.HostSource)
}

func TestCheckRejectedTokenIsAStatusAndAnError(t *testing.T) {
	v := &fakeVerifier{err: github.ErrBadCredentials}

	st, err := check(context.Background(), "", goodAuth(), v)

	require.ErrorIs(t, err, github.ErrBadCredentials)
	assert.Equal(t, Status{Host: "github.com", HostSource: "default", TokenSource: "GH_TOKEN", Valid: false}, st)
}

func TestCheckOtherFailureHasNoStatus(t *testing.T) {
	boom := errors.New("connection refused")

	st, err := check(context.Background(), "", goodAuth(), &fakeVerifier{err: boom})

	require.ErrorIs(t, err, boom)
	assert.Equal(t, Status{}, st)
}

func TestCheckWithoutATokenHasNoStatus(t *testing.T) {
	v := &fakeVerifier{}

	st, err := check(context.Background(), "", fakeAuth{defaultHost: "github.com", hostSource: "default"}, v)

	require.ErrorIs(t, err, github.ErrNoToken)
	assert.Equal(t, Status{}, st)
	assert.Empty(t, v.calls)
}

func TestRunReportsAnInstallationTokenInsteadOfFailing(t *testing.T) {
	v := &fakeVerifier{err: fmt.Errorf("on github.com: %w", github.ErrNotUserToken)}

	out, err := run(context.Background(), settings(clicfg.OutputText), "", goodAuth(), v)

	require.NoError(t, err)
	assert.Equal(t, "Host:   github.com (default)\nToken:  environment variable GH_TOKEN\nLogin:  installation token\n", out)
	assert.NotContains(t, out, secret)
}

func TestRunReportsAnInstallationTokenAsJSON(t *testing.T) {
	v := &fakeVerifier{err: github.ErrNotUserToken}

	out, err := run(context.Background(), settings(clicfg.OutputJSON), "", goodAuth(), v)

	require.NoError(t, err)
	assert.Contains(t, out, `"valid": true`)
	assert.Contains(t, out, `"installationToken": true`)
	assert.NotContains(t, out, `"login"`)
}
