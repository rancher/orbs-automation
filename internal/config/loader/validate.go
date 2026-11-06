package loader

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/ship/templates"
)

// Validate is ValidateAt for the current time.
func Validate(c config.Config) error {
	return ValidateAt(c, time.Now())
}

// ValidateAt checks c after WithDefaults, so an omitted field with a default is
// already filled in. A line's phase depends on the date, so now decides which
// lines are still in development. It reports every problem it finds, each
// naming the document and field, as one joined error; it returns nil when c is
// valid.
func ValidateAt(c config.Config, now time.Time) error {
	v := validator{projects: c.Projects}
	projects := map[string]bool{}
	for _, p := range c.Projects {
		projects[p.Name] = true
	}
	phases := map[string]string{} // line id to phase
	if c.Lines != nil {
		v.lines(c.Lines, now, phases)
	}
	for _, p := range c.Projects {
		v.project(p, phases, c.Lines != nil)
	}
	for _, x := range c.Externals {
		v.external(x, projects)
	}
	v.repoSlugsAreUnique(c)
	return errors.Join(v.errs...)
}

type validator struct {
	errs     []error
	projects []config.Project // every project, for what an edge names
}

func (v *validator) add(format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

func (v *validator) oneOf(where, field, got string, allowed ...string) {
	if !slices.Contains(allowed, got) {
		v.add("%s: %s must be one of %s, got %q", where, field, strings.Join(allowed, ", "), got)
	}
}

func (v *validator) lines(l *config.RancherLines, now time.Time, phases map[string]string) {
	for i, ln := range l.Lines {
		at := fmt.Sprintf("RancherLines %q: lines[%d]", l.Name, i)
		if ln.ID == "" {
			v.add("%s: id is required", at)
			continue
		}
		if _, dup := phases[ln.ID]; dup {
			v.add("%s: duplicate line %q", at, ln.ID)
		} else {
			phases[ln.ID] = ln.PhaseAt(now)
		}
		if ln.Phase != "" {
			v.oneOf(at, "phase", ln.Phase, config.PhaseDevelopment, config.PhaseGA, config.PhaseEOM, config.PhaseEOL)
		}
		if err := ln.Dates.Validate(); err != nil {
			v.add("%s: %v", at, err)
		}
	}
}

func (v *validator) repo(where string, r config.Repo) {
	if parts := strings.Split(r.Name, "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(r.Name, " \t") {
		v.add("%s: repo.name %q must be owner/name", where, r.Name)
	}
	if r.DefaultBranch == "" {
		v.add("%s: repo.defaultBranch is required", where)
	}
}

func (v *validator) images(where string, p config.Project, phases map[string]string, haveLines bool) {
	names := map[string]bool{}
	for i, img := range p.Images {
		at := fmt.Sprintf("%s: images[%d]", where, i)
		switch {
		case img.Name == "":
			v.add("%s: name is required", at)
		case strings.ContainsAny(img.Name, " \t"):
			v.add("%s: name %q must not contain spaces", at, img.Name)
		case names[img.Name]:
			v.add("%s: image %q is listed twice", at, img.Name)
		}
		names[img.Name] = true
		if img.Since != "" {
			_, known := phases[img.Since]
			switch {
			case len(p.Release.Lines) == 0:
				v.add("%s: since %q needs release.lines, and this project lists none", at, img.Since)
			case !haveLines:
				v.add("%s: since %q needs a RancherLines document, and there is none", at, img.Since)
			case !known:
				v.add("%s: since %q is not in RancherLines", at, img.Since)
			}
		}
		roles := map[string]bool{}
		for j, role := range img.Registries {
			rat := fmt.Sprintf("%s: registries[%d]", at, j)
			v.oneOf(rat, "registry", role, config.RegistryDockerHub, config.RegistryPrimeStaging, config.RegistryPrimeProduction)
			if roles[role] {
				v.add("%s: registry %q is listed twice", rat, role)
			}
			roles[role] = true
		}
	}
}

func (v *validator) project(p config.Project, phases map[string]string, haveLines bool) {
	where := fmt.Sprintf("Project %q", p.Name)
	v.repo(where, p.Repo)
	v.oneOf(where, "release.branching", p.Release.Branching, config.BranchingSingle, config.BranchingPerLine)
	if p.Release.TagPattern == "" {
		v.add("%s: release.tagPattern is required", where)
	}

	v.images(where, p, phases, haveLines)
	v.methods(where, p, phases, haveLines)
	v.edges(where, p, phases, haveLines)
	textVars := slices.Clone(templates.TextVars)
	for _, m := range p.UpdateMethods {
		textVars = append(textVars, templates.StrategyVars[m.Strategy]...)
	}
	if p.PullRequests.PushTo != "" {
		v.oneOf(where, "pullRequests.pushTo", p.PullRequests.PushTo, "fork", "upstream")
	}
	for field, text := range map[string]string{
		"pullRequests.title": p.PullRequests.Title, "pullRequests.branchPrefix": p.PullRequests.BranchPrefix,
	} {
		if err := templates.Validate(text, textVars); err != nil {
			v.add("%s: %s %q: %v", where, field, text, err)
		}
	}

	seen := map[string]bool{}
	for i, l := range p.Release.Lines {
		at := fmt.Sprintf("%s: release.lines[%d]", where, i)
		if l.Line == "" {
			v.add("%s: line is required", at)
			continue
		}
		if seen[l.Line] {
			v.add("%s: line %q is listed twice", at, l.Line)
		}
		seen[l.Line] = true
		phase, known := phases[l.Line]
		switch {
		case !haveLines:
			v.add("%s: line %q needs a RancherLines document, and there is none", at, l.Line)
		case !known:
			v.add("%s: line %q is not in RancherLines", at, l.Line)
		case l.Branch == "" && phase != config.PhaseDevelopment:
			v.add("%s: line %q has no branch and is in phase %q; only a development line uses the default branch", at, l.Line, phase)
		}
	}

}

func (v *validator) external(x config.External, projects map[string]bool) {
	where := fmt.Sprintf("External %q", x.Name)
	v.repo(where, x.Repo)
	seen := map[string]bool{}
	for i, r := range x.Relationships {
		at := fmt.Sprintf("%s: relationships[%d]", where, i)
		if !projects[r.Project] {
			v.add("%s: project %q is not a Project", at, r.Project)
		}
		v.oneOf(at, "action", r.Action, config.ActionTrackOnly, config.ActionReact, config.ActionNotify)
		if seen[r.Project] {
			v.add("%s: more than one action for project %q; a pair of projects has one", at, r.Project)
		}
		seen[r.Project] = true
	}
}

// repoSlugsAreUnique rejects two documents that claim the same repository,
// ignoring case as GitHub does, so a slug resolves to one document.
func (v *validator) repoSlugsAreUnique(c config.Config) {
	owner := map[string]string{} // lower-cased slug to the document that holds it
	claim := func(where, slug string) {
		if slug == "" {
			return
		}
		key := strings.ToLower(slug)
		if prev, taken := owner[key]; taken {
			v.add("%s: repo.name %q is also used by %s", where, slug, prev)
			return
		}
		owner[key] = where
	}
	for _, p := range c.Projects {
		claim(fmt.Sprintf("Project %q", p.Name), p.Repo.Name)
	}
	for _, x := range c.Externals {
		claim(fmt.Sprintf("External %q", x.Name), x.Repo.Name)
	}
}
