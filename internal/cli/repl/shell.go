package repl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/execx"
)

// ShellFunc runs the user's shell, the program at path shell, in dir with env
// as its whole environment, and returns when it exits. Its exit status is not
// an error: only a failure to start it is.
type ShellFunc func(ctx context.Context, shell, dir string, env []string) error

// ShellOn returns the ShellFunc that attaches the shell to in, out and errOut.
// The terminal is handed over as it is, so the shell is interactive on it.
func ShellOn(in io.Reader, out, errOut io.Writer) ShellFunc {
	return func(ctx context.Context, shell, dir string, env []string) error {
		_, err := execx.Passthrough(ctx, execx.PassCmd{
			Path: shell, Dir: dir, Env: env, In: in, Out: out, Err: errOut,
		})
		return err
	}
}

// shellPath is the program $SHELL names in env: an absolute path to an
// executable file, or an error. Nothing is guessed in its place.
func shellPath(env []string) (string, error) {
	var path string
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "SHELL="); ok {
			path = v
		}
	}
	if path == "" {
		return "", errors.New("$SHELL is not set, so there is no shell to start: use your own shell instead")
	}
	info, err := os.Stat(path)
	if err != nil || !filepath.IsAbs(path) || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("$SHELL is %q, which is not an absolute path to an executable file", path)
	}
	return path, nil
}

// markedEnv is env with GERO_REPL=1 in place of any GERO_REPL it holds, in a
// slice of its own.
func markedEnv(env []string) []string {
	marked := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, EnvRepl+"=") {
			marked = append(marked, e)
		}
	}
	return append(marked, EnvRepl+"=1")
}
