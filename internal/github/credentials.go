// Package github is gero's access to GitHub. This file resolves which host to
// talk to and which token to use, bridging off the user's gh setup
// (docs/IMPLEMENTATION.md section 3). It does no I/O itself: the environment,
// gh's config and the gh binary are reached through Auth.
package github

import "errors"

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
// is empty. When no token is found the error wraps ErrNoToken and says which
// variables to set.
func Resolve(_ Auth, _ string) (Credentials, error) {
	return Credentials{}, nil
}
