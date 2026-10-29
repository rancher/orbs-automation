package github

import (
	"errors"
	"fmt"
	"strings"

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
func NewClient(c Credentials, opts ...gogithub.ClientOptionsFunc) (*gogithub.Client, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("%w for %s", ErrNoToken, c.Host)
	}
	all := []gogithub.ClientOptionsFunc{gogithub.WithAuthToken(c.Token)}
	switch {
	case c.Host == "github.com":
	case c.Host == "github.localhost" || strings.HasSuffix(c.Host, ".ghe.com"):
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedHost, c.Host)
	default:
		base := "https://" + c.Host
		all = append(all, gogithub.WithEnterpriseURLs(base, base))
	}
	return gogithub.NewClient(append(all, opts...)...)
}
