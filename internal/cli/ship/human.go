package ship

import (
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/github/pulls"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// humanSink shows the events of a run as lines and tables on an Output, as they
// arrive: the title, each branch read, the images, what the PR would be, each
// commit as it is made, and a summary at the end. The branches read are a table,
// held until the last is in so that it aligns, and everything else streams.
type humanSink struct {
	out  ui.Output
	full bool            // show the commit message and body of each PR too
	st   *style.Renderer // how roles look; plain when there is no color

	header    domain.Plan         // from Resolved: who ships what to whom
	targets   []domain.TargetPlan // the branches read so far
	inTargets bool                // the table of branches is open
	wantShown bool                // the value the consumer should hold has been shown
	prShown   bool
	working   bool // the section for the work on branches has been started
	deciding  bool // the section for the open PRs of each branch has been started
}

var _ domain.Sink = (*humanSink)(nil)

func newHumanSink(out ui.Output, full bool) *humanSink {
	return newStyledSink(out, full, style.Plain())
}

// newStyledSink is newHumanSink with the report styled by st.
func newStyledSink(out ui.Output, full bool, st *style.Renderer) *humanSink {
	return &humanSink{out: out, full: full, st: st}
}

// label is the name of a field in a line, as secondary text.
func (h *humanSink) label(text string) string { return h.st.Style(style.Dim, text) }

// heading shows a heading.
func (h *humanSink) heading(text string) { h.out.Line(h.st.Style(style.Heading, text)) }

// table starts a table whose column heads are secondary text.
func (h *humanSink) table(head ...string) {
	styled := make([]string, len(head))
	for i, c := range head {
		styled[i] = h.st.Style(style.Dim, c)
	}
	h.out.BeginTable(styled...)
}

// status is a status word in the look of what it says.
func (h *humanSink) status(text string) string {
	if role, ok := statusRole(text); ok {
		return h.st.Style(role, text)
	}
	return text
}

// statusRole is the look of a status of a target, a result or an image.
func statusRole(status string) (style.Role, bool) {
	switch status {
	case "ready", "found", "opened", "refreshed", "committed":
		return style.OK, true
	case "already up to date", "already open", "would commit", "would refresh":
		return style.Dim, true
	case "blocked":
		return style.Warn, true
	case "failed", "not found", "unreachable":
		return style.Error, true
	}
	return style.Dim, false
}

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
		h.images(e.Release, e.Images)
		h.pr()
	case domain.EventActor:
		h.endTargets()
		h.pr()
		h.actor(e.Actor)
	case domain.EventDecided:
		h.endTargets()
		h.pr()
		h.decided(e)
	case domain.EventTargetStarted:
		h.endTargets()
		h.pr()
		h.started(e)
	case domain.EventCommit:
		h.out.Line(fmt.Sprintf("    %s  %s", h.st.Style(style.Dim, git.Short(e.Commit.Hash)), e.Commit.Subject))
		if files := filesText(e.Commit.Files); files != "" {
			h.out.Line("             " + files) // under the subject: four spaces, the short hash and two more
		}
	case domain.EventPushed:
		h.out.Line(fmt.Sprintf("  %s: %s %s to %s", name(e), h.st.Style(style.OK, "pushed"), e.NewBranch, e.Remote))
	case domain.EventPROpened:
		h.out.Line(fmt.Sprintf("  %s: %s #%d %s", name(e), h.status("opened"), e.PR.Number, e.PR.URL))
	case domain.EventSuperseded:
		h.out.Line(fmt.Sprintf("  %s: %s #%d", name(e), h.st.Style(style.Dim, "closed"), e.PR.Number))
	case domain.EventRefreshed:
		h.out.Line(fmt.Sprintf("  %s: %s #%d %s", name(e), h.status("refreshed"), e.PR.Number, e.PR.URL))
	case domain.EventTargetDone:
		h.done(e)
	case domain.EventDone:
		h.endTargets()
		h.pr()
		h.summary(e.Plan)
	}
}

func (h *humanSink) resolved(p *domain.Plan) {
	switch {
	case len(p.Releases) == 1:
		kind := "stable"
		if !p.Releases[0].Stable {
			kind = "prerelease"
		}
		h.heading(fmt.Sprintf("Ship %s %s (%s) to %s", p.Producer, p.Releases[0].Tag, kind, p.Consumer))
	default:
		// Each target has its own release, which the table of targets names.
		h.heading(fmt.Sprintf("Ship %s to %s", p.Producer, p.Consumer))
	}
	for _, note := range p.Notes {
		h.out.Line("  " + h.label("left out") + "  " + note)
	}
	h.out.Line("")
	h.header = *p
}

// targetChecked adds a branch to the table of branches, which is opened by the
// first one, after the method and the wanted value that its read made known.
func (h *humanSink) targetChecked(t *domain.TargetPlan) {
	if !h.inTargets {
		method := fmt.Sprintf("  %s  %s (%s", h.label("method"), h.header.Method, h.header.Strategy)
		if t.File != "" {
			method += ", " + t.File
		}
		h.out.Line(method + ")")
		if t.Want != "" && h.header.Uniform() {
			h.out.Line("  " + h.label("want") + "    " + t.Want)
			h.wantShown = true
		}
		h.out.Line("")
		h.heading("Targets")
		if h.header.Uniform() {
			h.table("LINE", "BRANCH", "HEAD", "NOW", "STATUS")
		} else {
			h.table("LINE", "BRANCH", "RELEASE", "HEAD", "NOW", "WANT", "STATUS")
		}
		h.inTargets = true
	}
	if h.header.Uniform() {
		h.out.Row(t.Key(), t.Branch, git.Short(t.Commit), t.Current, h.status(string(t.Status)))
	} else {
		h.out.Row(t.Key(), t.Branch, t.Release, git.Short(t.Commit), t.Current, t.Want, h.status(string(t.Status)))
	}
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
		h.out.Line(h.st.Style(style.Warn, fmt.Sprintf("  blocked (%s): %s", strings.Join(g.branches, ", "), g.text)))
	}
	for _, t := range h.targets {
		if t.Error != "" {
			h.out.Line(h.st.Style(style.Error, fmt.Sprintf("  %s failed: %s", t.Key(), t.Error)))
		}
	}
	for _, g := range groupTargets(h.targets, func(t domain.TargetPlan) []string { return t.Warnings }) {
		h.out.Line("")
		h.out.Line(h.st.Style(style.Warn, fmt.Sprintf("Warning (%s): %s", strings.Join(g.branches, ", "), g.text)))
	}
}

func (h *humanSink) images(release string, r *registry.Report) {
	if r == nil {
		return
	}
	if len(r.Rows) > 0 {
		h.out.Line("")
		if h.header.Uniform() {
			h.heading("Images")
		} else {
			h.heading("Images (" + release + ")")
		}
		h.table("IMAGE", "REGISTRY", "STATUS", "REFERENCE")
		for _, row := range r.Rows {
			status := h.status(row.Status)
			if row.Detail != "" {
				status += " (" + row.Detail + ")"
			}
			h.out.Row(row.Image, row.Label, status, row.Reference)
		}
		h.out.EndTable()
	}
	for _, w := range r.Warnings {
		h.out.Line("")
		h.out.Line(h.st.Style(style.Warn, "Warning: "+w))
	}
}

// actor says who the run acts as, before anything is written: the identity of
// its commits, whether they are signed, and the account the PRs are opened by.
func (h *humanSink) actor(a *domain.Actor) {
	signing := "commits not signed"
	if a.Signing {
		signing = "commits signed"
	}
	line := fmt.Sprintf("  %s  %s <%s>, %s", h.label("acting as"), a.Name, a.Email, signing)
	if a.Login != "" {
		line += ", PRs by " + a.Login
	}
	h.out.Line("")
	h.out.Line(line)
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
	h.heading("PR")
	h.table("LINE", "NEW BRANCH", "TITLE")
	for _, t := range prs {
		h.out.Row(t.Key(), t.Preview.Branch, t.Preview.Title)
	}
	h.out.EndTable()
	if labels := prs[0].Preview.Labels; len(labels) > 0 {
		h.out.Line("  labels: " + strings.Join(labels, ", "))
	}
	if h.full {
		for _, t := range prs {
			h.block("Commit message ("+t.Key()+")", t.Preview.CommitMessage)
			h.block("Body ("+t.Key()+")", t.Preview.Body)
			if len(t.Preview.Steps) > 0 {
				h.block("Step commits ("+t.Key()+")", stepsText(t.Preview.Steps))
			}
		}
	}
}

// block shows a heading and the text under it, indented.
func (h *humanSink) block(heading, text string) {
	h.out.Line("")
	h.heading(heading)
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
		h.heading("Open PRs")
	}
	d := e.Decision
	closing := ""
	if len(d.Supersede) > 0 {
		closing = " " + numbers(d.Supersede) + " will be closed"
	}
	switch d.Action {
	case domain.PublishBlocked:
		h.out.Line(h.st.Style(style.Warn, fmt.Sprintf("  %s: blocked by #%d, which is newer", name(e), d.Newer.Number)))
	case domain.PublishAlreadyOpen:
		h.out.Line(fmt.Sprintf("  %s: already open as #%d%s", name(e), d.Open.Number, strings.Replace(closing, " #", ", and #", 1)))
	default:
		if closing != "" {
			closing = ", and" + closing
		}
		h.out.Line(fmt.Sprintf("  %s: a new PR%s", name(e), closing))
	}
}

// name is what the report calls the target an event is about: its line, or its
// branch when it is in none.
func name(e domain.Event) string {
	if e.Line != "" {
		return e.Line
	}
	return e.Branch
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
		h.heading("Working")
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
	h.out.Line(fmt.Sprintf("  %s: %s, %s", name(e), newBranch, e.Place))
}

func (h *humanSink) done(e domain.Event) {
	h.startWork()
	r := e.Result
	switch r.Status {
	case domain.ResultFailed:
		h.out.Line(h.st.Style(style.Error, fmt.Sprintf("  %s: failed: %s", name(e), r.Error)))
	case domain.ResultBlocked:
		h.out.Line(h.st.Style(style.Warn, fmt.Sprintf("  %s: blocked: %s", name(e), r.Error)))
	case domain.ResultAlreadyOpen:
		behind := ""
		if r.BehindBy > 0 {
			hint := "; --refresh to rebuild"
			if h.header.Refresh {
				hint = "" // it was given, so the result says what came of it
			}
			behind = fmt.Sprintf(" (behind %s by %d%s)", e.Branch, r.BehindBy, hint)
		}
		note := ""
		if r.Note != "" {
			note = " (" + r.Note + ")"
		}
		h.out.Line(fmt.Sprintf("  %s: %s #%d %s%s%s", name(e), h.status("already open"), r.PR.Number, r.PR.URL, behind, note))
	case domain.ResultWouldRefresh:
		h.out.Line(fmt.Sprintf("  %s: %s #%d", name(e), h.status("would refresh"), r.PR.Number))
	default:
		h.out.Line(fmt.Sprintf("  %s: %s", name(e), h.status(string(r.Status))))
	}
}

// summary is the end of the report: what was done for each branch and where it
// is, how to review it, the state of the whole, and what was and was not done.
func (h *humanSink) summary(p *domain.Plan) {
	if hasResults(*p) {
		h.results(p)
	}
	h.out.Line("")
	if !h.wantShown && p.Want != "" {
		h.out.Line("  " + h.label("want") + "    " + p.Want)
	}
	if p.Push != nil {
		h.out.Line(fmt.Sprintf("  %s  %s (%s), as %s", h.label("push to"), p.Push.Role, p.Push.Remote, p.Push.Source))
	}
	h.out.Line("  " + h.label("status") + "  " + h.status(string(p.Status)))
	for _, blocker := range p.Blockers {
		h.out.Line(h.st.Style(style.Warn, "  blocked: "+blocker))
	}
	h.out.Line("")
	h.out.Line(h.trailer(p))
	h.links(p)
}

// links ends the report with the PRs the run opened, refreshed or found already
// open, each with its link, so they can be copied or clicked. A run that opened
// none, such as a dry run or one that stopped at the commits, has no list.
func (h *humanSink) links(p *domain.Plan) {
	var rows [][]string
	for _, t := range p.Targets {
		r := t.Result
		if r == nil || r.PR == nil {
			continue
		}
		switch r.Status {
		case domain.ResultOpened, domain.ResultRefreshed, domain.ResultAlreadyOpen:
			rows = append(rows, []string{string(r.Status), t.Key(), t.Branch, r.PR.URL})
		}
	}
	if len(rows) == 0 {
		return
	}
	h.out.Line("")
	h.heading("Pull requests")
	h.table("RESULT", "LINE", "BASE", "LINK")
	for _, row := range rows {
		row[0] = h.status(row[0])
		h.out.Row(row...)
	}
	h.out.EndTable()
}

// results is the table of what came of each target, with where the work was done
// when any was, then why a worktree was used and what failed, each grouped over
// the targets it applies to, and how to review the branches that were kept.
func (h *humanSink) results(p *domain.Plan) {
	h.out.Line("")
	h.heading("Results")
	// Where the work was done is only worth a column when some target had work done.
	anyPlace := false
	for _, t := range p.Targets {
		if r := t.Result; r != nil && r.Place != "" {
			anyPlace = true
		}
	}
	header := []string{"LINE", "RESULT", "BRANCH"}
	if anyPlace {
		header = []string{"LINE", "RESULT", "WHERE", "BRANCH"}
	}
	h.table(header...)
	for _, t := range p.Targets {
		r := t.Result
		if r == nil {
			continue
		}
		row := []string{t.Key(), h.status(string(r.Status)), r.Branch}
		if anyPlace {
			where := string(r.Place)
			if where == "" {
				where = "-"
			}
			row = []string{t.Key(), h.status(string(r.Status)), where, r.Branch}
		}
		h.out.Row(row...)
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
			h.out.Line(h.st.Style(statusRoleOf(status), fmt.Sprintf("  %s (%s): %s", status, strings.Join(g.branches, ", "), g.text)))
		}
	}
	for _, t := range p.Targets {
		if r := t.Result; r != nil && len(r.WouldSupersede) > 0 {
			h.out.Line(fmt.Sprintf("  %s: would close %s", t.Key(), refNumbers(r.WouldSupersede)))
		}
	}
	for _, g := range groupTargets(p.Targets, func(t domain.TargetPlan) []string {
		if t.Result == nil {
			return nil
		}
		return t.Result.Warnings
	}) {
		h.out.Line(h.st.Style(style.Warn, fmt.Sprintf("  warning (%s): %s", strings.Join(g.branches, ", "), g.text)))
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
		h.heading("Review:")
		for _, line := range review {
			h.out.Line(h.st.Style(style.Dim, line))
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

// statusRoleOf is statusRole for a status that is known to have one.
func statusRoleOf(status domain.ResultStatus) style.Role {
	role, _ := statusRole(string(status))
	return role
}

// trailer is the closing line, in the look of what came of the run: green when
// PRs were opened or refreshed, secondary text when nothing was written.
func (h *humanSink) trailer(p *domain.Plan) string {
	text := trailer(p)
	if strings.HasPrefix(text, "Opened") || strings.HasPrefix(text, "Refreshed") {
		return h.st.Style(style.OK, text)
	}
	return h.st.Style(style.Dim, text)
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

// filesText says what a commit changed on one line: the file for one, else how
// many with the first few named. It is empty for a commit whose files are not known.
func filesText(files []string) string {
	switch n := len(files); {
	case n == 0:
		return ""
	case n == 1:
		return files[0]
	case n <= 4:
		return fmt.Sprintf("%d files: %s", n, strings.Join(files, ", "))
	default:
		return fmt.Sprintf("%d files: %s and %d more", n, strings.Join(files[:3], ", "), n-3)
	}
}

// stepsText lists the commits the steps of a bump may make, in the order they run,
// with where each fits around the edit and what ran it.
func stepsText(steps []domain.StepPreview) string {
	lines := []string{"a step's commit is made only if it changes files"}
	for _, s := range steps {
		when, _, _ := strings.Cut(s.CommitMessage, "\n")
		where := "after the edit "
		if s.BeforeEdit {
			where = "before the edit"
		}
		lines = append(lines, fmt.Sprintf("%s  %s  (%s)", where, when, s.Phase))
	}
	return strings.Join(lines, "\n")
}
