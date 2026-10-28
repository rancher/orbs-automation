package branchtags

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/git/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// serve returns an opener that hands out r and records how often it is called.
func serve(r Repository, err error, calls *int) opener {
	return func(string) (Repository, error) {
		*calls++
		return r, err
	}
}

// neverOpened fails the test if the command opens a repository this way.
func neverOpened(t *testing.T) opener {
	t.Helper()
	return func(string) (Repository, error) {
		t.Error("opened a repository where none is expected")
		return nil, errors.New("unexpected open")
	}
}

func TestResolveFromCI(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := NewMockRepository(ctrl)
	// Head is not expected: in CI the ref and commit come from the environment.
	r.EXPECT().NewestFirst().Return([]string{"main-1111111", "v1.0.0"}, nil)
	vars := map[string]string{"GITHUB_REF": "refs/heads/main", "GITHUB_SHA": "abcdef1234567890"}

	var discovered int
	got, err := resolve(env(vars), ".", neverOpened(t), serve(r, nil, &discovered))

	require.NoError(t, err)
	assert.Equal(t, git.BranchTags{BranchTag: "head", BranchStaticTag: "main-abcdef1", PrevStaticTag: "main-1111111"}, got)
	assert.Equal(t, 1, discovered)
}

func TestResolveFromCIWithoutRepository(t *testing.T) {
	vars := map[string]string{"GITHUB_REF": "refs/heads/release/v2.x", "GITHUB_SHA": "abcdef1234567"}

	var calls int
	got, err := resolve(env(vars), ".", neverOpened(t), serve(nil, repo.ErrNotRepository, &calls))

	require.NoError(t, err, "a missing repository is not an error in CI mode")
	assert.Equal(t, git.BranchTags{BranchTag: "v2.x-head", BranchStaticTag: "v2.x-head-abcdef1"}, got)
}

func TestResolveFromCIOtherOpenErrorIsReturned(t *testing.T) {
	boom := errors.New("boom")
	vars := map[string]string{"GITHUB_REF": "refs/heads/main", "GITHUB_SHA": "abcdef1"}

	var calls int
	_, err := resolve(env(vars), ".", neverOpened(t), serve(nil, boom, &calls))

	assert.ErrorIs(t, err, boom)
}

func TestResolveFromRepository(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"no CI variables": nil,
		"only the ref":    {"GITHUB_REF": "refs/heads/main"},
		"only the sha":    {"GITHUB_SHA": "ffffffffffff"},
		"empty values":    {"GITHUB_REF": "", "GITHUB_SHA": ""},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := NewMockRepository(ctrl)
			r.EXPECT().Head().Return("refs/heads/release/v2.x", "abcdef12", nil)
			r.EXPECT().NewestFirst().Return([]string{"v2.x-head-1111111"}, nil)

			var calls int
			got, err := resolve(env(vars), ".", serve(r, nil, &calls), neverOpened(t))

			require.NoError(t, err)
			// The local abbreviation is used as given, even past 7 characters.
			assert.Equal(t, git.BranchTags{
				BranchTag:       "v2.x-head",
				BranchStaticTag: "v2.x-head-abcdef12",
				PrevStaticTag:   "v2.x-head-1111111",
			}, got)
		})
	}
}

func TestResolveErrors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name  string
		vars  map[string]string
		setup func(r *MockRepository) // nil: opening fails with openErr instead
		open  error
		want  error
	}{
		{name: "not a repository", open: repo.ErrNotRepository, want: repo.ErrNotRepository},
		{
			name:  "detached HEAD",
			setup: func(r *MockRepository) { r.EXPECT().Head().Return("", "", repo.ErrDetachedHead) },
			want:  repo.ErrDetachedHead,
		},
		{
			name:  "unsupported local branch",
			setup: func(r *MockRepository) { r.EXPECT().Head().Return("refs/heads/feature/x", "abcdef1", nil) },
			want:  git.ErrUnsupportedRef,
		},
		{
			name: "tag listing fails",
			setup: func(r *MockRepository) {
				r.EXPECT().Head().Return("refs/heads/main", "abcdef1", nil)
				r.EXPECT().NewestFirst().Return(nil, boom)
			},
			want: boom,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Repository
			if tt.setup != nil {
				mock := NewMockRepository(gomock.NewController(t))
				tt.setup(mock)
				r = mock
			}

			var calls int
			_, err := resolve(env(tt.vars), ".", serve(r, tt.open, &calls), neverOpened(t))

			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestResolveUnsupportedCIRef(t *testing.T) {
	r := NewMockRepository(gomock.NewController(t))
	// An unsupported ref is rejected before any tag is listed.
	vars := map[string]string{"GITHUB_REF": "refs/pull/12/merge", "GITHUB_SHA": "abcdef1"}

	var calls int
	_, err := resolve(env(vars), ".", neverOpened(t), serve(r, nil, &calls))

	assert.ErrorIs(t, err, git.ErrUnsupportedRef)
}

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		res  git.BranchTags
		want string
	}{
		{
			"all set",
			git.BranchTags{BranchTag: "head", BranchStaticTag: "main-abcdef1", PrevStaticTag: "main-1111111"},
			"branch_tag=head\nbranch_static_tag=main-abcdef1\nprev_static_tag=main-1111111\n",
		},
		{
			"empty previous tag keeps its line",
			git.BranchTags{BranchTag: "head", BranchStaticTag: "main-abcdef1"},
			"branch_tag=head\nbranch_static_tag=main-abcdef1\nprev_static_tag=\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Render(tt.res))
		})
	}
}

// The tests below use real directories, so the real opener functions run.

// newMainRepo creates a repository whose branch is main, with one commit
// tagged main-1111111, and returns its directory and commit.
func newMainRepo(t *testing.T) (dir string, commit plumbing.Hash) {
	t.Helper()
	dir = t.TempDir()
	r, err := gogit.PlainInitWithOptions(dir, &gogit.PlainInitOptions{
		InitOptions: gogit.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	require.NoError(t, err)
	wt, err := r.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o600))
	_, err = wt.Add("f.txt")
	require.NoError(t, err)
	sig := &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}
	commit, err = wt.Commit("first", &gogit.CommitOptions{Author: sig, Committer: sig})
	require.NoError(t, err)
	_, err = r.CreateTag("main-1111111", commit, nil)
	require.NoError(t, err)
	return dir, commit
}

func TestResolveRealDirectoryNotARepository(t *testing.T) {
	dir := t.TempDir()

	t.Run("local mode requires a repository", func(t *testing.T) {
		_, err := Resolve(env(nil), dir)

		assert.ErrorIs(t, err, repo.ErrNotRepository)
	})

	t.Run("CI mode treats it as having no tags", func(t *testing.T) {
		vars := map[string]string{"GITHUB_REF": "refs/heads/main", "GITHUB_SHA": "abcdef1234567"}

		got, err := Resolve(env(vars), dir)

		require.NoError(t, err)
		assert.Equal(t, git.BranchTags{BranchTag: "head", BranchStaticTag: "main-abcdef1"}, got)
	})
}

func TestResolveRealRepository(t *testing.T) {
	dir, commit := newMainRepo(t)

	t.Run("CI mode finds tags from a subdirectory", func(t *testing.T) {
		sub := filepath.Join(dir, "a", "b")
		require.NoError(t, os.MkdirAll(sub, 0o750))
		vars := map[string]string{"GITHUB_REF": "refs/heads/main", "GITHUB_SHA": "abcdef1234567"}

		got, err := Resolve(env(vars), sub)

		require.NoError(t, err)
		assert.Equal(t, git.BranchTags{BranchTag: "head", BranchStaticTag: "main-abcdef1", PrevStaticTag: "main-1111111"}, got)
	})

	t.Run("local mode reads the ref and commit", func(t *testing.T) {
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git binary not available")
		}

		got, err := Resolve(env(nil), dir)

		require.NoError(t, err)
		assert.Equal(t, "head", got.BranchTag)
		assert.Equal(t, "main-1111111", got.PrevStaticTag)
		short := strings.TrimPrefix(got.BranchStaticTag, "main-")
		assert.GreaterOrEqual(t, len(short), 7)
		assert.True(t, strings.HasPrefix(commit.String(), short), "%q is not a prefix of %s", short, commit)
	})
}
