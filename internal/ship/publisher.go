package ship

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/github/pulls"
)

// Remote is the GitHub side of publishing, as ship needs it. It is defined here,
// where it is used, and the PR service (internal/github/pulls) is what satisfies
// it, with Me from the token's login.
type Remote interface {
	// Me is the login that is running: whose PRs are the actor's own.
	Me(ctx context.Context) (string, error)
	OpenForBase(ctx context.Context, slug, base string, limit int) (pulls.Open, error)
	Create(ctx context.Context, slug string, in pulls.NewPR) (pulls.PR, error)
	Comment(ctx context.Context, slug string, number int, body string) error
	Close(ctx context.Context, slug string, number int) error
	RepoLabels(ctx context.Context, slug string) ([]string, error)
	AddLabels(ctx context.Context, slug string, number int, labels []string) error
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
}

// publish pushes the branch of a target that was committed, opens its PR with
// the labels that exist, and then comments on and closes each superseded PR, in
// that order, so a failure leaves the older PRs as they were. res is updated: a
// failure to push or to open the PR fails the target, and one to label or to close
// an older PR is a warning. Each step is reported to sink as it is done.
//
// The events are STUBBED until a later commit: none are emitted yet.
func (p Publisher) publish(ctx context.Context, consumer string, tp TargetPlan, m Marker, dec Publish, res *TargetResult, sink Sink) {
	branch := tp.Preview.Branch
	if err := p.Git.Push(ctx, p.Clone, p.Push.Remote, branch); err != nil {
		res.Status, res.Error = ResultFailed, fmt.Sprintf("push %s to %s: %v", branch, p.Push.Remote, err)
		return
	}
	res.Pushed = p.Push.Remote
	head := branch
	if p.Push.Role == RoleFork {
		head = p.HeadOwner + ":" + branch
	}
	created, err := p.Remote.Create(ctx, consumer, pulls.NewPR{
		Title: tp.Preview.Title, Body: tp.Preview.Body, Head: head, Base: tp.Branch,
	})
	if err != nil {
		res.Status, res.Error = ResultFailed, fmt.Sprintf("the branch is pushed to %s, but opening the PR failed: %v", p.Push.Remote, err)
		return
	}
	res.Status, res.PR = ResultOpened, &PRRef{Number: created.Number, URL: created.URL}

	p.label(ctx, consumer, created.Number, tp.Preview.Labels, res)
	for _, older := range dec.Supersede {
		note := fmt.Sprintf("Superseded by #%d (%s %s).", created.Number, m.Source, m.Version)
		if err := p.Remote.Comment(ctx, consumer, older.Number, note); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("could not comment on #%d, so it was left open: %v", older.Number, err))
			continue
		}
		if err := p.Remote.Close(ctx, consumer, older.Number); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("could not close #%d: %v", older.Number, err))
			continue
		}
		res.Superseded = append(res.Superseded, PRRef{Number: older.Number, URL: older.URL})
	}
}

// label applies the configured labels, and the bot's when the bot is the actor,
// that the repository has, after the PR exists. A label it lacks is a warning, and
// so is any failure: a label never fails the PR. The repository's labels are read
// only when there are labels to apply.
func (p Publisher) label(ctx context.Context, consumer string, number int, configured []string, res *TargetResult) {
	wanted := slices.Clone(configured)
	if p.IsBot {
		wanted = append(wanted, p.BotLabels...)
	}
	if len(wanted) == 0 {
		return
	}
	have, err := p.Remote.RepoLabels(ctx, consumer)
	if err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("could not read the labels of %s, so none were applied: %v", consumer, err))
		return
	}
	spelling := make(map[string]string, len(have))
	for _, l := range have {
		spelling[strings.ToLower(l)] = l
	}
	var apply []string
	for _, l := range wanted {
		actual, ok := spelling[strings.ToLower(l)]
		if !ok {
			res.Warnings = append(res.Warnings, fmt.Sprintf("label %s is not in %s, so it was not applied", l, consumer))
			continue
		}
		if !slices.Contains(apply, actual) {
			apply = append(apply, actual)
		}
	}
	if err := p.Remote.AddLabels(ctx, consumer, number, apply); err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("could not apply the labels: %v", err))
	}
}
