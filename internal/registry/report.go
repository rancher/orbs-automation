package registry

import (
	"context"
	"errors"
	"fmt"

	"sync"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/goproxy"
	"github.com/rancher/orbs-automation/internal/platform/ociregistry"
	"github.com/rancher/orbs-automation/internal/platform/semver"
)

// Prober looks for one image in one registry. ociregistry.Checker is the real one.
type Prober interface {
	Exists(ctx context.Context, host, image, tag string) ociregistry.Result
}

// Modules looks for one version of a Go module. goproxy.Checker is the real one.
type Modules interface {
	Exists(ctx context.Context, module, version string) goproxy.Result
}

// ModuleProber is what a Prober also is when it can look for a Go module: a check
// of a project looks for the modules it publishes only with one.
type ModuleProber interface {
	ModuleExists(ctx context.Context, module, version string) goproxy.Result
}

// Probers looks for images in registries and for modules on a Go module proxy. It
// is the Prober, and the ModuleProber, a command builds a check with.
type Probers struct {
	Images  Prober
	Modules Modules
	// Go is the list of Go releases, which names the newest patch of a minor for a
	// go.mod that pins no more than go 1.26 (bump.GoReleases). Nil means none.
	Go GoReleases
}

// GoReleases is the list of Go releases.
type GoReleases interface {
	NewestPatch(ctx context.Context, minor string) (string, error)
}

// NewestPatch names the newest stable patch of a Go minor, such as go1.26.10, from
// the list of Go releases.
func (p Probers) NewestPatch(ctx context.Context, minor string) (string, error) {
	if p.Go == nil {
		return "", errors.New("there is no list of Go releases")
	}
	return p.Go.NewestPatch(ctx, minor)
}

// Exists looks for an image in a registry.
func (p Probers) Exists(ctx context.Context, host, image, tag string) ociregistry.Result {
	return p.Images.Exists(ctx, host, image, tag)
}

// ModuleExists looks for a version of a Go module on the proxy.
func (p Probers) ModuleExists(ctx context.Context, module, version string) goproxy.Result {
	return p.Modules.Exists(ctx, module, version)
}

// ModuleProxy is the Registry of a row for a Go module: the proxy it is looked for on.
const ModuleProxy = "goproxy"

// Skipped is the status of a registry a release of the tag does not go to.
const Skipped = "skipped"

// Row is the outcome for one image in one registry. Status is an ociregistry.Status value,
// or Skipped.
type Row struct {
	Image     string `json:"image"`
	Registry  string `json:"registry"` // the role
	Label     string `json:"label"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

// Report is the outcome of checking the images of a release.
type Report struct {
	Project string `json:"project"` // the repository slug
	Tag     string `json:"tag"`
	Stable  bool   `json:"stable"`
	// Warnings are things that did not stop the check but that the reader
	// should know, such as a rule that could not be applied.
	Warnings []string `json:"warnings,omitempty"`
	Rows     []Row    `json:"results"`
}

// Failed reports whether any required image was not found or could not be
// looked for.
func (r Report) Failed() bool {
	for _, row := range r.Rows {
		if row.Status == string(ociregistry.NotFound) || row.Status == string(ociregistry.Unreachable) {
			return true
		}
	}
	return false
}

// CheckProject looks for what the Project publishes for a release of tag: the
// images (the image artifacts of its streams) in each registry the release goes
// to, and then the Go modules (the gomod artifacts of the streams the tag is a
// release of) on the Go module proxy, when c can look for one. An artifact that
// applies only from a line on is left out, not checked and not listed, when the
// tag's line is older; a tag in none of the Project's lines gets a plain check and
// a warning. Prime production is skipped for a prerelease; the proxy has every
// version, so a module is looked for whatever the tag. The checks run in parallel
// and the rows keep config order, the images first.
func CheckProject(ctx context.Context, p *chain.Project, c Prober, tag string) Report {
	report := Report{Project: p.Slug(), Tag: tag, Stable: !semver.Classify(tag).HasPrerelease}
	type job struct {
		row   int
		probe func() (status, detail string)
	}
	var jobs []job
	var images, modules []*chain.Artifact
	for _, a := range p.Artifacts() {
		switch {
		case a.IsImage():
			images = append(images, a)
		case a.IsGoMod() && a.Stream().TagMatches(tag):
			modules = append(modules, a)
		}
	}
	line, _ := p.LineOf(tag)
	if line == "" && anyRanged(images) {
		// Nothing to compare since with, so no image is held back: the check
		// is a plain one, and the report says the rule was not applied.
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"the tag %s is in none of the release lines of %s, so since was not applied and every image was checked",
			tag, p.Slug()))
	}
	for _, img := range images {
		if !img.AppliesTo(line) {
			continue // not expected for this tag, so it is neither checked nor listed
		}
		for _, role := range img.Registries(true) {
			reg, _ := Lookup(role) // validation has rejected an unknown role
			row := Row{Image: img.Ref(), Registry: role, Label: reg.Label, Reference: reg.Reference(img.Ref(), tag)}
			if Applies(role, tag) {
				host, image := reg.Host, img.Ref()
				jobs = append(jobs, job{row: len(report.Rows), probe: func() (string, string) {
					res := c.Exists(ctx, host, image, tag)
					return string(res.Status), res.Detail
				}})
			} else {
				row.Status, row.Detail = Skipped, "prerelease"
			}
			report.Rows = append(report.Rows, row)
		}
	}
	mp, canAsk := c.(ModuleProber)
	for _, mod := range modules {
		if !mod.AppliesTo(line) {
			continue
		}
		module, version := mod.Ref(), mod.Stream().BareTag(tag)
		row := Row{Image: module, Registry: ModuleProxy, Label: "Go module proxy", Reference: module + "@" + version}
		if canAsk {
			jobs = append(jobs, job{row: len(report.Rows), probe: func() (string, string) {
				res := mp.ModuleExists(ctx, module, version)
				return string(res.Status), res.Detail
			}})
		} else {
			row.Status, row.Detail = Skipped, "no Go module proxy to ask"
		}
		report.Rows = append(report.Rows, row)
	}
	// The rows are all in place before any goroutine starts, and each writes
	// only its own, so the report keeps config order whatever finishes first.
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row := &report.Rows[j.row]
			row.Status, row.Detail = j.probe()
		}()
	}
	wg.Wait()
	return report
}

// anyRanged reports whether any image applies to some lines only.
func anyRanged(images []*chain.Artifact) bool {
	for _, img := range images {
		if img.Ranged() {
			return true
		}
	}
	return false
}
