package github

import "sync/atomic"

// userAgentVersion is the build version in the User-Agent. It is set once at
// start-up, before any client exists, and read when a client is made.
var userAgentVersion atomic.Value

func init() { userAgentVersion.Store("dev") }

// SetUserAgentVersion sets the version in the User-Agent every client sends.
// The wiring layer calls it once at start-up with the build's version.
func SetUserAgentVersion(version string) {
	userAgentVersion.Store(version)
}

// UserAgent returns the HTTP User-Agent gero sends to GitHub, in the usual
// product/version (comment) form. It names the tool and the team in the way
// GitHub's logs and rate-limit mail can show. It is not the human-facing
// "Created with" credit used in PR and issue text.
func UserAgent() string {
	return "gero/" + userAgentVersion.Load().(string) +
		" (+https://github.com/rancher/orbs-automation; ORBS Team Automation)"
}
