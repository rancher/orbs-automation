package ship

import (
	"context"
	"errors"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github"
	"github.com/rancher/orbs-automation/internal/github/pulls"
	domain "github.com/rancher/orbs-automation/internal/ship"
)

// githubRemote is the PR service with the one thing it lacks: who is running.
type githubRemote struct {
	*pulls.Service
	creds github.Credentials
	s     clicfg.Settings
	opts  []gogithub.ClientOptionsFunc // a transport, for tests
}

var _ domain.Remote = githubRemote{}

// newRemote returns the GitHub side of publishing for the credentials.
func newRemote(c github.Credentials, s clicfg.Settings, opts ...gogithub.ClientOptionsFunc) (githubRemote, error) {
	svc, err := pulls.New(c, opts...)
	if err != nil {
		return githubRemote{}, err
	}
	return githubRemote{Service: svc, creds: c, s: s, opts: opts}, nil
}

// Me is the login whose PRs are the actor's own: the bot's, named by
// GERO_BOT_USER, in CI, where the token is an installation's and has no user,
// and otherwise the one the token belongs to.
func (r githubRemote) Me(ctx context.Context) (string, error) {
	if r.s.CI {
		if r.s.BotUser == "" {
			return "", errors.New("in CI the bot's login is needed to tell which PRs are its own: set GERO_BOT_USER")
		}
		return r.s.BotUser, nil
	}
	return github.Login(ctx, r.creds, r.opts...)
}
