package ship

import (
	"errors"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
)

var (
	// ErrNoRelease is returned when the argument names no release: a tag the
	// repository does not have, a line with no tags, or neither.
	ErrNoRelease = errors.New("no such release")
	// ErrPrerelease is returned for a prerelease the consumer does not accept
	// unless it is asked for.
	ErrPrerelease = errors.New("prerelease not accepted")
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
// prerelease tag needs prerelease unless the consumer accepts all releases.
func ResolveRelease(producer config.Project, tags []git.Tag, arg string, prerelease bool, accept string) (Release, error) {
	return Release{}, nil
}
