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
}
