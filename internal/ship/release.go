package ship

import (
	"errors"
	"fmt"
	"slices"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/tagscheme"
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

// ResolveRelease picks the release to ship from the producer's tags on the
// host. arg is a tag, used as given; a release line of the producer, which
// resolves to its latest tag; or empty, the producer's latest tag. A line or an
// empty arg resolves to the latest stable tag, or with prerelease to the latest
// of any kind. accept is the consumer's accept.when for this source: an explicit
// prerelease tag needs prerelease unless the consumer accepts all releases. What
// a tag, a line and the latest are is the producer's tag scheme's to say
// (internal/tagscheme).
func ResolveRelease(producer config.Project, tags []git.Tag, arg string, prerelease bool, accept string) (Release, error) {
	scheme, err := tagscheme.For(producer)
	if err != nil {
		return Release{}, err
	}
	var tag *git.Tag
	switch {
	case arg == "":
		tag = newest(scheme.Latest(tags, ""), prerelease)
		if tag == nil {
			return Release{}, fmt.Errorf("%w: %s has no release tags", ErrNoRelease, producer.Repo.Name)
		}
	case scheme.Valid(arg):
		for i := range tags {
			if tags[i].Name == arg {
				tag = &tags[i]
			}
		}
		if tag == nil {
			return Release{}, fmt.Errorf("%w: %s has no tag %s", ErrNoRelease, producer.Repo.Name, arg)
		}
		if !scheme.IsStable(arg) && !prerelease && accept != config.AcceptAll {
			return Release{}, fmt.Errorf("%w: %s is a prerelease and the consumer takes stable releases only; pass --prerelease to ship it", ErrPrerelease, arg)
		}
	default:
		if !slices.Contains(scheme.Lines(tags), arg) {
			return Release{}, fmt.Errorf("%w: %q is neither a tag nor a release line of %s", ErrNoRelease, arg, producer.Repo.Name)
		}
		if tag = newest(scheme.Latest(tags, arg), prerelease); tag == nil {
			return Release{}, fmt.Errorf("%w: %s has no tags in line %s", ErrNoRelease, producer.Repo.Name, arg)
		}
	}
	return Release{Tag: tag.Name, Commit: tag.Commit, Line: scheme.LineOf(tag.Name), Stable: scheme.IsStable(tag.Name)}, nil
}

// newest picks the latest stable tag, or with prerelease the latest of any kind.
func newest(l git.Latest, prerelease bool) *git.Tag {
	if prerelease && l.Prerelease != nil {
		return l.Prerelease
	}
	return l.Stable
}
