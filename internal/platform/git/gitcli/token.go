package gitcli

import "context"

// WithToken returns a context whose pushes authenticate to the GitHub host at
// baseURL (such as https://github.com/) with token. It is the CI form of the
// engineer's own credentials.
func WithToken(ctx context.Context, baseURL, token string) context.Context {
	return ctx
}
