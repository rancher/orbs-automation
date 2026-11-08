// Package registry checks that container images exist in the registries a
// release is published to. The registries are built in, named by role, because
// where gero publishes is not something a project configures
// (docs/specs/IMAGES_EXIST.md). It reads anonymously and performs no
// authentication of its own.
package registry

import (
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/platform/semver"
)

// Registry is a place images are published.
type Registry struct {
	Role  string // one of the config.Registry* values
	Label string // for people: "Docker Hub"
	Host  string // where the registry API is
	Name  string // how a reference to it is written, which can differ from Host
}

// Lookup returns the registry for a role, and false for an unknown one.
func Lookup(role string) (Registry, bool) {
	r, ok := registries[role]
	return r, ok
}

var registries = map[string]Registry{
	config.RegistryDockerHub: {
		Role: config.RegistryDockerHub, Label: "Docker Hub", Host: "registry-1.docker.io", Name: "docker.io",
	},
	config.RegistryPrimeStaging: {
		Role: config.RegistryPrimeStaging, Label: "Prime Staging", Host: "stgregistry.suse.com", Name: "stgregistry.suse.com",
	},
	config.RegistryPrimeProduction: {
		Role: config.RegistryPrimeProduction, Label: "Prime Production", Host: "registry.rancher.com", Name: "registry.rancher.com",
	},
}

// Applies reports whether a release of tag must be in the registry: all of
// them for a stable tag, and all but Prime production for a prerelease. A stable
// tag is one with no prerelease suffix, decided as the check-semver script does,
// so a tag that is not a version at all counts as stable.
func Applies(role, tag string) bool {
	if _, ok := registries[role]; !ok {
		return false
	}
	return role != config.RegistryPrimeProduction || !semver.Classify(tag).HasPrerelease
}

// Reference returns the full reference to an image in a registry, such as
// registry.rancher.com/rancher/scc-operator:v0.5.3.
func (r Registry) Reference(image, tag string) string {
	return r.Name + "/" + image + ":" + tag
}
