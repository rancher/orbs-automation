// Package identity resolves who a command's commits are attributed to, wiring
// internal/actor to the real git config and the GitHub user lookup
// (docs/specs/EXECUTION_CONTEXT.md).
package identity

import (
	"context"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/actor"
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/git/gitcli"
	"github.com/rancher/orbs-automation/internal/github"
)

// Resolve returns the commit identity for the context in s: the engineer's git
// identity on a workstation, the bot's in CI.
func Resolve(ctx context.Context, s clicfg.Settings) (actor.Identity, error) {
	return resolve(ctx, s, github.GoGh{}, gitcli.Config)
}

func resolve(ctx context.Context, s clicfg.Settings, a github.Auth, git actor.ConfigFunc, opts ...gogithub.ClientOptionsFunc) (actor.Identity, error) {
	return actor.Resolve(ctx, actor.Inputs{
		CI:      s.CI,
		BotUser: s.BotUser,
		Git:     git,
		UserID: func(ctx context.Context, login string) (int64, error) {
			// Resolved here, not up front: a workstation never needs a token
			// for this, and CI without a bot login fails before asking for one.
			creds, err := github.Resolve(a, "")
			if err != nil {
				return 0, err
			}
			return github.UserID(ctx, creds, login, opts...)
		},
	})
}
