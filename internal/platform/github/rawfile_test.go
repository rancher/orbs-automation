package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawHost serves files for the paths it has, and answers 404 to an anonymous
// read of a private one, as raw.githubusercontent.com does.
type rawHost struct {
	files   map[string]string // URL path to content
	private map[string]bool   // URL paths that need the token
	token   string
	seen    []*http.Request
	status  int // when set, every request gets it
}

func (h *rawHost) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	h.seen = append(h.seen, req)
	reply := func(code int, body string) (*http.Response, error) {
		return &http.Response{
			StatusCode: code, Status: http.StatusText(code), Body: io.NopCloser(strings.NewReader(body)),
			Header: http.Header{}, Request: req,
		}, nil
	}
	if h.status != 0 {
		return reply(h.status, "")
	}
	content, ok := h.files[req.URL.Path]
	if !ok {
		return reply(http.StatusNotFound, "404: Not Found")
	}
	if h.private[req.URL.Path] && req.Header.Get("Authorization") != "token "+h.token {
		return reply(http.StatusNotFound, "404: Not Found")
	}
	return reply(http.StatusOK, content)
}

func rawClient(h *rawHost) *http.Client { return &http.Client{Transport: h} }

func TestRawFile(t *testing.T) {
	h := &rawHost{files: map[string]string{"/rancher/rancher/abc123/build.yaml": "key: value\n"}}

	got, err := RawFile(context.Background(), rawClient(h), "github.com", "", "rancher/rancher", "abc123", "build.yaml")

	require.NoError(t, err)
	assert.Equal(t, "key: value\n", string(got))
	require.Len(t, h.seen, 1)
	assert.Equal(t, "raw.githubusercontent.com", h.seen[0].URL.Host)
	assert.Equal(t, "https", h.seen[0].URL.Scheme)
	assert.Empty(t, h.seen[0].Header.Get("Authorization"), "a public file is read anonymously")
	assert.Equal(t, UserAgent(), h.seen[0].Header.Get("User-Agent"))
}

func TestRawFileEscapesThePath(t *testing.T) {
	h := &rawHost{files: map[string]string{"/o/r/release/v2.15/dir with space/a#b.yaml": "x"}}
	got, err := RawFile(context.Background(), rawClient(h), "github.com", "", "o/r", "release/v2.15", "dir with space/a#b.yaml")
	require.NoError(t, err)
	assert.Equal(t, "x", string(got))
	assert.Equal(t, "/o/r/release/v2.15/dir%20with%20space/a%23b.yaml", h.seen[0].URL.EscapedPath())
	_ = url.URL{}
}

func TestRawFileAsksAgainWithTheTokenForAPrivateRepository(t *testing.T) {
	path := "/o/private/abc/build.yaml"
	h := &rawHost{files: map[string]string{path: "secret: 1"}, private: map[string]bool{path: true}, token: "t0k"}

	got, err := RawFile(context.Background(), rawClient(h), "github.com", "t0k", "o/private", "abc", "build.yaml")

	require.NoError(t, err)
	assert.Equal(t, "secret: 1", string(got))
	require.Len(t, h.seen, 2)
	assert.Empty(t, h.seen[0].Header.Get("Authorization"), "anonymous first")
	assert.Equal(t, "token t0k", h.seen[1].Header.Get("Authorization"))
}

func TestRawFileNotFound(t *testing.T) {
	h := &rawHost{}
	_, err := RawFile(context.Background(), rawClient(h), "github.com", "", "o/r", "abc", "nope.yaml")
	assert.ErrorIs(t, err, ErrFileNotFound)
	assert.Len(t, h.seen, 1, "without a token there is nothing to retry with")
}

func TestRawFileNotFoundEvenWithTheToken(t *testing.T) {
	h := &rawHost{}
	_, err := RawFile(context.Background(), rawClient(h), "github.com", "t0k", "o/r", "abc", "nope.yaml")
	assert.ErrorIs(t, err, ErrFileNotFound)
	assert.Len(t, h.seen, 2)
}

func TestRawFileOtherStatusesAreErrors(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusTooManyRequests, http.StatusForbidden} {
		h := &rawHost{status: status}
		_, err := RawFile(context.Background(), rawClient(h), "github.com", "t0k", "o/r", "abc", "a")
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrFileNotFound, status)
		assert.Contains(t, err.Error(), http.StatusText(status))
		assert.Len(t, h.seen, 1, "an error that is not a 404 is not retried")
	}
}

func TestRawFileUnsupportedHost(t *testing.T) {
	for _, host := range []string{"ghe.example.com", "acme.ghe.com", "github.localhost"} {
		_, err := RawFile(context.Background(), rawClient(&rawHost{}), host, "", "o/r", "abc", "a")
		assert.ErrorIs(t, err, ErrUnsupportedHost, host)
	}
}

func TestRawFileStopsOnACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := &rawHost{files: map[string]string{"/o/r/abc/a": "x"}}
	_, err := RawFile(ctx, rawClient(h), "github.com", "", "o/r", "abc", "a")
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRawFileIsBounded(t *testing.T) {
	h := &rawHost{files: map[string]string{"/o/r/abc/big": strings.Repeat("x", maxRawFile+10)}}
	_, err := RawFile(context.Background(), rawClient(h), "github.com", "", "o/r", "abc", "big")
	assert.ErrorContains(t, err, "larger than")
}
