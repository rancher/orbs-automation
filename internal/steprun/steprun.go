// Package steprun runs the steps of an update (the pre and post edit commands of
// a method or an edge) for the update dispatcher. The dispatcher decides which
// steps run and in what order; this package decides how one runs: which program,
// with what environment, in which directory.
//
// A step is a program and its arguments, run directly and never through a shell.
// A bare name such as `go` is a tool found on the PATH. A name with a slash is a
// script in a repository, resolved against the directory its `from` names and
// never outside it. Config declares what runs, so each step is allowed exactly
// the one program it names, and the program gets a scrubbed environment: the
// basics execx passes on, plus the GERO_ variables of the call.
package steprun

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/execx"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// The places a step's program can come from. An empty From is the consumer.
const (
	FromConsumer = "consumer"
	FromProducer = "producer"
	FromCentral  = "central"
)

var (
	// ErrEmptyCommand is returned for a step with no program.
	ErrEmptyCommand = errors.New("the step has no command")
	// ErrFromUnsupported is returned for a `from` that is not built yet.
	ErrFromUnsupported = errors.New("a step from this place is not supported yet")
	// ErrOutsideClone is returned for a script path that leaves the directory it
	// is resolved against.
	ErrOutsideClone = errors.New("the step's program is outside the repository")
)

// Runner runs steps. The zero value is ready to use.
type Runner struct{}

var _ updatedispatcher.StepRunner = Runner{}

// Run runs one step in env.Dir. A step that exits non-zero is an error that ends
// with the last of what it wrote to stderr, so the cause shows without a log.
func (Runner) Run(ctx context.Context, step config.Step, env updatedispatcher.StepEnv) error {
	if len(step.Run) == 0 || step.Run[0] == "" {
		return ErrEmptyCommand
	}
	switch step.From {
	case "", FromConsumer:
	case FromProducer, FromCentral:
		return fmt.Errorf("%w: from %s", ErrFromUnsupported, step.From)
	default:
		return fmt.Errorf("%w: from %q", ErrFromUnsupported, step.From)
	}
	program, err := program(step.Run[0], env.Dir)
	if err != nil {
		return err
	}
	// One program per step is allowed: the one config named.
	res, err := execx.New(program).Run(ctx, execx.Cmd{
		Name: program, Args: step.Run[1:], Dir: env.Dir, Env: environment(env.Env),
	})
	if err != nil {
		if tail := lastLines(string(res.Stderr), 5); tail != "" {
			return fmt.Errorf("%w: %s", err, tail)
		}
		return err
	}
	return nil
}

// program returns what to run for name: a bare name as it is, a path as an
// absolute path inside dir.
func program(name, dir string) (string, error) {
	if !strings.ContainsRune(name, '/') {
		return name, nil
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("%w: %s is an absolute path", ErrOutsideClone, name)
	}
	abs := filepath.Join(dir, name)
	rel, err := filepath.Rel(dir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrOutsideClone, name)
	}
	// A symlink in the repository must not lead out of it either.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		root, rerr := filepath.EvalSymlinks(dir)
		if rerr == nil {
			if r, err := filepath.Rel(root, resolved); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("%w: %s resolves outside it", ErrOutsideClone, name)
			}
		}
	}
	return abs, nil
}

// environment is the default environment with the call's variables added, in a
// stable order.
func environment(vars map[string]string) []string {
	env := execx.DefaultEnv()
	for _, k := range slices.Sorted(maps.Keys(vars)) {
		env = append(env, k+"="+vars[k])
	}
	return env
}

// lastLines returns the last n non-empty lines of s, joined by "; ".
func lastLines(s string, n int) string {
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
