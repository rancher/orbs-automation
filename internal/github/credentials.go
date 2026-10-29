// Package github is gero's access to GitHub. This file resolves which host to
// talk to and which token to use, bridging off the user's gh setup
// (docs/specs/GITHUB_ACCESS.md). It does no I/O itself: the environment,
// gh's config and the gh binary are reached through Auth.
package github

import (
	"errors"
	"fmt"
)

// ErrNoToken is returned when no source supplies a token for the host.
var ErrNoToken = errors.New("no GitHub token found")

// SourceExplicit is the Credentials.HostSource of a host the caller named.
const SourceExplicit = "explicit"

// Auth is where tokens and hosts come from. The go-gh adapter implements it;
// tests use a fake.
type Auth interface {
	// TokenForHost returns the token for host and where it came from, or ""
	// when there is none.
	TokenForHost(host string) (token, source string)
	// DefaultHost returns the host to use when the caller names none, and
	// where it came from.
	DefaultHost() (host, source string)
	// IsEnterprise reports whether host is a GitHub Enterprise Server, which
	// reads different environment variables than github.com.
	IsEnterprise(host string) bool
}

// Credentials is the resolved host and token, each with its source.
type Credentials struct {
	Host        string
	HostSource  string
	Token       string
	TokenSource string
}

// Resolve returns the credentials for host, or for the default host when host
// is empty. When no token is found the error is a *NoTokenError, which wraps
// ErrNoToken and says which variables to set.
func Resolve(a Auth, host string) (Credentials, error) {
	hostSource := SourceExplicit
	if host == "" {
		host, hostSource = a.DefaultHost()
	}
	token, tokenSource := a.TokenForHost(host)
	if token == "" {
		return Credentials{}, &NoTokenError{Host: host, Hint: tokenHint(a, host)}
	}
	return Credentials{Host: host, HostSource: hostSource, Token: token, TokenSource: tokenSource}, nil
}

// tokenHint says how to supply a token for host. Enterprise hosts ignore
// GH_TOKEN and GITHUB_TOKEN, so the advice differs.
func tokenHint(a Auth, host string) string {
	if a.IsEnterprise(host) {
		return fmt.Sprintf("set GH_ENTERPRISE_TOKEN or GITHUB_ENTERPRISE_TOKEN, or run \"gh auth login --hostname %s\"", host)
	}
	return `set GH_TOKEN or GITHUB_TOKEN, or run "gh auth login"`
}

// NoTokenError is the error Resolve returns when no source supplies a token. It
// carries the host and the advice separately, so a caller can show each in its
// own place. It wraps ErrNoToken.
type NoTokenError struct {
	Host string // the host no token was found for
	Hint string // how to supply one
}

func (e *NoTokenError) Error() string {
	return fmt.Sprintf("%s for %s: %s", ErrNoToken, e.Host, e.Hint)
}

func (*NoTokenError) Unwrap() error { return ErrNoToken }

// DescribeTokenSource turns the source name go-gh reports for a token, such as
// "GH_TOKEN" or "oauth_token", into words for a person. A source it does not
// know is returned as it is.
func DescribeTokenSource(source string) string {
	if s, ok := tokenSources[source]; ok {
		return s
	}
	return source
}

var tokenSources = map[string]string{
	"GH_TOKEN":                "environment variable GH_TOKEN",
	"GITHUB_TOKEN":            "environment variable GITHUB_TOKEN",
	"GH_ENTERPRISE_TOKEN":     "environment variable GH_ENTERPRISE_TOKEN",
	"GITHUB_ENTERPRISE_TOKEN": "environment variable GITHUB_ENTERPRISE_TOKEN",
	"oauth_token":             "gh config file",
	"gh":                      "gh auth token (keyring)",
}
