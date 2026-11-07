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
	"github.com/rancher/orbs-automation/internal/semver"
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

func (semverScheme) Valid(tag string) bool {
	_, ok := semver.Parse(tag)
	return ok
}

func (s semverScheme) Lines([]git.Tag) []string {
	var lines []string
	for _, l := range s.project.Release.Lines {
		lines = append(lines, l.Line)
	}
	return lines
}

func (s semverScheme) LineOf(tag string) string { return s.project.LineOf(tag) }

func (s semverScheme) Latest(tags []git.Tag, line string) git.Latest {
	if line != "" {
		series, ok := s.series(line)
		if !ok {
			return git.Latest{}
		}
		var inLine []git.Tag
		for _, t := range tags {
			if git.InSeries(t.Name, series) {
				inLine = append(inLine, t)
			}
		}
		tags = inLine
	}
	return git.LatestReleases(tags, s.project.Release.TagPattern)
}

// series is the version series of the project's line of that name.
func (s semverScheme) series(line string) (string, bool) {
	for _, l := range s.project.Release.Lines {
		if l.Line == line {
			return l.Series(), true
		}
	}
	return "", false
}

func (semverScheme) IsStable(tag string) bool { return !semver.Classify(tag).HasPrerelease }

func (semverScheme) Compare(a, b string) int {
	x, okA := semver.Parse(a)
	y, okB := semver.Parse(b)
	if !okA || !okB {
		return 0
	}
	return semver.Compare(x, y)
}
