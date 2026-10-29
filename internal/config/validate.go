package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The values an enumerated field may take (config/README.md).
const (
	BranchesSingle  = "single"
	BranchesPerLine = "perLine"

	AcceptStable = "stable"
	AcceptAll    = "all"

	AcceptLinesAll       = "all"
	AcceptLinesUniversal = "universal"

	ActionTrackOnly = "trackOnly"
	ActionReact     = "react"
	ActionNotify    = "notify"

	PhaseDevelopment = "development"
	PhaseGA          = "ga"
	PhaseEOM         = "eom"
	PhaseEOL         = "eol"
)

// Validate checks c after WithDefaults, so an omitted field with a default is
// already filled in. It reports every problem it finds, each naming the
// document and field, as one joined error; it returns nil when c is valid.
func (c Config) Validate() error {
	var v validator
	projects := map[string]bool{}
	for _, p := range c.Projects {
		projects[p.Name] = true
	}
	phases := map[string]string{} // line id to phase
	if c.Lines != nil {
		v.lines(c.Lines, phases)
	}
	for _, p := range c.Projects {
		v.project(p, projects, phases, c.Lines != nil)
	}
	for _, x := range c.Externals {
		v.external(x, projects)
	}
	v.repoSlugsAreUnique(c)
	return errors.Join(v.errs...)
}

type validator struct{ errs []error }

func (v *validator) add(format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

func (v *validator) oneOf(where, field, got string, allowed ...string) {
	if !slices.Contains(allowed, got) {
		v.add("%s: %s must be one of %s, got %q", where, field, strings.Join(allowed, ", "), got)
	}
}

func (v *validator) lines(l *RancherLines, phases map[string]string) {
	for i, ln := range l.Lines {
		at := fmt.Sprintf("RancherLines %q: lines[%d]", l.Name, i)
		if ln.ID == "" {
			v.add("%s: id is required", at)
			continue
		}
		if _, dup := phases[ln.ID]; dup {
			v.add("%s: duplicate line %q", at, ln.ID)
		} else {
			phases[ln.ID] = ln.Phase
		}
		v.oneOf(at, "phase", ln.Phase, PhaseDevelopment, PhaseGA, PhaseEOM, PhaseEOL)
	}
}

func (v *validator) repo(where string, r Repo) {
	if parts := strings.Split(r.Name, "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(r.Name, " \t") {
		v.add("%s: repo.name %q must be owner/name", where, r.Name)
	}
	if r.DefaultBranch == "" {
		v.add("%s: repo.defaultBranch is required", where)
	}
}

func (v *validator) project(p Project, projects map[string]bool, phases map[string]string, haveLines bool) {
	where := fmt.Sprintf("Project %q", p.Name)
	v.repo(where, p.Repo)
	v.oneOf(where, "release.branches", p.Release.Branches, BranchesSingle, BranchesPerLine)
	if p.Release.TagPattern == "" {
		v.add("%s: release.tagPattern is required", where)
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
		case l.Branch == "" && phase != PhaseDevelopment:
			v.add("%s: line %q has no branch and is in phase %q; only a development line uses the default branch", at, l.Line, phase)
		}
	}

	for i, s := range p.Sources {
		at := fmt.Sprintf("%s: sources[%d]", where, i)
		switch {
		case s.From == "":
			v.add("%s: from is required", at)
		case s.From == p.Name:
			v.add("%s: from %q is the project itself", at, s.From)
		case !projects[s.From]:
			v.add("%s: from %q is not a Project", at, s.From)
		}
		v.oneOf(at, "accept.when", s.Accept.When, AcceptStable, AcceptAll)
		v.oneOf(at, "accept.lines", s.Accept.Lines, AcceptLinesAll, AcceptLinesUniversal)
		if s.Update.Strategy == "" {
			v.add("%s: update.strategy is required", at)
		}
	}
}

func (v *validator) external(x External, projects map[string]bool) {
	where := fmt.Sprintf("External %q", x.Name)
	v.repo(where, x.Repo)
	seen := map[string]bool{}
	for i, r := range x.Relationships {
		at := fmt.Sprintf("%s: relationships[%d]", where, i)
		if !projects[r.Project] {
			v.add("%s: project %q is not a Project", at, r.Project)
		}
		v.oneOf(at, "action", r.Action, ActionTrackOnly, ActionReact, ActionNotify)
		if seen[r.Project] {
			v.add("%s: more than one action for project %q; a pair of projects has one", at, r.Project)
		}
		seen[r.Project] = true
	}
}

// repoSlugsAreUnique rejects two documents that claim the same repository,
// ignoring case as GitHub does, so a slug resolves to one document.
func (v *validator) repoSlugsAreUnique(c Config) {
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
