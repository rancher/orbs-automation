package backport

import (
	"context"
	"crypto/sha1" //nolint:gosec // a fingerprint of what is picked, not a secret
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/texts"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/platform/templates"
	"github.com/rancher/orbs-automation/internal/proposal"
)

// Status is where a target stands.
type Status string

// The states of a target. What a run did for one that was worked on is its Result.
const (
	// StatusReady: the picks are known and a PR is to be made.
	StatusReady Status = "ready"
	// StatusAlreadyBackported: the history of the target already holds the picked commits.
	StatusAlreadyBackported Status = "already backported"
	// StatusAborted: a conflict stopped the pick and the person abandoned it.
	StatusAborted Status = "aborted"
	// StatusNotAttempted: an earlier target was aborted, and nothing after it was tried.
	StatusNotAttempted Status = "not attempted"
)

// ErrAborted is the failure of a target whose conflict was abandoned.
var ErrAborted = errors.New("the cherry-pick was abandoned")

// MaxPRs is how many PRs one backport may bundle.
const MaxPRs = 10

// Source is a merged PR that is carried, and how it landed.
type Source struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	URL    string   `json:"url"`
	Shape  Shape    `json:"shape,omitempty"`
	Picks  []string `json:"picks,omitempty"` // the commits it picks, oldest first
}

// Host is what a backport reads of the PR on its host.
type Host interface {
	Get(ctx context.Context, slug string, number int) (pulls.Detail, error)
	Commits(ctx context.Context, slug string, number int) ([]pulls.Commit, error)
}

// Workspace is what a backport needs: the projects, what the host has, the PR, and the
// engineer's clone of the project and where to push.
type Workspace interface {
	// Project returns the project of a config name or slug, or the one the working
	// directory is in when the key is empty.
	Project(key string) (*chain.Project, error)
	// UpstreamRefs lists the tags and branch heads of a repository on its host.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	// PullRequests reads PRs from the host.
	PullRequests() Host
	proposal.Site
}

// Choice is what a person decides about a conflict.
type Choice int

// The two answers.
const (
	// Continue: the conflicts are resolved in the work directory.
	Continue Choice = iota + 1
	// Abort: abandon the pick, and with it the run.
	Abort
)

// Conflict is a pick that stopped, for a person to resolve.
type Conflict struct {
	Branch  string   // the target branch
	Line    string   // its Rancher line, when it has one
	PR      int      // the PR whose commit was being picked
	Commit  string   // the commit that was being picked
	Subject string   // its first line
	Dir     string   // the work directory, where the files are
	Files   []string // the files with conflicts
	// Left are the files that still hold a conflict marker after an earlier Continue.
	Left []string
}

// Resolver waits for a person to resolve a conflict, or abandons it when there is nobody to
// ask. It returns when the person says which.
type Resolver interface {
	Resolve(ctx context.Context, c Conflict) (Choice, error)
}

// Input is what the user asked to backport.
type Input struct {
	Project string   // the repo, by config name or slug; empty for the one the working directory is in
	Lines   []string // the targets: Rancher lines or release branches, as typed
	PR      int      // the number of the merged PR, when PRs is not set
	// PRs are the merged PRs to carry, one or several. Several are a bundle.
	PRs []int
	// Repo is the owner/name a PR URL named, or empty. It must be the repo of the project.
	Repo string
	// BranchPrefix is the user's branchPrefix setting, which replaces the project's.
	BranchPrefix string
	// DefaultBranchPrefix is used when neither BranchPrefix nor the project says one.
	DefaultBranchPrefix string
	DryRun              bool   // compute and report, and create no branch or PR
	NoPush              bool   // stop after the commits, leaving the branches in the clone
	PushTo              string // fork or upstream, or empty to let the settings decide
	CI                  bool   // the actor is the bot
	// Resolver is asked about each conflict. Nil abandons them.
	Resolver Resolver
	// Git is the git of the picks. Nil is GitCLI.
	Git Git
}

// Plan is what a backport would do, and what a run did.
type Plan struct {
	Repo   string               `json:"repo"` // the repository, owner/name
	PR     int                  `json:"pr"`
	Title  string               `json:"title"`
	URL    string               `json:"url"`
	Base   string               `json:"base"`
	Shape  Shape                `json:"shape,omitempty"`
	Picks  []string             `json:"picks,omitempty"`
	Clone  string               `json:"clone,omitempty"`
	Remote string               `json:"remote,omitempty"`
	Push   *proposal.PushTarget `json:"push,omitempty"`
	Actor  *proposal.Actor      `json:"actor,omitempty"`
	// PRs are the PRs that are carried, in the order they are picked.
	PRs     []Source `json:"prs,omitempty"`
	Targets []Target `json:"targets,omitempty"`
}

// Target is one release branch the PR is carried to.
type Target struct {
	Branch string `json:"branch"`
	Line   string `json:"line,omitempty"`
	Head   string `json:"head,omitempty"` // the commit at the head of the branch on the host
	Status Status `json:"status"`
	// PRs are the numbers of the PRs this target's PR carries, and Skipped those left out
	// because the target already had them. Picks are the commits picked, in order.
	PRs     []int    `json:"prs,omitempty"`
	Skipped []int    `json:"skipped,omitempty"`
	Picks   []string `json:"picks,omitempty"`
	// Files are the files that had conflicts, resolved by hand.
	Files   []string          `json:"conflicts,omitempty"`
	Preview *proposal.Preview `json:"preview,omitempty"`
	// Result is what a run did for a target that was worked on.
	Result *proposal.TargetResult `json:"result,omitempty"`
}

// item is one commit to pick, and the PR it is of.
type item struct {
	pr       int
	sha      string
	mainline int
	subject  string
}

// carried is a PR that is carried: what it is, and what it picks, read once for every target.
type carried struct {
	Source
	merge    string // the commit the host records as its merge
	mainline int
	subjects []string
}

// items are the picks of a PR, oldest first.
func (c carried) items() []item {
	out := make([]item, len(c.Picks))
	for i, sha := range c.Picks {
		out[i] = item{pr: c.Number, sha: sha, mainline: c.mainline, subject: c.subjects[i]}
	}
	return out
}

// state is a target as the run works on it: what its texts are made from, and its marker.
type state struct {
	tg     *Target
	vars   map[string]string
	marker proposal.Marker
	// bundle says the target's PR carries several PRs, so it has the texts of a bundle.
	bundle bool
	items  []item
}

// Run carries merged PRs to the targets, one PR for each, newest line first. Each target is
// checked against the host, its picks are made in the clone or a worktree of it, and, unless
// --no-push or --dry-run, it is pushed and opened as a PR after asking once. A conflict stops
// the whole run until the person has resolved it or abandoned it; abandoning ends the run,
// and the targets not yet tried are not attempted.
func Run(ctx context.Context, ws Workspace, in Input, ex *proposal.Executor) (Plan, error) {
	g := in.Git
	if g == nil {
		g = GitCLI{}
	}
	holder, err := ws.Project(in.Project)
	if err != nil {
		return Plan{}, err
	}
	slug := holder.Slug()
	if in.Repo != "" && in.Repo != slug {
		return Plan{}, fmt.Errorf("the PR is in %s, but the project is %s", in.Repo, slug)
	}
	numbers, err := distinct(in)
	if err != nil {
		return Plan{}, err
	}
	reads, err := readPRs(ctx, ws.PullRequests(), slug, numbers)
	plan := Plan{Repo: slug}
	if len(reads) > 0 {
		plan.Base = reads[0].detail.Base
	}
	if err != nil {
		return plan, err
	}
	targets, err := resolveTargets(holder, in.Lines, plan.Base)
	if err != nil {
		return plan, err
	}
	refs, err := ws.UpstreamRefs(ctx, slug)
	if err != nil {
		return plan, fmt.Errorf("read the branches of %s: %w", slug, err)
	}
	for i := range targets {
		b, ok := refs.Branch(targets[i].Branch)
		if !ok {
			return plan, fmt.Errorf("%s has no branch %s on its host", slug, targets[i].Branch)
		}
		targets[i].Head = b.Commit
	}
	plan.Targets = targets
	if ex == nil {
		for _, r := range reads {
			plan.PRs = append(plan.PRs, Source{Number: r.number, Title: r.detail.Title, URL: r.detail.URL})
		}
		plan.PR, plan.Title, plan.URL = plan.PRs[0].Number, plan.PRs[0].Title, plan.PRs[0].URL
		return plan, nil
	}
	sess, err := proposal.Begin(ctx, ws, ex, proposal.SessionInput{Repo: holder, PushTo: in.PushTo, CI: in.CI, NoPush: in.NoPush})
	if err != nil {
		return plan, err
	}
	plan.Clone, plan.Remote, plan.Push, plan.Actor = sess.Clone, sess.Remote, sess.Push, sess.Actor
	if _, err := g.Fetch(ctx, sess.Clone, sess.Remote, plan.Base); err != nil {
		return plan, fmt.Errorf("fetch %s of %s: %w", plan.Base, slug, err)
	}
	ordered, err := carry(ctx, g, sess.Clone, slug, reads)
	if err != nil {
		return plan, err
	}
	for _, c := range ordered {
		plan.PRs = append(plan.PRs, c.Source)
	}
	plan.PR, plan.Title, plan.URL, plan.Shape, plan.Picks = ordered[0].Number, ordered[0].Title, ordered[0].URL, ordered[0].Shape, ordered[0].Picks
	states := make([]*state, len(plan.Targets))
	for i := range plan.Targets {
		states[i], err = prepare(ctx, g, sess, in, holder, ordered, &plan.Targets[i])
		if err != nil {
			return plan, err
		}
	}
	return plan, enactAll(ctx, g, ex, sess, in, refs, &plan, states)
}

// distinct is the numbers of the PRs asked for, each once, in the order given. There must be some,
// and no more than MaxPRs.
func distinct(in Input) ([]int, error) {
	asked := in.PRs
	if len(asked) == 0 && in.PR != 0 {
		asked = []int{in.PR}
	}
	var out []int
	for _, n := range asked {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	switch {
	case len(out) == 0:
		return nil, errors.New("name at least one merged PR to backport")
	case len(out) > MaxPRs:
		return nil, fmt.Errorf("%d PRs are too many to bundle: a backport takes at most %d", len(out), MaxPRs)
	}
	return out, nil
}

// readPR is a PR as the host has it.
type readPR struct {
	number  int
	detail  pulls.Detail
	commits []pulls.Commit
}

// readPRs reads each PR from the host. Each must be merged, and all to one base. The ones read so
// far come back with an error, so a report can say which base it was looking at.
func readPRs(ctx context.Context, host Host, slug string, numbers []int) ([]readPR, error) {
	var reads []readPR
	for _, n := range numbers {
		detail, err := host.Get(ctx, slug, n)
		if err != nil {
			return reads, err
		}
		if !detail.Merged {
			return reads, fmt.Errorf("%s#%d is not merged: only a merged PR is backported", slug, n)
		}
		if len(reads) > 0 && detail.Base != reads[0].detail.Base {
			return reads, fmt.Errorf("%s#%d was merged to %s, but %s#%d to %s: a backport is carried from one base", slug, n, detail.Base, slug, reads[0].number, reads[0].detail.Base)
		}
		commits, err := host.Commits(ctx, slug, n)
		if err != nil {
			return reads, err
		}
		reads = append(reads, readPR{number: n, detail: detail, commits: commits})
	}
	return reads, nil
}

// carry says what each PR picks, and puts them in the order they were applied to the base: one
// PR comes before another when its merge is an ancestor of the other's.
func carry(ctx context.Context, g Git, dir, slug string, reads []readPR) ([]carried, error) {
	var out []carried
	for _, r := range reads {
		picks, err := Choose(ctx, g, dir, PR{
			Repo: slug, Number: r.number, Title: r.detail.Title, URL: r.detail.URL, Base: r.detail.Base, MergeCommit: r.detail.MergeCommit, Commits: r.commits,
		})
		if err != nil {
			return nil, err
		}
		subjects, err := subjectsOf(ctx, g, dir, picks.Commits)
		if err != nil {
			return nil, err
		}
		out = append(out, carried{
			Source: Source{Number: r.number, Title: r.detail.Title, URL: r.detail.URL, Shape: picks.Shape, Picks: picks.Commits},
			merge:  r.detail.MergeCommit, mainline: picks.Mainline, subjects: subjects,
		})
	}
	var orderErr error
	slices.SortStableFunc(out, func(a, b carried) int {
		if a.merge == b.merge || orderErr != nil {
			return 0
		}
		before, err := g.IsAncestor(ctx, dir, a.merge, b.merge)
		if err != nil {
			orderErr = fmt.Errorf("order %s#%d and #%d on the base: %w", slug, a.Number, b.Number, err)
			return 0
		}
		if before {
			return -1
		}
		return 1
	})
	return out, orderErr
}

// prepare works out what a target is to carry: the PRs of the bundle that the target does not
// already have, and from them its picks, its identity and its texts. A target that has them all is
// already backported and has no state.
func prepare(ctx context.Context, g Git, sess *proposal.Session, in Input, holder *chain.Project, ordered []carried, tg *Target) (*state, error) {
	tip, err := g.Fetch(ctx, sess.Clone, sess.Remote, tg.Branch)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", tg.Branch, err)
	}
	var keep []carried
	for _, c := range ordered {
		present, err := presentOn(ctx, g, sess.Clone, tip, c)
		if err != nil {
			return nil, err
		}
		if present {
			tg.Skipped = append(tg.Skipped, c.Number)
		} else {
			keep = append(keep, c)
		}
	}
	if len(keep) == 0 {
		tg.Status = StatusAlreadyBackported
		return nil, nil
	}
	st := &state{tg: tg, bundle: len(keep) > 1}
	numbers := make([]int, len(keep))
	for i, c := range keep {
		numbers[i] = c.Number
		tg.PRs = append(tg.PRs, c.Number)
		st.items = append(st.items, c.items()...)
	}
	for _, it := range st.items {
		tg.Picks = append(tg.Picks, it.sha)
	}
	tg.Status = StatusReady
	slices.Sort(numbers)
	st.marker = proposal.Marker{Source: "backport-" + joinInts(numbers, "+"), Consumer: holder.Slug(), Target: tg.Branch, Version: fingerprintOf(tg.Picks)}
	st.vars = varsOf(holder.Slug(), keep, tg, st.marker)
	preview, err := previewOf(in, holder, st)
	if err != nil {
		return nil, err
	}
	tg.Preview = &preview
	return st, nil
}

// presentOn reports whether the target already has a PR: every one of its picks is there, as the
// commit itself, which a branch that merged the base has, or found by the line a cherry-pick -x
// writes or by the patch, whoever made it. A merge commit has no patch and is found by the line alone.
func presentOn(ctx context.Context, g Git, dir, tip string, c carried) (bool, error) {
	for _, sha := range c.Picks {
		found, err := g.IsAncestor(ctx, dir, sha, tip)
		if err != nil {
			return false, err
		}
		if !found {
			if found, err = g.HoldsPickOf(ctx, dir, tip, sha); err != nil {
				return false, err
			}
		}
		if !found && c.mainline == 0 {
			if found, err = g.PatchPresent(ctx, dir, tip, sha); err != nil {
				return false, err
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// joinInts writes numbers with a separator.
func joinInts(numbers []int, sep string) string {
	text := make([]string, len(numbers))
	for i, n := range numbers {
		text[i] = strconv.Itoa(n)
	}
	return strings.Join(text, sep)
}

// resolveTargets turns what was typed into the branches to carry the PR to, newest line first,
// each once. A target that is the branch the PR was merged to is refused.
func resolveTargets(holder *chain.Project, typed []string, base string) ([]Target, error) {
	var out []Target
	for _, arg := range typed {
		branch, line, err := holder.ResolveLineOrBranch(arg)
		if err != nil {
			return nil, err
		}
		if branch == base {
			return nil, fmt.Errorf("%s is the branch the PR was merged to (%s), so there is nothing to carry it to", arg, base)
		}
		if line != "" {
			line = lines.Normalize(line)
		}
		if !slices.ContainsFunc(out, func(t Target) bool { return t.Branch == branch }) {
			out = append(out, Target{Branch: branch, Line: line})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("name at least one release line or branch to backport to")
	}
	slices.SortStableFunc(out, func(a, b Target) int {
		switch {
		case a.Line == "" || b.Line == "":
			return strings.Compare(b.Line, a.Line)
		}
		return lines.Compare(b.Line, a.Line)
	})
	return out, nil
}

// subjectsOf is the first line of each commit.
func subjectsOf(ctx context.Context, g Git, dir string, commits []string) ([]string, error) {
	out := make([]string, len(commits))
	for i, c := range commits {
		log, err := g.FirstParentLog(ctx, dir, c, 1)
		if err != nil || len(log) != 1 {
			return nil, fmt.Errorf("read the commit %s: %v", c, err)
		}
		out[i], _, _ = strings.Cut(log[0].Message, "\n")
	}
	return out, nil
}

// fingerprintOf says what is picked, so a later run knows its own PR as the same backport.
func fingerprintOf(commits []string) string {
	sum := sha1.Sum([]byte(strings.Join(commits, "\n"))) //nolint:gosec // see the import
	return hex.EncodeToString(sum[:])[:8]
}

// varsOf are the variables of the texts of a target, for the PRs it carries.
func varsOf(slug string, keep []carried, tg *Target, m proposal.Marker) map[string]string {
	label := tg.Line
	if label == "" {
		label = tg.Branch
	}
	bundle := len(keep) > 1
	var bullets, prs, numbers []string
	for _, c := range keep {
		numbers = append(numbers, "#"+strconv.Itoa(c.Number))
		prs = append(prs, "- ["+c.Title+"]("+c.URL+") (#"+strconv.Itoa(c.Number)+")")
		for i, sha := range c.Picks {
			bullet := "- `" + sha[:min(7, len(sha))] + "` " + c.subjects[i]
			if bundle {
				bullet += " (#" + strconv.Itoa(c.Number) + ")"
			}
			bullets = append(bullets, bullet)
		}
	}
	skipped := ""
	if len(tg.Skipped) > 0 {
		var left []string
		for _, n := range tg.Skipped {
			left = append(left, "#"+strconv.Itoa(n))
		}
		skipped = "Already on this branch, so not picked: " + strings.Join(left, ", ") + "."
	}
	first := keep[0]
	return map[string]string{
		"line": label, "branch": strings.TrimPrefix(tg.Branch, "release/"), "targetBranch": tg.Branch, "consumer": slug,
		"number": strconv.Itoa(first.Number), "title": first.Title, "url": first.URL,
		"numbers": strings.Join(numbers, ", "), "prs": strings.Join(prs, "\n"), "skipped": skipped,
		"commits":   strings.Join(bullets, "\n"),
		"conflicts": "", "marker": m.String(), "key": proposal.BranchKey(m.Source, m.Version, tg.Branch),
	}
}

// previewOf is the text a backport to the target would write: the branch, the title and the body.
func previewOf(in Input, holder *chain.Project, st *state) (proposal.Preview, error) {
	prs := holder.PullRequests()
	prefix := proposal.FirstNonEmpty(in.BranchPrefix, prs.BranchPrefix, in.DefaultBranchPrefix, texts.DefaultBranchPrefix())
	prefixVars := map[string]string{}
	for k, v := range st.vars {
		if k != "key" {
			prefixVars[k] = v
		}
	}
	renderedPrefix, err := templates.Render(prefix, prefixVars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("branch prefix %q: %w", prefix, err)
	}
	titleText, bodyText := texts.BackportTitle(), texts.BackportBody()
	if st.bundle {
		titleText, bodyText = texts.BackportBundleTitle(), texts.BackportBundleBody()
	}
	title, err := templates.Render(titleText, st.vars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("title: %w", err)
	}
	body, err := templates.Render(bodyText, st.vars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("body: %w", err)
	}
	return proposal.Preview{
		Branch: renderedPrefix + st.vars["key"], Key: st.vars["key"], Title: title, Labels: prs.Labels,
		Body: proposal.CollapseBlankLines(body),
	}, nil
}

// handFile is a file a person resolved, and the PR whose pick it was of.
type handFile struct {
	pr   int
	file string
}

// handResolved names the files a person resolved, in the body of the PR, so a reviewer knows.
func (st *state) handResolved(files []handFile) error {
	bullets := make([]string, len(files))
	names := make([]string, len(files))
	for i, f := range files {
		bullets[i] = "- `" + f.file + "`"
		if st.bundle {
			bullets[i] += " (#" + strconv.Itoa(f.pr) + ")"
		}
		names[i] = f.file
	}
	st.vars["conflicts"] = "\n## Conflicts resolved by hand\n" + strings.Join(bullets, "\n")
	bodyText := texts.BackportBody()
	if st.bundle {
		bodyText = texts.BackportBundleBody()
	}
	body, err := templates.Render(bodyText, st.vars)
	if err != nil {
		return err
	}
	st.tg.Files = names
	st.tg.Preview.Body = proposal.CollapseBlankLines(body)
	return nil
}

// enactAll decides each target, asks once, and works the targets in order. A conflict that is
// abandoned, or a resolver that fails, ends the run: the targets after it are not attempted.
func enactAll(ctx context.Context, g Git, ex *proposal.Executor, sess *proposal.Session, in Input, refs git.Refs, plan *Plan, states []*state) error {
	var work []int
	for i, st := range states {
		if st != nil {
			work = append(work, i)
		}
	}
	if len(work) == 0 {
		return nil
	}
	decided := make([]proposal.Decided, len(work))
	for k, i := range work {
		st := states[i]
		decided[k] = proposal.Decide(ctx, sess.Pub, sess.Me, proposal.DecideInput{
			Repo: plan.Repo, Subject: subjectOf(st), Marker: st.marker, Compare: sameFingerprint,
			Branches: refs.Branches, Noun: "backport",
		}, nil)
		overlap(ctx, sess, plan.Repo, st, &decided[k])
	}
	c, writes := confirmation(plan, work, decided)
	if err := sess.Confirm(ctx, ex, in.DryRun, c, writes); err != nil {
		return err
	}
	var stop error
	for k, i := range work {
		st := states[i]
		if stop != nil {
			st.tg.Status = StatusNotAttempted
			st.tg.Preview = nil
			continue
		}
		var abandoned error
		st.tg.Result = proposal.Enact(ctx, ex, proposal.EnactInput{
			Repo: plan.Repo, Session: sess, Subject: subjectOf(st), Marker: st.marker, Decided: decided[k], DryRun: in.DryRun, CI: in.CI,
			Apply: func(w *proposal.Work, onCommit func(proposal.CommitInfo)) (proposal.ApplyResult, error) {
				out, err := pick(ctx, g, ex, sess, in, w, st, onCommit)
				if errors.Is(err, ErrAborted) || errors.Is(err, errStopped) {
					abandoned = err
				}
				return out, err
			},
		}, nil)
		if abandoned != nil {
			st.tg.Status = StatusAborted
			stop = abandoned
		}
	}
	return stop
}

// overlap finds the open backport PRs of the target that carry some of the same PRs. Those of
// the actor's own that the new PR covers, every one of their PRs among its own, are closed once it
// is open; every other is only reported, and a PR of someone else's is never touched. The identical
// set is the decision's own.
func overlap(ctx context.Context, sess *proposal.Session, repo string, st *state, d *proposal.Decided) {
	if sess.Pub == nil || d.Done != nil {
		return
	}
	open, err := sess.Pub.Remote.OpenForBase(ctx, repo, st.tg.Branch, 0)
	if err != nil {
		d.Warnings = append(d.Warnings, fmt.Sprintf("could not look for open backports that overlap this one: %v", err))
		return
	}
	for _, p := range open.PRs {
		m, ok := proposal.ParseMarker(p.Body)
		if !ok || m.Consumer != repo || m.Target != st.tg.Branch || m.Source == st.marker.Source {
			continue
		}
		set, ok := strings.CutPrefix(m.Source, "backport-")
		if !ok {
			continue
		}
		theirs := parseNumbers(set)
		shared := common(theirs, st.tg.PRs)
		switch {
		case len(shared) == 0:
		case p.Author != sess.Me:
			d.Warnings = append(d.Warnings, fmt.Sprintf("#%d by %s also backports %s: it is not yours, so it is left open", p.Number, p.Author, hashed(shared)))
		case len(shared) == len(theirs):
			d.Decision.Supersede = append(d.Decision.Supersede, p)
		default:
			d.Warnings = append(d.Warnings, fmt.Sprintf("#%d also backports %s: it is left open, since this one does not cover all of its PRs", p.Number, hashed(shared)))
		}
	}
}

// parseNumbers reads the numbers of a marker's source, 12+15. A part that is not a number is left out.
func parseNumbers(set string) []int {
	var out []int
	for _, part := range strings.Split(set, "+") {
		if n, err := strconv.Atoi(part); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// common are the numbers of a that are also in b.
func common(a, b []int) []int {
	var out []int
	for _, n := range a {
		if slices.Contains(b, n) {
			out = append(out, n)
		}
	}
	return out
}

// hashed names PRs as #12, #15.
func hashed(numbers []int) string {
	names := make([]string, len(numbers))
	for i, n := range numbers {
		names[i] = "#" + strconv.Itoa(n)
	}
	return strings.Join(names, ", ")
}

// errStopped marks the failure of a resolver, which ends the run as an abandoned conflict does.
var errStopped = errors.New("could not ask about the conflict")

func subjectOf(st *state) proposal.Subject {
	return proposal.Subject{Branch: st.tg.Branch, Commit: st.tg.Head, Line: st.tg.Line, Preview: st.tg.Preview}
}

// sameFingerprint orders two backports of the same PRs to one branch: the same picks are the same,
// and any other is older, since a later run picked other commits and the PR of the earlier is stale.
func sameFingerprint(found, mine string) (int, bool) {
	if found == mine {
		return 0, true
	}
	return -1, true
}

// confirmation is what the run is about to do, and whether that writes anything.
func confirmation(plan *Plan, work []int, decided []proposal.Decided) (proposal.Confirmation, bool) {
	c := proposal.Confirmation{Consumer: plan.Repo, Release: "a backport of " + joinNumbers(plan.PRs)}
	if plan.Push != nil {
		c.Push = *plan.Push
	}
	for k, i := range work {
		tg, st := plan.Targets[i], decided[k]
		label := tg.Line
		if label == "" {
			label = tg.Branch
		}
		switch {
		case st.Done != nil:
		case st.Decision.Action == proposal.PublishAlreadyOpen:
			c.Already = append(c.Already, label)
		default:
			c.Opens = append(c.Opens, label)
		}
		c.Closes = append(c.Closes, proposal.Refs(st.Decision.Supersede)...)
	}
	return c, len(c.Opens) > 0 || len(c.Closes) > 0
}

// joinNumbers names the PRs as #12, #15.
func joinNumbers(prs []Source) string {
	numbers := make([]int, len(prs))
	for i, p := range prs {
		numbers[i] = p.Number
	}
	return hashed(numbers)
}

// pick makes the picks of a target in the work directory, stopping on a conflict for a person.
func pick(ctx context.Context, g Git, ex *proposal.Executor, sess *proposal.Session, in Input, w *proposal.Work, st *state, onCommit func(proposal.CommitInfo)) (proposal.ApplyResult, error) {
	var out proposal.ApplyResult
	var byHand []handFile
	for _, it := range st.items {
		p := gitcli.Pick{Commit: it.sha, Mainline: it.mainline, Name: sess.Committer.Name, Email: sess.Committer.Email, Sign: ex.Sign}
		hash, err := g.CherryPick(ctx, w.Dir, p)
		var conflict *gitcli.ConflictError
		if errors.As(err, &conflict) {
			hash, err = resolve(ctx, g, in, w, st, conflict, it, p)
			for _, f := range conflict.Files {
				byHand = append(byHand, handFile{pr: it.pr, file: f})
			}
		}
		if err != nil {
			return out, err
		}
		files, _ := w.CommitFiles(ctx, hash)
		info := proposal.CommitInfo{Hash: hash, Subject: it.subject, Files: files}
		onCommit(info)
		out.Commits = append(out.Commits, info)
	}
	if len(byHand) > 0 {
		if err := st.handResolved(byHand); err != nil {
			return out, err
		}
	}
	return out, nil
}

// resolve waits for a person to resolve a conflict, and finishes the pick when they have. It
// asks again while a conflict marker is left in the files, and abandons the pick, which leaves
// the work directory clean, when the answer is to abort, or when there is nobody to ask.
func resolve(ctx context.Context, g Git, in Input, w *proposal.Work, st *state, conflict *gitcli.ConflictError, it item, p gitcli.Pick) (string, error) {
	abandon := func(err error) (string, error) {
		return "", errors.Join(err, g.AbortPick(ctx, w.Dir))
	}
	short := conflict.Commit[:min(7, len(conflict.Commit))]
	if in.DryRun {
		// Nobody is waiting on a dry run, so a conflict is a finding that the others go past.
		return abandon(fmt.Errorf("would conflict: %s of #%d conflicts in %s", short, it.pr, strings.Join(conflict.Files, ", ")))
	}
	var left []string
	for {
		choice := Abort
		if in.Resolver != nil {
			var err error
			choice, err = in.Resolver.Resolve(ctx, Conflict{
				Branch: st.tg.Branch, Line: st.tg.Line, PR: it.pr, Commit: conflict.Commit, Subject: it.subject, Dir: w.Dir, Files: conflict.Files, Left: left,
			})
			if err != nil {
				return abandon(fmt.Errorf("%w: %w", errStopped, err))
			}
		}
		if choice == Abort {
			return abandon(fmt.Errorf("%w: %s of #%d conflicts in %s", ErrAborted, short, it.pr, strings.Join(conflict.Files, ", ")))
		}
		var err error
		if left, err = g.ConflictMarkers(w.Dir, conflict.Files); err != nil {
			return abandon(err)
		}
		if len(left) == 0 {
			return g.ContinuePick(ctx, w.Dir, conflict.Files, p)
		}
	}
}
