// Package imagesexist holds the logic of gero repo images exist, a port of
// validate_image_exists: it checks that the images a Project publishes are in
// the registries a release of a tag must be in. It is framework-free: cmd/ owns
// the cobra wiring.
package imagesexist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/goproxy"
	"github.com/rancher/orbs-automation/internal/platform/ociregistry"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrNotPublished is returned, with the report still written, when an image the
// release needs is not found or could not be looked for.
var ErrNotPublished = errors.New("some images are not published where they must be")

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory when the key is empty.
	Project(key string) (*chain.Project, error)
}

// Checker looks for one image in one registry.
type Checker interface {
	Exists(ctx context.Context, host, image, tag string) ociregistry.Result
}

// Skipped, Row and Report are the registry package's: the check itself lives
// there so the downstream bump can run it too.
const Skipped = registry.Skipped

// Row is the outcome for one image in one registry.
type Row = registry.Row

// Report is everything the command found.
type Report = registry.Report

// Run checks the images of the Project named by project, or of the one whose
// repository contains the working directory cwd when project is empty, for a
// release of tag, and renders the report as s.Output (text or json). The report
// is returned even with ErrNotPublished.
func Run(ctx context.Context, s clicfg.Settings, cwd, project, tag string) (string, error) {
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", fmt.Errorf("output format %q is not supported by repo images exist: use text or json", s.Output)
	}
	report, err := check(ctx, workspace.Open(s, cwd), registry.Probers{Images: ociregistry.Checker{}, Modules: goproxy.Checker{}}, project, tag)
	if err != nil {
		return "", err
	}
	var out string
	if s.Output == clicfg.OutputJSON {
		if out, err = renderJSON(report); err != nil {
			return "", err
		}
	} else {
		out = renderText(report, style.New(s.Terminal))
	}
	if report.Failed() {
		return out, ErrNotPublished
	}
	return out, nil
}

func check(ctx context.Context, ws Workspace, c Checker, project, tag string) (Report, error) {
	p, err := ws.Project(project)
	if err != nil {
		return Report{}, err
	}
	return registry.CheckProject(ctx, p, c, tag), nil
}

// statusRole is the look of the status of an image in a registry.
func statusRole(status string) (style.Role, bool) {
	switch status {
	case string(ociregistry.Found):
		return style.OK, true
	case string(ociregistry.NotFound), string(ociregistry.Unreachable):
		return style.Error, true
	case Skipped:
		return style.Dim, true
	}
	return style.Dim, false
}

func renderText(r Report, view *style.Renderer) string {
	var b strings.Builder
	kind := "stable"
	if !r.Stable {
		kind = "prerelease"
	}
	b.WriteString(view.Style(style.Heading, fmt.Sprintf("%s %s (%s)", r.Project, r.Tag, kind)) + "\n\n")
	if len(r.Rows) == 0 {
		for _, w := range r.Warnings {
			b.WriteString(view.Style(style.Warn, "Warning: "+w) + "\n\n")
		}
		b.WriteString(view.Style(style.Dim, "No image is expected for this tag.") + "\n")
		return b.String()
	}
	table := [][]string{{
		view.Style(style.Dim, "IMAGE"), view.Style(style.Dim, "REGISTRY"),
		view.Style(style.Dim, "STATUS"), view.Style(style.Dim, "REFERENCE"),
	}}
	var notFound, unreachable int
	for _, row := range r.Rows {
		status := row.Status
		if role, ok := statusRole(status); ok {
			status = view.Style(role, status)
		}
		if row.Detail != "" {
			status += " (" + row.Detail + ")"
		}
		switch row.Status {
		case string(ociregistry.NotFound):
			notFound++
		case string(ociregistry.Unreachable):
			unreachable++
		}
		table = append(table, []string{row.Image, row.Label, status, row.Reference})
	}
	b.WriteString(ui.Align(table, view.Width))
	b.WriteString("\n")
	for _, w := range r.Warnings {
		b.WriteString(view.Style(style.Warn, "Warning: "+w) + "\n\n")
	}
	if notFound+unreachable == 0 {
		b.WriteString(view.Style(style.OK, "All required images were found.") + "\n")
		return b.String()
	}
	var parts []string
	if notFound > 0 {
		parts = append(parts, fmt.Sprintf("%d not found", notFound))
	}
	if unreachable > 0 {
		parts = append(parts, fmt.Sprintf("%d unreachable", unreachable))
	}
	b.WriteString(view.Style(style.Error, strings.Join(parts, ", ")+": the release is not fully published.") + "\n")
	return b.String()
}

func renderJSON(r Report) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
