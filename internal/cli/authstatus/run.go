package authstatus

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github"
)

// Verifier proves a token works. It is defined here, where it is consumed, so
// tests can use a fake.
type Verifier interface {
	// Login returns the login the token belongs to, or an error wrapping
	// github.ErrBadCredentials when GitHub rejects it.
	Login(ctx context.Context, c github.Credentials) (string, error)
}

// Run reports the GitHub host, token source and login in the format s.Output.
// hostname names the host to check, or is empty for the default (GH_HOST, then
// gh's config, then github.com). A rejected token is reported and also returned
// as an error, so the command exits non-zero with the result still printed.
func Run(ctx context.Context, s clicfg.Settings, hostname string) (string, error) {
	return run(ctx, s, hostname, github.GoGh{}, loginVerifier{})
}

// loginVerifier asks the host's API, through internal/github.
type loginVerifier struct{}

func (loginVerifier) Login(ctx context.Context, c github.Credentials) (string, error) {
	return github.Login(ctx, c)
}

func run(ctx context.Context, s clicfg.Settings, hostname string, a github.Auth, v Verifier) (string, error) {
	// Refuse a format that cannot be rendered before spending an API call.
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", unsupported(s.Output)
	}
	creds, err := github.Resolve(a, hostname)
	if err != nil {
		return "", err
	}
	st := Status{Host: creds.Host, HostSource: creds.HostSource, TokenSource: creds.TokenSource}
	login, err := v.Login(ctx, creds)
	rejected := errors.Is(err, github.ErrBadCredentials)
	if err != nil && !rejected {
		return "", err
	}
	st.Valid, st.Login = !rejected, login
	out, renderErr := Render(st, s.Output)
	if renderErr != nil {
		return "", renderErr
	}
	return out, err
}
