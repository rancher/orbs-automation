package chain

import "github.com/rancher/orbs-automation/internal/apis/v1alpha2"

// Artifact is one thing each release of a stream produces.
type Artifact struct {
	Doc *v1alpha2.Artifact
}

// Name is the name of the document.
func (a *Artifact) Name() string { return "" }

// Stream is the stream whose releases produce the artifact.
func (a *Artifact) Stream() *Stream { return nil }

// IsImage reports whether the artifact is a container image, and not a file.
func (a *Artifact) IsImage() bool { return false }

// Ref is the artifact's name: an image's repository path, or a file's name
// template.
func (a *Artifact) Ref() string { return "" }

// Registries are the registries an image is checked in: those written, else
// Docker Hub and Prime staging, and Prime production too for a stable release. A
// file has none.
func (a *Artifact) Registries(_ bool) []string { return nil }

// AppliesTo reports whether releases for the line produce the artifact.
func (a *Artifact) AppliesTo(_ string) bool { return false }
