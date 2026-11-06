package ship

import (
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/workspace"
)

// shipWorkspace is the invocation's workspace with the two answers ship needs
// about the engineer's clone of the consumer.
type shipWorkspace struct{ *workspace.Workspace }

// CloneDir is the directory of the clone of slug, found as for every command.
// Without one the error says how to get one: ship works in the engineer's clone
// and never clones for itself.
func (w shipWorkspace) CloneDir(slug string) (string, error) {
	rp, err := w.RepoPath(slug)
	if err != nil {
		return "", fmt.Errorf("%s: clone it under a repos.search directory, or set repos.paths.%s (%v)", slug, slug, err)
	}
	return rp.Path, nil
}

// Remote is the name of the upstream remote of slug.
func (w shipWorkspace) Remote(slug string) (string, error) {
	remote := w.UpstreamFor(slug)
	if remote == "" {
		return "", errors.New("no upstream remote is set for " + slug + ": set upstream in the user file, or repos.overrides." + slug + ".upstream")
	}
	return remote, nil
}
