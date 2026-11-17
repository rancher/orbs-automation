// Package reposync brings what a project holds in line with config: for each holding that
// a project keeps and that config can say the value of, the value held is read at the head
// of the branch it is in and compared with what it should be, and what differs is
// corrected, branch by branch, in one proposal for each branch. It is a source of proposals
// for internal/proposal, which carries each out, as internal/ship is for a release; it
// decides what to propose and never how a PR is made (docs/specs/REPO_SYNC.md).
package reposync

import (
	"context"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/proposal"
)

// Status is where a check leaves a branch, and each row of it.
type Status string

// The states.
const (
	StatusReady    Status = "ready"
	StatusUpToDate Status = "already up to date"
	StatusBlocked  Status = "blocked"
	// StatusFailed is a row that could not be read: its branch or file could not be.
	// It is reported and the other rows go on.
	StatusFailed Status = "failed"
)

// Reader is what reading the state of a project needs from the invocation's workspace: the
// projects and the update engine, and what the host has. It is defined here, where it is
// consumed, so tests can use a fake.
type Reader interface {
	// Project returns the project of a config name or slug, or the one the working
	// directory is in when the key is empty.
	Project(key string) (*chain.Project, error)
	// Bump builds the update engine for the central config. It may return an engine along
	// with an error about edits or holdings that are not usable.
	Bump() (*bump.Engine, error)
	// UpstreamRefs lists the tags and branch heads of a repository on its host.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	// UpstreamFile reads a file of a repository at a commit from its host.
	UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error)
}

// Workspace is what a sync needs: what is read from the host, and the engineer's clone of
// the project and where to push.
type Workspace interface {
	Reader
	proposal.Site
}
