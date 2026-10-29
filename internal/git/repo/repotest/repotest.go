// Package repotest builds real repositories for tests in packages that read
// one. It lives under internal/git/repo because it is the only place besides
// that package allowed to import go-git. It uses only the testing package, so
// it stays inside the dependency allow-list.
package repotest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// NewMainRepo creates a repository whose branch is main, with one commit
// tagged main-1111111, and returns its directory and commit.
func NewMainRepo(t *testing.T) (dir string, commit plumbing.Hash) {
	t.Helper()
	dir = t.TempDir()
	r, err := gogit.PlainInitWithOptions(dir, &gogit.PlainInitOptions{
		InitOptions: gogit.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	check(t, err)
	wt, err := r.Worktree()
	check(t, err)
	check(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o600))
	_, err = wt.Add("f.txt")
	check(t, err)
	sig := &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}
	commit, err = wt.Commit("first", &gogit.CommitOptions{Author: sig, Committer: sig})
	check(t, err)
	_, err = r.CreateTag("main-1111111", commit, nil)
	check(t, err)
	return dir, commit
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// NewRepoAt initializes an empty repository in dir, creating dir if needed,
// with the given remotes (name to URL) and no commits. It stands in for a
// developer's clone when a test only needs a repository with remotes.
func NewRepoAt(t *testing.T, dir string, remotes map[string]string) {
	t.Helper()
	check(t, os.MkdirAll(dir, 0o750))
	r, err := gogit.PlainInit(dir, false)
	check(t, err)
	for name, url := range remotes {
		_, err := r.CreateRemote(&config.RemoteConfig{Name: name, URLs: []string{url}})
		check(t, err)
	}
}
