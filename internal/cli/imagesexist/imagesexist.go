// Package imagesexist holds the logic of gero repo images exist, a port of
// validate_image_exists: it checks that the images a Project publishes are in
// the registries a release of a tag must be in. It is framework-free: cmd/ owns
// the cobra wiring.
package imagesexist

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/registry"
)

// ErrNotPublished is returned, with the report still written, when an image the
// release needs is not found or could not be looked for.
var ErrNotPublished = errors.New("some images are not published where they must be")

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory when the key is empty.
	Project(key string) (config.Project, error)
}

// Checker looks for one image in one registry.
type Checker interface {
	Exists(ctx context.Context, host, image, tag string) registry.Result
}

// Skipped is the status of a registry a release of the tag does not go to.
const Skipped = "skipped"

// Row is the outcome for one image in one registry. Status is a registry.Status
// value, or Skipped.
type Row struct {
	Image     string `json:"image"`
	Registry  string `json:"registry"` // the role
	Label     string `json:"label"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

// Report is everything the command found.
type Report struct {
	Project string `json:"project"` // the repository slug
	Tag     string `json:"tag"`
	Stable  bool   `json:"stable"`
	Rows    []Row  `json:"results"`
}

// Failed reports whether any required image was not found or could not be
// looked for.
func (r Report) Failed() bool {
	return false
}

// Run checks the images of the Project named by project, or of the one whose
// repository contains the working directory cwd when project is empty, for a
// release of tag, and renders the report as s.Output (text or json). The report
// is returned even with ErrNotPublished.
func Run(ctx context.Context, s clicfg.Settings, cwd, project, tag string) (string, error) {
	return "", nil
}

func check(ctx context.Context, ws Workspace, c Checker, project, tag string) (Report, error) {
	return Report{}, nil
}

func renderText(r Report) string { return "" }

func renderJSON(r Report) (string, error) { return "", nil }
