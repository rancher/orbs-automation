package gitcli

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gh = "https://github.com/"

// closed is a host that refuses every connection, so a push to it fails at once
// without touching the network.
const closed = "http://127.0.0.1:1/"

func basic(token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token))
}

func TestWithTokenSendsItAsTheOnlyAuthorizationOfTheHost(t *testing.T) {
	dir := clone(t, upstream(t))
	// What actions/checkout leaves in the clone: a header for the workflow's own
	// token, which must not go with ours.
	run(t, dir, "config", "http."+gh+".extraheader", "Authorization: Basic d29ya2Zsb3c6dG9rZW4=")

	res, err := runAuthenticated(WithToken(ctx, gh, "s3cret"), "-C", dir, "config", "--get-all", "http."+gh+".extraheader")

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(res.Stdout)), "\n")
	assert.Equal(t, []string{"Authorization: " + basic("s3cret")}, lines, "ours alone: the one that was there is reset")
}

func TestWithTokenReachesNoOtherHost(t *testing.T) {
	dir := clone(t, upstream(t))

	res, err := runAuthenticated(WithToken(ctx, gh, "s3cret"), "-C", dir, "config", "--get-all", "http.https://example.com/.extraheader")

	assert.Error(t, err, "git config exits 1 for a key that is not set")
	assert.Empty(t, string(res.Stdout))
}

func TestWithoutATokenNoHeaderIsAdded(t *testing.T) {
	dir := clone(t, upstream(t))

	res, err := runAuthenticated(ctx, "-C", dir, "config", "--get-all", "http."+gh+".extraheader")

	assert.Error(t, err)
	assert.Empty(t, string(res.Stdout))
}

func TestPushWithATokenLeavesItNowhereInTheClone(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "remote", "add", "web", closed+"org/repo.git")
	run(t, dir, "checkout", "--quiet", "-b", "bot/x")
	write(t, dir, "build.yaml", "image: new\n")
	run(t, dir, "commit", "--quiet", "-am", "bump")

	// The port is closed, so the push fails; what it leaves behind may not
	// differ.
	err := Push(WithToken(ctx, closed, "s3cret"), dir, "web", "bot/x")

	cfg := run(t, dir, "config", "--local", "--list")
	assert.NotContains(t, cfg, "s3cret")
	assert.NotContains(t, cfg, base64.StdEncoding.EncodeToString([]byte("x-access-token:s3cret")))
	assert.Equal(t, closed+"org/repo.git", run(t, dir, "remote", "get-url", "web"), "the remote URL is as it was")
	if err != nil {
		assert.NotContains(t, err.Error(), "s3cret")
		assert.NotContains(t, err.Error(), base64.StdEncoding.EncodeToString([]byte("x-access-token:s3cret")))
	}
}
