package ship

import (
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/github/pulls"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui"
)

// humanSink shows the events of a run as lines and tables on an Output, as they
// arrive: the title, each branch read, the images, what the PR would be, each
// commit as it is made, and a summary at the end. The branches read are a table,
// held until the last is in so that it aligns, and everything else streams.
type humanSink struct {
	out  ui.Output
	full bool // show the commit message and body of each PR too

	header    domain.Plan         // from Resolved: who ships what to whom
	targets   []domain.TargetPlan // the branches read so far
	inTargets bool                // the table of branches is open
	wantShown bool                // the value the consumer should hold has been shown
	prShown   bool
	working   bool // the section for the work on branches has been started
	deciding  bool // the section for the open PRs of each branch has been started
}

var _ domain.Sink = (*humanSink)(nil)

func newHumanSink(out ui.Output, full bool) *humanSink { return &humanSink{out: out, full: full} }

// Event shows one event.
func (h *humanSink) Event(e domain.Event) {
	switch e.Kind {
	case domain.EventResolved:
		h.resolved(e.Plan)
	case domain.EventTargetChecked:
		h.targetChecked(e.Target)
	case domain.EventTargetsChecked:
		h.endTargets()
	case domain.EventImagesChecked:
		h.endTargets()
		h.images(e.Images)
		h.pr()
	case domain.EventDecided:
		h.endTargets()
		h.pr()
		h.decided(e)
	case domain.EventTargetStarted:
		h.endTargets()
		h.pr()
		h.started(e)
	case domain.EventCommit:
		h.out.Line(fmt.Sprintf("    %s  %s", git.Short(e.Commit.Hash), e.Commit.Subject))
	case domain.EventPushed:
		h.out.Line(fmt.Sprintf("  %s: pushed to %s", e.Branch, e.Remote))
	case domain.EventPROpened:
		h.out.Line(fmt.Sprintf("  %s: opened #%d %s", e.Branch, e.PR.Number, e.PR.URL))
	case domain.EventSuperseded:
		h.out.Line(fmt.Sprintf("  %s: closed #%d", e.Branch, e.PR.Number))
	case domain.EventRefreshed:
		h.out.Line(fmt.Sprintf("  %s: refreshed #%d %s", e.Branch, e.PR.Number, e.PR.URL))
	case domain.EventTargetDone:
		h.done(e)
	case domain.EventDone:
		h.endTargets()
		h.pr()
		h.summary(e.Plan)
	}
}

func (h *humanSink) resolved(p *domain.Plan) {
	kind := "stable"
	if !p.Release.Stable {
		kind = "prerelease"
	}
	h.out.Line(fmt.Sprintf("Ship %s %s (%s) to %s", p.Producer, p.Release.Tag, kind, p.Consumer))
	h.out.Line("")
	h.header = *p
}

// targetChecked adds a branch to the table of branches, which is opened by the
// first one, after the method and the wanted value that its read made known.
func (h *humanSink) targetChecked(t *domain.TargetPlan) {
	if !h.inTargets {
		method := fmt.Sprintf("  method  %s (%s", h.header.Method, h.header.Strategy)
		if t.File != "" {
			method += ", " + t.File
		}
		h.out.Line(method + ")")
		if t.Want != "" {
			h.out.Line("  want    " + t.Want)
			h.wantShown = true
		}
		h.out.Line("")
		h.out.Line("Targets")
		h.out.BeginTable("BRANCH", "HEAD", "NOW", "STATUS")
		h.inTargets = true
	}
	h.out.Row(t.Branch, git.Short(t.Commit), t.Current, string(t.Status))
	h.targets = append(h.targets, *t)
}

// endTargets prints the table of branches, with what stands in the way of each
// and what could not be read, when the last branch is in.
func (h *humanSink) endTargets() {
	if !h.inTargets {
		return
	}
	h.out.EndTable()
	h.inTargets = false
	for _, g := range groupTargets(h.targets, func(t domain.TargetPlan) []string { return t.Blockers }) {
		h.out.Line(fmt.Sprintf("  blocked (%s): %s", strings.Join(g.branches, ", "), g.text))
	}
	for _, t := range h.targets {
		if t.Error != "" {
			h.out.Line(fmt.Sprintf("  %s failed: %s", t.Branch, t.Error))
		}
	}
	for _, g := range groupTargets(h.targets, func(t domain.TargetPlan) []string { return t.Warnings }) {
		h.out.Line("")
		h.out.Line(fmt.Sprintf("Warning (%s): %s", strings.Join(g.branches, ", "), g.text))
	}
}

func (h *humanSink) images(r *registry.Report) {
	if r == nil {
		return
	}
	if len(r.Rows) > 0 {
		h.out.Line("")
		h.out.Line("Images")
		h.out.BeginTable("IMAGE", "REGISTRY", "STATUS", "REFERENCE")
		for _, row := range r.Rows {
			status := row.Status
			if row.Detail != "" {
				status += " (" + row.Detail + ")"
			}
			h.out.Row(row.Image, row.Label, status, row.Reference)
		}
		h.out.EndTable()
	}
	for _, w := range r.Warnings {
		h.out.Line("")
		h.out.Line("Warning: " + w)
	}
}

// pr shows what the PR of each branch that needs one would be, once.
func (h *humanSink) pr() {
	if h.prShown {
		return
	}
	h.prShown = true
	prs := previews(domain.Plan{Targets: h.targets})
	if len(prs) == 0 {
		return
	}
	h.out.Line("")
	h.out.Line("PR")
	h.out.BeginTable("BRANCH", "NEW BRANCH", "TITLE")
	for _, t := range prs {
		h.out.Row(t.Branch, t.Preview.Branch, t.Preview.Title)
	}
	h.out.EndTable()
	if labels := prs[0].Preview.Labels; len(labels) > 0 {
		h.out.Line("  labels: " + strings.Join(labels, ", "))
	}
	if h.full {
		for _, t := range prs {
			h.block("Commit message ("+t.Branch+")", t.Preview.CommitMessage)
			h.block("Body ("+t.Branch+")", t.Preview.Body)
		}
	}
}

// block shows a heading and the text under it, indented.
func (h *humanSink) block(heading, text string) {
	h.out.Line("")
	h.out.Line(heading)
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if line == "" {
			h.out.Line("")
			continue
		}
		h.out.Line("  " + line)
	}
}

// decided says what will be done about the PR of a branch, and which older ones
// will be closed.
func (h *humanSink) decided(e domain.Event) {
	if !h.deciding {
		h.deciding = true
		h.out.Line("")
		h.out.Line("Open PRs")
	}
	d := e.Decision
	closing := ""
	if len(d.Supersede) > 0 {
		closing = " " + numbers(d.Supersede) + " will be closed"
	}
	switch d.Action {
	case domain.PublishBlocked:
		h.out.Line(fmt.Sprintf("  %s: blocked by #%d, which is newer", e.Branch, d.Newer.Number))
	case domain.PublishAlreadyOpen:
		h.out.Line(fmt.Sprintf("  %s: already open as #%d%s", e.Branch, d.Open.Number, strings.Replace(closing, " #", ", and #", 1)))
	default:
		if closing != "" {
			closing = ", and" + closing
		}
		h.out.Line(fmt.Sprintf("  %s: a new PR%s", e.Branch, closing))
	}
}

// numbers lists PRs by number: #101, #90.
func numbers(prs []pulls.PR) string {
	out := make([]string, len(prs))
	for i, p := range prs {
		out[i] = fmt.Sprintf("#%d", p.Number)
	}
	return strings.Join(out, ", ")
}

func (h *humanSink) startWork() {
	if !h.working {
		h.working = true
		h.out.Line("")
		h.out.Line("Working")
	}
}

func (h *humanSink) started(e domain.Event) {
	h.startWork()
	newBranch := ""
	for _, t := range h.targets {
		if t.Branch == e.Branch && t.Preview != nil {
			newBranch = t.Preview.Branch
		}
	}
	h.out.Line(fmt.Sprintf("  %s: %s, %s", e.Branch, newBranch, e.Place))
}

func (h *humanSink) done(e domain.Event) {
	h.startWork()
	r := e.Result
	switch r.Status {
	case domain.ResultFailed:
		h.out.Line(fmt.Sprintf("  %s: failed: %s", e.Branch, r.Error))
	case domain.ResultBlocked:
		h.out.Line(fmt.Sprintf("  %s: blocked: %s", e.Branch, r.Error))
	case domain.ResultAlreadyOpen:
		behind := ""
		if r.BehindBy > 0 {
			behind = fmt.Sprintf(" (behind %s by %d; --refresh to rebuild)", e.Branch, r.BehindBy)
		}
		note := ""
		if r.Note != "" {
			note = " (" + r.Note + ")"
		}
		h.out.Line(fmt.Sprintf("  %s: already open #%d %s%s%s", e.Branch, r.PR.Number, r.PR.URL, behind, note))
	case domain.ResultWouldRefresh:
		h.out.Line(fmt.Sprintf("  %s: would refresh #%d", e.Branch, r.PR.Number))
	default:
		h.out.Line(fmt.Sprintf("  %s: %s", e.Branch, r.Status))
	}
}

// summary is the end of the report: what was done for each branch and where it
// is, how to review it, the state of the whole, and what was and was not done.
func (h *humanSink) summary(p *domain.Plan) {
	if hasResults(*p) {
		h.commits(p)
	}
	h.out.Line("")
	if !h.wantShown && p.Want != "" {
		h.out.Line("  want    " + p.Want)
	}
	if p.Push != nil {
		h.out.Line(fmt.Sprintf("  push to  %s (%s), as %s", p.Push.Role, p.Push.Remote, p.Push.Source))
	}
	h.out.Line("  status  " + string(p.Status))
	for _, blocker := range p.Blockers {
		h.out.Line("  blocked: " + blocker)
	}
	h.out.Line("")
	h.out.Line(trailer(p))
}

// commits is the table of what was done for each branch, why a worktree was used
// and what failed, each grouped over the branches it applies to, and how to
// review the branches that were kept.
func (h *humanSink) commits(p *domain.Plan) {
	h.out.Line("")
	h.out.Line("Commits")
	h.out.BeginTable("BRANCH", "RESULT", "WHERE", "NEW BRANCH")
	for _, t := range p.Targets {
		if r := t.Result; r != nil {
			h.out.Row(t.Branch, string(r.Status), string(r.Place), r.Branch)
		}
	}
	h.out.EndTable()
	text := func(field func(*domain.TargetResult) string) func(domain.TargetPlan) []string {
		return func(t domain.TargetPlan) []string {
			if t.Result == nil || field(t.Result) == "" {
				return nil
			}
			return []string{field(t.Result)}
		}
	}
	for _, g := range groupTargets(p.Targets, text(func(r *domain.TargetResult) string { return r.Reason })) {
		h.out.Line(fmt.Sprintf("  worktree (%s): %s", strings.Join(g.branches, ", "), g.text))
	}
	for _, status := range []domain.ResultStatus{domain.ResultFailed, domain.ResultBlocked} {
		only := func(r *domain.TargetResult) string {
			if r.Status != status {
				return ""
			}
			return r.Error
		}
		for _, g := range groupTargets(p.Targets, text(only)) {
			h.out.Line(fmt.Sprintf("  %s (%s): %s", status, strings.Join(g.branches, ", "), g.text))
		}
	}
	for _, t := range p.Targets {
		if r := t.Result; r != nil && len(r.WouldSupersede) > 0 {
			h.out.Line(fmt.Sprintf("  %s: would close %s", t.Branch, refNumbers(r.WouldSupersede)))
		}
	}
	for _, g := range groupTargets(p.Targets, func(t domain.TargetPlan) []string {
		if t.Result == nil {
			return nil
		}
		return t.Result.Warnings
	}) {
		h.out.Line(fmt.Sprintf("  warning (%s): %s", strings.Join(g.branches, ", "), g.text))
	}
	if p.DryRun {
		return // the branches were removed, so there is nothing to review
	}
	var review []string
	for _, t := range p.Targets {
		if r := t.Result; r != nil && r.Status == domain.ResultCommitted {
			rng := fmt.Sprintf("%s/%s", p.Remote, t.Branch)
			review = append(review,
				fmt.Sprintf("  git -C %s log --oneline %s..%s", p.Clone, rng, r.Branch),
				fmt.Sprintf("  git -C %s diff %s...%s", p.Clone, rng, r.Branch))
		}
	}
	if len(review) > 0 {
		h.out.Line("")
		h.out.Line("Review:")
		for _, line := range review {
			h.out.Line(line)
		}
	}
}

// refNumbers lists PR references by number: #101, #90.
func refNumbers(refs []domain.PRRef) string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = fmt.Sprintf("#%d", r.Number)
	}
	return strings.Join(out, ", ")
}

// trailer says what the run did and did not do.
func trailer(p *domain.Plan) string {
	switch {
	case p.DryRun && hasResults(*p):
		return "Dry run: the commits were made in temporary worktrees and removed. Nothing was pushed, and no branch or PR was created."
	case p.DryRun:
		return "Dry run: nothing was written, and no branch or PR was created."
	case p.NoPush && hasResults(*p):
		return "No push: the commits are on branches in your clone, with nothing pushed and no PR created."
	case p.NoPush:
		return "No push: no commit was made."
	}
	opened := 0
	for _, t := range p.Targets {
		if t.Result != nil && t.Result.Status == domain.ResultOpened {
			opened++
		}
	}
	refreshed := 0
	for _, t := range p.Targets {
		if t.Result != nil && t.Result.Status == domain.ResultRefreshed {
			refreshed++
		}
	}
	switch {
	case opened == 1:
		return "Opened 1 PR."
	case opened > 1:
		return fmt.Sprintf("Opened %d PRs.", opened)
	case refreshed == 1:
		return "Refreshed 1 PR."
	case refreshed > 1:
		return fmt.Sprintf("Refreshed %d PRs.", refreshed)
	case hasResults(*p):
		return "No PR was opened."
	}
	return "Nothing was written."
}
