package config

import "slices"

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
// each, in config order: the default branch for release.branches "single", and
// the effective branch of each line (see Branch) for "perLine".
func (p Project) ReleaseBranches() []string {
	if p.Release.Branches != BranchesPerLine {
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
