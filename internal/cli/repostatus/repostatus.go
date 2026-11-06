// Package repostatus holds the logic of gero repo status: where the releases of
// one repository stand, read from its host and compared with the local clone.
// Unlike repo info it reads the network. It is framework-free: cmd/ owns the
// cobra wiring.
package repostatus

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrIncomplete is returned by Run, after the report, when the host could not
// be read, so the report lacks what only the host knows.
var ErrIncomplete = errors.New("the host could not be read: the report is incomplete")

// Workspace is what Status needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory when the key is empty.
	Project(key string) (config.Project, error)
	RepoPath(slug string) (workspace.RepoPath, error)
	// UpstreamRefs lists the tags and branch heads of slug as its host reports
	// them.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
}

// Clone is what Status reads from a local clone.
type Clone interface {
	// BranchCommit returns the commit a local branch is at, and false when the
	// clone has no such branch.
	BranchCommit(branch string) (commit string, found bool, err error)
	// Count returns how many commits are reachable from to and not from from,
	// and false when the clone lacks either commit.
	Count(from, to string) (n int, known bool, err error)
}

// Opener opens the clone in dir.
type Opener func(dir string) (Clone, error)

// Status is everything the command reports about one repository.
type Status struct {
	Slug          string `json:"slug"`
	DefaultBranch string `json:"defaultBranch"`
	// Error is set when the host could not be read; Targets then have no tags
	// and no heads.
	Error string `json:"error,omitempty"`
	// Targets is a row per release line, or the one row of a project with a
	// single release branch.
	Targets []Target `json:"targets"`
	// Clone is why the comparisons with the local clone are missing, when
	// they are.
	Clone string `json:"clone,omitempty"`
}

// Target is one release branch and where its releases stand.
type Target struct {
	// Line and Series are set for a project with a branch per line.
	Line   string `json:"line,omitempty"`
	Series string `json:"series,omitempty"`
	Branch string `json:"branch"`
	// IsDefaultBranch is set when the config names no branch for the line, so
	// Branch is the Project's default branch.
	IsDefaultBranch bool `json:"isDefaultBranch,omitempty"`
	// Head is the commit at the head of Branch on the host, empty when the
	// host has no such branch.
	Head             string   `json:"head,omitempty"`
	LatestStable     *TagInfo `json:"latestStable,omitempty"`
	LatestPrerelease *TagInfo `json:"latestPrerelease,omitempty"`
	// Unreleased is the number of commits on Head since the latest tag (the
	// pre-release when there is a newer one), counted in the local clone. It is
	// nil when unknown: no tag, no head, or the clone lacks a commit.
	Unreleased *int `json:"unreleased,omitempty"`
	// Local is how the clone's branch compares with Head.
	Local string `json:"local,omitempty"`
}

// TagInfo is a tag and the commit it points at.
type TagInfo struct {
	Tag    string `json:"tag"`
	Commit string `json:"commit"`
}

// Run reports on the Project named by slugs, or on the one whose repository
// contains the working directory cwd, and renders it as s.Output (text or
// json). When the host could not be read it returns the report together with
// ErrIncomplete.
func Run(ctx context.Context, s clicfg.Settings, cwd string, slugs []string) (string, error) {
	return "", nil
}

func gather(ctx context.Context, ws Workspace, open Opener, slugs []string) (Status, error) {
	return Status{}, nil
}

func renderText(st Status) string { return "" }

func renderJSON(st Status) (string, error) { return "", nil }

// tagText renders a tag as "v2.15.3 (c4246d8)", or "-" for none.
func tagText(t *TagInfo) string { return "" }
