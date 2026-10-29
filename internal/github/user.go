package github

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v93/github"
)

// UserID returns the numeric id of the GitHub user with the given login, from
// the public GET /users/<login>. A bot's commit email needs it.
func UserID(ctx context.Context, c Credentials, login string, opts ...gogithub.ClientOptionsFunc) (int64, error) {
	client, err := NewClient(c, opts...)
	if err != nil {
		return 0, err
	}
	user, _, err := client.Users.Get(ctx, login)
	if err != nil {
		return 0, fmt.Errorf("look up GitHub user %q on %s: %w", login, c.Host, err)
	}
	return user.GetID(), nil
}
