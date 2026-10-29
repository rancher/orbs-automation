// Package ghcli runs the gh binary, for the one thing gero needs from it
// directly: telling the user, in gero doctor, whether gh is installed and which
// one. Tokens are not fetched here: go-gh's pkg/auth runs "gh auth token" itself
// (internal/github). Like internal/git/gitcli, this is a place allowed to
// import os/exec (see .golangci.yml and DEPENDENCY_POLICY).
package ghcli

import (
	"context"
	"errors"
)

// ErrNotFound is returned when no gh binary can be found.
var ErrNotFound = errors.New("gh not found")

// Locate returns the path of the gh binary that go-gh would run for a token:
// GH_PATH when it is set, otherwise gh on PATH. getenv is os.Getenv in
// production.
func Locate(_ func(string) string) (string, error) {
	return "", nil
}

// Version runs "gh --version" for the binary at path and returns its first line,
// for example "gh version 2.100.0 (2026-09-03)".
func Version(_ context.Context, _ string) (string, error) {
	return "", nil
}
