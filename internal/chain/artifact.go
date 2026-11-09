package chain

import "github.com/rancher/orbs-automation/internal/apis/v1alpha2"

// Artifact is one thing each release of a stream produces.
type Artifact struct {
	Doc    *v1alpha2.Artifact
	stream *Stream
}

// Name is the name of the document.
func (a *Artifact) Name() string { return a.Doc.Name }

// Stream is the stream whose releases produce the artifact.
func (a *Artifact) Stream() *Stream { return a.stream }

// IsImage reports whether the artifact is a container image, and not a file.
func (a *Artifact) IsImage() bool { return a.Doc.Spec.Kind == v1alpha2.ArtifactImage }

// IsGoMod reports whether the artifact is a Go module.
func (a *Artifact) IsGoMod() bool { return a.Doc.Spec.Kind == v1alpha2.ArtifactGoMod }

// Ref is the artifact's name: an image's repository path, or a file's name
// template.
func (a *Artifact) Ref() string { return a.Doc.Spec.Name }

// Registries are the registries an image is checked in: those written, else
// Docker Hub and Prime staging, and Prime production too for a stable release. A
// file has none.
func (a *Artifact) Registries(stable bool) []string {
	if !a.IsImage() {
		return nil
	}
	if len(a.Doc.Spec.Registries) > 0 {
		return append([]string(nil), a.Doc.Spec.Registries...)
	}
	regs := []string{RegistryDockerHub, RegistryPrimeStaging}
	if stable {
		regs = append(regs, RegistryPrimeProduction)
	}
	return regs
}

// AppliesTo reports whether releases for the line produce the artifact.
func (a *Artifact) AppliesTo(line string) bool { return a.Doc.Spec.Lines.Contains(line) }
