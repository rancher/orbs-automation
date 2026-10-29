package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var epoch = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

func sig(offset time.Duration) *object.Signature {
	return &object.Signature{Name: "t", Email: "t@example.com", When: epoch.Add(offset)}
}

type testRepo struct {
	t    *testing.T
	dir  string
	repo *git.Repository
}

func newRepo(t *testing.T) *testRepo {
	t.Helper()
	dir := t.TempDir()
	r, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	return &testRepo{t: t, dir: dir, repo: r}
}

// commit makes a commit dated epoch+offset and returns it.
func (r *testRepo) commit(msg string, offset time.Duration) plumbing.Hash {
	r.t.Helper()
	wt, err := r.repo.Worktree()
	require.NoError(r.t, err)
	require.NoError(r.t, os.WriteFile(filepath.Join(r.dir, "f.txt"), []byte(msg), 0o600))
	_, err = wt.Add("f.txt")
	require.NoError(r.t, err)
	h, err := wt.Commit(msg, &git.CommitOptions{Author: sig(offset), Committer: sig(offset)})
	require.NoError(r.t, err)
	return h
}

func (r *testRepo) lightweight(name string, at plumbing.Hash) {
	r.t.Helper()
	_, err := r.repo.CreateTag(name, at, nil)
	require.NoError(r.t, err)
}

func (r *testRepo) annotated(name string, at plumbing.Hash, offset time.Duration) {
	r.t.Helper()
	_, err := r.repo.CreateTag(name, at, &git.CreateTagOptions{Tagger: sig(offset), Message: name})
	require.NoError(r.t, err)
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

// gitTags is the oracle: what real git lists.
func gitTags(t *testing.T, dir string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "tag", "--sort=-creatordate").Output()
	require.NoError(t, err)
	return strings.Fields(string(out))
}

func TestOpenRequiresGitDirectory(t *testing.T) {
	r := newRepo(t)
	_, err := Open(r.dir)
	require.NoError(t, err, "the repository root")

	sub := filepath.Join(r.dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o750))
	_, err = Open(sub)
	assert.ErrorIs(t, err, ErrNotRepository, "a subdirectory")

	_, err = Open(t.TempDir())
	assert.ErrorIs(t, err, ErrNotRepository, "an empty directory")

	// A worktree or submodule has a .git file, not a directory.
	file := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(file, ".git"), []byte("gitdir: /elsewhere\n"), 0o600))
	_, err = Open(file)
	assert.ErrorIs(t, err, ErrNotRepository, "a .git file")
}

func TestDiscoverFindsRepositoryUpward(t *testing.T) {
	r := newRepo(t)
	r.lightweight("main-1111111", r.commit("only", 0))
	sub := filepath.Join(r.dir, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	repo, err := Discover(sub)
	require.NoError(t, err)
	got, err := repo.NewestFirst()
	require.NoError(t, err)
	assert.Equal(t, []string{"main-1111111"}, got)

	_, err = Discover(t.TempDir())
	assert.ErrorIs(t, err, ErrNotRepository)
}

func TestNewestFirst(t *testing.T) {
	tests := []struct {
		name  string
		build func(r *testRepo)
		want  []string
	}{
		{"no tags", func(r *testRepo) { r.commit("a", 0) }, []string{}},
		{
			"lightweight tags order by commit date",
			func(r *testRepo) {
				old := r.commit("old", 0)
				mid := r.commit("mid", time.Hour)
				recent := r.commit("recent", 2*time.Hour)
				r.lightweight("v-mid", mid)
				r.lightweight("v-recent", recent)
				r.lightweight("v-old", old)
			},
			[]string{"v-recent", "v-mid", "v-old"},
		},
		{
			"annotated tags order by tagger date, not commit date",
			func(r *testRepo) {
				c := r.commit("only", 0)
				r.annotated("a-early", c, time.Hour)
				r.annotated("a-late", c, 5*time.Hour)
				r.annotated("a-mid", c, 3*time.Hour)
			},
			[]string{"a-late", "a-mid", "a-early"},
		},
		{
			"annotated and lightweight are compared on their own dates",
			func(r *testRepo) {
				old := r.commit("old", 0)
				recent := r.commit("recent", 4*time.Hour)
				r.lightweight("light-recent", recent)
				r.annotated("ann-between", old, 2*time.Hour)
				r.annotated("ann-latest", old, 9*time.Hour)
			},
			[]string{"ann-latest", "light-recent", "ann-between"},
		},
		{
			"equal dates fall back to ref name ascending",
			func(r *testRepo) {
				c := r.commit("only", 0)
				for _, n := range []string{"main-ccc", "main-aaa", "main-bbb"} {
					r.lightweight(n, c)
				}
				r.annotated("v-zzz", c, 0)
				r.annotated("v-yyy", c, 0)
			},
			[]string{"main-aaa", "main-bbb", "main-ccc", "v-yyy", "v-zzz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRepo(t)
			tt.build(r)
			repo, err := Open(r.dir)
			require.NoError(t, err)

			got, err := repo.NewestFirst()

			require.NoError(t, err)
			assert.Equal(t, tt.want, nonNil(got))
			if _, err := exec.LookPath("git"); err == nil {
				assert.Equal(t, nonNil(gitTags(t, r.dir)), nonNil(got), "must match git tag --sort=-creatordate")
			}
		})
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func TestHead(t *testing.T) {
	requireGit(t)
	r := newRepo(t)
	h := r.commit("first", 0)
	repo, err := Open(r.dir)
	require.NoError(t, err)

	ref, short, err := repo.Head()
	require.NoError(t, err)
	assert.Contains(t, []string{"refs/heads/master", "refs/heads/main"}, ref, "the default branch")
	assert.GreaterOrEqual(t, len(short), 7)
	assert.True(t, strings.HasPrefix(h.String(), short), "%q is not a prefix of %s", short, h)

	// A named branch.
	branch := plumbing.NewBranchReferenceName("release/v2.x")
	require.NoError(t, r.repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, branch)))
	require.NoError(t, r.repo.Storer.SetReference(plumbing.NewHashReference(branch, h)))
	ref, _, err = repo.Head()
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/release/v2.x", ref)
}

func TestHeadDetached(t *testing.T) {
	r := newRepo(t)
	h := r.commit("first", 0)
	require.NoError(t, r.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, h)))
	repo, err := Open(r.dir)
	require.NoError(t, err)

	_, _, err = repo.Head()

	assert.ErrorIs(t, err, ErrDetachedHead)
}

func TestHeadNoCommits(t *testing.T) {
	requireGit(t)
	r := newRepo(t)
	repo, err := Open(r.dir)
	require.NoError(t, err)

	_, _, err = repo.Head()

	assert.Error(t, err, "a repository with no commits has no HEAD commit")
}

func TestShallowClone(t *testing.T) {
	requireGit(t)
	src := newRepo(t)
	src.commit("old", 0)
	src.lightweight("main-1111111", src.commit("head", time.Hour))

	clone := func(t *testing.T, extra ...string) *Repo {
		t.Helper()
		dst := filepath.Join(t.TempDir(), "clone")
		args := append([]string{"clone", "--quiet", "--depth", "1"}, extra...)
		args = append(args, "file://"+src.dir, dst)
		out, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, "git %v\n%s", args, out)
		repo, err := Open(dst)
		require.NoError(t, err)
		return repo
	}

	t.Run("with tags", func(t *testing.T) {
		repo := clone(t)

		got, err := repo.NewestFirst()
		require.NoError(t, err)
		assert.Equal(t, []string{"main-1111111"}, got)

		_, _, err = repo.Head()
		assert.NoError(t, err)
	})

	t.Run("without tags", func(t *testing.T) {
		got, err := clone(t, "--no-tags").NewestFirst()

		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestRemotes(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		repo, err := Open(newRepo(t).dir)
		require.NoError(t, err)

		got, err := repo.Remotes()

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("urls by remote name", func(t *testing.T) {
		r := newRepo(t)
		_, err := r.repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"git@github.com:danpock/rancher.git"}})
		require.NoError(t, err)
		_, err = r.repo.CreateRemote(&config.RemoteConfig{Name: "upstream", URLs: []string{
			"https://github.com/rancher/rancher.git", "https://mirror.example.com/rancher/rancher.git"}})
		require.NoError(t, err)
		repo, err := Open(r.dir)
		require.NoError(t, err)

		got, err := repo.Remotes()

		require.NoError(t, err)
		assert.Equal(t, map[string][]string{
			"origin":   {"git@github.com:danpock/rancher.git"},
			"upstream": {"https://github.com/rancher/rancher.git", "https://mirror.example.com/rancher/rancher.git"},
		}, got)
	})

	t.Run("agrees with git remote -v", func(t *testing.T) {
		requireGit(t)
		r := newRepo(t)
		for _, args := range [][]string{
			{"remote", "add", "origin", "git@github.com:danpock/rancher.git"},
			{"remote", "add", "upstream", "https://github.com/rancher/rancher.git"},
		} {
			out, err := exec.Command("git", append([]string{"-C", r.dir}, args...)...).CombinedOutput()
			require.NoError(t, err, "%s", out)
		}
		repo, err := Open(r.dir)
		require.NoError(t, err)

		got, err := repo.Remotes()

		require.NoError(t, err)
		assert.Equal(t, map[string][]string{
			"origin":   {"git@github.com:danpock/rancher.git"},
			"upstream": {"https://github.com/rancher/rancher.git"},
		}, got)
	})
}
