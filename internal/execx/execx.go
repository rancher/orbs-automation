package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNotFound is returned when an allowed binary is not on PATH.
	ErrNotFound = errors.New("binary not found on PATH")
	// ErrNotAllowed is returned when a command names a binary the Runner was
	// not built with.
	ErrNotAllowed = errors.New("binary is not allowed")
)

// waitDelay bounds how long Run waits for output to drain after the process was
// killed, so a grandchild that still holds the pipes cannot hang a cancelled
// run.
const waitDelay = 2 * time.Second

// Cmd is one program to run: a name and an argument vector, never a shell
// string.
type Cmd struct {
	// Name is the binary to run. It must be one the Runner was built with.
	Name string
	Args []string
	// Dir is the working directory, or "" for the current one.
	Dir string
	// Env is the environment as "KEY=value" entries, exactly what the program
	// gets. Nil means DefaultEnv, not everything gero inherited. To add to the
	// default, append to DefaultEnv(); to get what os/exec would give, use
	// InheritEnv().
	Env []string
}

// Result is what a run produced, as raw bytes: nothing is trimmed or decoded.
type Result struct {
	Stdout, Stderr []byte
	// ExitCode is the process exit code, or -1 when it did not start or was
	// killed before it exited.
	ExitCode int
}

// ExitError is returned, with the Result, when a program ran and exited with a
// non-zero code. A program that could not start gives ErrNotFound instead.
type ExitError struct {
	Name     string
	ExitCode int
	Err      error
}

func (e *ExitError) Error() string { return fmt.Sprintf("%s: %v", e.Name, e.Err) }

func (e *ExitError) Unwrap() error { return e.Err }

// defaultEnvNames and defaultEnvPrefixes are what DefaultEnv passes on.
var (
	defaultEnvNames    = []string{"PATH", "HOME", "USER", "LOGNAME", "TMPDIR", "TERM", "LANG"}
	defaultEnvPrefixes = []string{"LC_", "XDG_"}
)

// DefaultEnv returns the part of gero's environment a program gets when Cmd.Env
// is nil: PATH, HOME, USER, LOGNAME, TMPDIR, TERM, LANG, and the LC_ and XDG_
// variables. Credentials and the rest, such as GH_TOKEN, are left out so a
// child process only sees what it was meant to.
func DefaultEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if slices.Contains(defaultEnvNames, key) || hasAnyPrefix(key, defaultEnvPrefixes) {
			env = append(env, kv)
		}
	}
	return env
}

func hasAnyPrefix(s string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(s, p) })
}

// InheritEnv returns the whole environment of the gero process, which is what
// os/exec gives a program when Cmd.Env is nil. It is the explicit opt-out of
// DefaultEnv: a caller that needs it says so, and that is easy to find in
// review.
func InheritEnv() []string { return os.Environ() }

// Runner runs the binaries it was built with.
type Runner struct {
	allow []string

	mu       sync.Mutex
	resolved map[string]string
}

// New returns a Runner that runs only the named binaries, each looked up on
// PATH once, the first time it is used.
func New(allow ...string) *Runner {
	return &Runner{allow: allow, resolved: map[string]string{}}
}

// Lookup returns the absolute path of an allowed binary, ErrNotAllowed for a
// name the Runner was not built with, or ErrNotFound when it is not on PATH. A
// path that was found is kept, so later calls return it without looking again.
func (r *Runner) Lookup(name string) (string, error) {
	if !slices.Contains(r.allow, name) {
		return "", fmt.Errorf("%w: %s", ErrNotAllowed, name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if path, ok := r.resolved[name]; ok {
		return path, nil
	}
	path, err := exec.LookPath(name)
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return "", fmt.Errorf("execx: look up %s: %w", name, err)
	}
	if path, err = filepath.Abs(path); err != nil {
		return "", fmt.Errorf("execx: look up %s: %w", name, err)
	}
	r.resolved[name] = path
	return path, nil
}

// Run runs c and returns what it wrote and how it exited. A non-zero exit is an
// *ExitError with the Result filled in; cancelling ctx kills the process.
func (r *Runner) Run(ctx context.Context, c Cmd) (Result, error) {
	failed := Result{ExitCode: -1}
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	path, err := r.Lookup(c.Name)
	if err != nil {
		return failed, err
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, path, c.Args...)
	cmd.Args = append([]string{c.Name}, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	if cmd.Env == nil {
		cmd.Env = DefaultEnv()
	}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = waitDelay

	err = cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if err == nil {
		return res, nil
	}
	res.ExitCode = -1
	if ctxErr := ctx.Err(); ctxErr != nil {
		return res, ctxErr
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		res.ExitCode = exit.ExitCode()
		return res, &ExitError{Name: c.Name, ExitCode: res.ExitCode, Err: err}
	}
	return res, fmt.Errorf("execx: run %s: %w", c.Name, err)
}
