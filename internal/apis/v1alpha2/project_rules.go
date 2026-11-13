package v1alpha2

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/rancher/orbs-automation/internal/platform/templates"
)

// The defaults of a Project, and the layouts and push targets it may name.
const (
	DefaultBranch = "main"

	LayoutSingle  = "single"
	LayoutPerLine = "perLine"

	PushToFork     = "fork"
	PushToUpstream = "upstream"
)

// prVars are the variables a pull request text may use: the ones every text has,
// and the ones a strategy adds, since a text is written for the repo it is opened
// in and the strategy that edits it is another document.
var prVars = func() []string {
	vars := slices.Clone(templates.TextVars)
	for _, extra := range templates.StrategyVars {
		vars = append(vars, extra...)
	}
	return vars
}()

// prefixVars are the variables a branchPrefix may use: those of the other texts,
// less the key, which gero appends to the prefix.
var prefixVars = slices.DeleteFunc(slices.Clone(prVars), func(v string) bool { return v == "key" })

// SetDefaults fills the default branch and the layout. managedBy is not filled:
// whether a team owns the repo depends on the active team, which only the CLI
// settings know, so it is read through a getter.
func (p *Project) SetDefaults() {
	if p.Spec.Repo.DefaultBranch == "" {
		p.Spec.Repo.DefaultBranch = DefaultBranch
	}
	if p.Spec.Branches.Layout == "" {
		p.Spec.Branches.Layout = LayoutSingle
	}
}

// Validate checks what a Project can alone: its repo, its pull request texts and
// its branch layout. What needs another document is checked across documents.
func (p *Project) Validate(_ config.Context, r *config.Report) {
	repo := p.Spec.Repo
	switch owner, name, ok := strings.Cut(repo.Name, "/"); {
	case repo.Name == "":
		r.Errorf("spec.repo.name", "is required")
	case !ok || owner == "" || name == "" || strings.Contains(name, "/"):
		r.Errorf("spec.repo.name", "%q must be owner/name", repo.Name)
	case strings.ContainsAny(repo.Name, " \t"):
		r.Errorf("spec.repo.name", "%q has a space", repo.Name)
	}
	if repo.ManagedBy != "" && !slug.MatchString(repo.ManagedBy) {
		r.Errorf("spec.repo.managedBy", "%q must be a lowercase slug such as release-team", repo.ManagedBy)
	}
	if repo.External && repo.ManagedBy != "" {
		r.Errorf("spec.repo.external", "a repo outside the organization has no managing team: remove managedBy")
	}

	p.validateAliases(r)

	pr := p.Spec.PullRequests
	if pr.PushTo != "" {
		oneOf(r, "spec.pullRequests.pushTo", pr.PushTo, PushToFork, PushToUpstream)
	}
	for path, text := range map[string]string{
		"spec.pullRequests.title":        pr.Title,
		"spec.pullRequests.branchPrefix": pr.BranchPrefix,
	} {
		vars := prVars
		if path == "spec.pullRequests.branchPrefix" {
			vars = prefixVars // the key is appended to the prefix, so it cannot be in it
		}
		if err := templates.Validate(text, vars); err != nil {
			r.Errorf(path, "%v", err)
		}
	}

	p.validateBranches(r)
}

// validateAliases checks that each alias is a lowercase slug, listed once, and
// not the Project's own name, which already finds it. Whether another Project has
// the alias or the name is checked across documents.
func (p *Project) validateAliases(r *config.Report) {
	seen := map[string]bool{}
	for i, alias := range p.Spec.Aliases {
		at := fmt.Sprintf("spec.aliases[%d]", i)
		switch {
		case !slug.MatchString(alias):
			r.Errorf(at, "%q must be a lowercase slug such as rk", alias)
		case seen[alias]:
			r.Errorf(at, "%q is listed twice", alias)
		case alias == p.Name:
			r.Errorf(at, "%q is the name of the Project, which finds it already", alias)
		}
		seen[alias] = true
	}
}

func (p *Project) validateBranches(r *config.Report) {
	b := p.Spec.Branches
	if !oneOf(r, "spec.branches.layout", b.Layout, LayoutSingle, LayoutPerLine) {
		return
	}
	if b.Layout == LayoutSingle && len(b.Lines) > 0 {
		r.Errorf("spec.branches.lines", "are not given with the single layout, which keeps every release on the default branch")
	}

	declared := map[string]bool{}
	for i, st := range b.Stages {
		path := fmt.Sprintf("spec.branches.stages[%d]", i)
		switch {
		case st == "":
			r.Errorf(path, "is empty")
		case declared[st]:
			r.Errorf(path, "%q is listed twice", st)
		}
		declared[st] = true
	}

	seen := map[string]bool{}
	var onDefault []string
	for i, l := range b.Lines {
		at := fmt.Sprintf("spec.branches.lines[%d]", i)
		switch {
		case l.Line == "":
			r.Errorf(at+".line", "is required")
		case seen[l.Line]:
			r.Errorf(at+".line", "%q is listed twice", l.Line)
		default:
			checkLineID(r, at+".line", l.Line)
		}
		seen[l.Line] = true

		if l.Branch == nil {
			onDefault = append(onDefault, l.Line)
			continue
		}
		if l.Branch.Stages == nil {
			continue
		}
		if len(b.Stages) == 0 {
			r.Errorf(at+".branch", "is a branch for each stage, so branches.stages must name the stages")
			continue
		}
		for stage := range l.Branch.Stages {
			if !declared[stage] {
				r.Errorf(at+".branch."+stage, "%q is not a stage of branches.stages (%s)", stage, quoteAll(b.Stages))
			}
		}
	}
	if len(onDefault) > 1 {
		r.Errorf("spec.branches.lines", "%s all use the default branch: give all but one a branch, or \"\" for a line that has none yet", strings.Join(onDefault, ", "))
	}
}
