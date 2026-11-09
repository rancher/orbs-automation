package gitcli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/execx"
)

// The functions in this file put a repository on a workstation, for gero repo
// clone. Why not go-git: the clone must authenticate the way the engineer's own
// git does (the SSH agent, ~/.ssh/config, a credential helper), which go-git does
// not, and it must run the same global config and hooks. TODO(go-git): revisit
// when go-git can use the engineer's SSH setup.

// Clone clones url into dir, which must not exist or must be empty, and names the
// remote it clones from remote, as git clone --origin does. It never prompts on a
// terminal: a missing credential fails at once with what git said.
func Clone(ctx context.Context, url, dir, remote string) error {
	res, err := runAuthenticated(ctx, "clone", "--quiet", "--origin", remote, "--", url, dir)
	return said(err, res, "git clone "+url)
}

// AddRemote adds a remote called name at url to the clone in dir and fetches it,
// so its branches are there to use.
func AddRemote(ctx context.Context, dir, name, url string) error {
	res, err := runAuthenticated(ctx, "-C", dir, "remote", "add", "--fetch", name, url)
	return said(err, res, "git remote add "+name)
}

// RemoteURL returns the URL of the remote called name in the clone in dir, and ""
// with no error when the clone has no such remote.
func RemoteURL(ctx context.Context, dir, name string) (string, error) {
	out, err := inDir(ctx, dir, "remote", "get-url", name)
	if err != nil {
		// git remote get-url exits 2 for a remote that is not there.
		if exitCode(err) == 2 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// said turns a failed git run into an error that carries the tail of what git
// said, which is where the cause is.
func said(err error, res execx.Result, what string) error {
	if err == nil {
		return nil
	}
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		return wrap(err, what)
	}
	return fmt.Errorf("%s: %w: %s", what, err, tail(withoutHints(string(res.Stderr)), 4))
}
