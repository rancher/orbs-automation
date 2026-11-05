package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRegistry serves manifests for the images it has, optionally behind an
// anonymous bearer token as Docker Hub does.
type fakeRegistry struct {
	images     map[string]bool // "name:tag"
	needsToken bool
	tokenJSON  string // the token endpoint's answer; "" means {"token":"t0k"}
	status     int    // when set, every manifest request gets it
	manifests  atomic.Int32
	tokens     atomic.Int32
	methods    []string
	accepts    []string
	srv        *httptest.Server
}

func newFake(t *testing.T, images ...string) *fakeRegistry {
	t.Helper()
	f := &fakeRegistry{images: map[string]bool{}}
	for _, i := range images {
		f.images[i] = true
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRegistry) host() string { return strings.TrimPrefix(f.srv.URL, "http://") }

func (f *fakeRegistry) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		f.tokens.Add(1)
		answer := f.tokenJSON
		if answer == "" {
			answer = `{"token":"t0k"}`
		}
		_, _ = w.Write([]byte(answer))
		return
	}
	f.manifests.Add(1)
	f.methods = append(f.methods, r.Method)
	f.accepts = append(f.accepts, r.Header.Get("Accept"))
	if f.needsToken && r.Header.Get("Authorization") != "Bearer t0k" {
		w.Header().Set("WWW-Authenticate",
			`Bearer realm="`+f.srv.URL+`/token",service="registry.test",scope="repository:rancher/scc-operator:pull"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
		return
	}
	// /v2/<name>/manifests/<tag>
	rest := strings.TrimPrefix(r.URL.Path, "/v2/")
	name, tag, _ := strings.Cut(rest, "/manifests/")
	if f.images[name+":"+tag] {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func checker() Checker { return Checker{Scheme: "http"} }

func TestExistsFound(t *testing.T) {
	f := newFake(t, "rancher/scc-operator:v0.5.3")
	got := checker().Exists(context.Background(), f.host(), "rancher/scc-operator", "v0.5.3")
	assert.Equal(t, Result{Status: Found}, got)
}

func TestExistsNotFound(t *testing.T) {
	f := newFake(t, "rancher/scc-operator:v0.5.3")
	got := checker().Exists(context.Background(), f.host(), "rancher/scc-operator", "v9.9.9")
	assert.Equal(t, Result{Status: NotFound}, got)
}

func TestExistsAsksWithHeadAndTheManifestTypes(t *testing.T) {
	f := newFake(t, "o/a:v1")
	checker().Exists(context.Background(), f.host(), "o/a", "v1")
	require.Len(t, f.methods, 1)
	assert.Equal(t, http.MethodHead, f.methods[0], "the image is not downloaded")
	for _, mt := range []string{
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
	} {
		assert.Contains(t, f.accepts[0], mt, "multi-arch images are indexes, which are only served when asked for")
	}
}

func TestExistsFetchesAnAnonymousTokenWhenChallenged(t *testing.T) {
	f := newFake(t, "rancher/scc-operator:v0.5.3")
	f.needsToken = true

	got := checker().Exists(context.Background(), f.host(), "rancher/scc-operator", "v0.5.3")

	assert.Equal(t, Result{Status: Found}, got)
	assert.EqualValues(t, 1, f.tokens.Load())
	assert.EqualValues(t, 2, f.manifests.Load(), "the challenge, then the retry with the token")
}

func TestExistsNotFoundBehindAToken(t *testing.T) {
	f := newFake(t)
	f.needsToken = true
	got := checker().Exists(context.Background(), f.host(), "rancher/scc-operator", "v1")
	assert.Equal(t, Result{Status: NotFound}, got)
}

func TestExistsAcceptsAnAccessTokenField(t *testing.T) {
	f := newFake(t, "rancher/scc-operator:v1")
	f.needsToken = true
	f.tokenJSON = `{"access_token":"t0k"}`
	got := checker().Exists(context.Background(), f.host(), "rancher/scc-operator", "v1")
	assert.Equal(t, Result{Status: Found}, got)
}

func TestExistsIsUnreachableWhenTheTokenIsRefused(t *testing.T) {
	f := newFake(t, "rancher/scc-operator:v1")
	f.needsToken = true
	f.tokenJSON = `{"token":"wrong"}`

	got := checker().Exists(context.Background(), f.host(), "rancher/scc-operator", "v1")

	assert.Equal(t, Unreachable, got.Status)
	assert.Contains(t, got.Detail, "401")
	assert.EqualValues(t, 2, f.manifests.Load(), "it asks again once and no more")
}

func TestExistsIsUnreachableWhenAuthenticationIsNotAnonymous(t *testing.T) {
	f := newFake(t)
	f.status = http.StatusUnauthorized // no bearer challenge to follow
	got := checker().Exists(context.Background(), f.host(), "o/a", "v1")
	assert.Equal(t, Unreachable, got.Status)
	assert.Contains(t, got.Detail, "authentication")
}

func TestExistsIsUnreachableOnOtherStatuses(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusForbidden, http.StatusTooManyRequests} {
		f := newFake(t)
		f.status = status
		got := checker().Exists(context.Background(), f.host(), "o/a", "v1")
		assert.Equal(t, Unreachable, got.Status, status)
		assert.Contains(t, got.Detail, http.StatusText(status))
	}
}

func TestExistsIsUnreachableWhenTheRegistryCannotBeReached(t *testing.T) {
	f := newFake(t)
	host := f.host()
	f.srv.Close()
	got := checker().Exists(context.Background(), host, "o/a", "v1")
	assert.Equal(t, Unreachable, got.Status)
	assert.NotEmpty(t, got.Detail)
}

func TestExistsStopsWhenTheContextIsCancelled(t *testing.T) {
	f := newFake(t, "o/a:v1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := checker().Exists(ctx, f.host(), "o/a", "v1")
	assert.Equal(t, Unreachable, got.Status)
	assert.EqualValues(t, 0, f.manifests.Load())
}

func TestExistsIsUnreachableOnABrokenTokenAnswer(t *testing.T) {
	f := newFake(t, "o/a:v1")
	f.needsToken = true
	f.tokenJSON = `not json`
	got := checker().Exists(context.Background(), f.host(), "o/a", "v1")
	assert.Equal(t, Unreachable, got.Status)
}

func TestResultJSON(t *testing.T) {
	b, err := json.Marshal(Result{Status: NotFound})
	require.NoError(t, err)
	assert.Contains(t, string(b), "not found")
}
