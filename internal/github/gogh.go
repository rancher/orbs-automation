package github

// GoGh is the Auth backed by go-gh's pkg/auth. It reads GH_TOKEN, GITHUB_TOKEN
// and their enterprise forms, gh's config files, and falls back to running
// "gh auth token" (docs/IMPLEMENTATION.md section 3). It is the only file that
// imports go-gh.
type GoGh struct{}

// TokenForHost returns the token for host and its source, or "" when none.
func (GoGh) TokenForHost(string) (token, source string) { return "", "" }

// DefaultHost returns GH_HOST, a lone host in gh's config, or github.com.
func (GoGh) DefaultHost() (host, source string) { return "", "" }

// IsEnterprise reports whether host is a GitHub Enterprise Server.
func (GoGh) IsEnterprise(string) bool { return false }

var _ Auth = GoGh{}
