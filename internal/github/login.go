package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	gogithub "github.com/google/go-github/v93/github"
)

// ErrBadCredentials is returned when GitHub rejects the token.
var ErrBadCredentials = errors.New("GitHub rejected the token")

// Login returns the login of the user the token belongs to, by asking the
// host's API who is calling. It is the one cheap call that proves a token
// works.
func Login(ctx context.Context, c Credentials, opts ...gogithub.ClientOptionsFunc) (string, error) {
	client, err := NewClient(c, opts...)
	if err != nil {
		return "", err
	}
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		var resp *gogithub.ErrorResponse
		if errors.As(err, &resp) && resp.Response != nil && resp.Response.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf("%w on %s", ErrBadCredentials, c.Host)
		}
		return "", fmt.Errorf("ask %s who the token belongs to: %w", c.Host, err)
	}
	return user.GetLogin(), nil
}
