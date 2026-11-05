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
	"sync"
	"text/tabwriter"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/rancher/orbs-automation/internal/semver"
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
	for _, row := range r.Rows {
		if row.Status == string(registry.NotFound) || row.Status == string(registry.Unreachable) {
			return true
		}
	}
	return false
}

// Run checks the images of the Project named by project, or of the one whose
// repository contains the working directory cwd when project is empty, for a
// release of tag, and renders the report as s.Output (text or json). The report
// is returned even with ErrNotPublished.
func Run(ctx context.Context, s clicfg.Settings, cwd, project, tag string) (string, error) {
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", fmt.Errorf("output format %q is not supported by repo images exist: use text or json", s.Output)
	}
	report, err := check(ctx, workspace.Open(s, cwd), registry.Checker{}, project, tag)
	if err != nil {
		return "", err
	}
	var out string
	if s.Output == clicfg.OutputJSON {
		if out, err = renderJSON(report); err != nil {
			return "", err
		}
	} else {
		out = renderText(report)
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
	report := Report{Project: p.Repo.Name, Tag: tag, Stable: !semver.Classify(tag).HasPrerelease}
	type job struct {
		row  int
		host string
	}
	var jobs []job
	for _, img := range p.EffectiveImages() {
		for _, role := range img.Registries {
			reg, _ := registry.Lookup(role) // validation has rejected an unknown role
			row := Row{Image: img.Name, Registry: role, Label: reg.Label, Reference: reg.Reference(img.Name, tag)}
			if registry.Applies(role, tag) {
				jobs = append(jobs, job{row: len(report.Rows), host: reg.Host})
			} else {
				row.Status, row.Detail = Skipped, "prerelease"
			}
			report.Rows = append(report.Rows, row)
		}
	}
	// The rows are all in place before any goroutine starts, and each writes
	// only its own, so the report keeps config order whatever finishes first.
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row := &report.Rows[j.row]
			res := c.Exists(ctx, j.host, row.Image, tag)
			row.Status, row.Detail = string(res.Status), res.Detail
		}()
	}
	wg.Wait()
	return report, nil
}

func renderText(r Report) string {
	var b strings.Builder
	kind := "stable"
	if !r.Stable {
		kind = "prerelease"
	}
	fmt.Fprintf(&b, "%s %s (%s)\n\n", r.Project, r.Tag, kind)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "IMAGE\tREGISTRY\tSTATUS\tREFERENCE")
	var notFound, unreachable int
	for _, row := range r.Rows {
		status := row.Status
		if row.Detail != "" {
			status += " (" + row.Detail + ")"
		}
		switch row.Status {
		case string(registry.NotFound):
			notFound++
		case string(registry.Unreachable):
			unreachable++
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", row.Image, row.Label, status, row.Reference)
	}
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	b.WriteString("\n")
	if notFound+unreachable == 0 {
		b.WriteString("All required images were found.\n")
		return b.String()
	}
	var parts []string
	if notFound > 0 {
		parts = append(parts, fmt.Sprintf("%d not found", notFound))
	}
	if unreachable > 0 {
		parts = append(parts, fmt.Sprintf("%d unreachable", unreachable))
	}
	b.WriteString(strings.Join(parts, ", ") + ": the release is not fully published.\n")
	return b.String()
}

func renderJSON(r Report) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
