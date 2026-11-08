package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	gogithub "github.com/google/go-github/v93/github"
)

// ForkExists reports whether user has a fork of the repository slug (owner/name)
// on the host: a repository of the same name under user that is a fork of it. A
// repository that is not there, is not a fork, or forks another one is false with
// no error; a failed lookup is an error. A token is not needed to read a public
// repository, so none is not an error on github.com.
func ForkExists(ctx context.Context, c Credentials, user, slug string, opts ...gogithub.ClientOptionsFunc) (bool, error) {
	_, name, _ := strings.Cut(slug, "/")
	client, err := forkClient(c, opts)
	if err != nil {
		return false, err
	}
	repo, resp, err := client.Repositories.Get(ctx, user, name)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("look up %s/%s on %s: %w", user, name, c.Host, err)
	}
	return repo.GetFork() && strings.EqualFold(repo.GetParent().GetFullName(), slug), nil
}

// forkClient is the client for the credentials, or an anonymous one on github.com
// when there is no token.
func forkClient(c Credentials, opts []gogithub.ClientOptionsFunc) (*gogithub.Client, error) {
	if c.Token == "" && (c.Host == "" || c.Host == "github.com") {
		return gogithub.NewClient(append([]gogithub.ClientOptionsFunc{gogithub.WithUserAgent(UserAgent())}, opts...)...)
	}
	return NewClient(c, opts...)
}
