package github

import (
	"context"
	"fmt"
	"time"

	gogithub "github.com/google/go-github/v93/github"
)

// RateLimit is the core REST limit of a token: how many requests it may make
// per window, how many are left, and when the window resets.
type RateLimit struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// CoreRateLimit asks the host for the token's core REST rate limit. The call
// itself does not count against the limit. A rejected token is reported as
// ErrBadCredentials, as Login does.
func CoreRateLimit(ctx context.Context, c Credentials, opts ...gogithub.ClientOptionsFunc) (RateLimit, error) {
	client, err := NewClient(c, opts...)
	if err != nil {
		return RateLimit{}, err
	}
	limits, _, err := client.RateLimit.Get(ctx)
	if err != nil {
		if isUnauthorized(err) {
			return RateLimit{}, fmt.Errorf("%w on %s", ErrBadCredentials, c.Host)
		}
		return RateLimit{}, fmt.Errorf("read the rate limit on %s: %w", c.Host, err)
	}
	if limits.Core == nil {
		return RateLimit{}, fmt.Errorf("%s did not report a core rate limit", c.Host)
	}
	return RateLimit{
		Limit:     limits.Core.Limit,
		Remaining: limits.Core.Remaining,
		Reset:     limits.Core.Reset.UTC(),
	}, nil
}
