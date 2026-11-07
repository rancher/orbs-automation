package gitcli

import "context"

// CreateTag makes the lightweight tag name at commit in the clone in dir:
// `git tag <name> <commit>`. It is a ref and nothing more, with no tagger, no
// message and no signature, and it moves no branch and nothing in the working
// tree. It fails when the tag exists, and never moves one.
//
// STUB: makes nothing until a later commit.
func CreateTag(ctx context.Context, dir, name, commit string) error { return nil }

// TagCommit returns the commit a local tag is on, an annotated tag at the commit
// it peels to, and false when there is no such tag.
//
// STUB: finds nothing until a later commit.
func TagCommit(ctx context.Context, dir, name string) (string, bool, error) { return "", false, nil }

// PushTag sends the tag name to remote, as `git push <remote> refs/tags/<name>`,
// never forced. A tag the remote has on another commit is rejected
// (ErrPushRejected), and one it has on this commit is nothing to do. The
// credentials and the environment are those of Push.
//
// STUB: sends nothing until a later commit.
func PushTag(ctx context.Context, dir, remote, name string) error { return nil }
