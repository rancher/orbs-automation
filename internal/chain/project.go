package chain

import (
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
)

// Project is a repo as a place, with the streams it releases.
type Project struct {
	Doc     *v1alpha2.Project
	graph   *Graph
	streams []*Stream

	artifacts []*Artifact
	held      []*Holding
	sourced   []*Holding
}

// Name is the name of the document.
func (p *Project) Name() string { return p.Doc.Name }

// Slug is the repo's owner/name.
func (p *Project) Slug() string { return p.Doc.Spec.Repo.Name }

// Steward is the team responsible for the project: managedBy if given, else orbs
// if the repo is owned, else none. A repo outside the organization has none.
func (p *Project) Steward() string {
	repo := p.Doc.Spec.Repo
	switch {
	case repo.External:
		return ""
	case repo.ManagedBy != "":
		return repo.ManagedBy
	case repo.Owned:
		return "orbs"
	}
	return ""
}

// DefaultBranch is the repo's default branch.
func (p *Project) DefaultBranch() string { return p.Doc.Spec.Repo.DefaultBranch }

// Streams returns the streams the project releases, in the order read.
func (p *Project) Streams() []*Stream { return p.streams }

// Artifacts returns what the releases of every stream of the project produce, in
// the order read.
func (p *Project) Artifacts() []*Artifact { return p.artifacts }

// Held returns the holdings in the project's repo, in the order read.
func (p *Project) Held() []*Holding { return p.held }

// Sourced returns the holdings, in other projects or its own, of something of the
// project, in the order read.
func (p *Project) Sourced() []*Holding { return p.sourced }

// StreamOf returns the stream a tag is a release of: the first of the project's
// streams whose pattern the tag matches and none of whose excludes it matches. It
// is nil when there is none.
func (p *Project) StreamOf(tag string) *Stream {
	for _, s := range p.streams {
		if s.TagMatches(tag) {
			return s
		}
	}
	return nil
}

// LineOf returns the line a tag is of, through the stream it is a release of.
func (p *Project) LineOf(tag string) (line string, ok bool) {
	if s := p.StreamOf(tag); s != nil {
		return s.LineOf(tag)
	}
	return "", false
}

// single reports whether every release is on the default branch.
func (p *Project) single() bool { return p.Doc.Spec.Branches.Layout != v1alpha2.LayoutPerLine }

// Lines returns the ids of the lines the project lists, as written.
func (p *Project) Lines() []string {
	var ids []string
	for _, bl := range p.Doc.Spec.Branches.Lines {
		ids = append(ids, bl.Line)
	}
	return ids
}

// lists reports whether the project lists the line for its branches: a single
// layout keeps every line on its one branch, so it lists every line.
func (p *Project) lists(line string) bool {
	if p.single() {
		return true
	}
	return p.listsExplicitly(line)
}

// listsExplicitly reports whether the line is one of the project's branch lines.
func (p *Project) listsExplicitly(line string) bool {
	for _, id := range p.Lines() {
		if lineKey(id) == lineKey(line) {
			return true
		}
	}
	return false
}

// serves reports whether a stream of the project serves the line, or the project
// lists it itself.
func (p *Project) serves(line string) bool {
	for _, s := range p.streams {
		if s.Serves(line) {
			return true
		}
	}
	return p.listsExplicitly(line)
}

// lineKey writes a line id the one way lines are compared: lowercase, with its v.
func lineKey(line string) string {
	line = strings.ToLower(line)
	if !strings.HasPrefix(line, "v") {
		line = "v" + line
	}
	return line
}

// BranchOfLine returns the branch a line is on, for a stage when the line has a
// branch for each. The line may be written with or without its v, in any case. A
// single layout has the default branch for every line, and a line in
// development with no branch written is on the default branch. ok is false when
// the project does not list the line, when the line has no branch yet, or when
// the stage has none.
func (p *Project) BranchOfLine(line, stage string) (branch string, ok bool) {
	if p.single() {
		return p.DefaultBranch(), true
	}
	for _, bl := range p.Doc.Spec.Branches.Lines {
		if lineKey(bl.Line) != lineKey(line) {
			continue
		}
		switch {
		case bl.Branch == nil:
			return p.DefaultBranch(), true
		case bl.Branch.Stages != nil:
			b := bl.Branch.Stages[stage]
			return b, stage != "" && b != ""
		}
		return bl.Branch.Name, bl.Branch.Name != ""
	}
	return "", false
}

// branchesOfLine is every branch a listed line is on: the one, or one for each
// stage in the order of the project's stages.
func (p *Project) branchesOfLine(bl v1alpha2.BranchLine) []string {
	switch {
	case bl.Branch == nil:
		return []string{p.DefaultBranch()}
	case bl.Branch.Stages != nil:
		var out []string
		for _, stage := range p.Doc.Spec.Branches.Stages {
			if b := bl.Branch.Stages[stage]; b != "" {
				out = append(out, b)
			}
		}
		return out
	case bl.Branch.Name != "":
		return []string{bl.Branch.Name}
	}
	return nil
}

// LineOfBranch returns the line a branch is of: the first line whose branch, or
// a stage's branch, it is. It is empty when there is none, and always for a
// single layout, whose branch holds every line.
func (p *Project) LineOfBranch(branch string) string {
	if branch == "" || p.single() {
		return ""
	}
	for _, bl := range p.Doc.Spec.Branches.Lines {
		for _, b := range p.branchesOfLine(bl) {
			if b == branch {
				return bl.Line
			}
		}
	}
	return ""
}

// ReleaseBranches returns the branches a release is made on: the default branch
// for a single layout, else the distinct branches of the lines, in order.
func (p *Project) ReleaseBranches() []string {
	if p.single() {
		return []string{p.DefaultBranch()}
	}
	var out []string
	seen := map[string]bool{}
	for _, bl := range p.Doc.Spec.Branches.Lines {
		for _, b := range p.branchesOfLine(bl) {
			if !seen[b] {
				seen[b] = true
				out = append(out, b)
			}
		}
	}
	return out
}
