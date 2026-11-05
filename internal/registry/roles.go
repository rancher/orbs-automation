// Package registry checks that container images exist in the registries a
// release is published to. The registries are built in, named by role, because
// where gero publishes is not something a project configures
// (docs/specs/IMAGES_EXIST.md). It reads anonymously and performs no
// authentication of its own.
package registry

// Registry is a place images are published.
type Registry struct {
	Role  string // one of the config.Registry* values
	Label string // for people: "Docker Hub"
	Host  string // where the registry API is
	Name  string // how a reference to it is written, which can differ from Host
}

// Lookup returns the registry for a role, and false for an unknown one.
func Lookup(role string) (Registry, bool) {
	return Registry{}, false
}

// Applies reports whether a release of tag must be in the registry: all of
// them for a stable tag, and all but Prime production for a prerelease. A stable
// tag is one with no prerelease suffix, decided as the check-semver script does,
// so a tag that is not a version at all counts as stable.
func Applies(role, tag string) bool {
	return false
}

// Reference returns the full reference to an image in a registry, such as
// registry.rancher.com/rancher/scc-operator:v0.5.3.
func (r Registry) Reference(image, tag string) string {
	return ""
}
