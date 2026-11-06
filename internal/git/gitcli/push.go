package gitcli

import (
	"context"
	"errors"
)

// ErrPushRejected is returned by Push when the remote refused the branch, such
// as when it already has commits this one does not. A push is never forced, so
// the engineer decides what to do about it.
var ErrPushRejected = errors.New("the remote rejected the push")

// authEnv are the variables a push needs to reach the engineer's credentials,
// which the default environment of execx leaves out: the SSH agent and the
// commands that stand in for ssh and for a password prompt. Nothing that is
// itself a secret (GH_TOKEN and the like) is passed on.
var authEnv = []string{
	"SSH_AUTH_SOCK", "GIT_SSH", "GIT_SSH_COMMAND", "GIT_ASKPASS", "SSH_ASKPASS",
	"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM",
}

// Push pushes branch to remote from the clone in dir and makes the remote's copy
// its upstream: the script's `git push -u $REMOTE $BRANCH`, with no refspec and,
// as there, never forced. The engineer's own credentials do the authenticating
// (the SSH agent, a credential helper, ~/.ssh/config), reached through the
// variables in authEnv. git is not allowed to prompt on a terminal, so a missing
// credential fails at once with git's message and does not hang a run that is
// showing progress.
//
// Why not go-git: it does not use the engineer's SSH agent and ~/.ssh/config the
// way git does, nor their credential helpers or the hooks of the repository.
//
// STUB: pushes nothing until a later commit.
func Push(ctx context.Context, dir, remote, branch string) error { return nil }
