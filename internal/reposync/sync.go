package reposync

import (
	"context"
	"crypto/sha1" //nolint:gosec // a fingerprint of what a sync writes, not a secret
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/texts"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/templates"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/wanted"
)

// Input is what the user asked to sync: the held values of one project.
type Input struct {
	Project  string   // the holder, by config name or slug; empty for the one the working directory is in
	Holdings []string // limit the run to these holdings, by name
	Lines    []string // limit the run to these Rancher lines
	// BranchPrefix is the user's branchPrefix setting, which replaces the project's.
	BranchPrefix string
	// DefaultBranchPrefix is used when neither BranchPrefix nor the project says one.
	DefaultBranchPrefix string
	DryRun              bool   // compute and report, and create no branch or PR
	NoPush              bool   // stop after the commit, leaving the branch in the clone
	PushTo              string // fork or upstream, or empty to let the settings decide
	CI                  bool   // the actor is the bot
}

// Plan is what a sync of a project would do, and what a run did: a branch of the
// holder for each of its branches that holds something to correct, and in it every
// correction, whatever holds it.
type Plan struct {
	Holder   string               `json:"holder"` // the repository of the project, owner/name
	Clone    string               `json:"clone,omitempty"`
	Remote   string               `json:"remote,omitempty"`
	Push     *proposal.PushTarget `json:"push,omitempty"`
	Actor    *proposal.Actor      `json:"actor,omitempty"`
	Branches []Branch             `json:"branches,omitempty"`
}

// Branch is one branch of the holder: what each holding in it holds against what
// it should, and, when something is to be corrected, the one PR that corrects it.
// A PR has one base, so a correction on another branch is another Branch.
type Branch struct {
	Branch string `json:"branch"`
	Head   string `json:"head,omitempty"` // the commit at the head of the branch on the host
	Status Status `json:"status"`
	Rows   []Row  `json:"rows"`
	// proposal.Preview is what a run would write, when a row is ready. Result is what a run did.
	Preview *proposal.Preview      `json:"preview,omitempty"`
	Result  *proposal.TargetResult `json:"result,omitempty"`

	holder *chain.Project
	route  bump.Route
	ready  []*chain.Holding
	// pulled is the release each release holding among ready is written with.
	pulled map[*chain.Holding]bump.Release
	// fingerprint says what the PR writes, so a later run knows its own PR as the
	// same fix, and an older one as stale.
	fingerprint string
}

// Row is one holding on one line of a branch.
type Row struct {
	SeriesLine
	Holding string `json:"holding"`
	Kind    string `json:"kind"`
	Status  Status `json:"status"`
	// Reason says why a row that differs is not ready: no Edit follows the holding, the
	// difference is acknowledged, or the lines of the branch want different values of
	// one place.
	Reason string `json:"reason,omitempty"`

	holding *chain.Holding
	// release is the release a release holding is to be written with.
	release *bump.Release
	// requiredBy is, for the row of the virtual holding a pin makes, the pinning release whose
	// go.mod requires it.
	requiredBy string
}

// marker is what identifies the PR of the branch: the holder, the branch and the
// fingerprint of what it writes.
func (b Branch) marker(holder string) proposal.Marker {
	return proposal.Marker{Source: "sync", Consumer: holder, Target: b.Branch, Version: b.fingerprint}
}

// target is the branch as the engine writes it: the holdings that are ready and the releases the
// release holdings among them are written with.
func (b Branch) target() bump.Target {
	return bump.Target{Branch: b.Branch, Line: b.line(), Holdings: b.ready, Pulled: b.pulled}
}

// line is the Rancher line of the branch: that of its first row.
func (b Branch) line() string {
	if len(b.Rows) > 0 {
		return b.Rows[0].Line
	}
	return ""
}

// subject is the branch as the publisher knows it.
func (b Branch) subject() proposal.Subject {
	return proposal.Subject{Branch: b.Branch, Commit: b.Head, Line: b.line(), Preview: b.Preview}
}

// Run brings what a project holds in line with config, branch by branch: for each
// branch of the holder, every holding whose value differs from what it should be and
// that an Edit follows is written in one commit, followed by the steps of the Edits, and
// the branch is pushed and opened as one PR. A holding that no Edit follows, or whose
// difference the document acknowledges, is only reported, since nothing writes a holding
// until an Edit says what a write of it takes. A series holding is the one kind brought
// in line now; a kind that is pulled later is another row source in rowsOf.
//
// With ex nil it only checks. A branch that fails is reported and the others go on.
func Run(ctx context.Context, ws Workspace, in Input, ex *proposal.Executor) (Plan, error) {
	holder, err := ws.Project(in.Project)
	if err != nil {
		return Plan{}, err
	}
	engine, engineErr := ws.Bump()
	if engine == nil {
		return Plan{}, engineErr
	}
	plan := Plan{Holder: holder.Slug()}
	holdings, err := pulledHoldingsOf(holder, in.Holdings)
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
	var rows []Row
	tags := wanted.New(ws)
	for _, h := range holdings {
		rows = append(rows, rowsOf(ctx, ws, in, holder, refs, tags, h)...)
	}
	plan.Branches = syncBranches(holder, engine, in, rows)
	if ex == nil {
		return plan, nil
	}
	return plan, runSync(ctx, ws, in, ex, holder, refs, &plan)
}

// pulledHoldingsOf are the holdings of a project that are brought in line, or the ones named.
// A name that is none of them is an error.
func pulledHoldingsOf(holder *chain.Project, names []string) ([]*chain.Holding, error) {
	var out []*chain.Holding
	for _, h := range holder.Held() {
		if !h.Virtual() && (h.Kind() == chain.HoldsSeries || h.Kind() == chain.HoldsRelease) && (len(names) == 0 || slices.Contains(names, h.Name())) {
			out = append(out, h)
		}
	}
	for _, name := range names {
		if !slices.ContainsFunc(out, func(h *chain.Holding) bool { return h.Name() == name }) {
			return nil, fmt.Errorf("%s has no holding named %q that is brought in line", holder.Slug(), name)
		}
	}
	return out, nil
}

// rowsOf reads a holding on each of its lines: what it holds against what config and the host
// say it should. This is where a kind of holding that is pulled says what its value should be:
// a series is the series its stream has for the line, and a release is the newest stable one of
// its stream for the line.
func rowsOf(ctx context.Context, ws Reader, in Input, holder *chain.Project, refs git.Refs, tags *wanted.Resolver, h *chain.Holding) []Row {
	var rows []Row
	for _, line := range seriesLinesOf(holder, h) {
		if len(in.Lines) > 0 && !slices.ContainsFunc(in.Lines, func(l string) bool { return lines.Normalize(l) == lines.Normalize(line) }) {
			continue
		}
		got := heldLine(ctx, ws, holder, refs, tags, h, line)
		row := rowFor(h, got)
		companions := companionRows(ctx, ws, holder, refs, tags, h, line, got, row)
		for _, c := range companions {
			if c.Status == StatusFailed && row.Status == StatusReady {
				// the write of wrangler alone would put go.mod out of step with what it requires
				row.Status, row.Reason = StatusBlocked, fmt.Sprintf("its companion %s cannot be derived: %s", c.Holding, c.Reason)
			}
		}
		rows = append(rows, row)
		rows = append(rows, companions...)
	}
	return rows
}

// rowFor is the row of a holding on a line, from what was read of it.
func rowFor(h *chain.Holding, got held) Row {
	row := Row{SeriesLine: got.SeriesLine, Holding: h.Name(), Kind: h.Kind(), holding: h, release: got.release}
	switch {
	case got.Error != "":
		row.Status, row.Reason = StatusFailed, got.Error
	case got.UpToDate:
		row.Status = StatusUpToDate
	case got.Acknowledged != "":
		row.Status, row.Reason = StatusBlocked, "the difference is acknowledged: "+got.Acknowledged
	case len(bump.PullEdits(h)) == 0:
		row.Status, row.Reason = StatusBlocked, fmt.Sprintf("%v: Holding %q is only read until one does", bump.ErrNoEdit, h.Name())
	default:
		row.Status = StatusReady
	}
	return row
}

// companionRows reads what a holding that pins another Artifact holds of it on a line: the
// release of the pinned module that the go.mod of the release the holding is brought to
// requires, as ship derives it. It is a floor, so a newer one is current. A holding whose
// own value could not be chosen has no companions, and the files that do not require the
// module have none either.
func companionRows(ctx context.Context, ws Reader, holder *chain.Project, refs git.Refs, tags *wanted.Resolver, h *chain.Holding, line string, got held, row Row) []Row {
	if got.release == nil || h.Kind() != chain.HoldsRelease {
		return nil
	}
	var rows []Row
	for _, v := range h.Pinned() {
		if !v.AppliesTo(line) {
			continue
		}
		w := tags.ForPinnerTag(ctx, v, got.release.Tag)
		var read held
		switch {
		case w.Err != nil:
			read = held{SeriesLine: SeriesLine{Line: line, Branch: row.Branch, Head: row.Head, Error: w.Err.Error()}}
		default:
			read = companionLine(ctx, ws, holder, refs, v, line, w.Release)
		}
		if read.Absent {
			continue
		}
		cr := rowFor(v, read)
		cr.requiredBy = w.RequiredBy
		rows = append(rows, cr)
	}
	return rows
}

// syncBranches groups the rows by the branch they are in, in the order they come, and
// works out what each branch needs: its status, the route that writes it, and the
// fingerprint of what it writes. Rows of one holding that want different values of one
// place on one branch (a holder with a single branch for lines that differ) cannot both
// be written, so they are left alone.
func syncBranches(holder *chain.Project, engine *bump.Engine, in Input, rows []Row) []Branch {
	var out []Branch
	for _, r := range rows {
		i := slices.IndexFunc(out, func(b Branch) bool { return b.Branch == r.Branch })
		if i < 0 {
			out = append(out, Branch{Branch: r.Branch, Head: r.Head, holder: holder})
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
				if r.release != nil {
					if b.pulled == nil {
						b.pulled = map[*chain.Holding]bump.Release{}
					}
					b.pulled[r.holding] = *r.release
				}
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
func leaveConflicts(b *Branch) {
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
func statusOfBranch(b *Branch) Status {
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
func syncPreview(in Input, holder *chain.Project, b Branch) (proposal.Preview, error) {
	prs := holder.PullRequests()
	pre, post := b.route.Steps(b.target())
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
		changes = append(changes, "- "+r.Change+" ("+sourceOf(r)+")")
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
		"key": proposal.BranchKey("sync", b.fingerprint, b.Branch),
	}
	prefix := proposal.FirstNonEmpty(in.BranchPrefix, prs.BranchPrefix, in.DefaultBranchPrefix, texts.DefaultBranchPrefix())
	prefixVars := map[string]string{}
	for k, v := range vars {
		if k != "key" {
			prefixVars[k] = v
		}
	}
	renderedPrefix, err := templates.Render(prefix, prefixVars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("branch prefix %q: %w", prefix, err)
	}
	title, err := templates.Render(texts.SyncTitle(), vars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("title: %w", err)
	}
	commit, err := templates.Render(texts.SyncCommitMessage(), vars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("commit message: %w", err)
	}
	body, err := templates.Render(texts.SyncBody(), vars)
	if err != nil {
		return proposal.Preview{}, fmt.Errorf("body: %w", err)
	}
	var stepPreviews []proposal.StepPreview
	for _, ref := range steps {
		message, err := syncStepMessage(vars)(bump.Unit{Kind: bump.UnitStep, Phase: ref.Phase, Edit: ref.Edit, Index: ref.Index, Run: ref.Step.Run})
		if err != nil {
			return proposal.Preview{}, err
		}
		stepPreviews = append(stepPreviews, proposal.StepPreview{
			Phase: string(ref.Phase), Edit: ref.Edit, Run: strings.Join(ref.Step.Run, " "),
			BeforeEdit: ref.Phase == bump.PreEdit, CommitMessage: message,
		})
	}
	return proposal.Preview{
		Branch: renderedPrefix + vars["key"], Key: vars["key"], Title: title, Labels: prs.Labels,
		CommitMessage: commit, Body: body, Steps: stepPreviews,
	}, nil
}

// sourceOf says where the value of a row comes from, for the PR to read: the series its stream
// has for the line, or the release it is written with.
func sourceOf(r Row) string {
	if r.requiredBy != "" {
		return fmt.Sprintf("%s: `%s`, which `%s` requires", r.Holding, r.release.Tag, r.requiredBy)
	}
	stream := streamOf(r.holding)
	if r.release != nil {
		return fmt.Sprintf("%s: `%s` of `%s`", r.Holding, r.release.Tag, stream.Name())
	}
	series, _ := stream.Series(r.Line)
	return fmt.Sprintf("%s: the series `%s` of `%s` for %s", r.Holding, series, stream.Name(), r.Line)
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
		v := map[string]string{"holdings": vars["holdings"], "line": vars["line"], "strategy": u.Strategy, "edit": u.Edit, "changes": proposal.Bullets(u.Changes)}
		msg, err := templates.Render(texts.SyncStrategyCommitMessage(), v)
		if err != nil {
			return "", err
		}
		return proposal.CollapseBlankLines(msg), nil
	}
}

// runSync makes the commit of each ready branch and, with a publisher, pushes it and
// opens its PR, after the one question when the run writes to GitHub. The conveyor of
// internal/proposal does what a release's run does too: begin, decide, ask, enact.
func runSync(ctx context.Context, ws Workspace, in Input, ex *proposal.Executor, holder *chain.Project, refs git.Refs, plan *Plan) error {
	var ready []int
	for i, b := range plan.Branches {
		if b.Status == StatusReady && b.Preview != nil {
			ready = append(ready, i)
		}
	}
	if len(ready) == 0 {
		return nil
	}
	sess, err := proposal.Begin(ctx, ws, ex, proposal.SessionInput{Repo: holder, PushTo: in.PushTo, CI: in.CI, NoPush: in.NoPush})
	if err != nil {
		return err
	}
	plan.Clone, plan.Remote, plan.Push, plan.Actor = sess.Clone, sess.Remote, sess.Push, sess.Actor
	decided := make([]proposal.Decided, len(ready))
	for k, i := range ready {
		b := plan.Branches[i]
		decided[k] = proposal.Decide(ctx, sess.Pub, sess.Me, proposal.DecideInput{
			Repo: holder.Slug(), Subject: b.subject(), Marker: b.marker(holder.Slug()), Compare: sameFingerprint,
			Branches: refs.Branches, Noun: "sync",
		}, nil)
	}
	c, writes := syncConfirmation(holder.Slug(), plan, ready, decided)
	if err := sess.Confirm(ctx, ex, in.DryRun, c, writes); err != nil {
		return err
	}
	for k, i := range ready {
		b := plan.Branches[i]
		plan.Branches[i].Result = proposal.Enact(ctx, ex, proposal.EnactInput{
			Repo: holder.Slug(), Session: sess, Subject: b.subject(), Marker: b.marker(holder.Slug()), Decided: decided[k],
			DryRun: in.DryRun, CI: in.CI,
			Apply: func(w *proposal.Work, onCommit func(proposal.CommitInfo)) (proposal.ApplyResult, error) {
				vars := map[string]string{"holdings": holdingsOf(b), "line": b.line()}
				return proposal.Apply{
					Route: b.route, Target: b.target(),
					Message: b.Preview.CommitMessage, Runner: ex.Runner, Sign: ex.Sign, Name: sess.Committer.Name, Email: sess.Committer.Email,
					OnCommit: onCommit, StepMessage: syncStepMessage(vars), StrategyMessage: syncStrategyMessage(vars),
				}.Run(ctx, w)
			},
		}, nil)
	}
	return nil
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

// syncConfirmation is what the run is about to do, and whether that writes anything.
func syncConfirmation(holder string, plan *Plan, ready []int, decided []proposal.Decided) (proposal.Confirmation, bool) {
	c := proposal.Confirmation{Consumer: holder, Release: "a sync"}
	if plan.Push != nil {
		c.Push = *plan.Push
	}
	for k, i := range ready {
		b, st := plan.Branches[i], decided[k]
		label := b.line()
		if label == "" {
			label = b.Branch
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

// holdingsOf is the names of the holdings a branch writes, joined.
func holdingsOf(b Branch) string {
	var names []string
	for _, h := range b.ready {
		names = append(names, h.Name())
	}
	return strings.Join(names, ", ")
}
