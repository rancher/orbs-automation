package github

// SetUserAgentVersion sets the version in the User-Agent every client sends.
// The wiring layer calls it once at start-up with the build's version.
func SetUserAgentVersion(version string) {}

// UserAgent returns the HTTP User-Agent gero sends to GitHub, in the usual
// product/version (comment) form.
func UserAgent() string { return "" }
