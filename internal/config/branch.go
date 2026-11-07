package config

import (
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/git"
)

// Branch returns the branch a line of the Project lives on, and whether that
// is the Project's default branch because the config names none. The config
// stores only what was written (ProjectLine.Branch is the raw value, empty for
// a development line); callers that need to know where the line's work
// happens use this instead of reading the field.
func (p Project) Branch(l ProjectLine) (branch string, isDefault bool) {
	if l.Branch == "" {
		return p.Repo.DefaultBranch, true
	}
	return l.Branch, false
}

// IsUniversal reports whether the Project is not tied to a Rancher line: it
// lists no release.lines, and "universal" is the default (config/README.md).
func (p Project) IsUniversal() bool {
	return len(p.Release.Lines) == 0
}

// ReleaseBranches returns the branches a release of the Project touches, once
// each, in config order: the default branch for release.branching "single", and
// the effective branch of each line (see Branch) for "perLine".
func (p Project) ReleaseBranches() []string {
	if p.Release.Branching != BranchingPerLine {
		return []string{p.Repo.DefaultBranch}
	}
	var out []string
	for _, l := range p.Release.Lines {
		if b, _ := p.Branch(l); !slices.Contains(out, b) {
			out = append(out, b)
		}
	}
	return out
}

// Series returns the version series of a line's tags in this Project: the
// version written for it ("9.x" for kuberlr-kubectl on v2.16), else the line
// itself ("v2.15"), which is how rancher/rancher tags its releases. It is the
// pattern git.InSeries matches tags against.
func (l ProjectLine) Series() string {
	if l.Version != "" {
		return l.Version
	}
	return l.Line
}

// LineOf returns the Rancher line a tag belongs to in this Project, found by
// matching the tag against the series of each of its release lines, or "" when
// it is in none. A Project with no release lines is in no line.
func (p Project) LineOf(tag string) string {
	for _, l := range p.Release.Lines {
		if git.InSeries(tag, l.Series()) {
			return l.Line
		}
	}
	return ""
}

// BranchOfLine returns the branch the Project keeps a Rancher line on, and
// whether it has the line. The line may be written without the v, in any case. For release
// branches perLine it is the line's own branch, or the default branch for a line
// with none; for single it is the default branch, whatever the line says, as
// ReleaseBranches does.
func (p Project) BranchOfLine(line string) (branch string, ok bool) {
	if !strings.HasPrefix(strings.ToLower(line), "v") {
		line = "v" + line
	}
	for _, l := range p.Release.Lines {
		if !strings.EqualFold(l.Line, line) {
			continue
		}
		if p.Release.Branching == BranchingSingle {
			return p.Repo.DefaultBranch, true
		}
		b, _ := p.Branch(l)
		return b, true
	}
	return "", false
}

// Steward returns the team responsible for the project: the one it names, else
// orbs when the project is owned, else "" for an unowned project that names none,
// which is shown as unspecified and never assumed to be orbs.
func (p Project) Steward() string {
	switch {
	case p.Repo.ManagedBy != "":
		return p.Repo.ManagedBy
	case p.Repo.Owned:
		return TeamOrbs
	}
	return ""
}

// TagSchemeStages returns the stages a tag scheme gives a tag, or none for a
// scheme whose tags carry no stage.
func TagSchemeStages(scheme string) []string {
	if scheme == TagSchemeCalverLineStage {
		return []string{StageDev, StageRelease}
	}
	return nil
}
