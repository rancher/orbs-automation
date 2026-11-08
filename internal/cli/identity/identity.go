// Package identity resolves who a command's commits are attributed to, wiring
// internal/actor to the real git config and the GitHub user lookup
// (docs/specs/EXECUTION_CONTEXT.md).
package identity

import (
	"context"

	"github.com/rancher/orbs-automation/internal/actor"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
	"github.com/rancher/orbs-automation/internal/platform/github"
)

// userLookup returns the numeric id of a GitHub user, using credentials c.
type userLookup func(ctx context.Context, c github.Credentials, login string) (int64, error)

// Resolve returns the commit identity for the context in s: the engineer's git
// identity on a workstation, the bot's in CI.
func Resolve(ctx context.Context, s clicfg.Settings) (actor.Identity, error) {
	return resolve(ctx, s, github.GoGh{}, gitcli.Config, func(ctx context.Context, c github.Credentials, login string) (int64, error) {
		return github.UserID(ctx, c, login)
	})
}

// ResolveIn is Resolve for commits made in the clone in dir: on a workstation the
// identity is what git reads there, so a user.email set in that clone alone is the
// one a commit made in it would carry. In CI it is the bot's, as for Resolve.
func ResolveIn(ctx context.Context, s clicfg.Settings, dir string) (actor.Identity, error) {
	git := func(ctx context.Context, key string) (string, error) { return gitcli.ConfigIn(ctx, dir, key) }
	return resolve(ctx, s, github.GoGh{}, git, func(ctx context.Context, c github.Credentials, login string) (int64, error) {
		return github.UserID(ctx, c, login)
	})
}

func resolve(ctx context.Context, s clicfg.Settings, a github.Auth, git actor.ConfigFunc, lookup userLookup) (actor.Identity, error) {
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
			return lookup(ctx, creds, login)
		},
	})
}
