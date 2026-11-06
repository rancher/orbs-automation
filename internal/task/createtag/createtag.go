// Package createtag is the task that tags a commit of a project's release line
// on the host: one action, run alone or as a step of a longer process
// (docs/specs/PROJECT_LAYOUT.md, Tasks). Which tag, and on which commit, is the
// caller's to say; the task checks that it is a tag the line can have, and never
// moves or replaces one.
package createtag

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/git"
)

var (
	// ErrNotAVersion is returned for a tag that is not a SemVer version.
	ErrNotAVersion = errors.New("the tag is not a version")
	// ErrPattern is returned for a tag the project's tagPattern does not match.
	ErrPattern = errors.New("the tag does not match the project's tag pattern")
	// ErrNotInSeries is returned for a version outside the series of the line.
	ErrNotInSeries = errors.New("the tag is not in the line's series")
	// ErrNotNewer is returned for a version that is not newer than the newest tag
	// the line already has.
	ErrNotNewer = errors.New("the tag is not newer than the line's newest")
	// ErrTagExists is returned when the tag is already there, on another commit. A
	// tag is never moved.
	ErrTagExists = errors.New("the tag already exists on another commit")
)

// Input says which tag to make, on what, and what the repository holds already.
type Input struct {
	Repo   string // owner/name
	Tag    string
	Commit string // the commit to tag: the head of the line's branch, read from the host
	// Tags are the repository's tags as the host has them, with the commit each
	// one is on (an annotated tag at the commit it peels to).
	Tags    []git.Tag
	Pattern string // the project's tagPattern, such as "v*"; "" matches every tag
	Series  string // the line's series, such as "0.8.x" or "v2.15"; "" for no series
}

// Action is what making the tag comes to.
type Action string

// The actions.
const (
	// Create: the tag does not exist and will be made.
	Create Action = "create"
	// Already: the tag exists on this very commit, so there is nothing to do.
	Already Action = "already"
)

// Decision is what Check found.
type Decision struct {
	Action Action
	// Newest is the tag the new one follows in the line, when there is one.
	Newest *git.Tag
}

// Remote is what making the tag needs from GitHub. It is defined here, where it
// is used.
type Remote interface {
	Create(ctx context.Context, slug, name, commit string) error
}

// Check says what making the tag would do, or why it cannot be made, and reads
// nothing and writes nothing.
//
// STUB: always Create until a later commit.
func Check(in Input) (Decision, error) { return Decision{Action: Create}, nil }

// Run checks the tag and makes it. A tag that is already on the commit is not
// made again, and is not an error.
//
// STUB: makes nothing until a later commit.
func Run(ctx context.Context, r Remote, in Input) (Decision, error) {
	return Check(in)
}
