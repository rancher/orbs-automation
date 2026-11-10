package ship

import (
	"errors"
	"fmt"
	"slices"

	"github.com/rancher/orbs-automation/internal/platform/git"
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
// of any kind. acceptAll says every holding takes every release (accept: all): an
// explicit prerelease tag needs prerelease unless they do. What a tag, a line and
// the latest are is the stream's tag scheme's to say (internal/tagscheme); slug is
// the producer's owner/name, for the messages.
func ResolveRelease(scheme tagscheme.Scheme, slug string, tags []git.Tag, arg string, prerelease, acceptAll bool) (Release, error) {
	var tag *git.Tag
	switch {
	case arg == "":
		latest := scheme.Latest(tags, "")
		if tag = newest(latest, prerelease); tag == nil {
			if pre := latest.Newest(); pre != nil {
				return Release{}, fmt.Errorf("%w: %s has no stable release, only %s; pass --prerelease to ship it, or name a tag", ErrNoRelease, slug, pre.Name)
			}
			return Release{}, fmt.Errorf("%w: %s has no release tags", ErrNoRelease, slug)
		}
	case scheme.Valid(arg):
		for i := range tags {
			if tags[i].Name == arg {
				tag = &tags[i]
			}
		}
		if tag == nil {
			return Release{}, fmt.Errorf("%w: %s has no tag %s", ErrNoRelease, slug, arg)
		}
		if !scheme.IsStable(arg) && !prerelease && !acceptAll {
			return Release{}, fmt.Errorf("%w: %s is a prerelease and the consumer takes stable releases only; pass --prerelease to ship it", ErrPrerelease, arg)
		}
	default:
		if !slices.Contains(scheme.Lines(tags), arg) {
			return Release{}, fmt.Errorf("%w: %q is neither a tag nor a release line of %s", ErrNoRelease, arg, slug)
		}
		latest := scheme.Latest(tags, arg)
		if tag = newest(latest, prerelease); tag == nil {
			if pre := latest.Newest(); pre != nil {
				return Release{}, fmt.Errorf("%w: %s has no stable release in line %s, only %s; pass --prerelease to ship it, or name a tag", ErrNoRelease, slug, arg, pre.Name)
			}
			return Release{}, fmt.Errorf("%w: %s has no tags in line %s", ErrNoRelease, slug, arg)
		}
	}
	return Release{Tag: tag.Name, Commit: tag.Commit, Line: scheme.LineOf(tag.Name), Stable: scheme.IsStable(tag.Name)}, nil
}

// newest picks the latest stable tag, or with prerelease the latest of any kind.
func newest(l git.Latest, prerelease bool) *git.Tag {
	if prerelease {
		return l.Newest()
	}
	return l.Stable
}
