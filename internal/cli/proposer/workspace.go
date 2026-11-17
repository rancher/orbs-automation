package proposer

import (
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is the invocation's workspace with the answers a run that proposes changes needs
// about the engineer's clone of the consumer.
type Workspace struct{ *workspace.Workspace }

// CloneDir is the directory of the clone of slug, found as for every command.
// Without one the error says how to get one: ship works in the engineer's clone
// and never clones for itself.
func (w Workspace) CloneDir(slug string) (string, error) {
	rp, err := w.RepoPath(slug)
	if err != nil {
		return "", fmt.Errorf("%s: clone it under a repos.search directory, or set repos.paths.%s (%v)", slug, slug, err)
	}
	return rp.Path, nil
}

// PushSettings are the user's settings that decide where a branch for slug is
// pushed.
func (w Workspace) PushSettings(slug string) proposal.PushSettings {
	p := w.Workspace.PushSettings(slug)
	return proposal.PushSettings{
		UserRepo: p.UserRepo, UserDefault: p.UserDefault, Fork: p.Fork, Upstream: p.Upstream, GitHubUser: p.GitHubUser,
	}
}

// Remote is the name of the upstream remote of slug.
func (w Workspace) Remote(slug string) (string, error) {
	remote := w.UpstreamFor(slug)
	if remote == "" {
		return "", errors.New("no upstream remote is set for " + slug + ": set upstream in the user file, or repos.overrides." + slug + ".upstream")
	}
	return remote, nil
}
