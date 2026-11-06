package ship

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/github/pulls"
	"github.com/rancher/orbs-automation/internal/task/createpr"
	"github.com/rancher/orbs-automation/internal/task/pushbranch"
)

// Remote is the GitHub side of publishing, as ship needs it. It is defined here,
// where it is used, and the PR service (internal/github/pulls) is what satisfies
// it, with Me from the token's login.
type Remote interface {
	// Me is the login that is running: whose PRs are the actor's own.
	Me(ctx context.Context) (string, error)
	OpenForBase(ctx context.Context, slug, base string, limit int) (pulls.Open, error)
	// OpenForHead is the open PRs from one head, owner:branch, with one request.
	OpenForHead(ctx context.Context, slug, head string) ([]pulls.PR, error)
	Create(ctx context.Context, slug string, in pulls.NewPR) (pulls.PR, error)
	Comment(ctx context.Context, slug string, number int, body string) error
	Close(ctx context.Context, slug string, number int) error
	RepoLabels(ctx context.Context, slug string) ([]string, error)
	AddLabels(ctx context.Context, slug string, number int, labels []string) error
	// BehindBy is how many commits the base has that the head does not.
	BehindBy(ctx context.Context, slug, base, head string) (int, error)
	// UpdateBody replaces a PR's title and body.
	UpdateBody(ctx context.Context, slug string, number int, title, body string) error
	// Comments lists a PR's comments, oldest first, up to limit.
	Comments(ctx context.Context, slug string, number, limit int) ([]pulls.Comment, error)
	// EditComment replaces the body of a comment.
	EditComment(ctx context.Context, slug string, id int64, body string) error
}

// Publisher pushes a committed branch, opens its PR and closes the PRs it
// supersedes.
type Publisher struct {
	Git    Git
	Remote Remote
	Clone  string     // the clone the branch is in
	Push   PushTarget // where the branch is pushed
	// HeadOwner is the owner of the fork, for the PR's head when the branch is
	// pushed there: the head is HeadOwner:branch. It is not used for the upstream.
	HeadOwner string
	// IsBot says the actor is the bot, which adds BotLabels to the PR's labels.
	IsBot     bool
	BotLabels []string
	// Refresh lets a branch of ours that is already on the remote, with commits
	// that are not the ones just built, be replaced.
	Refresh bool
}

// publish pushes the branch of a target that was committed, opens its PR with
// the labels that exist, and then comments on and closes each superseded PR, in
// that order, so a failure leaves the older PRs as they were. res is updated: a
// failure to push or to open the PR fails the target, and one to label or to close
// an older PR is a warning. Each step is reported to sink as it is done.
func (p Publisher) publish(ctx context.Context, consumer string, tp TargetPlan, m Marker, dec Publish, res *TargetResult, sink Sink) {
	branch := tp.Preview.Branch
	if err := p.pushBranch(ctx, tp); err != nil {
		res.Status, res.Error = ResultFailed, err.Error()
		return
	}
	res.Pushed = p.Push.Remote
	emit(sink, Event{Kind: EventPushed, Branch: tp.Branch, Remote: p.Push.Remote})
	head := branch
	if p.Push.Role == RoleFork {
		head = p.HeadOwner + ":" + branch
	}
	pr, err := createpr.Run(ctx, p.Remote, createpr.Input{
		Repo: consumer, Title: tp.Preview.Title, Body: tp.Preview.Body, Head: head, Base: tp.Branch,
		Labels: p.labels(tp.Preview.Labels),
	})
	if err != nil {
		res.Status, res.Error = ResultFailed, fmt.Sprintf("the branch is pushed to %s, but opening the PR failed: %v", p.Push.Remote, err)
		return
	}
	res.Status, res.PR = ResultOpened, &PRRef{Number: pr.Number, URL: pr.URL}
	res.Warnings = append(res.Warnings, pr.Warnings...)
	emit(sink, Event{Kind: EventPROpened, Branch: tp.Branch, PR: res.PR})

	p.supersede(ctx, consumer, pr.Number, m, tp.Branch, dec.Supersede, res, sink)
}

// pushBranch puts the branch on the remote with the pushbranch task, and says
// how to go on when the remote has an earlier attempt's commits that were not
// replaced.
func (p Publisher) pushBranch(ctx context.Context, tp TargetPlan) error {
	_, err := pushbranch.Run(ctx, p.Git, pushbranch.Input{
		Dir: p.Clone, Remote: p.Push.Remote, Branch: tp.Preview.Branch, Base: tp.Commit, Replace: p.Refresh,
	})
	if errors.Is(err, pushbranch.ErrNeedsReplace) {
		return fmt.Errorf("%w: run with --refresh to replace them", err)
	}
	return err
}

// supersede tells each older PR that the one numbered by is its replacement, and
// closes it, in that order: a PR that could not be told is left open. A failure
// is a warning, and only what was closed is recorded.
func (p Publisher) supersede(ctx context.Context, consumer string, by int, m Marker, branch string, older []pulls.PR, res *TargetResult, sink Sink) {
	for _, old := range older {
		note := fmt.Sprintf("Superseded by #%d (%s %s).", by, m.Source, m.Version)
		if err := p.Remote.Comment(ctx, consumer, old.Number, note); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("could not comment on #%d, so it was left open: %v", old.Number, err))
			continue
		}
		if err := p.Remote.Close(ctx, consumer, old.Number); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("could not close #%d: %v", old.Number, err))
			continue
		}
		ref := PRRef{Number: old.Number, URL: old.URL}
		res.Superseded = append(res.Superseded, ref)
		emit(sink, Event{Kind: EventSuperseded, Branch: branch, PR: &ref})
	}
}

// headOwner is the owner of the repository a branch pushed by this publisher is
// in: the fork's, or the repository's own for the upstream.
func (p Publisher) headOwner(consumer string) string {
	if p.Push.Role == RoleFork {
		return p.HeadOwner
	}
	owner, _, _ := strings.Cut(consumer, "/")
	return owner
}

// refreshMarker identifies the one comment gero keeps on a PR to record that it
// rebuilt the branch.
const refreshMarker = "<!-- gero:refresh v1 -->"

// recordRefresh refreshes the PR's title and body from the preview, and records
// the refresh in the comment gero keeps for the purpose: one that is edited in
// place, found by its marker and by being the actor's own, with a line added for
// each refresh, and created the first time. A failure is a warning.
func (p Publisher) recordRefresh(ctx context.Context, consumer string, tp TargetPlan, base string, now time.Time, res *TargetResult) {
	number := res.PR.Number
	if err := p.Remote.UpdateBody(ctx, consumer, number, tp.Preview.Title, tp.Preview.Body); err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("could not refresh the title and body of #%d: %v", number, err))
	}
	line := fmt.Sprintf("- Rebuilt on %s@%s (%s)", tp.Branch, short(base), now.UTC().Format("2006-01-02 15:04 UTC"))
	me, err := p.Remote.Me(ctx)
	var comments []pulls.Comment
	if err == nil {
		comments, err = p.Remote.Comments(ctx, consumer, number, 0)
	}
	if err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("could not record the refresh on #%d: %v", number, err))
		return
	}
	for _, c := range comments {
		if c.Author == me && strings.Contains(c.Body, refreshMarker) {
			body := strings.TrimRight(strings.Replace(c.Body, refreshMarker, "", 1), " \n") + "\n" + line + "\n\n" + refreshMarker
			if err := p.Remote.EditComment(ctx, consumer, c.ID, body); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("could not record the refresh on #%d: %v", number, err))
			}
			return
		}
	}
	body := "Rebuilt by gero ship:\n\n" + line + "\n\n" + refreshMarker
	if err := p.Remote.Comment(ctx, consumer, number, body); err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("could not record the refresh on #%d: %v", number, err))
	}
}

// labels is the configured labels, and the bot's when the bot is the actor.
func (p Publisher) labels(configured []string) []string {
	wanted := slices.Clone(configured)
	if p.IsBot {
		wanted = append(wanted, p.BotLabels...)
	}
	return wanted
}
