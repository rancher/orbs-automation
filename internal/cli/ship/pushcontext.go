package ship

import (
	"context"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
	"github.com/rancher/orbs-automation/internal/platform/github"
)

// pushContext is ctx with the credentials a push is made with: in CI the token
// that opens the PRs, since a workflow has no SSH agent or credential helper; on
// a workstation ctx as it was, so the engineer's own credentials are used.
func pushContext(ctx context.Context, s clicfg.Settings, c github.Credentials) context.Context {
	if !s.CI || c.Token == "" {
		return ctx
	}
	return gitcli.WithToken(ctx, "https://"+c.Host+"/", c.Token)
}
