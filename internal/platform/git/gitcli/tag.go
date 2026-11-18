package gitcli

import (
	"context"
	"strings"
)

// Tag is what an annotated tag says about who made it and what it is.
type Tag struct {
	// Message is the whole tag message. Git needs one for every annotated tag.
	Message string
	// Name and Email are the tagger, or both empty for the git configuration of
	// the clone, as for Commit.
	Name, Email string
	// Sign says whether git signs the tag with the engineer's own configuration.
	// Nil leaves it to git: it signs when tag.gpgSign or tag.forceSignAnnotated is
	// set. True signs, and false never does, whatever the configuration says.
	Sign *bool
}

// CreateTag makes the annotated tag name at commit in the clone in dir:
// `git tag -m <message> <name> <commit>`. It moves no branch and nothing in the
// working tree. It fails when the tag exists, and never moves one.
func CreateTag(ctx context.Context, dir, name, commit string, t Tag) error {
	_, err := inDir(ctx, dir, "tag", "--no-sign", name, commit+"^{commit}")
	return err
}

// TagCommit returns the commit a local tag is on, an annotated tag at the commit
// it peels to, and false when there is no such tag.
func TagCommit(ctx context.Context, dir, name string) (string, bool, error) {
	out, err := inDir(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/tags/"+name+"^{commit}")
	if exitCode(err) == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(out), true, nil
}

// PushTag sends the tag name to remote, as `git push <remote> refs/tags/<name>`,
// never forced. A tag the remote has on another commit is rejected
// (ErrPushRejected), and one it has on this commit is nothing to do. The
// credentials and the environment are those of Push, which is the engineer's own
// on a workstation, so it is the engineer that GitHub sees, and a workflow that
// pushes a tag with its own token starts the workflows that run on tags.
func PushTag(ctx context.Context, dir, remote, name string) error {
	return push(ctx, dir, remote, name, "push", remote, "refs/tags/"+name)
}
