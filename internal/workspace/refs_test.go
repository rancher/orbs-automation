package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/git"
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
