// Package actor decides who commits are attributed to: the engineer's git
// identity on a workstation, the bot's in CI (docs/specs/EXECUTION_CONTEXT.md).
// It does no I/O itself: git config and the GitHub user lookup are injected.
package actor

import "context"

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

// Bot builds the bot's commit identity from its login: the name is the login
// and the email is <numeric id>+<login>@users.noreply.github.com.
func Bot(ctx context.Context, login string, userID UserIDFunc) (Identity, error) {
	return Identity{}, nil
}

// Workstation reads the engineer's identity from git config. It fails naming
// user.name and user.email when either is missing, and never invents one.
func Workstation(ctx context.Context, git ConfigFunc) (Identity, error) {
	return Identity{}, nil
}

// Resolve returns the identity commits carry in the given context. In CI the
// bot login is required.
func Resolve(ctx context.Context, in Inputs) (Identity, error) {
	return Identity{}, nil
}
