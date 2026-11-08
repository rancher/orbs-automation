package ship

import (
	"errors"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github/pulls"
	"github.com/rancher/orbs-automation/internal/platform/github"
	domain "github.com/rancher/orbs-automation/internal/ship"
)

// newRemote returns the GitHub side of publishing for the credentials. In CI the
// token is an installation's and has no user, so the bot is who is running, as the
// workflow names it in GERO_BOT_USER, and without it there is no telling which PRs
// are the bot's own.
func newRemote(c github.Credentials, s clicfg.Settings) (domain.Remote, error) {
	svc, err := pulls.New(c)
	if err != nil {
		return nil, err
	}
	if !s.CI {
		return svc, nil
	}
	if s.BotUser == "" {
		return nil, errors.New("in CI the bot's login is needed to tell which PRs are its own: set GERO_BOT_USER")
	}
	return svc.AsBot(s.BotUser), nil
}
