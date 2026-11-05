// Package repoinfo holds the logic of gero repo info: the central-config entry
// of one repository, its latest releases as the host reports them, and where it
// is cloned on this machine with the state of that clone. It is
// framework-free: cmd/ owns the cobra wiring.
package repoinfo

import (
	"context"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Info needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory when the key is empty.
	Project(key string) (config.Project, error)
	RepoPath(slug string) (workspace.RepoPath, error)
	// UpstreamTags lists the tags of slug as its host reports them.
	UpstreamTags(ctx context.Context, slug string) ([]git.Tag, error)
}

// Clone is what Info reads from a local clone.
type Clone interface {
	// Head returns the full ref HEAD points to and its abbreviated commit.
	Head() (ref, short string, err error)
}

// Opener opens the clone in dir.
type Opener func(dir string) (Clone, error)

// Options are the choices of one run.
type Options struct {
	// Offline skips everything that needs the network: the tags on the host.
	Offline bool
}

// Info is everything the command reports about one repository.
type Info struct {
	Slug          string        `json:"slug"`
	Owned         bool          `json:"owned"`
	DefaultBranch string        `json:"defaultBranch"`
	Release       ReleaseInfo   `json:"release"`
	Upstream      *UpstreamInfo `json:"upstream,omitempty"` // nil when offline
	Local         LocalInfo     `json:"local"`
}

// ReleaseInfo is the Project's release policy with defaults applied.
type ReleaseInfo struct {
	Branches           string     `json:"branches"`
	TagPattern         string     `json:"tagPattern"`
	RequiresPrerelease bool       `json:"requiresPrerelease"`
	Lines              []LineInfo `json:"lines,omitempty"`
}

// LineInfo maps one Rancher line to the Project's series and branch, with the
// latest tags of that series when they were read.
type LineInfo struct {
	Line   string `json:"line"`
	Series string `json:"series"`
	Branch string `json:"branch"`
	// IsDefaultBranch is set when the config names no branch, so Branch is the
	// Project's default branch.
	IsDefaultBranch  bool     `json:"isDefaultBranch,omitempty"`
	LatestStable     *TagInfo `json:"latestStable,omitempty"`
	LatestPrerelease *TagInfo `json:"latestPrerelease,omitempty"`
}

// TagInfo is a tag and the commit it points at.
type TagInfo struct {
	Tag    string `json:"tag"`
	Commit string `json:"commit"`
}

// UpstreamInfo is what the host reports. The latest tags are the highest by
// version among those matching the Project's tag pattern; a prerelease is
// there only when it is newer than the latest stable tag. Error is set instead
// when the host could not be read.
type UpstreamInfo struct {
	LatestStable     *TagInfo `json:"latestStable,omitempty"`
	LatestPrerelease *TagInfo `json:"latestPrerelease,omitempty"`
	Error            string   `json:"error,omitempty"`
}

// LocalInfo is the local clone and its state. Error is set instead of the
// state when the clone is missing, ambiguous or unreadable. It may be stale:
// it is what the clone had at its last fetch.
type LocalInfo struct {
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
	Branch string `json:"branch,omitempty"`
	Commit string `json:"commit,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Run reports on the Project named by slugs, or on the one whose repository
// contains the working directory cwd, and renders it as s.Output (text or
// json).
func Run(ctx context.Context, s clicfg.Settings, cwd string, slugs []string, opts Options) (string, error) {
	return "", nil
}

func gather(ctx context.Context, ws Workspace, open Opener, slugs []string, opts Options) (Info, error) {
	return Info{}, nil
}

func renderText(i Info) string { return "" }

func tagText(t *TagInfo) string { return "" }

func renderJSON(i Info) (string, error) { return "", nil }
