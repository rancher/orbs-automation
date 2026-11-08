package gitcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/execx"
)

// ErrNotFound is returned when there is no git binary on PATH.
var ErrNotFound = errors.New("git not found on PATH")

// configEnv are the variables that change which git config files git reads. The
// default environment of execx leaves them out, and Config forwards them so it
// answers for the git the user would run, not for a cleaner one.
var configEnv = []string{"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM"}

// runGit runs git with the default environment of execx, which does not carry
// GIT_DIR, tokens or the like into git.
func runGit(ctx context.Context, args ...string) (execx.Result, error) {
	return runGitWith(ctx, nil, args...)
}

// runGitWith is runGit plus the named variables, for those that gero has set.
func runGitWith(ctx context.Context, forward []string, args ...string) (execx.Result, error) {
	env := execx.DefaultEnv()
	for _, k := range forward {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return execx.New("git").Run(ctx, execx.Cmd{Name: "git", Args: args, Env: env})
}

// Version returns the first line of "git --version", for example
// "git version 2.51.0", or ErrNotFound when git is not installed. It is for
// gero doctor, to say which git the escape hatches in this package would run.
func Version(ctx context.Context) (string, error) {
	res, err := runGit(ctx, "--version")
	if err != nil {
		return "", wrap(err, "git --version")
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(res.Stdout)), "\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return "", errors.New("git --version printed nothing")
	}
	return line, nil
}

// Config returns what git itself would use for key (system, global and local
// config, includes and all), or "" when it is not set, for example "user.name".
// It returns ErrNotFound when git is not installed.
//
// Why not go-git: the question gero doctor asks is what the git binary would
// do, and go-git reads config its own way, without every include rule git has.
func Config(ctx context.Context, key string) (string, error) {
	res, err := runGitWith(ctx, configEnv, "config", "--get", key)
	var exit *execx.ExitError
	if errors.As(err, &exit) && exit.ExitCode == 1 {
		return "", nil // git config --get exits 1 for a key that is not set
	}
	if err != nil {
		return "", wrap(err, "git config --get "+key)
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}

// wrap names the command that failed, and turns a missing binary into
// ErrNotFound.
func wrap(err error, command string) error {
	if errors.Is(err, execx.ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", command, err)
}
