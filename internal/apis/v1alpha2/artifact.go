package v1alpha2

import "github.com/rancher/orbs-automation/internal/apis/lines"

// Artifact is one thing each release of a stream produces.
type Artifact struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       ArtifactSpec `yaml:"spec"`
}

// ArtifactSpec is the fields of an Artifact.
type ArtifactSpec struct {
	Kind       string      `yaml:"kind"` // image, file or gomod
	Name       string      `yaml:"name"` // an image's repository path, a file name template or a Go module path
	Lines      lines.Range `yaml:"lines"`
	Registries []string    `yaml:"registries"`
	// Pins qualifies refs.pins, the Artifacts a release of this one pins.
	Pins ArtifactPins `yaml:"pins"`
}

// DefaultPinFile is the file of a release that a pin reads when none is named.
const DefaultPinFile = "go.mod"

// ArtifactPins is what a pin reads: the file of the pinning release that says which
// version of each pinned Artifact goes with it. The key read in it is the pinned
// Artifact's own name, its module path.
type ArtifactPins struct {
	File string `yaml:"file"`
}
