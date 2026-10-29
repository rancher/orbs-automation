// Package actor decides who commits are attributed to: the engineer's git
// identity on a workstation, the bot's in CI (docs/specs/EXECUTION_CONTEXT.md).
// It does no I/O itself: git config and the GitHub user lookup are injected.
package actor

import (
	"context"
	"errors"
	"fmt"
)

// Credit is how gero names itself to people, for the "Created with" line in PR
// and issue text. The HTTP User-Agent is separate (internal/github.UserAgent).
const Credit = "gero (ORBS Team Automation)"

// Identity is the author and committer of a commit.
type Identity struct {
	Name  string
	Email string
}

// ConfigFunc returns what git would use for a config key such as "user.name",
// or "" when it is not set.
type ConfigFunc func(ctx context.Context, key string) (string, error)

// UserIDFunc returns the numeric id of a GitHub user, from GET /users/<login>.
type UserIDFunc func(ctx context.Context, login string) (int64, error)

// Inputs is everything Resolve needs.
type Inputs struct {
	CI      bool       // the CI context: the bot acts
	BotUser string     // GERO_BOT_USER, the bot's login
	Git     ConfigFunc // git config lookup, for a workstation
	UserID  UserIDFunc // GitHub user lookup, for the bot
}

// EnvBotUser is the variable that names the bot's login in CI.
const EnvBotUser = "GERO_BOT_USER"

// Bot builds the bot's commit identity from its login: the name is the login
// and the email is <numeric id>+<login>@users.noreply.github.com.
func Bot(ctx context.Context, login string, userID UserIDFunc) (Identity, error) {
	if login == "" {
		return Identity{}, fmt.Errorf("%s is required for a command that commits in CI: set it to the bot's login, for example rancher-scc-bot[bot]", EnvBotUser)
	}
	id, err := userID(ctx, login)
	if err != nil {
		return Identity{}, fmt.Errorf("look up the id of bot %q: %w", login, err)
	}
	return Identity{
		Name:  login,
		Email: fmt.Sprintf("%d+%s@users.noreply.github.com", id, login),
	}, nil
}

// Workstation reads the engineer's identity from git config. It fails naming
// user.name and user.email when either is missing, and never invents one.
func Workstation(ctx context.Context, git ConfigFunc) (Identity, error) {
	name, err := git(ctx, "user.name")
	if err != nil {
		return Identity{}, err
	}
	email, err := git(ctx, "user.email")
	if err != nil {
		return Identity{}, err
	}
	var missing []string
	if name == "" {
		missing = append(missing, "user.name")
	}
	if email == "" {
		missing = append(missing, "user.email")
	}
	if len(missing) > 0 {
		return Identity{}, errors.New("git has no " + joinAnd(missing) + `: set it with "git config --global ` + missing[0] + ` <value>"`)
	}
	return Identity{Name: name, Email: email}, nil
}

func joinAnd(keys []string) string {
	if len(keys) == 2 {
		return keys[0] + " or " + keys[1]
	}
	return keys[0]
}

// Resolve returns the identity commits carry in the given context. In CI the
// bot login is required.
func Resolve(ctx context.Context, in Inputs) (Identity, error) {
	if in.CI {
		return Bot(ctx, in.BotUser, in.UserID)
	}
	return Workstation(ctx, in.Git)
}
