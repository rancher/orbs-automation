// Package branchtags holds the logic of gero repo branch-tags, a port of the
// branch-tags.sh script. It is framework-free: cmd/ owns the cobra wiring.
package branchtags

import (
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/git/repo"
)

// Repository is what the command needs from a local repository. It is defined
// here, where it is consumed, so tests can use a fake.
type Repository interface {
	git.Tags
	// Head returns the full ref HEAD points to and its abbreviated commit.
	Head() (ref, short string, err error)
}

// opener opens the repository in dir.
type opener func(dir string) (Repository, error)

// Resolve computes the branch tags for the current ref and commit. Both come
// from GITHUB_REF and GITHUB_SHA when both are set, else from the repository
// in dir, which must have a .git directory there.
func Resolve(getenv func(string) string, dir string) (git.BranchTags, error) {
	return resolve(getenv, dir, openStrict, openDiscover)
}

func openStrict(dir string) (Repository, error) {
	r, err := repo.Open(dir)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func openDiscover(dir string) (Repository, error) {
	r, err := repo.Discover(dir)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func resolve(getenv func(string) string, dir string, open, discover opener) (git.BranchTags, error) {
	if ref, sha := getenv("GITHUB_REF"), getenv("GITHUB_SHA"); ref != "" && sha != "" {
		return git.ResolveBranchTags(ref, git.Short(sha), tagsFrom(dir, discover))
	}

	r, err := open(dir)
	if err != nil {
		return git.BranchTags{}, err
	}
	ref, short, err := r.Head()
	if err != nil {
		return git.BranchTags{}, err
	}
	return git.ResolveBranchTags(ref, short, r)
}

// tagsFrom lists tags from the repository found from dir upward. Like the
// script, which only runs git tag here, a missing repository is not an error:
// there are simply no tags.
func tagsFrom(dir string, discover opener) git.Tags {
	return lazyTags(func() ([]string, error) {
		r, err := discover(dir)
		if errors.Is(err, repo.ErrNotRepository) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return r.NewestFirst()
	})
}

type lazyTags func() ([]string, error)

func (f lazyTags) NewestFirst() ([]string, error) { return f() }

// Render formats res as the script's three lowercase key=value lines.
func Render(res git.BranchTags) string {
	return fmt.Sprintf("branch_tag=%s\nbranch_static_tag=%s\nprev_static_tag=%s\n",
		res.BranchTag, res.BranchStaticTag, res.PrevStaticTag)
}
