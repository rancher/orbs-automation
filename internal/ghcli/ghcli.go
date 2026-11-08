// Package ghcli runs the gh binary, for the one thing gero needs from it
// directly: telling the user, in gero doctor, whether gh is installed and which
// one. Tokens are not fetched here: go-gh's pkg/auth runs "gh auth token" itself
// (internal/github). Like internal/platform/git/gitcli, it starts the binary through
// internal/platform/execx, the one package allowed to import os/exec (see .golangci.yml
// and DEPENDENCY_POLICY).
package ghcli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/execx"
)

// ErrNotFound is returned when no gh binary can be found.
var ErrNotFound = errors.New("gh not found")

// Locate returns the path of the gh binary that go-gh would run for a token:
// GH_PATH when it is set, otherwise gh on PATH. getenv is os.Getenv in
// production.
func Locate(getenv func(string) string) (string, error) {
	name := "gh"
	if p := getenv("GH_PATH"); p != "" {
		name = p
	}
	path, err := execx.New(name).Lookup(name)
	if err != nil && name != "gh" {
		return "", fmt.Errorf("%w: GH_PATH is set to %s, which is not an executable file", ErrNotFound, name)
	}
	if err != nil {
		return "", fmt.Errorf("%w on PATH", ErrNotFound)
	}
	return path, nil
}

// Version runs "gh --version" for the binary at path and returns its first line,
// for example "gh version 2.100.0 (2026-09-03)".
func Version(ctx context.Context, path string) (string, error) {
	res, err := execx.New(path).Run(ctx, execx.Cmd{Name: path, Args: []string{"--version"}})
	if err != nil {
		return "", fmt.Errorf("gh --version: %w", err)
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(res.Stdout)), "\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return "", errors.New("gh --version printed nothing")
	}
	return line, nil
}
