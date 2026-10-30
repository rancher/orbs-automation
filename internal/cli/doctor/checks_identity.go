package doctor

import (
	"context"

	"github.com/rancher/orbs-automation/internal/actor"
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
	return ""
}
func (c commitIdentityCheck) Online() bool    { return false }
func (c commitIdentityCheck) Needs() []string { return nil }

func (c commitIdentityCheck) Run(ctx context.Context) Result {
	return Result{}
}
