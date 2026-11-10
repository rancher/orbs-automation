package goreleases

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const list = `[
 {"version":"go1.27rc1","stable":false},
 {"version":"go1.26.10","stable":true},
 {"version":"go1.26.9","stable":true},
 {"version":"go1.26.2","stable":true},
 {"version":"go1.26rc1","stable":false},
 {"version":"go1.25.9","stable":true},
 {"version":"go1.20","stable":true},
 {"version":"go1.20.14","stable":true},
 {"version":"go1.260.1","stable":true}
]`

// roundTrip is a transport that answers every request from the function.
type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fake is a client that answers with the body and status, and counts the requests: a
// fake transport, so no port is opened.
func fake(body string, status int) (*http.Client, *atomic.Int32) {
	var calls atomic.Int32
	return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}, &calls
}

func TestNewestPatchIsTheHighestStablePatchOfTheMinor(t *testing.T) {
	client, _ := fake(list, http.StatusOK)
	c := Client{HTTP: client}

	for minor, want := range map[string]string{"1.26": "go1.26.10", "go1.26": "go1.26.10", "1.25": "go1.25.9", "1.20": "go1.20.14"} {
		got, err := c.NewestPatch(context.Background(), minor)

		require.NoError(t, err, minor)
		assert.Equal(t, want, got, minor)
	}
}

func TestNewestPatchIgnoresReleaseCandidatesAndOtherMinors(t *testing.T) {
	client, _ := fake(list, http.StatusOK)

	_, err := Client{HTTP: client}.NewestPatch(context.Background(), "1.27")

	require.ErrorIs(t, err, ErrNoRelease, "only a candidate of 1.27 is listed")
	assert.ErrorContains(t, err, "go1.27")
}

func TestNewestPatchOfAMinorTheListHasNoneOf(t *testing.T) {
	client, _ := fake(list, http.StatusOK)

	_, err := Client{HTTP: client}.NewestPatch(context.Background(), "1.99")

	require.ErrorIs(t, err, ErrNoRelease)
}

func TestNewestPatchSaysWhenTheListCannotBeRead(t *testing.T) {
	down, _ := fake("", http.StatusBadGateway)
	_, err := Client{HTTP: down}.NewestPatch(context.Background(), "1.26")
	require.Error(t, err)
	assert.ErrorContains(t, err, "502")

	junk, _ := fake("<html>", http.StatusOK)
	_, err = Client{HTTP: junk}.NewestPatch(context.Background(), "1.26")
	require.Error(t, err)
	assert.ErrorContains(t, err, "not JSON")

	unreachable := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) { return nil, errors.New("no route to host") })}
	_, err = Client{HTTP: unreachable}.NewestPatch(context.Background(), "1.26")
	require.Error(t, err)
	assert.ErrorContains(t, err, "read the list of Go releases")
}

func TestTheListIsKeptAndUsedWhileItIsFresh(t *testing.T) {
	client, calls := fake(list, http.StatusOK)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	c := Client{HTTP: client, CacheDir: t.TempDir(), Now: func() time.Time { return now }}

	first, err := c.NewestPatch(context.Background(), "1.26")
	require.NoError(t, err)
	second, err := c.NewestPatch(context.Background(), "1.25")
	require.NoError(t, err)

	assert.Equal(t, "go1.26.10", first)
	assert.Equal(t, "go1.25.9", second)
	assert.Equal(t, int32(1), calls.Load(), "the second answer came from the file")
}

func TestAStaleListIsReadAgain(t *testing.T) {
	client, calls := fake(list, http.StatusOK)
	now := time.Now()
	c := Client{HTTP: client, CacheDir: t.TempDir(), Now: func() time.Time { return now }}
	_, err := c.NewestPatch(context.Background(), "1.26")
	require.NoError(t, err)

	c.Now = func() time.Time { return now.Add(25 * time.Hour) }
	_, err = c.NewestPatch(context.Background(), "1.26")

	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load(), "a day on, the list is asked for again")
}

func TestACacheThatIsNotTheListIsReplacedByTheNetwork(t *testing.T) {
	client, calls := fake(list, http.StatusOK)
	dir := t.TempDir()
	c := Client{HTTP: client, CacheDir: dir}
	_, err := c.NewestPatch(context.Background(), "1.26")
	require.NoError(t, err)
	require.NoError(t, writeFile(dir, "go-releases.json", "not json"))

	got, err := c.NewestPatch(context.Background(), "1.26")

	require.NoError(t, err)
	assert.Equal(t, "go1.26.10", got)
	assert.Equal(t, int32(2), calls.Load())
}

func writeFile(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
}
