package chain

import (
	"maps"
	"slices"

	"github.com/rancher/orbs-automation/internal/apis/lines"
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

// Aliases returns the shorthand names the Project is also found by.
func (p *Project) Aliases() []string { return p.Doc.Spec.Aliases }

// ManagedBy is the team responsible for the project, as written: none when it
// names none, and always none for a repo outside the organization. It may be
// shared (Shared).
func (p *Project) ManagedBy() string {
	if p.Doc.Spec.Repo.External {
		return ""
	}
	return p.Doc.Spec.Repo.ManagedBy
}

// Owned reports whether the team owns the project: it is the one that manages it,
// and gero ships from what the active team owns. A project nobody manages, a
// shared one and one outside the organization are owned by no team.
func (p *Project) Owned(team string) bool {
	by := p.ManagedBy()
	return team != "" && by == team && by != v1alpha2.ManagedByShared
}

// Shared reports whether the project is managed by every team together: no team
// owns it, and any may write into it.
func (p *Project) Shared() bool { return p.ManagedBy() == v1alpha2.ManagedByShared }

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
func (p *Project) single() bool { return p.Doc.Spec.Branches.Layout != LayoutPerLine }

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
		if lines.Normalize(id) == lines.Normalize(line) {
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
		if lines.Normalize(bl.Line) != lines.Normalize(line) {
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

// ResolveLineOrBranch finds what a person typed to name where a release is made:
// a release line, written with or without its v, or one of the project's release
// branches. It returns the branch, and the line the branch is of, which is empty
// for a branch in no line. It is an error naming what the project has when the
// text is neither.
func (p *Project) ResolveLineOrBranch(arg string) (branch, line string, err error) {
	return "", "", nil
}

// External reports whether the repo is outside the organization: gero never ships
// into it.
func (p *Project) External() bool { return p.Doc.Spec.Repo.External }

// Layout is how the project's releases are laid out over branches: single or
// perLine.
func (p *Project) Layout() string { return p.Doc.Spec.Branches.Layout }

// Stages are the stages of the project's branch layout, for a layout whose stages
// are branches.
func (p *Project) Stages() []string { return p.Doc.Spec.Branches.Stages }

// LineBranch is a listed line and where it is.
type LineBranch struct {
	Line string
	// Branch is the branch of the line: the default branch when none is written
	// (IsDefault), and empty when the line has none yet (Pending) or has one for
	// each stage (Stages).
	Branch    string
	IsDefault bool
	Pending   bool
	Stages    map[string]string
}

// LineBranches returns the listed lines with their branches, in the order written;
// none for a single layout, which lists none.
func (p *Project) LineBranches() []LineBranch {
	var out []LineBranch
	for _, bl := range p.Doc.Spec.Branches.Lines {
		lb := LineBranch{Line: bl.Line}
		switch {
		case bl.Branch == nil:
			lb.Branch, lb.IsDefault = p.DefaultBranch(), true
		case bl.Branch.Stages != nil:
			lb.Stages = maps.Clone(bl.Branch.Stages)
		case bl.Branch.Name == "":
			lb.Pending = true
		default:
			lb.Branch = bl.Branch.Name
		}
		out = append(out, lb)
	}
	return out
}

// BranchesOfStage returns the branches a release of a stage goes to: the branch of
// each listed line for the stage, where a line has one for each, and its one
// branch where it has one. With no stage it is the release branches. A single
// layout has its default branch.
func (p *Project) BranchesOfStage(stage string) []string {
	if stage == "" || p.single() {
		return p.ReleaseBranches()
	}
	var out []string
	for _, bl := range p.Doc.Spec.Branches.Lines {
		var b string
		switch {
		case bl.Branch == nil:
			b = p.DefaultBranch()
		case bl.Branch.Stages != nil:
			b = bl.Branch.Stages[stage]
		default:
			b = bl.Branch.Name
		}
		if b != "" && !slices.Contains(out, b) {
			out = append(out, b)
		}
	}
	return out
}

// PullRequests are the conventions for the pull requests opened against the
// project's repo.
func (p *Project) PullRequests() v1alpha2.PullRequests { return p.Doc.Spec.PullRequests }

// FinalDestination reports whether releases end in the project: the others ship
// to it, directly or through another project, and it ships to none.
func (p *Project) FinalDestination() bool { return p.Doc.Spec.FinalDestination }
