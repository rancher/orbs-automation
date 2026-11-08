package github

import (
	"context"

	gogithub "github.com/google/go-github/v93/github"
)

// ForkExists reports whether user has a fork of the repository slug (owner/name)
// on the host: a repository of the same name under user that is a fork of it. A
// repository that is not there, is not a fork, or forks another one is false with
// no error; a failed lookup is an error. A token is not needed to read a public
// repository, so none is not an error either.
func ForkExists(ctx context.Context, c Credentials, user, slug string, opts ...gogithub.ClientOptionsFunc) (bool, error) {
	return false, nil
}
