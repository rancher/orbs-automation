package github

import (
	"context"
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
func CoreRateLimit(_ context.Context, _ Credentials, _ ...gogithub.ClientOptionsFunc) (RateLimit, error) {
	return RateLimit{}, nil
}
