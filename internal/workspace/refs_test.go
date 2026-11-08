package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpstreamRefs(t *testing.T) {
	w := Open(settings(t, "", nil, nil), t.TempDir())
	var gotURL, gotToken string
	w.credentials = func() (string, string) { return "github.example", "secret" }
	want := git.Refs{
		Tags:     []git.Tag{{Name: "v1.0.0", Commit: "abc"}},
		Branches: []git.Branch{{Name: "main", Commit: "def"}},
	}
	w.remoteRefs = func(_ context.Context, url, token string) (git.Refs, error) {
		gotURL, gotToken = url, token
		return want, nil
	}

	got, err := w.UpstreamRefs(context.Background(), "rancher/rancher")

	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, "https://github.example/rancher/rancher.git", gotURL)
	assert.Equal(t, "secret", gotToken)
}

func TestUpstreamRefsReturnsTheReadError(t *testing.T) {
	w := Open(settings(t, "", nil, nil), t.TempDir())
	boom := errors.New("boom")
	w.credentials = func() (string, string) { return "github.com", "" }
	w.remoteRefs = func(context.Context, string, string) (git.Refs, error) { return git.Refs{}, boom }
	_, err := w.UpstreamRefs(context.Background(), "rancher/rancher")
	assert.ErrorIs(t, err, boom)
}

func TestUpstreamFile(t *testing.T) {
	w := Open(settings(t, "", nil, nil), t.TempDir())
	var gotHost, gotToken, gotSlug, gotRef, gotPath string
	w.credentials = func() (string, string) { return "github.com", "secret" }
	w.remoteFile = func(_ context.Context, host, token, slug, ref, path string) ([]byte, error) {
		gotHost, gotToken, gotSlug, gotRef, gotPath = host, token, slug, ref, path
		return []byte("key: value\n"), nil
	}

	got, err := w.UpstreamFile(context.Background(), "rancher/rancher", "abc123", "build.yaml")

	require.NoError(t, err)
	assert.Equal(t, "key: value\n", string(got))
	assert.Equal(t, []string{"github.com", "secret", "rancher/rancher", "abc123", "build.yaml"},
		[]string{gotHost, gotToken, gotSlug, gotRef, gotPath})
}

func TestUpstreamFileReturnsTheReadError(t *testing.T) {
	w := Open(settings(t, "", nil, nil), t.TempDir())
	boom := errors.New("boom")
	w.credentials = func() (string, string) { return "github.com", "" }
	w.remoteFile = func(context.Context, string, string, string, string, string) ([]byte, error) { return nil, boom }
	_, err := w.UpstreamFile(context.Background(), "o/r", "abc", "a")
	assert.ErrorIs(t, err, boom)
}
