// Package gitcli is the escape hatch to the git binary. gero uses go-git for
// repository access (internal/git/repo) and runs git only for an operation
// go-git cannot do. Every function here records why. git is started through
// internal/platform/execx, the one package allowed to import os/exec (see .golangci.yml
// and DEPENDENCY_POLICY).
package gitcli

import (
	"context"
	"fmt"
	"strings"
)

// ShortCommit returns the abbreviated HEAD commit of the repository in dir, the
// same call the branch-tags script makes: git rev-parse --short HEAD.
//
// Why not go-git: git's abbreviation length grows with the repository's object
// count (7 characters minimum, 8 or more from roughly 16k objects) and
// lengthens on ambiguity, and go-git has no equivalent. The default output has
// to match git.
func ShortCommit(dir string) (string, error) {
	res, err := runGit(context.Background(), "-C", dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --short HEAD: %w", err)
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}
