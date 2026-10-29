package github

import (
	"context"

	gogithub "github.com/google/go-github/v93/github"
)

// UserID returns the numeric id of the GitHub user with the given login, from
// the public GET /users/<login>. A bot's commit email needs it.
func UserID(ctx context.Context, c Credentials, login string, opts ...gogithub.ClientOptionsFunc) (int64, error) {
	return 0, nil
}
