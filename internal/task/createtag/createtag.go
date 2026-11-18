// Package createtag is the task that tags a commit of a project's release line
// in the engineer's clone: one action, run alone or as a step of a longer process
// (docs/specs/PROJECT_LAYOUT.md, Tasks). Which tag, and on which commit, is the
// caller's to say; the task checks that it is a tag the line can have
// (internal/releasetag), and never moves or replaces one.
package createtag

import (
	"context"

	"github.com/rancher/orbs-automation/internal/releasetag"
)

// Annotation is what the tag says: its message, who made it, and whether it is
// signed.
type Annotation struct {
	Message string
	// Name and Email are the tagger, or empty for the clone's own git configuration.
	Name, Email string
	// Sign says whether the tag is signed with the engineer's own key. Nil leaves
	// it to git's own configuration (tag.gpgSign, tag.forceSignAnnotated).
	Sign *bool
}

// Input says which tag to make, on what, and what the repository holds already.
type Input struct {
	CloneDir   string // the clone the tag is made in
	Annotation Annotation
	releasetag.Input
}

// Git is what making the tag needs from git. It is defined here, where it is
// used.
type Git interface {
	// CreateTag makes the annotated tag name at commit in the clone in dir.
	CreateTag(ctx context.Context, dir, name, commit string, a Annotation) error
}

// Run checks the tag and makes it. A tag that is already on the commit is not
// made again, and is not an error.
func Run(ctx context.Context, g Git, in Input) (releasetag.Decision, error) {
	d, err := releasetag.Check(in.Input)
	if err != nil || d.Action == releasetag.Already {
		return d, err
	}
	if err := g.CreateTag(ctx, in.CloneDir, in.Tag, in.Commit, in.Annotation); err != nil {
		return releasetag.Decision{}, err
	}
	return d, nil
}
