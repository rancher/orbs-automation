package ship

import (
	"context"
	"crypto/sha1" //nolint:gosec // a fingerprint of what a sync writes, not a secret
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/texts"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/platform/templates"
	"github.com/rancher/orbs-automation/internal/proposal"
)

// SyncInput is what the user asked to sync: the held values of one project.
type SyncInput struct {
	Project string   // the holder, by config name or slug; empty for the one the working directory is in
	Holding string   // limit the run to one holding, by name
	Lines   []string // limit the run to these Rancher lines
	// BranchPrefix is the user's branchPrefix setting, which replaces the project's.
	BranchPrefix string
	DryRun       bool   // compute and report, and create no branch or PR
	NoPush       bool   // stop after the commit, leaving the branch in the clone
	PushTo       string // fork or upstream, or empty to let the settings decide
	CI           bool   // the actor is the bot
}

// SyncPlan is what a sync of a project would do, and what a run did: a branch of the
// holder for each of its branches that holds something to correct, and in it every
// correction, whatever holds it.
type SyncPlan struct {
	Holder   string       `json:"holder"` // the repository of the project, owner/name
	Clone    string       `json:"clone,omitempty"`
	Remote   string       `json:"remote,omitempty"`
	Push     *PushTarget  `json:"push,omitempty"`
	Actor    *Actor       `json:"actor,omitempty"`
	Branches []SyncBranch `json:"branches,omitempty"`
}

// SyncBranch is one branch of the holder: what each holding in it holds against what
// it should, and, when something is to be corrected, the one PR that corrects it.
// A PR has one base, so a correction on another branch is another SyncBranch.
type SyncBranch struct {
	Branch string    `json:"branch"`
	Head   string    `json:"head,omitempty"` // the commit at the head of the branch on the host
	Status Status    `json:"status"`
	Rows   []SyncRow `json:"rows"`
	// Preview is what a run would write, when a row is ready. Result is what a run did.
	Preview *Preview      `json:"preview,omitempty"`
	Result  *TargetResult `json:"result,omitempty"`

	holder *chain.Project
	route  bump.Route
	ready  []*chain.Holding
	// fingerprint says what the PR writes, so a later run knows its own PR as the
	// same fix, and an older one as stale.
	fingerprint string
}

// SyncRow is one holding on one line of a branch.
type SyncRow struct {
	SeriesLine
	Holding string `json:"holding"`
	Kind    string `json:"kind"`
	Status  Status `json:"status"`
	// Reason says why a row that differs is not ready: no Edit follows the holding, the
	// difference is acknowledged, or the lines of the branch want different values of
	// one place.
	Reason string `json:"reason,omitempty"`

	holding *chain.Holding
}

// marker is what identifies the PR of the branch: the holder, the branch and the
// fingerprint of what it writes.
func (b SyncBranch) marker(holder string) Marker {
	return Marker{Source: "sync", Consumer: holder, Target: b.Branch, Version: b.fingerprint}
}

// line is the Rancher line of the branch: that of its first row.
func (b SyncBranch) line() string {
	if len(b.Rows) > 0 {
		return b.Rows[0].Line
	}
	return ""
}

// subject is the branch as the publisher knows it.
func (b SyncBranch) subject() proposal.Subject {
	return proposal.Subject{Branch: b.Branch, Commit: b.Head, Line: b.line(), Preview: b.Preview}
}

// Sync brings what a project holds in line with config, branch by branch: for each
// branch of the holder, every holding whose value differs from what it should be and
// that an Edit follows is written in one commit, followed by the steps of the Edits, and
// the branch is pushed and opened as one PR. A holding that no Edit follows, or whose
// difference the document acknowledges, is only reported, since nothing writes a holding
// until an Edit says what a write of it takes. A series holding is the one kind brought
// in line now; a kind that is pulled later is another row source in rowsOf.
//
// With ex nil it only checks. A branch that fails is reported and the others go on.
func Sync(ctx context.Context, ws Workspace, in SyncInput, ex *Executor) (SyncPlan, error) {
	holder, err := ws.Project(in.Project)
	if err != nil {
		return SyncPlan{}, err
	}
	engine, engineErr := ws.Bump()
	if engine == nil {
		return SyncPlan{}, engineErr
	}
	plan := SyncPlan{Holder: holder.Slug()}
	holdings, err := pulledHoldingsOf(holder, in.Holding)
	if err != nil {
		return plan, err
	}
	if len(holdings) == 0 {
		return plan, nil
	}
	refs, err := ws.UpstreamRefs(ctx, holder.Slug())
	if err != nil {
		return plan, fmt.Errorf("read the branches of %s: %w", holder.Slug(), err)
	}
	var rows []SyncRow
	for _, h := range holdings {
		for _, r := range rowsOf(ctx, ws, in, holder, refs, h) {
			rows = append(rows, r)
		}
	}
	plan.Branches = syncBranches(holder, engine, in, rows)
	if ex == nil {
		return plan, nil
	}
	return plan, runSync(ctx, ws, in, ex, holder, &plan)
}

// pulledHoldingsOf are the holdings of a project that are brought in line from config
// and not shipped from a release, or the one named. A name that is none of them is an
// error.
func pulledHoldingsOf(holder *chain.Project, name string) ([]*chain.Holding, error) {
	var out []*chain.Holding
	for _, h := range holder.Held() {
		if h.Kind() == chain.HoldsSeries && !h.Virtual() && (name == "" || h.Name() == name) {
			out = append(out, h)
		}
	}
	if name != "" && len(out) == 0 {
		return nil, fmt.Errorf("%s has no holding named %q that is brought in line from config", holder.Slug(), name)
	}
	return out, nil
}

// rowsOf reads a holding on each of its lines: what it holds against what config says
// it should. This is where a kind of holding that is pulled says what its value should
// be; a series is the series its stream has for the line.
func rowsOf(ctx context.Context, ws Reader, in SyncInput, holder *chain.Project, refs git.Refs, h *chain.Holding) []SyncRow {
	var rows []SyncRow
	for _, line := range seriesLinesOf(holder, h) {
		if len(in.Lines) > 0 && !slices.ContainsFunc(in.Lines, func(l string) bool { return lines.Normalize(l) == lines.Normalize(line) }) {
			continue
		}
		sl := seriesLine(ctx, ws, holder, refs, h, line)
		row := SyncRow{SeriesLine: sl, Holding: h.Name(), Kind: h.Kind(), holding: h}
		switch {
		case sl.Error != "":
			row.Status, row.Reason = StatusFailed, sl.Error
		case sl.UpToDate:
			row.Status = StatusUpToDate
		case sl.Acknowledged != "":
			row.Status, row.Reason = StatusBlocked, "the difference is acknowledged: "+sl.Acknowledged
		case len(h.Edits()) == 0:
			row.Status, row.Reason = StatusBlocked, fmt.Sprintf("%v: Holding %q is only read until one does", bump.ErrNoEdit, h.Name())
		default:
			row.Status = StatusReady
		}
		rows = append(rows, row)
	}
	return rows
}

// syncBranches groups the rows by the branch they are in, in the order they come, and
// works out what each branch needs: its status, the route that writes it, and the
// fingerprint of what it writes. Rows of one holding that want different values of one
// place on one branch (a holder with a single branch for lines that differ) cannot both
// be written, so they are left alone.
func syncBranches(holder *chain.Project, engine *bump.Engine, in SyncInput, rows []SyncRow) []SyncBranch {
	var out []SyncBranch
	for _, r := range rows {
		i := slices.IndexFunc(out, func(b SyncBranch) bool { return b.Branch == r.Branch })
		if i < 0 {
			out = append(out, SyncBranch{Branch: r.Branch, Head: r.Head, holder: holder})
			i = len(out) - 1
		}
		out[i].Rows = append(out[i].Rows, r)
	}
	for i := range out {
		b := &out[i]
		leaveConflicts(b)
		b.Status = statusOfBranch(b)
		if b.Status != StatusReady {
			continue
		}
		var ready []string
		for _, r := range b.Rows {
			if r.Status == StatusReady && !slices.Contains(b.ready, r.holding) {
				b.ready = append(b.ready, r.holding)
			}
			if r.Status == StatusReady {
				ready = append(ready, r.Holding+"="+r.Want)
			}
		}
		route, err := engine.PullRoute(b.ready...)
		if err != nil {
			b.Status = StatusBlocked
			for j := range b.Rows {
				if b.Rows[j].Status == StatusReady {
					b.Rows[j].Status, b.Rows[j].Reason = StatusBlocked, err.Error()
				}
			}
			continue
		}
		b.route = route
		slices.Sort(ready)
		sum := sha1.Sum([]byte(strings.Join(ready, "\n"))) //nolint:gosec // see the import
		b.fingerprint = hex.EncodeToString(sum[:])[:8]
		preview, err := syncPreview(in, holder, *b)
		if err != nil {
			b.Status = StatusFailed
			b.Rows[0].Reason = err.Error()
			continue
		}
		b.Preview = &preview
	}
	return out
}

// leaveConflicts marks the ready rows of one holding as left alone when another line of
// the holding on the branch wants a different value, whether that row differs or not:
// writing one would put the other out.
func leaveConflicts(b *SyncBranch) {
	for i, r := range b.Rows {
		if r.Status != StatusReady {
			continue
		}
		for j, o := range b.Rows {
			if i != j && o.Status != StatusFailed && o.Holding == r.Holding && o.Want != r.Want {
				b.Rows[i].Status = StatusBlocked
				b.Rows[i].Reason = fmt.Sprintf("%s and %s want different values of one place on %s", r.Line, o.Line, b.Branch)
				break
			}
		}
	}
}

// statusOfBranch is ready when a row is, failed when none can be read, up to date when
// every row is, and blocked when what differs is all left alone.
func statusOfBranch(b *SyncBranch) Status {
	var ready, upToDate, failed int
	for _, r := range b.Rows {
		switch r.Status {
		case StatusReady:
			ready++
		case StatusUpToDate:
			upToDate++
		case StatusFailed:
			failed++
		}
	}
	switch {
	case ready > 0:
		return StatusReady
	case failed == len(b.Rows):
		return StatusFailed
	case upToDate == len(b.Rows):
		return StatusUpToDate
	case upToDate+failed == len(b.Rows):
		return StatusFailed
	}
	return StatusBlocked
}

// syncPreview is the text a sync of the branch would write: the branch, the PR's title and
// body, the commit message and the steps that run around the write.
func syncPreview(in SyncInput, holder *chain.Project, b SyncBranch) (Preview, error) {
	prs := holder.PullRequests()
	target := bump.Target{Branch: b.Branch, Line: b.line(), Holdings: b.ready}
	pre, post := b.route.Steps(target)
	steps := append(slices.Clone(pre), post...)

	var holdings, linesOf, changes []string
	for _, r := range b.Rows {
		if r.Status != StatusReady {
			continue
		}
		if !slices.Contains(holdings, r.Holding) {
			holdings = append(holdings, r.Holding)
		}
		if !slices.Contains(linesOf, r.Line) {
			linesOf = append(linesOf, r.Line)
		}
		series, _ := r.holding.Stream().Series(r.Line)
		changes = append(changes, fmt.Sprintf("- %s (%s: the series `%s` of `%s` for %s)", r.Change, r.Holding, series, r.holding.Stream().Name(), r.Line))
	}
	for _, ref := range steps {
		changes = append(changes, "- Ran `"+strings.Join(ref.Step.Run, " ")+"`")
	}
	noun := "1 change"
	if n := len(holdings); n > 1 || len(linesOf) > 1 {
		noun = fmt.Sprintf("%d holdings", n)
	}
	vars := map[string]string{
		"holdings": strings.Join(holdings, ", "), "count": noun, "line": strings.Join(linesOf, ", "),
		"branch": strings.TrimPrefix(b.Branch, "release/"), "targetBranch": b.Branch, "consumer": holder.Slug(),
		"changes": strings.Join(changes, "\n"), "marker": b.marker(holder.Slug()).String(),
		"key": BranchKey("sync", b.fingerprint, b.Branch),
	}
	prefix := firstNonEmpty(in.BranchPrefix, prs.BranchPrefix, texts.DefaultBranchPrefix())
	prefixVars := map[string]string{}
	for k, v := range vars {
		if k != "key" {
			prefixVars[k] = v
		}
	}
	renderedPrefix, err := templates.Render(prefix, prefixVars)
	if err != nil {
		return Preview{}, fmt.Errorf("branch prefix %q: %w", prefix, err)
	}
	title, err := templates.Render(texts.SyncTitle(), vars)
	if err != nil {
		return Preview{}, fmt.Errorf("title: %w", err)
	}
	commit, err := templates.Render(texts.SyncCommitMessage(), vars)
	if err != nil {
		return Preview{}, fmt.Errorf("commit message: %w", err)
	}
	body, err := templates.Render(texts.SyncBody(), vars)
	if err != nil {
		return Preview{}, fmt.Errorf("body: %w", err)
	}
	var stepPreviews []StepPreview
	for _, ref := range steps {
		message, err := syncStepMessage(vars)(bump.Unit{Kind: bump.UnitStep, Phase: ref.Phase, Edit: ref.Edit, Index: ref.Index, Run: ref.Step.Run})
		if err != nil {
			return Preview{}, err
		}
		stepPreviews = append(stepPreviews, StepPreview{
			Phase: string(ref.Phase), Edit: ref.Edit, Run: strings.Join(ref.Step.Run, " "),
			BeforeEdit: ref.Phase == bump.PreEdit, CommitMessage: message,
		})
	}
	return Preview{
		Branch: renderedPrefix + vars["key"], Key: vars["key"], Title: title, Labels: prs.Labels,
		CommitMessage: commit, Body: body, Steps: stepPreviews,
	}, nil
}

// syncStepMessage is the commit message of a step that changed files.
func syncStepMessage(vars map[string]string) func(bump.Unit) (string, error) {
	return func(u bump.Unit) (string, error) {
		v := map[string]string{"holdings": vars["holdings"], "line": vars["line"], "command": strings.Join(u.Run, " "), "phase": string(u.Phase)}
		return templates.Render(texts.SyncStepCommitMessage(), v)
	}
}

// syncStrategyMessage is the commit message of a strategy that changed files.
func syncStrategyMessage(vars map[string]string) func(bump.Unit) (string, error) {
	return func(u bump.Unit) (string, error) {
		v := map[string]string{"holdings": vars["holdings"], "line": vars["line"], "strategy": u.Strategy, "edit": u.Edit, "changes": bullets(u.Changes)}
		msg, err := templates.Render(texts.SyncStrategyCommitMessage(), v)
		if err != nil {
			return "", err
		}
		return blankLines.ReplaceAllString(msg, "\n\n"), nil
	}
}

// runSync makes the commit of each ready branch and, with a publisher, pushes it and
// opens its PR, after the one question when the run writes to GitHub.
func runSync(ctx context.Context, ws Workspace, in SyncInput, ex *Executor, holder *chain.Project, plan *SyncPlan) error {
	var ready []int
	for i, b := range plan.Branches {
		if b.Status == StatusReady && b.Preview != nil {
			ready = append(ready, i)
		}
	}
	if len(ready) == 0 {
		return nil
	}
	clone, err := ws.CloneDir(holder.Slug())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoClone, err)
	}
	remote, err := ws.Remote(holder.Slug())
	if err != nil {
		return err
	}
	plan.Clone, plan.Remote = clone, remote
	var pub *Publisher
	var me string
	if ex.Publish != nil && !in.NoPush {
		if pub, me, plan.Push, err = proposal.PublisherFor(ctx, ws, ex, holder, clone, in.PushTo, in.CI, false); err != nil {
			return err
		}
	}
	var committer Identity
	if ex.Identify != nil {
		actor, err := proposal.ActorFor(ctx, ex, clone, me)
		if err != nil {
			return err
		}
		plan.Actor = actor
		if in.CI {
			committer = actor.Identity
		}
	}
	states := make([]syncState, len(ready))
	for k, i := range ready {
		states[k] = decideSync(ctx, pub, me, holder.Slug(), plan.Branches[i])
	}
	if pub != nil && !in.DryRun && ex.Confirm != nil {
		if c, writes := syncConfirmation(holder.Slug(), plan, ready, states); writes {
			ok, err := ex.Confirm(ctx, c)
			if err != nil {
				return err
			}
			if !ok {
				return ErrDeclined
			}
		}
	}
	for k, i := range ready {
		plan.Branches[i].Result = workSync(ctx, ex, pub, clone, remote, in, holder.Slug(), plan.Branches[i], states[k], committer)
	}
	return nil
}

// syncState is what was found out about one branch before any work: the decision about
// its PR, and what to warn about.
type syncState struct {
	dec      Publish
	m        Marker
	warnings []string
	done     *TargetResult // set when there is nothing to make: the lookup failed
}

// sameFingerprint orders two syncs of a branch: the same fix is the same, and any other
// is older, since a later run found different corrections and the PR of an earlier one
// is stale.
func sameFingerprint(found, mine string) (int, bool) {
	if found == mine {
		return 0, true
	}
	return -1, true
}

// decideSync looks at the open PRs of a branch, when there is a publisher, and decides
// what to do about them, as decideTarget does for a release. It only reads.
func decideSync(ctx context.Context, pub *Publisher, me, holder string, b SyncBranch) syncState {
	st := syncState{m: b.marker(holder)}
	if pub == nil {
		return st
	}
	open, err := pub.Remote.OpenForBase(ctx, holder, b.Branch, 0)
	if err != nil {
		st.done = &TargetResult{Status: ResultFailed, Branch: b.Preview.Branch, Error: fmt.Sprintf("look for the open PRs of %s: %v", holder, err)}
		return st
	}
	prs := open.PRs
	byHead, err := pub.Remote.OpenForHead(ctx, holder, pub.HeadOwnerOf(holder)+":"+b.Preview.Branch)
	if err != nil {
		st.warnings = append(st.warnings, fmt.Sprintf("could not look for the PR of %s: %v", b.Preview.Branch, err))
	}
	for _, p := range byHead {
		if !slices.ContainsFunc(prs, func(q pulls.PR) bool { return q.Number == p.Number }) {
			prs = append(prs, p)
		}
	}
	st.dec = decidePublishBy(prs, me, st.m, sameFingerprint)
	if open.Truncated {
		st.warnings = append(st.warnings, fmt.Sprintf("only the newest %d open PRs of %s were looked at, so an older PR of this sync may have been missed", len(open.PRs), holder))
	}
	for _, a := range st.dec.Also {
		st.warnings = append(st.warnings, alsoNote(a, b.Branch, 0, false))
	}
	return st
}

// syncConfirmation is what the run is about to do, and whether that writes anything.
func syncConfirmation(holder string, plan *SyncPlan, ready []int, states []syncState) (Confirmation, bool) {
	c := Confirmation{Consumer: holder}
	if plan.Push != nil {
		c.Push = *plan.Push
	}
	var what []string
	for k, i := range ready {
		b, st := plan.Branches[i], states[k]
		label := b.Branch
		if len(b.Rows) > 0 {
			label = b.Rows[0].Line
		}
		what = append(what, b.Preview.Title)
		switch {
		case st.done != nil:
		case st.dec.Action == PublishAlreadyOpen:
			c.Already = append(c.Already, label)
		default:
			c.Opens = append(c.Opens, label)
		}
		for _, p := range st.dec.Supersede {
			c.Closes = append(c.Closes, PRRef{Number: p.Number, URL: p.URL})
		}
	}
	c.Release = "a sync"
	return c, len(c.Opens) > 0 || len(c.Closes) > 0
}

// workSync does the work for one branch that was decided, as workTarget does for a
// release: a result of its own when there is nothing to make, otherwise the commits and,
// with a publisher, the push and the PR.
func workSync(ctx context.Context, ex *Executor, pub *Publisher, clone, remote string, in SyncInput, holder string, b SyncBranch, st syncState, committer Identity) *TargetResult {
	if st.done != nil {
		return st.done
	}
	if pub != nil && st.dec.Action == PublishAlreadyOpen {
		result := &TargetResult{Status: ResultAlreadyOpen, Branch: b.Preview.Branch, Warnings: st.warnings,
			PR: &PRRef{Number: st.dec.Open.Number, URL: st.dec.Open.URL}}
		if in.DryRun {
			result.WouldSupersede = refs(st.dec.Supersede)
		} else {
			pub.Supersede(ctx, holder, st.dec.Open.Number, st.m, b.subject(), st.dec.Supersede, result, nil)
		}
		return result
	}
	result := commitSync(ctx, ex, clone, remote, in, b, committer)
	result.Warnings = append(result.Warnings, st.warnings...)
	switch {
	case pub == nil:
	case in.DryRun:
		result.WouldSupersede = refs(st.dec.Supersede)
	case result.Status == ResultCommitted:
		pub.Publish(ctx, holder, b.subject(), st.m, st.dec, result, nil)
	}
	return result
}

// commitSync makes the commits for one branch and ends the work, whatever happened, so
// the clone is as it was found apart from the branch it keeps.
func commitSync(ctx context.Context, ex *Executor, clone, remote string, in SyncInput, b SyncBranch, committer Identity) *TargetResult {
	result := &TargetResult{Branch: b.Preview.Branch}
	fail := func(err error) *TargetResult {
		result.Status, result.Error = ResultFailed, err.Error()
		return result
	}
	w, err := Prepare(ctx, ex.Git, PrepareRequest{
		CloneDir: clone, Remote: remote, Target: b.Branch, Base: b.Head,
		Branch: b.Preview.Branch, DryRun: in.DryRun, TempDir: ex.TempDir,
		// A run in CI owns its checkout, so what an earlier step left in it is the work.
		StageUntracked: in.CI,
	})
	if errors.Is(err, ErrBaseMoved) {
		return fail(fmt.Errorf("%w; run it again", err))
	}
	if err != nil {
		return fail(err)
	}
	result.Place, result.Reason = w.Place, w.Reason
	if w.Reused {
		// The branch is one of ours that is current, so nothing is built.
		result.Commits = w.Existing
		result.Status = ResultCommitted
		if in.DryRun {
			result.Status = ResultWouldCommit
		}
		return result
	}
	vars := map[string]string{"holdings": holdingsOf(b), "line": b.line()}
	applied, err := Apply{
		Route: b.route, Target: bump.Target{Branch: b.Branch, Line: b.line(), Holdings: b.ready},
		Message: b.Preview.CommitMessage, Runner: ex.Runner, Sign: ex.Sign, Name: committer.Name, Email: committer.Email,
		StepMessage: syncStepMessage(vars), StrategyMessage: syncStrategyMessage(vars),
	}.Run(ctx, w)
	if errors.Is(err, ErrNothingChanged) {
		result.Status = ResultUpToDate
		return finish(result, w.Finish(ctx, false))
	}
	if err != nil {
		return fail(errors.Join(err, w.Abort(ctx)))
	}
	result.Commits = applied.Commits
	for _, c := range applied.Changes {
		result.Changes = append(result.Changes, c.Description)
	}
	result.Status = ResultCommitted
	if in.DryRun {
		result.Status = ResultWouldCommit
	}
	return finish(result, w.Finish(ctx, !in.DryRun))
}

// holdingsOf is the names of the holdings a branch writes, joined.
func holdingsOf(b SyncBranch) string {
	var names []string
	for _, h := range b.ready {
		names = append(names, h.Name())
	}
	return strings.Join(names, ", ")
}
