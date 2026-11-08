package v1alpha2

import (
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/rancher/orbs-automation/internal/platform/templates"
)

// The kinds of artifact, and the registries an image may name.
const (
	ArtifactImage = "image"
	ArtifactFile  = "file"

	RegistryDockerHub       = "dockerhub"
	RegistryPrimeStaging    = "prime-staging"
	RegistryPrimeProduction = "prime-production"
)

// fileNameVars are the variables the name of a file may use.
var fileNameVars = []string{"tag", "version"}

// SetDefaults does nothing: an artifact has no default. Its registries, when it
// names none, are the ones its release kind goes to, which a getter gives.
func (a *Artifact) SetDefaults() {}

// Validate checks what an Artifact can alone: its kind, its name, the registries
// of an image and the range of lines that produce it.
func (a *Artifact) Validate(_ config.Context, r *config.Report) {
	spec := a.Spec
	kindOK := oneOf(r, "spec.kind", spec.Kind, ArtifactImage, ArtifactFile)
	switch {
	case spec.Name == "":
		r.Errorf("spec.name", "is required")
	case kindOK && spec.Kind == ArtifactImage && strings.ContainsAny(spec.Name, "{}"):
		r.Errorf("spec.name", "%q is a repository path and has no variables", spec.Name)
	case kindOK && spec.Kind == ArtifactFile:
		if err := templates.Validate(spec.Name, fileNameVars); err != nil {
			r.Errorf("spec.name", "%v", err)
		}
	}

	if len(spec.Registries) > 0 && kindOK && spec.Kind != ArtifactImage {
		r.Errorf("spec.registries", "are only for an image")
	}
	seen := map[string]bool{}
	for i, reg := range spec.Registries {
		at := fmt.Sprintf("spec.registries[%d]", i)
		if !oneOf(r, at, reg, RegistryDockerHub, RegistryPrimeStaging, RegistryPrimeProduction) {
			continue
		}
		if seen[reg] {
			r.Errorf(at, "%q is listed twice", reg)
		}
		seen[reg] = true
	}

	checkRange(r, "spec.lines", spec.Lines)
}
