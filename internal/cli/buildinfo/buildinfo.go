// Package buildinfo hands the build's version to the packages that report it
// to the outside world. cmd/ reads the version (cmd/version); nothing deeper
// does.
package buildinfo

import "github.com/rancher/orbs-automation/internal/github"

// Apply sets the version that appears in gero's HTTP User-Agent.
func Apply(version string) {
	github.SetUserAgentVersion(version)
}
