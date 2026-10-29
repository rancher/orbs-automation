package doctor

import (
	"context"
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/cli/authstatus"
	"github.com/rancher/orbs-automation/internal/github"
)

// authTokenCheck finds the GitHub token gero would use, without calling GitHub:
// it asks the same resolver every command uses, so the answer is the one a
// command would act on. It reports where the token came from, never the token.
type authTokenCheck struct{ auth github.Auth }

// NewAuthTokenCheck returns the check named "auth-token".
func NewAuthTokenCheck(a github.Auth) Check { return authTokenCheck{auth: a} }

func (authTokenCheck) Name() string     { return "auth-token" }
func (authTokenCheck) Category() string { return "authentication" }
func (authTokenCheck) Description() string {
	return "Finds the GitHub token gero would use, without calling GitHub"
}
func (authTokenCheck) Online() bool    { return false }
func (authTokenCheck) Needs() []string { return nil }

func (c authTokenCheck) Run(context.Context) Result {
	creds, err := github.Resolve(c.auth, "")
	var noToken *github.NoTokenError
	switch {
	case errors.As(err, &noToken):
		return Result{Status: StatusFail, Message: "no token found for " + noToken.Host, Remediation: noToken.Hint}
	case err != nil:
		return Result{Status: StatusFail, Message: err.Error()}
	}
	return Result{Status: StatusOK, Message: fmt.Sprintf("%s for %s", github.DescribeTokenSource(creds.TokenSource), creds.Host)}
}

// TokenVerifier asks GitHub whether the token gero would use works. It is
// defined here, where it is consumed, so tests can use a fake.
type TokenVerifier interface {
	// Verify returns who the token belongs to. A rejected token comes back as a
	// Status with Valid false and an error wrapping github.ErrBadCredentials.
	Verify(ctx context.Context) (authstatus.Status, error)
}

// authAPICheck asks GitHub who the token belongs to, which is the one cheap
// call that proves it works. It is online and needs auth-token.
type authAPICheck struct{ verifier TokenVerifier }

// NewAuthAPICheck returns the check named "auth-api".
func NewAuthAPICheck(v TokenVerifier) Check { return authAPICheck{verifier: v} }

func (authAPICheck) Name() string     { return "auth-api" }
func (authAPICheck) Category() string { return "authentication" }
func (authAPICheck) Description() string {
	return "Asks GitHub who the token belongs to, to prove it works"
}
func (authAPICheck) Online() bool    { return true }
func (authAPICheck) Needs() []string { return []string{"auth-token"} }

func (authAPICheck) Run(context.Context) Result { return Result{} }

// authstatusVerifier is the TokenVerifier backed by gero auth status.
type authstatusVerifier struct{}

func (authstatusVerifier) Verify(ctx context.Context) (authstatus.Status, error) {
	return authstatus.Check(ctx, "")
}
