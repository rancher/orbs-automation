package ship

import (
	"context"
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
)

// SeriesInput is what the user asked to fix: the series holdings of one project.
type SeriesInput struct {
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

// SeriesPlan is what a fix of the series holdings of a project would do, and what
// a run did.
type SeriesPlan struct {
	Holder  string         `json:"holder"` // the repository of the project, owner/name
	Clone   string         `json:"clone,omitempty"`
	Remote  string         `json:"remote,omitempty"`
	Push    *PushTarget    `json:"push,omitempty"`
	Actor   *Actor         `json:"actor,omitempty"`
	Targets []SeriesTarget `json:"targets,omitempty"`
}

// SeriesTarget is one line of one series holding: what its branch holds, what it
// should, and whether a PR is needed.
type SeriesTarget struct {
	SeriesLine
	Holding string `json:"holding"`
	Status  Status `json:"status"`
	// Reason says why a target is not ready: no Edit follows the holding, so nothing
	// writes it, the difference is acknowledged, or the branch could not be read.
	Reason  string        `json:"reason,omitempty"`
	Preview *Preview      `json:"preview,omitempty"` // what a run would write
	Result  *TargetResult `json:"result,omitempty"`  // what a run did, for a target that was ready

	holding *chain.Holding
	route   bump.Route
}

// plan is the target as the publisher knows it.
func (t SeriesTarget) plan() TargetPlan {
	return TargetPlan{Target: Target{Branch: t.Branch, Commit: t.Head}, Line: t.Line, Preview: t.Preview}
}

// marker is what identifies the PR of the target: the holding and the value it
// would write, in the holder's branch, so a later run finds it.
func (t SeriesTarget) marker(holder string) Marker {
	return Marker{Source: t.Holding, Consumer: holder, Target: t.Branch, Version: t.Want}
}

// FixSeries finds each line of the series holdings of a project whose branch holds a
// series other than the one its stream has, and, when ex is not nil, makes the commit
// for each that an Edit follows, as Ship does for a release: in the engineer's clone
// or a worktree of it, and with a publisher pushed and opened as a PR. A holding no Edit
// follows is only reported, since nothing writes a holding until an Edit says what a
// write of it takes. A target that fails is reported and the others go on.
func FixSeries(ctx context.Context, ws Workspace, in SeriesInput, ex *Executor) (SeriesPlan, error) {
	holder, err := ws.Project(in.Project)
	if err != nil {
		return SeriesPlan{}, err
	}
	engine, engineErr := ws.Bump()
	if engine == nil {
		return SeriesPlan{}, engineErr
	}
	plan := SeriesPlan{Holder: holder.Slug()}
	holdings, err := seriesHoldingsOf(holder, in.Holding)
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
	for _, h := range holdings {
		route, routeErr := engine.PullRoute(h)
		for _, line := range seriesLinesOf(holder, h) {
			if len(in.Lines) > 0 && !slices.ContainsFunc(in.Lines, func(l string) bool { return lines.Normalize(l) == lines.Normalize(line) }) {
				continue
			}
			plan.Targets = append(plan.Targets, seriesTargetOf(ctx, ws, in, holder, refs, h, route, routeErr, line))
		}
	}
	if ex == nil {
		return plan, nil
	}
	return plan, runSeries(ctx, ws, in, ex, holder, &plan)
}

// seriesHoldingsOf are the series holdings of a project, or the one named. A name that
// is none of them is an error.
func seriesHoldingsOf(holder *chain.Project, name string) ([]*chain.Holding, error) {
	var out []*chain.Holding
	for _, h := range holder.Held() {
		if h.Kind() == chain.HoldsSeries && !h.Virtual() && (name == "" || h.Name() == name) {
			out = append(out, h)
		}
	}
	if name != "" && len(out) == 0 {
		return nil, fmt.Errorf("%s has no series holding named %q", holder.Slug(), name)
	}
	return out, nil
}

// seriesTargetOf reads one line of a series holding and decides what it needs: nothing,
// a PR, or to be left alone and why.
func seriesTargetOf(ctx context.Context, ws Reader, in SeriesInput, holder *chain.Project, refs git.Refs, h *chain.Holding, route bump.Route, routeErr error, line string) SeriesTarget {
	t := SeriesTarget{SeriesLine: seriesLine(ctx, ws, holder, refs, h, line), Holding: h.Name(), holding: h, route: route}
	switch {
	case t.Error != "":
		t.Status, t.Reason = StatusFailed, t.Error
	case t.UpToDate:
		t.Status = StatusUpToDate
	case t.Acknowledged != "":
		t.Status, t.Reason = StatusBlocked, "the difference is acknowledged: "+t.Acknowledged
	case routeErr != nil:
		t.Status, t.Reason = StatusBlocked, routeErr.Error()
	default:
		preview, err := seriesPreview(in, holder, t)
		if err != nil {
			t.Status, t.Reason = StatusFailed, err.Error()
			break
		}
		t.Status, t.Preview = StatusReady, &preview
	}
	return t
}

// seriesPreview is the text a fix of the target would write: the branch, the PR's title
// and body, the commit message and the steps that run around the write.
func seriesPreview(in SeriesInput, holder *chain.Project, t SeriesTarget) (Preview, error) {
	h, prs := t.holding, holder.PullRequests()
	target := bump.Target{Branch: t.Branch, Line: t.Line, Holdings: []*chain.Holding{h}}
	pre, post := t.route.Steps(target)
	steps := append(slices.Clone(pre), post...)

	var changes []string
	changes = append(changes, "- "+t.Change)
	for _, ref := range steps {
		changes = append(changes, "- Ran `"+strings.Join(ref.Step.Run, " ")+"`")
	}
	series, _ := h.Stream().Series(t.Line)
	vars := map[string]string{
		"holding": h.Name(), "stream": h.Stream().Name(), "source": h.Producer().Name(), "sourceRepo": h.Producer().Slug(),
		"consumer": holder.Slug(), "line": t.Line, "branch": strings.TrimPrefix(t.Branch, "release/"), "targetBranch": t.Branch,
		"series": series, "from": t.Current, "to": t.Want, "file": t.File, "changes": strings.Join(changes, "\n"),
		"marker": t.marker(holder.Slug()).String(), "key": BranchKey(h.Name(), t.Want, t.Branch),
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
	title, err := templates.Render(texts.SeriesTitle(), vars)
	if err != nil {
		return Preview{}, fmt.Errorf("title: %w", err)
	}
	commit, err := templates.Render(texts.SeriesCommitMessage(), vars)
	if err != nil {
		return Preview{}, fmt.Errorf("commit message: %w", err)
	}
	body, err := templates.Render(texts.SeriesBody(), vars)
	if err != nil {
		return Preview{}, fmt.Errorf("body: %w", err)
	}
	var stepPreviews []StepPreview
	for _, ref := range steps {
		message, err := seriesStepMessage(vars)(bump.Unit{Kind: bump.UnitStep, Phase: ref.Phase, Edit: ref.Edit, Index: ref.Index, Run: ref.Step.Run})
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

// seriesStepMessage is the commit message of a step that changed files, from the same
// embedded text as a release's, with the value written in place of a tag.
func seriesStepMessage(vars map[string]string) func(bump.Unit) (string, error) {
	return func(u bump.Unit) (string, error) {
		v := map[string]string{"source": vars["source"], "tag": vars["to"], "command": strings.Join(u.Run, " "), "phase": string(u.Phase)}
		return templates.Render(texts.StepCommitMessage(), v)
	}
}

// seriesStrategyMessage is the commit message of a strategy that changed files.
func seriesStrategyMessage(vars map[string]string) func(bump.Unit) (string, error) {
	return func(u bump.Unit) (string, error) {
		v := map[string]string{"source": vars["source"], "tag": vars["to"], "strategy": u.Strategy, "edit": u.Edit, "changes": bullets(u.Changes)}
		msg, err := templates.Render(texts.StrategyCommitMessage(), v)
		if err != nil {
			return "", err
		}
		return blankLines.ReplaceAllString(msg, "\n\n"), nil
	}
}

// runSeries makes the commit of each ready target and, with a publisher, pushes it and
// opens its PR, after the one question when the run writes to GitHub.
func runSeries(ctx context.Context, ws Workspace, in SeriesInput, ex *Executor, holder *chain.Project, plan *SeriesPlan) error {
	var ready []int
	for i, t := range plan.Targets {
		if t.Status == StatusReady && t.Preview != nil {
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
		if pub, me, plan.Push, err = publisherFor(ctx, ws, ex, holder, clone, in.PushTo, in.CI, false); err != nil {
			return err
		}
	}
	var committer Identity
	if ex.Identify != nil {
		actor, err := actorFor(ctx, ex, clone, me)
		if err != nil {
			return err
		}
		plan.Actor = actor
		if in.CI {
			committer = actor.Identity
		}
	}
	states := make([]seriesState, len(ready))
	for k, i := range ready {
		states[k] = decideSeries(ctx, pub, me, holder.Slug(), plan.Targets[i])
	}
	if pub != nil && !in.DryRun && ex.Confirm != nil {
		if c, writes := seriesConfirmation(holder.Slug(), plan, ready, states); writes {
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
		plan.Targets[i].Result = workSeries(ctx, ex, pub, clone, remote, in, holder.Slug(), plan.Targets[i], states[k], committer)
	}
	return nil
}

// seriesState is what was found out about one target before any work: the decision
// about its PR, and what to warn about.
type seriesState struct {
	dec      Publish
	m        Marker
	warnings []string
	done     *TargetResult // set when there is nothing to make: the lookup failed, or a newer PR blocks it
}

// decideSeries looks at the open PRs of a target's branch, when there is a publisher,
// and decides what to do about them, as decideTarget does for a release. It only reads.
func decideSeries(ctx context.Context, pub *Publisher, me, holder string, t SeriesTarget) seriesState {
	st := seriesState{m: t.marker(holder)}
	if pub == nil {
		return st
	}
	open, err := pub.Remote.OpenForBase(ctx, holder, t.Branch, 0)
	if err != nil {
		st.done = &TargetResult{Status: ResultFailed, Branch: t.Preview.Branch, Error: fmt.Sprintf("look for the open PRs of %s: %v", holder, err)}
		return st
	}
	prs := open.PRs
	byHead, err := pub.Remote.OpenForHead(ctx, holder, pub.headOwner(holder)+":"+t.Preview.Branch)
	if err != nil {
		st.warnings = append(st.warnings, fmt.Sprintf("could not look for the PR of %s: %v", t.Preview.Branch, err))
	}
	for _, p := range byHead {
		if !slices.ContainsFunc(prs, func(q pulls.PR) bool { return q.Number == p.Number }) {
			prs = append(prs, p)
		}
	}
	st.dec = decidePublish(prs, me, st.m)
	if open.Truncated {
		st.warnings = append(st.warnings, fmt.Sprintf("only the newest %d open PRs of %s were looked at, so an older PR of this fix may have been missed", len(open.PRs), holder))
	}
	for _, a := range st.dec.Also {
		st.warnings = append(st.warnings, alsoNote(a, t.Branch, 0, false))
	}
	if st.dec.Action == PublishBlocked {
		newer, _ := ParseMarker(st.dec.Newer.Body)
		st.done = &TargetResult{Status: ResultBlocked, Branch: t.Preview.Branch, Warnings: st.warnings,
			Error: fmt.Sprintf("#%d already writes %s to %s, which is newer than %s", st.dec.Newer.Number, newer.Version, t.Branch, t.Want)}
	}
	return st
}

// seriesConfirmation is what the run is about to do, and whether that writes anything.
func seriesConfirmation(holder string, plan *SeriesPlan, ready []int, states []seriesState) (Confirmation, bool) {
	c := Confirmation{Consumer: holder}
	if plan.Push != nil {
		c.Push = *plan.Push
	}
	var wants []string
	for k, i := range ready {
		t, st := plan.Targets[i], states[k]
		if !slices.Contains(wants, t.Want) {
			wants = append(wants, t.Want)
		}
		switch {
		case st.done != nil && st.done.Status == ResultBlocked:
			c.Blocked = append(c.Blocked, t.Line)
		case st.dec.Action == PublishAlreadyOpen:
			c.Already = append(c.Already, t.Line)
		case st.done == nil:
			c.Opens = append(c.Opens, t.Line)
		}
		for _, p := range st.dec.Supersede {
			c.Closes = append(c.Closes, PRRef{Number: p.Number, URL: p.URL})
		}
	}
	c.Release = strings.Join(wants, ", ")
	return c, len(c.Opens) > 0 || len(c.Closes) > 0
}

// workSeries does the work for one target that was decided, as workTarget does for a
// release: a result of its own when there is nothing to make, otherwise the commits
// and, with a publisher, the push and the PR.
func workSeries(ctx context.Context, ex *Executor, pub *Publisher, clone, remote string, in SeriesInput, holder string, t SeriesTarget, st seriesState, committer Identity) *TargetResult {
	if st.done != nil {
		return st.done
	}
	if pub != nil && st.dec.Action == PublishAlreadyOpen {
		result := &TargetResult{Status: ResultAlreadyOpen, Branch: t.Preview.Branch, Warnings: st.warnings,
			PR: &PRRef{Number: st.dec.Open.Number, URL: st.dec.Open.URL}}
		if in.DryRun {
			result.WouldSupersede = refs(st.dec.Supersede)
		} else {
			pub.supersede(ctx, holder, st.dec.Open.Number, st.m, t.plan(), st.dec.Supersede, result, nil)
		}
		return result
	}
	result := commitSeries(ctx, ex, clone, remote, in, t, committer)
	result.Warnings = append(result.Warnings, st.warnings...)
	switch {
	case pub == nil:
	case in.DryRun:
		result.WouldSupersede = refs(st.dec.Supersede)
	case result.Status == ResultCommitted:
		pub.publish(ctx, holder, t.plan(), st.m, st.dec, result, nil)
	}
	return result
}

// commitSeries makes the commits for one target and ends the work, whatever happened,
// so the clone is as it was found apart from the branch it keeps.
func commitSeries(ctx context.Context, ex *Executor, clone, remote string, in SeriesInput, t SeriesTarget, committer Identity) *TargetResult {
	result := &TargetResult{Branch: t.Preview.Branch}
	fail := func(err error) *TargetResult {
		result.Status, result.Error = ResultFailed, err.Error()
		return result
	}
	w, err := Prepare(ctx, ex.Git, PrepareRequest{
		CloneDir: clone, Remote: remote, Target: t.Branch, Base: t.Head,
		Branch: t.Preview.Branch, DryRun: in.DryRun, TempDir: ex.TempDir,
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
	vars := map[string]string{"source": t.holding.Producer().Name(), "to": t.Want}
	applied, err := Apply{
		Route: t.route, Target: bump.Target{Branch: t.Branch, Line: t.Line, Holdings: []*chain.Holding{t.holding}},
		Message: t.Preview.CommitMessage, Runner: ex.Runner, Sign: ex.Sign, Name: committer.Name, Email: committer.Email,
		StepMessage: seriesStepMessage(vars), StrategyMessage: seriesStrategyMessage(vars),
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
