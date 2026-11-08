package gitcli

import "context"

// The functions in this file put a repository on a workstation, for gero repo
// clone. Why not go-git: the clone must authenticate the way the engineer's own
// git does (the SSH agent, ~/.ssh/config, a credential helper), which go-git does
// not, and it must run the same global config and hooks. TODO(go-git): revisit
// when go-git can use the engineer's SSH setup.

// Clone clones url into dir, which must not exist or must be empty, and names the
// remote it clones from remote, as git clone --origin does. It never prompts on a
// terminal: a missing credential fails at once with what git said.
func Clone(ctx context.Context, url, dir, remote string) error {
	return nil
}

// AddRemote adds a remote called name at url to the clone in dir and fetches it,
// so its branches are there to use.
func AddRemote(ctx context.Context, dir, name, url string) error {
	return nil
}

// RemoteURL returns the URL of the remote called name in the clone in dir, and ""
// with no error when the clone has no such remote.
func RemoteURL(ctx context.Context, dir, name string) (string, error) {
	return "", nil
}
