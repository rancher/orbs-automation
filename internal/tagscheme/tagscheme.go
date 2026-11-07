// Package tagscheme is what every reader of a project's releases asks of its tags:
// which lines they carry, which line a tag is in, the newest of a line, whether a
// tag is stable and how two compare. A project's scheme follows from
// release.branching and release.tagScheme, so ship, repo status, repo tag and
// lines resolve call it and carry no scheme code of their own
// (docs/specs/CHAIN_TRACING.md). It does no I/O.
package tagscheme

import (
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
)

// Scheme answers the questions of one way of naming releases.
type Scheme interface {
	// Name is the release.tagScheme that selects it.
	Name() string
	// Valid reports whether a name is a release tag of the scheme at all.
	Valid(tag string) bool
	// Lines is the Rancher lines of the project, in order. Semver gives the lines
	// the config lists, whatever the tags; a scheme that finds lines in its tags
	// gives the ones the tags carry, newest first.
	Lines(tags []git.Tag) []string
	// LineOf is the line a tag is in, or "" when it is in none.
	LineOf(tag string) string
	// Latest is the newest stable tag of a line and the newest prerelease tag newer
	// than it, each nil when there is none. The line "" is every line. For a scheme
	// with stages a prerelease is a tag of the dev stage.
	Latest(tags []git.Tag, line string) git.Latest
	// IsStable reports whether a tag is not a prerelease.
	IsStable(tag string) bool
	// Compare orders two tags: negative when a is older, zero when they are equal
	// or either is not a release tag of the scheme.
	Compare(a, b string) int
}

// For returns the scheme of a project.
func For(p config.Project) (Scheme, error) {
	return semverScheme{project: p}, nil
}

// semverScheme is the scheme of a project with per-line branches, and the default:
// tags are versions, and a line is the version series the config gives it.
type semverScheme struct {
	project config.Project
}

func (semverScheme) Name() string { return config.TagSchemeSemver }

func (semverScheme) Valid(string) bool { return false }

func (semverScheme) Lines([]git.Tag) []string { return nil }

func (semverScheme) LineOf(string) string { return "" }

func (semverScheme) Latest([]git.Tag, string) git.Latest { return git.Latest{} }

func (semverScheme) IsStable(string) bool { return false }

func (semverScheme) Compare(_, _ string) int { return 0 }
