package github

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const hostsYML = `github.com:
    oauth_token: config-token
    user: someone
ghe.example.com:
    oauth_token: ghe-config-token
    user: someone
`

// go-gh reads gh's config once per process, so the fixture must exist before
// the first call.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ghcfg")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hosts.yml"), []byte(hostsYML), 0o600); err != nil {
		panic(err)
	}
	if err := os.Setenv("GH_CONFIG_DIR", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// cleanEnv clears every variable that feeds token and host resolution and
// leaves no gh on PATH.
func cleanEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_HOST", "GH_PATH",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("PATH", t.TempDir())
}

// stubGH puts a fake gh on PATH that records its arguments and prints token.
func stubGH(t *testing.T, token string) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\necho \"$@\" > " + argsFile + "\necho " + token + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", dir)
	return argsFile
}

func TestGoGhTokenOrder(t *testing.T) {
	cleanEnv(t)
	stubGH(t, "keyring-token")
	a := GoGh{}

	t.Setenv("GH_TOKEN", "gh-token")
	t.Setenv("GITHUB_TOKEN", "github-token")
	token, source := a.TokenForHost("github.com")
	assert.Equal(t, []string{"gh-token", "GH_TOKEN"}, []string{token, source}, "GH_TOKEN comes first")

	t.Setenv("GH_TOKEN", "")
	token, source = a.TokenForHost("github.com")
	assert.Equal(t, []string{"github-token", "GITHUB_TOKEN"}, []string{token, source}, "then GITHUB_TOKEN")

	t.Setenv("GITHUB_TOKEN", "")
	token, source = a.TokenForHost("github.com")
	assert.Equal(t, []string{"config-token", "oauth_token"}, []string{token, source}, "then gh's config file")
}

func TestGoGhTokenFromGhKeyring(t *testing.T) {
	cleanEnv(t)
	args := stubGH(t, "keyring-token")

	token, source := GoGh{}.TokenForHost("other.example.com") // not in hosts.yml

	assert.Equal(t, []string{"keyring-token", "gh"}, []string{token, source})
	got, err := os.ReadFile(args)
	require.NoError(t, err)
	assert.Equal(t, "auth token --secure-storage --hostname other.example.com\n", string(got))
}

func TestGoGhNoTokenIsEmpty(t *testing.T) {
	cleanEnv(t) // no gh on PATH

	token, _ := GoGh{}.TokenForHost("other.example.com")

	assert.Empty(t, token)
}

func TestGoGhEnterpriseIgnoresGHToken(t *testing.T) {
	cleanEnv(t)
	t.Setenv("GH_TOKEN", "dotcom-token")
	a := GoGh{}

	token, source := a.TokenForHost("ghe.example.com")
	assert.Equal(t, []string{"ghe-config-token", "oauth_token"}, []string{token, source}, "GH_TOKEN is not for GHES")

	t.Setenv("GH_ENTERPRISE_TOKEN", "ghe-env")
	token, source = a.TokenForHost("ghe.example.com")
	assert.Equal(t, []string{"ghe-env", "GH_ENTERPRISE_TOKEN"}, []string{token, source})
}

func TestGoGhDefaultHost(t *testing.T) {
	cleanEnv(t)
	a := GoGh{}

	host, source := a.DefaultHost() // two hosts in the config, so no single default
	assert.Equal(t, []string{"github.com", "default"}, []string{host, source})

	t.Setenv("GH_HOST", "ghe.example.com")
	host, source = a.DefaultHost()
	assert.Equal(t, []string{"ghe.example.com", "GH_HOST"}, []string{host, source})
}

func TestGoGhIsEnterprise(t *testing.T) {
	a := GoGh{}

	assert.False(t, a.IsEnterprise("github.com"))
	assert.False(t, a.IsEnterprise("acme.ghe.com"), "tenancy hosts use the github.com variables")
	assert.True(t, a.IsEnterprise("ghe.example.com"))
}
