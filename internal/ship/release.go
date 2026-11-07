package ship

import (
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/semver"
)

var (
	// ErrNoRelease is returned when the argument names no release: a tag the
	// repository does not have, a line with no tags, or neither.
	ErrNoRelease = errors.New("no such release")
	// ErrPrerelease is returned for a prerelease the consumer does not accept
	// unless it is asked for.
	ErrPrerelease = errors.New("prerelease not accepted")
	// ErrLineMismatch is returned when a release is asked to go to a line that is
	// not the one it is in, for a producer whose releases follow the lines.
	ErrLineMismatch = errors.New("release is in another line")
)

// Release is the version of the producer that is being shipped.
type Release struct {
	Tag    string `json:"tag"`
	Commit string `json:"commit"`
	Line   string `json:"line,omitempty"` // the producer's line the tag is in, if any
	Stable bool   `json:"stable"`
}

// TODO(releasetag): ResolveRelease, latest and lineSeries below are release-line
// policy that is pure, as is internal/git/releasetag, which holds the rest of it
// (what a line's newest tag is, what it could have next, which tags it may not
// have). Fold them in there, with their tests, and let ship call it, so a line's
// "latest" has one definition for ship, repo tag and repo status. releasetag.Newest
// is the any-kind newest; the stable-or-prerelease choice here is its sibling.
// Do it as its own step, with the ship tests as the proof, since ErrNoRelease and
// the accept rule are ship's own to keep. See docs/specs/REPO_STATUS.md.

// ResolveRelease picks the release to ship from the producer's tags on the
// host. arg is a tag, used as given; a release line of the producer, which
// resolves to its latest tag; or empty, the producer's latest tag. A line or an
// empty arg resolves to the latest stable tag, or with prerelease to the latest
// of any kind. accept is the consumer's accept.when for this source: an explicit
// prerelease tag needs prerelease unless the consumer accepts all releases.
func ResolveRelease(producer config.Project, tags []git.Tag, arg string, prerelease bool, accept string) (Release, error) {
	pattern := producer.Release.TagPattern
	var tag *git.Tag
	switch _, isVersion := semver.Parse(arg); {
	case arg == "":
		tag = latest(tags, pattern, prerelease)
		if tag == nil {
			return Release{}, fmt.Errorf("%w: %s has no release tags", ErrNoRelease, producer.Repo.Name)
		}
	case isVersion:
		for i := range tags {
			if tags[i].Name == arg {
				tag = &tags[i]
			}
		}
		if tag == nil {
			return Release{}, fmt.Errorf("%w: %s has no tag %s", ErrNoRelease, producer.Repo.Name, arg)
		}
		if semverPre(arg) && !prerelease && accept != config.AcceptAll {
			return Release{}, fmt.Errorf("%w: %s is a prerelease and the consumer takes stable releases only; pass --prerelease to ship it", ErrPrerelease, arg)
		}
	default:
		series, ok := lineSeries(producer, arg)
		if !ok {
			return Release{}, fmt.Errorf("%w: %q is neither a tag nor a release line of %s", ErrNoRelease, arg, producer.Repo.Name)
		}
		var inLine []git.Tag
		for _, t := range tags {
			if git.InSeries(t.Name, series) {
				inLine = append(inLine, t)
			}
		}
		if tag = latest(inLine, pattern, prerelease); tag == nil {
			return Release{}, fmt.Errorf("%w: %s has no tags in line %s", ErrNoRelease, producer.Repo.Name, arg)
		}
	}
	return Release{Tag: tag.Name, Commit: tag.Commit, Line: producer.LineOf(tag.Name), Stable: !semverPre(tag.Name)}, nil
}

// latest picks the highest stable tag, or with prerelease the highest of any
// kind, among the tags that match the pattern.
func latest(tags []git.Tag, pattern string, prerelease bool) *git.Tag {
	l := git.LatestReleases(tags, pattern)
	if prerelease && l.Prerelease != nil {
		return l.Prerelease
	}
	return l.Stable
}

// lineSeries returns the series of the producer's line named line.
func lineSeries(p config.Project, line string) (string, bool) {
	for _, l := range p.Release.Lines {
		if l.Line == line {
			return l.Series(), true
		}
	}
	return "", false
}

func semverPre(tag string) bool { return semver.Classify(tag).HasPrerelease }
