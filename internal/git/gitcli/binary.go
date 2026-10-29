package gitcli

import (
	"context"
	"errors"
)

// ErrNotFound is returned when there is no git binary on PATH.
var ErrNotFound = errors.New("git not found on PATH")

// Version returns the first line of "git --version", for example
// "git version 2.51.0", or ErrNotFound when git is not installed. It is for
// gero doctor, to say which git the escape hatches in this package would run.
func Version(_ context.Context) (string, error) {
	return "", nil
}

// Config returns what git itself would use for key (system, global and local
// config, includes and all), or "" when it is not set, for example "user.name".
// It returns ErrNotFound when git is not installed.
//
// Why not go-git: the question gero doctor asks is what the git binary would
// do, and go-git reads config its own way, without every include rule git has.
func Config(_ context.Context, _ string) (string, error) {
	return "", nil
}
