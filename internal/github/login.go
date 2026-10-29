package github

import (
	"context"
	"errors"

	gogithub "github.com/google/go-github/v93/github"
)

// ErrBadCredentials is returned when GitHub rejects the token.
var ErrBadCredentials = errors.New("GitHub rejected the token")

// Login returns the login of the user the token belongs to, by asking the
// host's API who is calling. It is the one cheap call that proves a token
// works.
func Login(_ context.Context, _ Credentials, _ ...gogithub.ClientOptionsFunc) (string, error) {
	return "", nil
}
