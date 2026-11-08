package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/platform/semver"
)

// CheckLegacyProject is CheckProject for a project of the v1alpha1 model, with the
// rules it had: the Project's effective images, an image with since left out when
// the tag's line is older. It goes when ship moves to the chain.
func CheckLegacyProject(ctx context.Context, p config.Project, c Prober, tag string) Report {
	report := Report{Project: p.Repo.Name, Tag: tag, Stable: !semver.Classify(tag).HasPrerelease}
	type job struct {
		row  int
		host string
	}
	var jobs []job
	images := p.EffectiveImages()
	line := p.LineOf(tag)
	if line == "" && anyLegacySince(images) {
		// Nothing to compare since with, so no image is held back: the check
		// is a plain one, and the report says the rule was not applied.
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"the tag %s is in none of the release lines of %s, so since was not applied and every image was checked",
			tag, p.Repo.Name))
	}
	for _, img := range images {
		if !img.AppliesTo(line) {
			continue // not expected for this tag, so it is neither checked nor listed
		}
		for _, role := range img.Registries {
			reg, _ := Lookup(role) // validation has rejected an unknown role
			row := Row{Image: img.Name, Registry: role, Label: reg.Label, Reference: reg.Reference(img.Name, tag)}
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

func anyLegacySince(images []config.Image) bool {
	for _, img := range images {
		if img.Since != "" {
			return true
		}
	}
	return false
}
