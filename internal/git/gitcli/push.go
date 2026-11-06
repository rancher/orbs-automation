package gitcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rancher/orbs-automation/internal/execx"
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
func Push(ctx context.Context, dir, remote, branch string) error {
	env := execx.DefaultEnv()
	for _, k := range authEnv {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	res, err := execx.New("git").Run(ctx, execx.Cmd{
		Name: "git", Args: []string{"-C", dir, "push", "--set-upstream", remote, branch}, Env: env,
	})
	if err == nil {
		return nil
	}
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		return wrap(err, "git push")
	}
	stderr := string(res.Stderr)
	said := tail(withoutHints(stderr), 4)
	if strings.Contains(stderr, "[rejected]") || strings.Contains(stderr, "non-fast-forward") {
		return fmt.Errorf("%w: %s", ErrPushRejected, said)
	}
	return fmt.Errorf("git push %s %s: %w: %s", remote, branch, err, said)
}

// withoutHints drops git's advice lines, so what is left is what happened and
// not how to fix it for someone typing the command.
func withoutHints(s string) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "hint:") {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

// tail returns the last n non-empty lines of git's output, joined by "; ".
func tail(s string, n int) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "; ")
}
