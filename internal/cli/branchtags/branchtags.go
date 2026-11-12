// Package branchtags holds the logic of gero repo build-tags, a port of the
// branch-tags.sh script. It is framework-free: cmd/ owns the cobra wiring.
package branchtags

import (
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/gero/buildtags"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/workspace"
)

//go:generate go tool mockgen -source=branchtags.go -destination=mock_repository_test.go -package=branchtags

// Repository is what the command needs from a local repository. It is defined
// here, where it is consumed, so tests can use a fake.
type Repository interface {
	buildtags.Tags
	// Head returns the full ref HEAD points to and its abbreviated commit.
	Head() (ref, short string, err error)
}

// opener opens the repository the workspace is in.
type opener func() (Repository, error)

// Resolve computes the branch tags for the current ref and commit. Both come
// from GITHUB_REF and GITHUB_SHA when both are set, else from the repository
// in the working directory cwd, which must have a .git directory there.
func Resolve(s clicfg.Settings, getenv func(string) string, cwd string) (buildtags.BranchTags, error) {
	ws := workspace.Open(s, cwd)
	return resolve(getenv, func() (Repository, error) {
		r, err := ws.CurrentAtRoot()
		if err != nil {
			return nil, err
		}
		return r, nil
	}, func() (Repository, error) {
		r, err := ws.Current()
		if err != nil {
			return nil, err
		}
		return r, nil
	})
}

func resolve(getenv func(string) string, open, discover opener) (buildtags.BranchTags, error) {
	if ref, sha := getenv("GITHUB_REF"), getenv("GITHUB_SHA"); ref != "" && sha != "" {
		return buildtags.ResolveBranchTags(ref, git.Short(sha), tagsFrom(discover))
	}

	r, err := open()
	if err != nil {
		return buildtags.BranchTags{}, err
	}
	ref, short, err := r.Head()
	if err != nil {
		return buildtags.BranchTags{}, err
	}
	return buildtags.ResolveBranchTags(ref, short, r)
}

// tagsFrom lists tags from the repository found from the working directory
// upward. Like the script, which only runs git tag here, a missing repository
// is not an error: there are simply no tags.
func tagsFrom(discover opener) buildtags.Tags {
	return lazyTags(func() ([]string, error) {
		r, err := discover()
		if errors.Is(err, workspace.ErrNotRepository) {
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
func Render(res buildtags.BranchTags) string {
	return fmt.Sprintf("branch_tag=%s\nbranch_static_tag=%s\nprev_static_tag=%s\n",
		res.BranchTag, res.BranchStaticTag, res.PrevStaticTag)
}
