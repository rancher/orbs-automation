package backport

import (
	"context"
	"crypto/sha1" //nolint:gosec // a fingerprint of what is picked, not a secret
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
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
	PR      int      // the number of the merged PR
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
	Repo    string               `json:"repo"` // the repository, owner/name
	PR      int                  `json:"pr"`
	Title   string               `json:"title"`
	URL     string               `json:"url"`
	Base    string               `json:"base"`
	Shape   Shape                `json:"shape,omitempty"`
	Picks   []string             `json:"picks,omitempty"`
	Clone   string               `json:"clone,omitempty"`
	Remote  string               `json:"remote,omitempty"`
	Push    *proposal.PushTarget `json:"push,omitempty"`
	Actor   *proposal.Actor      `json:"actor,omitempty"`
	Targets []Target             `json:"targets,omitempty"`
}

// Target is one release branch the PR is carried to.
type Target struct {
	Branch string `json:"branch"`
	Line   string `json:"line,omitempty"`
	Head   string `json:"head,omitempty"` // the commit at the head of the branch on the host
	Status Status `json:"status"`
	// Files are the files that had conflicts, resolved by hand.
	Files   []string          `json:"conflicts,omitempty"`
	Preview *proposal.Preview `json:"preview,omitempty"`
	// Result is what a run did for a target that was worked on.
	Result *proposal.TargetResult `json:"result,omitempty"`
}

// state is a target as the run works on it: what its texts are made from, and its marker.
type state struct {
	tg     *Target
	vars   map[string]string
	marker proposal.Marker
	// subjects are the first lines of the picked commits, in order.
	subjects []string
}

// Run carries a merged PR to the targets, one PR for each, newest line first. Each target is
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
	detail, err := ws.PullRequests().Get(ctx, slug, in.PR)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Repo: slug, PR: in.PR, Title: detail.Title, URL: detail.URL, Base: detail.Base}
	if !detail.Merged {
		return plan, fmt.Errorf("%s#%d is not merged: only a merged PR is backported", slug, in.PR)
	}
	commits, err := ws.PullRequests().Commits(ctx, slug, in.PR)
	if err != nil {
		return plan, err
	}
	targets, err := resolveTargets(holder, in.Lines, detail.Base)
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
		return plan, nil
	}
	sess, err := proposal.Begin(ctx, ws, ex, proposal.SessionInput{Repo: holder, PushTo: in.PushTo, CI: in.CI, NoPush: in.NoPush})
	if err != nil {
		return plan, err
	}
	plan.Clone, plan.Remote, plan.Push, plan.Actor = sess.Clone, sess.Remote, sess.Push, sess.Actor
	if _, err := g.Fetch(ctx, sess.Clone, sess.Remote, detail.Base); err != nil {
		return plan, fmt.Errorf("fetch %s of %s: %w", detail.Base, slug, err)
	}
	picks, err := Choose(ctx, g, sess.Clone, PR{
		Repo: slug, Number: in.PR, Title: detail.Title, URL: detail.URL, Base: detail.Base, MergeCommit: detail.MergeCommit, Commits: commits,
	})
	if err != nil {
		return plan, err
	}
	plan.Shape, plan.Picks = picks.Shape, picks.Commits
	subjects, err := subjectsOf(ctx, g, sess.Clone, picks.Commits)
	if err != nil {
		return plan, err
	}
	fingerprint := fingerprintOf(picks.Commits)
	states := make([]*state, len(plan.Targets))
	for i := range plan.Targets {
		tg := &plan.Targets[i]
		held, err := holds(ctx, g, sess, tg.Branch, picks.Commits)
		if err != nil {
			return plan, err
		}
		if held {
			tg.Status = StatusAlreadyBackported
			continue
		}
		tg.Status = StatusReady
		st := &state{tg: tg, subjects: subjects, marker: proposal.Marker{
			Source: fmt.Sprintf("backport-%d", in.PR), Consumer: slug, Target: tg.Branch, Version: fingerprint,
		}}
		st.vars = varsOf(slug, detail, *tg, st.marker, picks.Commits, subjects)
		preview, err := previewOf(in, holder, st)
		if err != nil {
			return plan, err
		}
		tg.Preview = &preview
		states[i] = st
	}
	return plan, enactAll(ctx, g, ex, sess, in, holder, refs, &plan, states, picks)
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

// holds reports whether the history of the target, as the host has it, already holds every
// picked commit, by the line a pick writes.
func holds(ctx context.Context, g Git, sess *proposal.Session, branch string, commits []string) (bool, error) {
	tip, err := g.Fetch(ctx, sess.Clone, sess.Remote, branch)
	if err != nil {
		return false, fmt.Errorf("fetch %s: %w", branch, err)
	}
	for _, c := range commits {
		found, err := g.HoldsPickOf(ctx, sess.Clone, tip, c)
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// varsOf are the variables of the texts of a target.
func varsOf(slug string, pr pulls.Detail, tg Target, m proposal.Marker, commits, subjects []string) map[string]string {
	label := tg.Line
	if label == "" {
		label = tg.Branch
	}
	bullets := make([]string, len(commits))
	for i, c := range commits {
		bullets[i] = "- `" + c[:min(7, len(c))] + "` " + subjects[i]
	}
	return map[string]string{
		"line": label, "branch": strings.TrimPrefix(tg.Branch, "release/"), "targetBranch": tg.Branch, "consumer": slug,
		"number": fmt.Sprint(pr.Number), "title": pr.Title, "url": pr.URL, "commits": strings.Join(bullets, "\n"),
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
	title, err := templates.Render(texts.BackportTitle(), st.vars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("title: %w", err)
	}
	body, err := templates.Render(texts.BackportBody(), st.vars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("body: %w", err)
	}
	return proposal.Preview{
		Branch: renderedPrefix + st.vars["key"], Key: st.vars["key"], Title: title, Labels: prs.Labels,
		Body: proposal.CollapseBlankLines(body),
	}, nil
}

// handResolved names the files a person resolved, in the body of the PR, so a reviewer knows.
func (st *state) handResolved(files []string) error {
	bullets := make([]string, len(files))
	for i, f := range files {
		bullets[i] = "- `" + f + "`"
	}
	st.vars["conflicts"] = "\n## Conflicts resolved by hand\n" + strings.Join(bullets, "\n")
	body, err := templates.Render(texts.BackportBody(), st.vars)
	if err != nil {
		return err
	}
	st.tg.Files = files
	st.tg.Preview.Body = proposal.CollapseBlankLines(body)
	return nil
}

// enactAll decides each target, asks once, and works the targets in order. A conflict that is
// abandoned, or a resolver that fails, ends the run: the targets after it are not attempted.
func enactAll(ctx context.Context, g Git, ex *proposal.Executor, sess *proposal.Session, in Input, holder *chain.Project, refs git.Refs, plan *Plan, states []*state, picks Picks) error {
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
				out, err := pick(ctx, g, ex, sess, in, w, st, picks, onCommit)
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

// errStopped marks the failure of a resolver, which ends the run as an abandoned conflict does.
var errStopped = errors.New("could not ask about the conflict")

func subjectOf(st *state) proposal.Subject {
	return proposal.Subject{Branch: st.tg.Branch, Commit: st.tg.Head, Line: st.tg.Line, Preview: st.tg.Preview}
}

// sameFingerprint orders two backports of one PR to one branch: the same picks are the same,
// and any other is older, since a later run picked other commits and the PR of the earlier is stale.
func sameFingerprint(found, mine string) (int, bool) {
	if found == mine {
		return 0, true
	}
	return -1, true
}

// confirmation is what the run is about to do, and whether that writes anything.
func confirmation(plan *Plan, work []int, decided []proposal.Decided) (proposal.Confirmation, bool) {
	c := proposal.Confirmation{Consumer: plan.Repo, Release: fmt.Sprintf("a backport of #%d", plan.PR)}
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

// pick makes the picks of a target in the work directory, stopping on a conflict for a person.
func pick(ctx context.Context, g Git, ex *proposal.Executor, sess *proposal.Session, in Input, w *proposal.Work, st *state, picks Picks, onCommit func(proposal.CommitInfo)) (proposal.ApplyResult, error) {
	var out proposal.ApplyResult
	var byHand []string
	for i, sha := range picks.Commits {
		p := gitcli.Pick{Commit: sha, Mainline: picks.Mainline, Name: sess.Committer.Name, Email: sess.Committer.Email, Sign: ex.Sign}
		hash, err := g.CherryPick(ctx, w.Dir, p)
		var conflict *gitcli.ConflictError
		if errors.As(err, &conflict) {
			hash, err = resolve(ctx, g, in, w, st, conflict, st.subjects[i], p)
			byHand = append(byHand, conflict.Files...)
		}
		if err != nil {
			return out, err
		}
		files, _ := w.CommitFiles(ctx, hash)
		info := proposal.CommitInfo{Hash: hash, Subject: st.subjects[i], Files: files}
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
func resolve(ctx context.Context, g Git, in Input, w *proposal.Work, st *state, conflict *gitcli.ConflictError, subject string, p gitcli.Pick) (string, error) {
	abandon := func(err error) (string, error) {
		return "", errors.Join(err, g.AbortPick(ctx, w.Dir))
	}
	if in.DryRun {
		// Nobody is waiting on a dry run, so a conflict is a finding that the others go past.
		return abandon(fmt.Errorf("would conflict: %s conflicts in %s", conflict.Commit[:min(7, len(conflict.Commit))], strings.Join(conflict.Files, ", ")))
	}
	var left []string
	for {
		choice := Abort
		if in.Resolver != nil {
			var err error
			choice, err = in.Resolver.Resolve(ctx, Conflict{
				Branch: st.tg.Branch, Line: st.tg.Line, Commit: conflict.Commit, Subject: subject, Dir: w.Dir, Files: conflict.Files, Left: left,
			})
			if err != nil {
				return abandon(fmt.Errorf("%w: %w", errStopped, err))
			}
		}
		if choice == Abort {
			return abandon(fmt.Errorf("%w: %s conflicts in %s", ErrAborted, conflict.Commit[:min(7, len(conflict.Commit))], strings.Join(conflict.Files, ", ")))
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
