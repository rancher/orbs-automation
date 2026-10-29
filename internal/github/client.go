package github

import (
	"errors"

	gogithub "github.com/google/go-github/v93/github"
)

// ErrUnsupportedHost is returned for a host whose API address gero cannot
// work out yet: github.com data residency hosts (*.ghe.com) and
// github.localhost.
var ErrUnsupportedHost = errors.New("unsupported GitHub host")

// NewClient returns a go-github REST client for the resolved credentials: the
// token goes only to the host's own API, and a GitHub Enterprise Server host
// gets its /api/v3 and /api/uploads URLs. Extra options (a transport for
// tests, a user agent) are applied last.
func NewClient(_ Credentials, opts ...gogithub.ClientOptionsFunc) (*gogithub.Client, error) {
	return gogithub.NewClient(opts...)
}
