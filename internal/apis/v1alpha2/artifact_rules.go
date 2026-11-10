package v1alpha2

import (
	"fmt"
	"path"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/rancher/orbs-automation/internal/platform/templates"
)

// The kinds of artifact, and the registries an image may name.
const (
	ArtifactImage = "image"
	ArtifactFile  = "file"
	ArtifactGoMod = "gomod"

	RegistryDockerHub       = "dockerhub"
	RegistryPrimeStaging    = "prime-staging"
	RegistryPrimeProduction = "prime-production"
)

// fileNameVars are the variables the name of a file may use.
var fileNameVars = []string{"tag", "version"}

// SetDefaults fills the file a pin reads, for an Artifact that pins. Its
// registries, when it names none, are the ones its release kind goes to, which a
// getter gives.
func (a *Artifact) SetDefaults() {
	if len(a.Refs["pins"]) > 0 && a.Spec.Pins.File == "" {
		a.Spec.Pins.File = DefaultPinFile
	}
}

// Validate checks what an Artifact can alone: its kind, its name, the registries
// of an image and the range of lines that produce it.
func (a *Artifact) Validate(_ config.Context, r *config.Report) {
	spec := a.Spec
	kindOK := oneOf(r, "spec.kind", spec.Kind, ArtifactImage, ArtifactFile, ArtifactGoMod)
	switch {
	case spec.Name == "":
		r.Errorf("spec.name", "is required")
	case kindOK && spec.Kind == ArtifactImage && strings.ContainsAny(spec.Name, "{}"):
		r.Errorf("spec.name", "%q is a repository path and has no variables", spec.Name)
	case kindOK && spec.Kind == ArtifactImage && strings.ContainsAny(spec.Name, " \t"):
		r.Errorf("spec.name", "%q is a repository path and has no space", spec.Name)
	case kindOK && spec.Kind == ArtifactGoMod && strings.ContainsAny(spec.Name, "{} \t"):
		r.Errorf("spec.name", "%q is a module path and has no variables or space", spec.Name)
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

	a.validatePins(r, kindOK)
	checkRange(r, "spec.lines", spec.Lines)
}

// validatePins checks a pin alone: only a Go module pins, the file it reads is a
// relative path inside the repo, and a file is named only when something is pinned.
// That what it pins is a Go module too, and that the pins do not circle, is
// checked across documents.
func (a *Artifact) validatePins(r *config.Report, kindOK bool) {
	pinned, file := a.Refs["pins"], a.Spec.Pins.File
	if len(pinned) > 0 && kindOK && a.Spec.Kind != ArtifactGoMod {
		r.Errorf("refs.pins", "only a %s Artifact pins another, and this one is %s", ArtifactGoMod, a.Spec.Kind)
	}
	if len(pinned) == 0 {
		if file != "" {
			r.Errorf("spec.pins", "names the file %q and nothing is pinned: add refs.pins", file)
		}
		return
	}
	switch {
	case strings.ContainsAny(file, "{}"):
		r.Errorf("spec.pins.file", "%q is a path in the repo and has no variables", file)
	case path.IsAbs(file):
		r.Errorf("spec.pins.file", "%q must be relative to the repo", file)
	case path.Clean(file) == ".." || strings.HasPrefix(path.Clean(file), "../"):
		r.Errorf("spec.pins.file", "%q must stay inside the repo", file)
	}
}
