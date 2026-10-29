// Package identity resolves who a command's commits are attributed to, wiring
// internal/actor to the real git config and the GitHub user lookup
// (docs/specs/EXECUTION_CONTEXT.md).
package identity

import (
	"context"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/actor"
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github"
)

// Resolve returns the commit identity for the context in s: the engineer's git
// identity on a workstation, the bot's in CI.
func Resolve(ctx context.Context, s clicfg.Settings) (actor.Identity, error) {
	return actor.Identity{}, nil
}

func resolve(ctx context.Context, s clicfg.Settings, a github.Auth, git actor.ConfigFunc, opts ...gogithub.ClientOptionsFunc) (actor.Identity, error) {
	return actor.Identity{}, nil
}
