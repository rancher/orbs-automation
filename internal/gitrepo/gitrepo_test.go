package gitrepo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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
	if err != nil {
		t.Fatal(err)
	}
	return &testRepo{t: t, dir: dir, repo: r}
}

// commit makes an empty-tree-change commit dated epoch+offset and returns it.
func (r *testRepo) commit(msg string, offset time.Duration) plumbing.Hash {
	r.t.Helper()
	wt, err := r.repo.Worktree()
	if err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.dir, "f.txt"), []byte(msg), 0o600); err != nil {
		r.t.Fatal(err)
	}
	if _, err := wt.Add("f.txt"); err != nil {
		r.t.Fatal(err)
	}
	h, err := wt.Commit(msg, &git.CommitOptions{Author: sig(offset), Committer: sig(offset)})
	if err != nil {
		r.t.Fatal(err)
	}
	return h
}

func (r *testRepo) lightweight(name string, at plumbing.Hash) {
	r.t.Helper()
	if _, err := r.repo.CreateTag(name, at, nil); err != nil {
		r.t.Fatal(err)
	}
}

func (r *testRepo) annotated(name string, at plumbing.Hash, offset time.Duration) {
	r.t.Helper()
	_, err := r.repo.CreateTag(name, at, &git.CreateTagOptions{Tagger: sig(offset), Message: name})
	if err != nil {
		r.t.Fatal(err)
	}
}

func gitBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

// gitTags is the oracle: what real git lists.
func gitTags(t *testing.T, dir string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "tag", "--sort=-creatordate").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(out))
}

func TestOpenRequiresGitDirectory(t *testing.T) {
	r := newRepo(t)
	if _, err := Open(r.dir); err != nil {
		t.Fatalf("Open(repo root) = %v", err)
	}

	sub := filepath.Join(r.dir, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(sub); !errors.Is(err, ErrNotRepository) {
		t.Errorf("Open(subdirectory) = %v, want ErrNotRepository", err)
	}
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotRepository) {
		t.Errorf("Open(empty dir) = %v, want ErrNotRepository", err)
	}

	// A worktree or submodule has a .git file, not a directory.
	file := t.TempDir()
	if err := os.WriteFile(filepath.Join(file, ".git"), []byte("gitdir: /elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(file); !errors.Is(err, ErrNotRepository) {
		t.Errorf("Open(.git file) = %v, want ErrNotRepository", err)
	}
}

func TestDiscoverFindsRepositoryUpward(t *testing.T) {
	r := newRepo(t)
	c := r.commit("only", 0)
	r.lightweight("main-1111111", c)
	sub := filepath.Join(r.dir, "a", "b")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	repo, err := Discover(sub)
	if err != nil {
		t.Fatalf("Discover(subdirectory) = %v", err)
	}
	got, err := repo.NewestFirst()
	if err != nil || !slices.Equal(got, []string{"main-1111111"}) {
		t.Errorf("NewestFirst = %v, %v", got, err)
	}

	if _, err := Discover(t.TempDir()); !errors.Is(err, ErrNotRepository) {
		t.Errorf("Discover(not a repository) = %v, want ErrNotRepository", err)
	}
}

func TestNewestFirst(t *testing.T) {
	tests := []struct {
		name  string
		build func(r *testRepo)
		want  []string
	}{
		{"no tags", func(r *testRepo) { r.commit("a", 0) }, nil},
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
			if err != nil {
				t.Fatal(err)
			}
			got, err := repo.NewestFirst()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("NewestFirst = %v, want %v", got, tt.want)
			}
			// Real git must agree.
			if _, err := exec.LookPath("git"); err == nil {
				if oracle := gitTags(t, r.dir); !slices.Equal(got, oracle) {
					t.Errorf("differs from git tag --sort=-creatordate: got %v, git %v", got, oracle)
				}
			}
		})
	}
}

func TestHead(t *testing.T) {
	gitBinary(t)
	r := newRepo(t)
	h := r.commit("first", 0)
	repo, err := Open(r.dir)
	if err != nil {
		t.Fatal(err)
	}

	ref, short, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if ref != "refs/heads/master" && ref != "refs/heads/main" {
		t.Errorf("ref = %q, want the default branch", ref)
	}
	if len(short) < 7 || !strings.HasPrefix(h.String(), short) {
		t.Errorf("short = %q, want a prefix (7+ chars) of %s", short, h)
	}

	// A named branch.
	if err := r.repo.Storer.SetReference(
		plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("release/v2.x"))); err != nil {
		t.Fatal(err)
	}
	if err := r.repo.Storer.SetReference(
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("release/v2.x"), h)); err != nil {
		t.Fatal(err)
	}
	if ref, _, err = repo.Head(); err != nil || ref != "refs/heads/release/v2.x" {
		t.Errorf("Head = %q, %v, want refs/heads/release/v2.x", ref, err)
	}
}

func TestHeadDetached(t *testing.T) {
	r := newRepo(t)
	h := r.commit("first", 0)
	if err := r.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, h)); err != nil {
		t.Fatal(err)
	}
	repo, err := Open(r.dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Head(); !errors.Is(err, ErrDetachedHead) {
		t.Errorf("Head on detached HEAD = %v, want ErrDetachedHead", err)
	}
}

func TestHeadNoCommits(t *testing.T) {
	gitBinary(t)
	r := newRepo(t)
	repo, err := Open(r.dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Head(); err == nil {
		t.Error("Head in a repository with no commits: want an error")
	}
}

func TestShallowClone(t *testing.T) {
	gitBinary(t)
	src := newRepo(t)
	src.commit("old", 0)
	head := src.commit("head", time.Hour)
	src.lightweight("main-1111111", head)

	clone := func(t *testing.T, extra ...string) *Repo {
		t.Helper()
		dst := filepath.Join(t.TempDir(), "clone")
		args := append([]string{"clone", "--quiet", "--depth", "1"}, extra...)
		args = append(args, "file://"+src.dir, dst)
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		repo, err := Open(dst)
		if err != nil {
			t.Fatal(err)
		}
		return repo
	}

	t.Run("with tags", func(t *testing.T) {
		repo := clone(t)
		got, err := repo.NewestFirst()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, []string{"main-1111111"}) {
			t.Errorf("NewestFirst = %v, want [main-1111111]", got)
		}
		if _, _, err := repo.Head(); err != nil {
			t.Errorf("Head: %v", err)
		}
	})

	t.Run("without tags", func(t *testing.T) {
		got, err := clone(t, "--no-tags").NewestFirst()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("NewestFirst = %v, want none", got)
		}
	})
}
