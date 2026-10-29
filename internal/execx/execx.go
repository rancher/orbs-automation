package execx

import (
	"context"
	"errors"
)

var (
	// ErrNotFound is returned when an allowed binary is not on PATH.
	ErrNotFound = errors.New("binary not found on PATH")
	// ErrNotAllowed is returned when a command names a binary the Runner was
	// not built with.
	ErrNotAllowed = errors.New("binary is not allowed")
)

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

func (e *ExitError) Error() string { return "" }

func (e *ExitError) Unwrap() error { return nil }

// DefaultEnv returns the part of gero's environment a program gets when Cmd.Env
// is nil: PATH, HOME, USER, LOGNAME, TMPDIR, TERM, LANG, and the LC_ and XDG_
// variables. Credentials and the rest, such as GH_TOKEN, are left out so a
// child process only sees what it was meant to.
func DefaultEnv() []string { return nil }

// InheritEnv returns the whole environment of the gero process, which is what
// os/exec gives a program when Cmd.Env is nil. It is the explicit opt-out of
// DefaultEnv: a caller that needs it says so, and that is easy to find in
// review.
func InheritEnv() []string { return nil }

// Runner runs the binaries it was built with.
type Runner struct {
	allow []string
}

// New returns a Runner that runs only the named binaries, each looked up on
// PATH once, the first time it is used.
func New(allow ...string) *Runner { return &Runner{allow: allow} }

// Lookup returns the absolute path of an allowed binary, ErrNotAllowed for a
// name the Runner was not built with, or ErrNotFound when it is not on PATH.
func (r *Runner) Lookup(_ string) (string, error) { return "", nil }

// Run runs c and returns what it wrote and how it exited. A non-zero exit is an
// *ExitError with the Result filled in; cancelling ctx kills the process.
func (r *Runner) Run(_ context.Context, _ Cmd) (Result, error) { return Result{}, nil }
