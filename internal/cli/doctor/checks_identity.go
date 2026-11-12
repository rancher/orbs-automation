package doctor

import (
	"context"
	"fmt"

	"github.com/rancher/orbs-automation/internal/cli/identity"
	"github.com/rancher/orbs-automation/internal/gero/actor"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
)

// IdentityResolver resolves the commit identity for the current context. It is
// defined here, where it is consumed, so tests can use a fake.
type IdentityResolver interface {
	Resolve(ctx context.Context) (actor.Identity, error)
}

// commitIdentityCheck resolves who a commit would be attributed to.
type commitIdentityCheck struct {
	resolver IdentityResolver
	ci       bool
}

// NewCommitIdentityCheck returns the check named "commit-identity". ci is the
// context: in CI the bot's id is looked up on GitHub, so the check is online and
// needs auth-api; on a workstation it reads git config and stays offline.
func NewCommitIdentityCheck(r IdentityResolver, ci bool) Check {
	return commitIdentityCheck{resolver: r, ci: ci}
}

func (commitIdentityCheck) Name() string     { return "commit-identity" }
func (commitIdentityCheck) Category() string { return "authentication" }
func (commitIdentityCheck) Description() string {
	return "Resolves who a commit would be attributed to: your git identity, or the bot's in CI"
}
func (c commitIdentityCheck) Online() bool { return c.ci }
func (c commitIdentityCheck) Needs() []string {
	if c.ci {
		return []string{"auth-api"}
	}
	return nil
}

// Run warns, and does not fail, when no identity resolves: commands that do not
// commit work without one. The commands that do commit fail at the point of use.
func (c commitIdentityCheck) Run(ctx context.Context) Result {
	id, err := c.resolver.Resolve(ctx)
	if err != nil {
		return Result{
			Status:      StatusWarn,
			Message:     "no commit identity: " + err.Error(),
			Remediation: c.remediation(),
		}
	}
	return Result{Status: StatusOK, Message: fmt.Sprintf("commits as %s <%s>", id.Name, id.Email)}
}

func (c commitIdentityCheck) remediation() string {
	if c.ci {
		return "set " + actor.EnvBotUser + " to the bot's login, for example rancher-scc-bot[bot], next to its token"
	}
	return `set your identity with "git config --global user.name <name>" and "git config --global user.email <email>"`
}

// settingsResolver is the IdentityResolver backed by internal/cli/identity.
type settingsResolver struct{ s clicfg.Settings }

func (r settingsResolver) Resolve(ctx context.Context) (actor.Identity, error) {
	return identity.Resolve(ctx, r.s)
}
