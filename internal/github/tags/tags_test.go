package tags

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// stub answers every request the same way and records them, with their bodies.
type stub struct {
	status int
	body   string
	reqs   []*http.Request
	bodies []string
}

func (s *stub) option() gogithub.ClientOptionsFunc {
	return gogithub.WithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body string
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			body = string(data)
		}
		s.reqs, s.bodies = append(s.reqs, r), append(s.bodies, body)
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		rec.WriteHeader(s.status)
		_, _ = rec.WriteString(s.body)
		return rec.Result(), nil
	}))
}

func service(t *testing.T, s *stub) *Service {
	t.Helper()
	svc, err := New(github.Credentials{Host: "github.com", Token: "tok"}, s.option())
	require.NoError(t, err)
	return svc
}

func TestCreateMakesTheTagAsARefAtTheCommit(t *testing.T) {
	s := &stub{status: 201, body: `{"ref":"refs/tags/v0.8.4-rc.1"}`}

	err := service(t, s).Create(bg, "rancher/shell", "v0.8.4-rc.1", "8c613980aa")

	require.NoError(t, err)
	require.Len(t, s.reqs, 1, "one request")
	assert.Equal(t, "POST", s.reqs[0].Method)
	assert.Equal(t, "/repos/rancher/shell/git/refs", s.reqs[0].URL.Path)
	assert.JSONEq(t, `{"ref":"refs/tags/v0.8.4-rc.1","sha":"8c613980aa"}`, s.bodies[0], "a ref and no tag object")
}

func TestCreateOfATagThatExistsSaysSoAndNamesIt(t *testing.T) {
	s := &stub{status: 422, body: `{"message":"Reference already exists"}`}

	err := service(t, s).Create(bg, "rancher/shell", "v0.8.4-rc.1", "8c613980aa")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rancher/shell")
	assert.Contains(t, err.Error(), "v0.8.4-rc.1")
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreateWithATokenThatMayNotWriteSaysWhy(t *testing.T) {
	s := &stub{status: 403, body: `{"message":"Resource not accessible by integration"}`}

	err := service(t, s).Create(bg, "rancher/shell", "v1.0.0", "abc")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Resource not accessible")
}

func TestARepositoryThatIsNotOwnerSlashNameIsAnErrorAndMakesNoRequest(t *testing.T) {
	for _, slug := range []string{"shell", "rancher/", "/shell", "a/b/c", ""} {
		s := &stub{status: 201, body: `{}`}

		err := service(t, s).Create(bg, slug, "v1.0.0", "abc")

		require.Error(t, err, slug)
		assert.Empty(t, s.reqs, slug)
	}
}
