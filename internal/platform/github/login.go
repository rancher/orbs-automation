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

// ErrNotUserToken is returned by Login when GitHub refuses GET /user with 403,
// which is how a GitHub App installation token answers: it works, but it does
// not belong to a user. The refusal is unverified against a real installation
// token (docs/specs/EXECUTION_CONTEXT.md).
var ErrNotUserToken = errors.New("the token does not belong to a user")

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
		if isUnauthorized(err) {
			return "", fmt.Errorf("%w on %s", ErrBadCredentials, c.Host)
		}
		if isForbidden(err) {
			return "", fmt.Errorf("%w on %s", ErrNotUserToken, c.Host)
		}
		return "", fmt.Errorf("ask %s who the token belongs to: %w", c.Host, err)
	}
	return user.GetLogin(), nil
}

// isUnauthorized reports whether err is GitHub answering 401: the token was
// not accepted.
func isUnauthorized(err error) bool {
	var resp *gogithub.ErrorResponse
	return errors.As(err, &resp) && resp.Response != nil && resp.Response.StatusCode == http.StatusUnauthorized
}

// isForbidden reports whether err is GitHub answering 403 (and not a rate
// limit, which go-github reports as its own error type).
func isForbidden(err error) bool {
	var resp *gogithub.ErrorResponse
	return errors.As(err, &resp) && resp.Response != nil && resp.Response.StatusCode == http.StatusForbidden
}
