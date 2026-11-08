package chain

import "github.com/rancher/orbs-automation/internal/apis/v1alpha2"

// Project is a repo as a place, with the streams it releases.
type Project struct {
	Doc *v1alpha2.Project
}

// Name is the name of the document.
func (p *Project) Name() string { return "" }

// Slug is the repo's owner/name.
func (p *Project) Slug() string { return "" }

// Steward is the team responsible for the project: managedBy if given, else orbs
// if the repo is owned, else none. A repo outside the organization has none.
func (p *Project) Steward() string { return "" }

// DefaultBranch is the repo's default branch.
func (p *Project) DefaultBranch() string { return "" }

// Streams returns the streams the project releases, in the order read.
func (p *Project) Streams() []*Stream { return nil }

// Lines returns the ids of the lines the project lists, as written.
func (p *Project) Lines() []string { return nil }

// BranchOfLine returns the branch a line is on, for a stage when the line has a
// branch for each. The line may be written with or without its v, in any case. A
// single layout has the default branch for every line, and a line in
// development with no branch written is on the default branch. ok is false when
// the project does not list the line, when the line has no branch yet, or when
// the stage has none.
func (p *Project) BranchOfLine(_, _ string) (branch string, ok bool) { return "", false }

// LineOfBranch returns the line a branch is of: the first line whose branch, or
// a stage's branch, it is. It is empty when there is none, and always for a
// single layout, whose branch holds every line.
func (p *Project) LineOfBranch(_ string) string { return "" }

// ReleaseBranches returns the branches a release is made on: the default branch
// for a single layout, else the distinct branches of the lines, in order.
func (p *Project) ReleaseBranches() []string { return nil }
