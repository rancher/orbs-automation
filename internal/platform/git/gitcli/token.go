package gitcli

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
)

type tokenKey struct{}

// credential is a token and the GitHub host it is for.
type credential struct{ baseURL, token string }

// WithToken returns a context whose pushes authenticate to the GitHub host at
// baseURL (such as https://github.com/) with token, as a workflow does: the
// token stands in for the engineer's own credentials. It reaches git as
// configuration in that process's environment, so it is in no remote URL, no
// argument and no file.
func WithToken(ctx context.Context, baseURL, token string) context.Context {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return context.WithValue(ctx, tokenKey{}, credential{baseURL: baseURL, token: token})
}

// tokenEnv is the environment that gives git the token in ctx, or nil when ctx
// has none. It sets http.<baseURL>.extraheader to the way GitHub takes a token
// over HTTPS, the way actions/checkout does for its own. It sets the key empty
// first: a header that the clone already has, from actions/checkout and for the
// workflow's own token, is in the same list, and git would send both.
func tokenEnv(ctx context.Context) []string {
	c, ok := ctx.Value(tokenKey{}).(credential)
	if !ok || c.token == "" {
		return nil
	}
	key := "http." + c.baseURL + ".extraheader"
	header := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+c.token))
	env := []string{"GIT_CONFIG_COUNT=2"}
	for i, value := range []string{"", header} {
		n := strconv.Itoa(i)
		env = append(env, "GIT_CONFIG_KEY_"+n+"="+key, "GIT_CONFIG_VALUE_"+n+"="+value)
	}
	return env
}
