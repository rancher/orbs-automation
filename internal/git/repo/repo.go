// Package repo reads the facts commands need from a local repository: the
// current ref, the abbreviated commit and the tags by date. It uses go-git, and
// reaches for internal/git/gitcli only for what go-git cannot do. It is the
// only package that imports go-git.
package repo

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/rancher/orbs-automation/internal/git/gitcli"
)

var (
	// ErrNotRepository is returned when the directory has no .git directory.
	ErrNotRepository = errors.New("not a git repository: no .git directory here")
	// ErrDetachedHead is returned when HEAD is not a symbolic ref to a branch.
	ErrDetachedHead = errors.New("HEAD is detached")
)

// Repo is a repository opened in a directory.
type Repo struct {
	dir  string
	repo *git.Repository
}

// Open opens the repository in dir. Like the script, it requires a .git
// directory in dir itself: subdirectories, worktrees and submodules (where
// .git is a file) are not discovered.
func Open(dir string) (*Repo, error) {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil || !info.IsDir() {
		return nil, ErrNotRepository
	}
	r, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	return &Repo{dir: dir, repo: r}, nil
}

// Discover finds the repository from dir upward, as git itself does. The
// script relies on this when it only lists tags (the CI case), where
// subdirectories and worktrees work.
func Discover(dir string) (*Repo, error) {
	r, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true, EnableDotGitCommonDir: true})
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, ErrNotRepository
	}
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	return &Repo{dir: dir, repo: r}, nil
}

// Head returns the full ref HEAD points to, for example refs/heads/main, and
// the abbreviated commit it resolves to (git's own abbreviation, see gitcli).
// A detached HEAD is ErrDetachedHead.
func (r *Repo) Head() (ref, short string, err error) {
	head, err := r.repo.Reference(plumbing.HEAD, false)
	if err != nil {
		return "", "", fmt.Errorf("read HEAD: %w", err)
	}
	if head.Type() != plumbing.SymbolicReference {
		return "", "", ErrDetachedHead
	}
	short, err = gitcli.ShortCommit(r.dir)
	if err != nil {
		return "", "", err
	}
	return head.Target().String(), short, nil
}

type datedTag struct {
	ref  string // full ref name, the tie-breaker
	name string
	when int64
}

// NewestFirst lists tag names ordered like git tag --sort=-creatordate: by the
// tagger date for an annotated tag and the committer date for a lightweight
// one, newest first. Equal dates are ordered by ref name, ascending, as git
// does.
func (r *Repo) NewestFirst() ([]string, error) {
	iter, err := r.repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	var tags []datedTag
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		when, err := r.creatorDate(ref.Hash())
		if err != nil {
			return fmt.Errorf("tag %s: %w", ref.Name().Short(), err)
		}
		tags = append(tags, datedTag{ref: ref.Name().String(), name: ref.Name().Short(), when: when})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(tags, func(a, b datedTag) int {
		return cmp.Or(cmp.Compare(b.when, a.when), cmp.Compare(a.ref, b.ref))
	})
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.name
	}
	return names, nil
}

// creatorDate returns the Unix time git sorts a tag by: the tagger date when
// hash is a tag object, the committer date when it is a commit, else zero.
func (r *Repo) creatorDate(hash plumbing.Hash) (int64, error) {
	if tag, err := r.repo.TagObject(hash); err == nil {
		return tag.Tagger.When.Unix(), nil
	} else if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return 0, err
	}
	commit, err := r.repo.CommitObject(hash)
	switch {
	case err == nil:
		return commit.Committer.When.Unix(), nil
	case errors.Is(err, plumbing.ErrObjectNotFound):
		return 0, nil
	default:
		return 0, err
	}
}

// Remotes returns the URLs of each configured remote, keyed by remote name. A
// repository without remotes gives an empty map.
func (r *Repo) Remotes() (map[string][]string, error) {
	remotes, err := r.repo.Remotes()
	if err != nil {
		return nil, fmt.Errorf("read remotes: %w", err)
	}
	out := make(map[string][]string, len(remotes))
	for _, rm := range remotes {
		c := rm.Config()
		out[c.Name] = slices.Clone(c.URLs)
	}
	return out, nil
}
