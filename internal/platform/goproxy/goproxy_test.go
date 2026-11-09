package goproxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const base = "https://proxy.test"

// fakeProxy is a Go module proxy that answers each path with a status and a body,
// and records what it was asked. It is an http.RoundTripper, so tests need no port.
type fakeProxy struct {
	answers map[string]answer
	paths   []string
	methods []string
	agents  []string
	down    bool // every request fails to connect
	hang    bool // no answer comes: the request waits until it is cancelled
}

type answer struct {
	status int
	body   string
}

func (f *fakeProxy) RoundTrip(r *http.Request) (*http.Response, error) {
	f.paths = append(f.paths, r.URL.EscapedPath())
	f.methods = append(f.methods, r.Method)
	f.agents = append(f.agents, r.Header.Get("User-Agent"))
	if f.down {
		return nil, errors.New("dial tcp: connection refused")
	}
	// A transport gives up on a cancelled request before it does anything else; the
	// order is not left to a select, which picks at random when both are ready.
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	if f.hang {
		<-r.Context().Done()
		return nil, r.Context().Err()
	}
	a, ok := f.answers[r.URL.EscapedPath()]
	if !ok {
		a = answer{status: http.StatusNotFound, body: "not found: unknown module\n"}
	}
	return &http.Response{
		StatusCode: a.status, Status: http.StatusText(a.status), Header: http.Header{},
		Body: io.NopCloser(strings.NewReader(a.body)), Request: r,
	}, nil
}

// checker is a Checker that asks the fake.
func checker(f *fakeProxy) Checker {
	return Checker{BaseURL: base, Client: &http.Client{Transport: f}}
}

func answers(path string, status int, body string) *fakeProxy {
	return &fakeProxy{answers: map[string]answer{path: {status, body}}}
}

func TestAVersionTheProxyHasIsFound(t *testing.T) {
	f := answers("/github.com/rancher/wrangler/v3/@v/v3.7.2.info", 200, `{"Version":"v3.7.2","Time":"2026-03-04T05:06:07Z"}`)

	got := checker(f).Exists(context.Background(), "github.com/rancher/wrangler/v3", "v3.7.2")

	assert.Equal(t, Result{Status: Found}, got)
	assert.Equal(t, []string{"/github.com/rancher/wrangler/v3/@v/v3.7.2.info"}, f.paths, "the module path keeps its major suffix")
	assert.Equal(t, []string{http.MethodGet}, f.methods)
}

func TestAVersionTheProxyDoesNotHaveIsNotFound(t *testing.T) {
	for name, status := range map[string]int{"not found": http.StatusNotFound, "gone": http.StatusGone} {
		t.Run(name, func(t *testing.T) {
			f := answers("/github.com/acme/lib/@v/v1.0.0.info", status, "not found: invalid version: unknown revision v1.0.0\n")

			got := checker(f).Exists(context.Background(), "github.com/acme/lib", "v1.0.0")

			assert.Equal(t, NotFound, got.Status)
		})
	}
}

func TestAnythingElseTheProxySaysIsUnreachableWithTheStatus(t *testing.T) {
	for name, status := range map[string]int{"server error": 500, "bad gateway": 502, "too many requests": 429, "forbidden": 403} {
		t.Run(name, func(t *testing.T) {
			f := answers("/github.com/acme/lib/@v/v1.0.0.info", status, "try later")

			got := checker(f).Exists(context.Background(), "github.com/acme/lib", "v1.0.0")

			assert.Equal(t, Unreachable, got.Status)
			assert.Contains(t, got.Detail, http.StatusText(status))
		})
	}
}

func TestAnAnswerThatIsNotTheInfoOfTheVersionIsUnreachable(t *testing.T) {
	tests := map[string]string{
		"not JSON":               "<html>a captive portal</html>",
		"another version":        `{"Version":"v1.0.1","Time":"2026-03-04T05:06:07Z"}`,
		"no version in the JSON": `{"Time":"2026-03-04T05:06:07Z"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			f := answers("/github.com/acme/lib/@v/v1.0.0.info", 200, body)

			got := checker(f).Exists(context.Background(), "github.com/acme/lib", "v1.0.0")

			assert.Equal(t, Unreachable, got.Status, "a 200 that is not the version is not the proxy saying it has it")
			assert.NotEmpty(t, got.Detail)
		})
	}
}

func TestACapitalInAModulePathOrVersionIsEscapedAsTheProxyWantsIt(t *testing.T) {
	f := answers("/github.com/!burnt!sushi/toml/@v/v1.4.0-!r!c.1.info", 200, `{"Version":"v1.4.0-RC.1"}`)

	got := checker(f).Exists(context.Background(), "github.com/BurntSushi/toml", "v1.4.0-RC.1")

	assert.Equal(t, Found, got.Status)
	assert.Equal(t, []string{"/github.com/!burnt!sushi/toml/@v/v1.4.0-!r!c.1.info"}, f.paths)
}

func TestAnIncompatibleVersionAndAPseudoVersionAreAsked(t *testing.T) {
	for _, version := range []string{"v2.0.0+incompatible", "v0.0.0-20260102030405-abcdef123456"} {
		f := answers("/github.com/acme/lib/@v/"+version+".info", 200, `{"Version":"`+version+`"}`)

		got := checker(f).Exists(context.Background(), "github.com/acme/lib", version)

		assert.Equal(t, Found, got.Status, version)
	}
}

func TestABaseURLWithASlashAtTheEndIsFine(t *testing.T) {
	f := answers("/github.com/acme/lib/@v/v1.0.0.info", 200, `{"Version":"v1.0.0"}`)

	got := Checker{BaseURL: base + "/", Client: &http.Client{Transport: f}}.Exists(context.Background(), "github.com/acme/lib", "v1.0.0")

	assert.Equal(t, Found, got.Status)
}

func TestAProxyThatCannotBeReachedIsUnreachable(t *testing.T) {
	got := checker(&fakeProxy{down: true}).Exists(context.Background(), "github.com/acme/lib", "v1.0.0")

	assert.Equal(t, Unreachable, got.Status)
	assert.Contains(t, got.Detail, "connection refused")
}

func TestACancelledRequestIsUnreachable(t *testing.T) {
	f := answers("/github.com/acme/lib/@v/v1.0.0.info", 200, `{"Version":"v1.0.0"}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := checker(f).Exists(ctx, "github.com/acme/lib", "v1.0.0")

	assert.Equal(t, Unreachable, got.Status)
}

func TestARequestThatTakesTooLongIsUnreachable(t *testing.T) {
	f := &fakeProxy{hang: true}

	got := Checker{BaseURL: base, Client: &http.Client{Transport: f, Timeout: 20 * time.Millisecond}}.Exists(context.Background(), "github.com/acme/lib", "v1.0.0")

	assert.Equal(t, Unreachable, got.Status)
}

func TestTheDefaultIsThePublicProxyOverHTTPS(t *testing.T) {
	assert.Equal(t, "https://proxy.golang.org", DefaultBaseURL)
	require.True(t, strings.HasPrefix(DefaultBaseURL, "https://"))
}

func TestTheZeroValueAsksThePublicProxy(t *testing.T) {
	f := answers("/github.com/acme/lib/@v/v1.0.0.info", 200, `{"Version":"v1.0.0"}`)
	var host string
	c := Checker{Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		host = r.URL.Host
		return f.RoundTrip(r)
	})}}

	got := c.Exists(context.Background(), "github.com/acme/lib", "v1.0.0")

	assert.Equal(t, Found, got.Status)
	assert.Equal(t, "proxy.golang.org", host)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestARequestSaysWhoIsAsking(t *testing.T) {
	f := answers("/github.com/acme/lib/@v/v1.0.0.info", 200, `{"Version":"v1.0.0"}`)

	checker(f).Exists(context.Background(), "github.com/acme/lib", "v1.0.0")

	require.Len(t, f.agents, 1)
	assert.True(t, strings.HasPrefix(f.agents[0], "gero"), "got %q", f.agents[0])
}
