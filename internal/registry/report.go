package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/semver"
)

// Prober looks for one image in one registry. Checker is the real one.
type Prober interface {
	Exists(ctx context.Context, host, image, tag string) Result
}

// Skipped is the status of a registry a release of the tag does not go to.
const Skipped = "skipped"

// Row is the outcome for one image in one registry. Status is a Status value,
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
		if row.Status == string(NotFound) || row.Status == string(Unreachable) {
			return true
		}
	}
	return false
}

// CheckProject looks for the images the Project publishes (the image artifacts
// of its streams) for a release of tag, in each registry the release goes to. An
// image that applies only from a line on is left out, not checked and not listed,
// when the tag's line is older; a tag in none of the Project's lines gets a plain
// check and a warning. Prime production is skipped for a prerelease. The checks
// run in parallel and the rows keep config order.
func CheckProject(ctx context.Context, p *chain.Project, c Prober, tag string) Report {
	report := Report{Project: p.Slug(), Tag: tag, Stable: !semver.Classify(tag).HasPrerelease}
	type job struct {
		row  int
		host string
	}
	var jobs []job
	var images []*chain.Artifact
	for _, a := range p.Artifacts() {
		if a.IsImage() {
			images = append(images, a)
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
	return report
}

// anyRanged reports whether any image applies to some lines only.
func anyRanged(images []*chain.Artifact) bool {
	for _, img := range images {
		if img.Doc.Spec.Lines != (v1alpha2.Range{}) {
			return true
		}
	}
	return false
}
