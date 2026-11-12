package chain

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

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

// Named reports whether a person's name for the project is this one: its config
// name, its owner/name slug or one of its aliases, whatever the case. It is what
// Graph.Project finds a project by.
func (p *Project) Named(key string) bool {
	return strings.EqualFold(p.Name(), key) || strings.EqualFold(p.Slug(), key) ||
		slices.ContainsFunc(p.Aliases(), func(a string) bool { return strings.EqualFold(a, key) })
}

// ListsLine reports whether the project lists the line itself for its branches,
// written with or without its v, in any case. A single layout lists none: it keeps
// every line on one branch, and ListsLine is not what says it serves one.
func (p *Project) ListsLine(line string) bool {
	_, ok := p.LineID(line)
	return ok
}

// LineID returns the id the project lists a line under, for a line written with or
// without its v, in any case; ok is false when the project does not list it. Like
// ListsLine it knows only the lines listed for branches.
func (p *Project) LineID(line string) (id string, ok bool) {
	i := slices.IndexFunc(p.Lines(), func(l string) bool { return lines.Normalize(l) == lines.Normalize(line) })
	if i < 0 {
		return "", false
	}
	return p.Lines()[i], true
}

// LinesInTags reports whether the project finds its lines in its tags: one branch
// for them all, and streams that list the lines. ReleaseTargets then has a row for
// each line.
func (p *Project) LinesInTags() bool { return false }

// StreamListsLine reports whether a stream of the project lists the line itself,
// which a universal stream does not: its releases belong to no line.
func (p *Project) StreamListsLine(line string) bool {
	return slices.ContainsFunc(p.streams, func(s *Stream) bool {
		return slices.ContainsFunc(s.Lines(), func(l string) bool { return lines.Normalize(l) == lines.Normalize(line) })
	})
}

// DestinationHoldings returns the holdings of a release of the project in the final
// destination that are for the line.
func (p *Project) DestinationHoldings(line string) []*Holding {
	var out []*Holding
	for _, h := range p.sourced {
		if h.Holder().FinalDestination() && h.Kind() == HoldsRelease && h.AppliesTo(line) {
			out = append(out, h)
		}
	}
	return out
}

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

// ReleaseVersion is the version of a release of the project, through the stream
// the tag is a release of. A tag no stream matches loses only its v.
func (p *Project) ReleaseVersion(tag string) string {
	if s := p.StreamOf(tag); s != nil {
		return s.Version(tag)
	}
	return strings.TrimPrefix(tag, "v")
}

var (
	// ErrNoStream is returned by OnlyStream for a project with no stream.
	ErrNoStream = errors.New("the project has no stream")
	// ErrSeveralStreams is returned by OnlyStream for a project with more than one.
	ErrSeveralStreams = errors.New("the project has several streams")
)

// OnlyStream returns the one stream of a project. A project with none, or with
// several, has no one stream, and the caller names it: the error wraps
// ErrNoStream or ErrSeveralStreams.
func (p *Project) OnlyStream() (*Stream, error) {
	switch len(p.streams) {
	case 0:
		return nil, &streamCountError{kind: ErrNoStream, msg: p.Slug() + " has no stream"}
	case 1:
		return p.streams[0], nil
	}
	return nil, &streamCountError{kind: ErrSeveralStreams, msg: fmt.Sprintf("%s has %d streams", p.Slug(), len(p.streams))}
}

// streamCountError says how many streams a project has, and is the sentinel for it.
type streamCountError struct {
	kind error
	msg  string
}

func (e *streamCountError) Error() string { return e.msg }
func (e *streamCountError) Unwrap() error { return e.kind }

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
	return p.ListsLine(line)
}

// serves reports whether a stream of the project serves the line, or the project
// lists it itself.
func (p *Project) serves(line string) bool {
	for _, s := range p.streams {
		if s.Serves(line) {
			return true
		}
	}
	return p.ListsLine(line)
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
	if b, ok := p.BranchOfLine(arg, ""); ok && p.knowsLine(arg) {
		return b, arg, nil
	}
	if slices.Contains(p.ReleaseBranches(), arg) {
		return arg, p.LineOfBranch(arg), nil
	}
	have := p.Lines()
	if len(have) == 0 {
		have = p.ReleaseBranches()
	}
	return "", "", fmt.Errorf("%s has no release line or branch %q: it has %s", p.Slug(), arg, strings.Join(have, ", "))
}

// knowsLine reports whether the line is one the project lists, or one a stream of
// it gives a series. A single layout lists none, and answers to every line in
// BranchOfLine, so this is what tells a line from any other text.
func (p *Project) knowsLine(line string) bool {
	if p.ListsLine(line) {
		return true
	}
	return slices.ContainsFunc(p.streams, func(s *Stream) bool {
		_, ok := s.Series(line)
		return ok
	})
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

// ReleaseTarget is one release branch of a project and what it is for.
type ReleaseTarget struct {
	// Line and Series are set for a project with a branch per line, and for each
	// line the streams of a single layout list. Series is empty for a line no stream
	// gives one.
	Line, Series string
	// Stage is set for a line that has a branch for each stage.
	Stage string
	// Branch is empty for a line that has none yet.
	Branch string
	// IsDefault is set when Branch is the project's default branch because the line
	// has none of its own, or the layout is single.
	IsDefault bool
}

// ReleaseTargets returns the release branches of the project, in the order of its
// lines: a row for each listed line, and for each stage of a line that has a
// branch for each. A single layout has one branch, with a row for each line its
// streams list, or the one row of a project whose streams list none.
func (p *Project) ReleaseTargets() []ReleaseTarget {
	if p.single() {
		var out []ReleaseTarget
		for _, s := range p.streams {
			for _, line := range s.Lines() {
				out = append(out, ReleaseTarget{Line: line, Series: p.SeriesOf(line), Branch: p.DefaultBranch(), IsDefault: true})
			}
		}
		if len(out) == 0 {
			return []ReleaseTarget{{Branch: p.DefaultBranch(), IsDefault: true}}
		}
		return out
	}
	var out []ReleaseTarget
	for _, lb := range p.LineBranches() {
		series := p.SeriesOf(lb.Line)
		if len(lb.Stages) == 0 {
			out = append(out, ReleaseTarget{Line: lb.Line, Series: series, Branch: lb.Branch, IsDefault: lb.IsDefault})
			continue
		}
		for _, stage := range p.Stages() {
			if branch, ok := lb.Stages[stage]; ok {
				out = append(out, ReleaseTarget{Line: lb.Line, Series: series, Stage: stage, Branch: branch})
			}
		}
	}
	return out
}

// ReleaseTargetsOf returns the rows of ReleaseTargets for one line, written with or
// without its v, in any case.
func (p *Project) ReleaseTargetsOf(line string) []ReleaseTarget {
	var out []ReleaseTarget
	for _, t := range p.ReleaseTargets() {
		if t.Line != "" && lines.Normalize(t.Line) == lines.Normalize(line) {
			out = append(out, t)
		}
	}
	return out
}

// SeriesOf is the version series a stream of the project gives a line, the first to
// give one; empty when none does.
func (p *Project) SeriesOf(line string) string {
	for _, s := range p.streams {
		if series, ok := s.Series(line); ok {
			return series
		}
	}
	return ""
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
