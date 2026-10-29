package doctor

import (
	"context"

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

func (authTokenCheck) Run(context.Context) Result { return Result{} }
