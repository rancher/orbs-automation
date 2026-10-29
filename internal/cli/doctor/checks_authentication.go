package doctor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

func (c authAPICheck) Run(ctx context.Context) Result {
	st, err := c.verifier.Verify(ctx)
	switch {
	case err == nil:
		return Result{Status: StatusOK, Message: fmt.Sprintf("signed in as %s on %s", st.Login, st.Host)}
	case errors.Is(err, github.ErrBadCredentials):
		return Result{
			Status:      StatusFail,
			Message:     fmt.Sprintf("GitHub rejected the token from %s on %s", github.DescribeTokenSource(st.TokenSource), st.Host),
			Remediation: rejectedRemediation(st),
		}
	}
	// Anything else, such as a refused connection, says nothing about the token.
	return Result{
		Status:      StatusWarn,
		Message:     "could not ask GitHub: " + err.Error(),
		Remediation: "check your network connection, proxy settings or VPN, then run again",
	}
}

// rejectedRemediation says how to replace the token that GitHub rejected, which
// depends on where it came from: an environment variable is replaced or unset,
// while a gh login is renewed with gh.
func rejectedRemediation(st authstatus.Status) string {
	if strings.HasSuffix(st.TokenSource, "_TOKEN") {
		return fmt.Sprintf("the token in %s is invalid or expired: replace it, or unset it to fall back to your gh login", st.TokenSource)
	}
	login := `"gh auth login"`
	if st.Host != "github.com" {
		login = fmt.Sprintf(`"gh auth login --hostname %s"`, st.Host)
	}
	return "the gh login is invalid or expired: sign in again with " + login
}

// authstatusVerifier is the TokenVerifier backed by gero auth status.
type authstatusVerifier struct{}

func (authstatusVerifier) Verify(ctx context.Context) (authstatus.Status, error) {
	return authstatus.Check(ctx, "")
}

// RateLimiter asks GitHub how much of the token's rate limit is left. It is
// defined here, where it is consumed, so tests can use a fake.
type RateLimiter interface {
	RateLimit(ctx context.Context) (github.RateLimit, error)
}

// rateLimitCheck warns when little of the token's core rate limit is left,
// since commands that make many requests would fail partway. It is online and
// needs auth-api, which proves the token works first.
type rateLimitCheck struct {
	limiter RateLimiter
	now     func() time.Time
}

// NewRateLimitCheck returns the check named "rate-limit". now is time.Now in
// production.
func NewRateLimitCheck(r RateLimiter, now func() time.Time) Check {
	return rateLimitCheck{limiter: r, now: now}
}

func (rateLimitCheck) Name() string     { return "rate-limit" }
func (rateLimitCheck) Category() string { return "authentication" }
func (rateLimitCheck) Description() string {
	return "Warns when little of the token's GitHub rate limit is left"
}
func (rateLimitCheck) Online() bool    { return true }
func (rateLimitCheck) Needs() []string { return []string{"auth-api"} }

func (rateLimitCheck) Run(context.Context) Result { return Result{} }

// githubRateLimiter is the RateLimiter backed by internal/github, for the token
// every command would use.
type githubRateLimiter struct{}

func (githubRateLimiter) RateLimit(ctx context.Context) (github.RateLimit, error) {
	creds, err := github.Resolve(github.GoGh{}, "")
	if err != nil {
		return github.RateLimit{}, err
	}
	return github.CoreRateLimit(ctx, creds)
}
